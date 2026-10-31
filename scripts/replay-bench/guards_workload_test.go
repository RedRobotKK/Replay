package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
	"github.com/RedRobotKK/Replay/internal/proxy"
)

// readStep is one Read a scripted body returns: after waiting, data, then err.
type readStep struct {
	wait time.Duration
	data string
	err  error
}

// scriptedBody is a response body that reads exactly as scripted, so do's
// timing and error handling can be checked without a network or a race.
type scriptedBody struct{ steps []readStep }

func (b *scriptedBody) Read(p []byte) (int, error) {
	if len(b.steps) == 0 {
		return 0, io.EOF
	}
	s := b.steps[0]
	b.steps = b.steps[1:]
	time.Sleep(s.wait)
	return copy(p, s.data), s.err
}
func (*scriptedBody) Close() error { return nil }

// roundTrip answers every request from a function, in process.
type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// scriptedClient answers every request with status and the scripted body.
func scriptedClient(status int, steps ...readStep) *http.Client {
	return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: &scriptedBody{steps: steps}, Request: r}, nil
	})}
}

// gap is a pause long enough that a timing taken on the wrong side of it
// cannot be mistaken for one taken on the right side.
const gap = 30 * time.Millisecond

func TestDoTimesFirstByteAtTheFirstReadNotTheLast(t *testing.T) {
	const firstByte = 5 * time.Millisecond
	cl := scriptedClient(http.StatusOK, readStep{wait: firstByte, data: "first"}, readStep{wait: gap, data: "second"})
	got, err := do(cl, "http://bench.invalid", nil, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.ttfb < firstByte {
		t.Errorf("ttfb = %v, want at least the %v the first byte took", got.ttfb, firstByte)
	}
	// The second chunk arrives at least gap after the first, so the first
	// byte is at least gap before the end.
	if got.total-got.ttfb < gap {
		t.Errorf("ttfb %v is within %v of total %v: it was taken at a later read", got.ttfb, gap, got.total)
	}
}

func TestDoTimesTheStreamTailFromMessageStop(t *testing.T) {
	cl := scriptedClient(http.StatusOK,
		readStep{data: "data: " + sseFinalMarker + "\n\n"},
		readStep{wait: gap, err: io.EOF})
	got, err := do(cl, "http://bench.invalid", nil, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.tail < gap {
		t.Errorf("tail = %v, want at least the %v between message_stop and EOF", got.tail, gap)
	}
}

func TestDoRefusesAStreamWithoutMessageStop(t *testing.T) {
	cl := scriptedClient(http.StatusOK, readStep{data: "data: {\"type\":\"ping\"}\n\n"})
	if _, err := do(cl, "http://bench.invalid", nil, 1, ""); err == nil || !strings.Contains(err.Error(), "message_stop") {
		t.Errorf("err = %v, want a missing message_stop", err)
	}
}

func TestDoReportsAReadThatFailsMidBody(t *testing.T) {
	cl := scriptedClient(http.StatusOK, readStep{data: "part"}, readStep{err: io.ErrUnexpectedEOF})
	if _, err := do(cl, "http://bench.invalid", nil, 0, ""); err == nil {
		t.Error("a body that failed mid-read was timed as complete")
	}
}

func TestDoReportsANonOKStatusWithItsBody(t *testing.T) {
	cl := scriptedClient(http.StatusBadGateway, readStep{data: "upstream gone\n"})
	_, err := do(cl, "http://bench.invalid", nil, 0, "")
	if err == nil || !strings.Contains(err.Error(), "status 502: upstream gone") {
		t.Errorf("err = %v, want the status and the body", err)
	}
}

func TestDoReportsATransportFailureAndABadURL(t *testing.T) {
	failing := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) { return nil, errInjected })}
	if _, err := do(failing, "http://bench.invalid", nil, 0, ""); err == nil {
		t.Error("a transport failure was timed as a request")
	}
	if _, err := do(http.DefaultClient, "::not a url", nil, 0, ""); err == nil {
		t.Error("an unparseable base URL was accepted")
	}
}

func TestDoSendsTheSessionAndStreamHeadersOnlyWhenAsked(t *testing.T) {
	var seen []http.Header
	cl := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		seen = append(seen, r.Header.Clone())
		body := &scriptedBody{steps: []readStep{{data: "data: " + sseFinalMarker + "\n\n"}}}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body, Request: r}, nil
	})}
	if _, err := do(cl, "http://bench.invalid", nil, 3, "sess-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := do(cl, "http://bench.invalid", nil, 0, ""); err != nil {
		t.Fatal(err)
	}
	if got := seen[0].Get(proxy.HeaderSessionID); got != "sess-1" {
		t.Errorf("session header = %q, want sess-1", got)
	}
	if got := seen[0].Get(headerSSEEvents); got != "3" {
		t.Errorf("stream header = %q, want 3", got)
	}
	if seen[1].Get(proxy.HeaderSessionID) != "" || seen[1].Get(headerSSEEvents) != "" {
		t.Errorf("headers sent unasked: %v", seen[1])
	}
}

func TestStreamingRequestBodyAsksForAStream(t *testing.T) {
	if !bytes.Contains(requestBody(size45KB, true), []byte(`"stream":true,`)) {
		t.Error(`a streaming request body lacks "stream":true`)
	}
	if bytes.Contains(requestBody(size45KB, false), []byte(`"stream"`)) {
		t.Error("a JSON request body asks for a stream")
	}
}

// erroringBody fails its first read, as a client that hangs up mid-upload does.
type erroringBody struct{}

func (erroringBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestUpstreamAnswersJSONOrAFlushedStreamAndRefusesABrokenBody(t *testing.T) {
	h := upstreamHandler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, messagesPath, strings.NewReader("{}")))
	if ct := rec.Header().Get("content-type"); ct != "application/json" || rec.Header().Get("content-length") == "" {
		t.Errorf("no stream header: content-type %q, content-length %q; want JSON with a length", ct, rec.Header().Get("content-length"))
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, messagesPath, strings.NewReader("{}"))
	req.Header.Set(headerSSEEvents, "2")
	h.ServeHTTP(rec, req)
	if ct := rec.Header().Get("content-type"); ct != "text/event-stream" {
		t.Errorf("stream content-type = %q", ct)
	}
	// Flushed event by event, or the client's first byte would be the
	// upstream's last and TTFB would measure nothing.
	if !rec.Flushed {
		t.Error("the stream was never flushed")
	}
	if !strings.Contains(rec.Body.String(), sseFinalMarker) {
		t.Error("the stream does not end with message_stop")
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, messagesPath, erroringBody{}))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a broken request body got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// isMkdirTemp reports whether err is os.MkdirTemp's own failure, which names
// the temporary directory it could not use, rather than a later one from
// carrying on with an empty directory name in the working directory.
func isMkdirTemp(err error) bool {
	var pe *fs.PathError
	return errors.As(err, &pe) && strings.HasPrefix(pe.Path, os.TempDir())
}

// missingTempDir points os.MkdirTemp at a directory that does not exist.
func missingTempDir(t *testing.T) {
	t.Helper()
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
}

func TestStartProxyReportsEachConstructorFailure(t *testing.T) {
	up := newUpstream()
	defer up.Close()

	// The refusal is the URL parser's, not a later one from building a proxy
	// with no upstream.
	var ue *url.Error
	if px, err := startProxy("://no-scheme", false); !errors.As(err, &ue) {
		if px != nil {
			_ = px.Close() // the assertion has already failed; this only tidies up
		}
		t.Errorf("an unparseable upstream: err = %v, want the URL parse error", err)
	}
	t.Run("tempdir", func(t *testing.T) {
		missingTempDir(t)
		if _, err := startProxy(up.URL, false); !isMkdirTemp(err) {
			t.Errorf("err = %v, want the failed MkdirTemp itself", err)
		}
	})
	for _, c := range []struct {
		name  string
		mask  bool
		setup func(t *testing.T)
	}{
		{"ledger", false, func(t *testing.T) {
			swap(t, &openLedger, func(string) (*ledger.Store, error) { return nil, errInjected })
		}},
		{"vault", true, func(t *testing.T) {
			swap(t, &openVault, func(string) (*masking.Vault, error) { return nil, errInjected })
		}},
		{"server", false, func(t *testing.T) {
			swap(t, &newProxyServer, func(proxy.Config) (*proxy.Server, error) { return nil, errInjected })
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			c.setup(t)
			// The failure reported is the constructor's own, not a later
			// one caused by carrying on without what it should have built.
			if _, err := startProxy(up.URL, c.mask); !errors.Is(err, errInjected) {
				t.Fatalf("a failed %s: err = %v, want the injected failure", c.name, err)
			}
			// A proxy that did not start leaves no directory behind.
			if left, err := os.ReadDir(tmp); err != nil || len(left) != 0 {
				t.Errorf("a failed %s left %v behind (%v)", c.name, left, err)
			}
		})
	}
}

func TestMaskOnProxyOpensAVaultAndMaskOffDoesNot(t *testing.T) {
	up := newUpstream()
	defer up.Close()
	opened := 0
	orig := openVault
	swap(t, &openVault, func(dir string) (*masking.Vault, error) { opened++; return orig(dir) })
	for _, mask := range []bool{false, true} {
		px, err := startProxy(up.URL, mask)
		if err != nil {
			t.Fatal(err)
		}
		if err := px.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if opened != 1 {
		t.Errorf("vault opened %d times for one mask-on and one mask-off proxy, want 1", opened)
	}
}
