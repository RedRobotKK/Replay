package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RedRobotKK/Buffy/internal/ledger"
)

const secret = "sk-ant-test-secret-value"

const requestBody = `{"model":"claude-opus-5","max_tokens":50,"system":"be brief","messages":[{"role":"user","content":"hi"}]}`

const messageResponse = `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":4,"cache_creation_input_tokens":30,"cache_read_input_tokens":300,"output_tokens":2}}`

// upstream is the fake provider. Fields written by the handler are read by
// the test after the response completes, so they are guarded by a mutex.
type upstream struct {
	mu       sync.Mutex
	t        *testing.T
	gotAuth  string
	gotBeta  string
	gotBody  []byte
	gotXFF   string
	gotHost  string
	release  chan struct{}
	mode     string
	requests int
}

func (u *upstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		u.t.Errorf("upstream read: %v", err)
	}
	u.mu.Lock()
	u.requests++
	u.gotAuth = r.Header.Get("x-api-key")
	u.gotBeta = r.Header.Get("anthropic-beta")
	u.gotXFF = r.Header.Get("X-Forwarded-For")
	u.gotHost = r.Host
	u.gotBody = body
	u.mu.Unlock()
	w.Header().Set("request-id", "req_upstream_1")
	switch u.mode {
	case "stream":
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":4,\"cache_creation_input_tokens\":30,\"cache_read_input_tokens\":300,\"output_tokens\":1}}}\n\n")
		fl.Flush()
		<-u.release
		_, _ = io.WriteString(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\nevent: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\nevent: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{},\"usage\":{\"output_tokens\":6}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		fl.Flush()
	case "gzip":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		zw := gzip.NewWriter(w)
		_, _ = zw.Write([]byte(messageResponse))
		_ = zw.Close()
	case "error":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`)
	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, messageResponse)
	}
}

// seen returns a copy of what the upstream observed.
func (u *upstream) seen() upstream {
	u.mu.Lock()
	defer u.mu.Unlock()
	return upstream{gotAuth: u.gotAuth, gotBeta: u.gotBeta, gotBody: u.gotBody, gotXFF: u.gotXFF, gotHost: u.gotHost, requests: u.requests}
}

// syncBuffer is a log sink the test can read while the handler still
// writes its post-response line.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func startProxy(t *testing.T, up http.Handler, token string) (string, string, *syncBuffer) {
	t.Helper()
	upSrv := httptest.NewServer(up)
	t.Cleanup(upSrv.Close)
	target, err := url.Parse(upSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Token: token, Store: store, Logger: log.New(logs, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("server exit: %v", err)
		}
	})
	return "http://" + srv.Addr(), dir, logs
}

func post(t *testing.T, base, path string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", secret)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "context-management-2025-06-27,fast-mode-2026-02-01")
	req.Header.Set(HeaderSessionID, "session-abc")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// ledgerWait bounds how long a test waits for the record that is written
// after the response has been delivered to the client.
const ledgerWait = 3 * time.Second

// waitLedger polls until n records exist or the wait expires.
func waitLedger(t *testing.T, dir string, n int) []ledger.Record {
	t.Helper()
	deadline := time.Now().Add(ledgerWait)
	for {
		recs := readLedger(t, dir)
		if len(recs) >= n || time.Now().After(deadline) {
			return recs
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readLedger(t *testing.T, dir string) []ledger.Record {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []ledger.Record
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
			var rec ledger.Record
			if err := json.Unmarshal(line, &rec); err != nil {
				t.Fatalf("bad ledger line %q: %v", line, err)
			}
			out = append(out, rec)
		}
	}
	return out
}

func TestPassthroughIsByteExactAndRecordsUsage(t *testing.T) {
	up := &upstream{t: t}
	base, dir, logs := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != messageResponse {
		t.Fatalf("response altered: status %d body %q", resp.StatusCode, body)
	}
	seen := up.seen()
	if string(seen.gotBody) != requestBody {
		t.Fatalf("request body altered: %q", seen.gotBody)
	}
	if seen.gotAuth != secret || seen.gotBeta != "context-management-2025-06-27,fast-mode-2026-02-01" {
		t.Fatalf("headers not forwarded verbatim: key=%q beta=%q", seen.gotAuth, seen.gotBeta)
	}
	if seen.gotXFF != "" {
		t.Fatalf("proxy added X-Forwarded-For: %q", seen.gotXFF)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 {
		t.Fatalf("ledger records = %d", len(recs))
	}
	rec := recs[0]
	if rec.SessionID != "session-abc" || rec.RequestID != "req_upstream_1" || rec.Status != 200 || rec.Model != "claude-opus-5" {
		t.Fatalf("record metadata wrong: %+v", rec)
	}
	if rec.Response.Usage == nil || rec.Response.Usage.CacheRead != 300 || rec.Prompt.SystemBytes != len("be brief") {
		t.Fatalf("record content wrong: %+v", rec)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "session-abc.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{secret, "be brief", "hello", "hi\""} {
		if bytes.Contains(raw, []byte(leak)) || strings.Contains(logs.String(), leak) {
			t.Fatalf("ledger or log contains %q", leak)
		}
	}
}

func TestStreamingIsFlushedIncrementally(t *testing.T) {
	up := &upstream{t: t, mode: "stream", release: make(chan struct{})}
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	defer resp.Body.Close() //nolint:errcheck // test cleanup
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content type %q", got)
	}
	// The first event must arrive before the upstream is released.
	buf := make([]byte, 4096)
	readDone := make(chan int, 1)
	go func() {
		n, _ := resp.Body.Read(buf)
		readDone <- n
	}()
	select {
	case n := <-readDone:
		if !strings.Contains(string(buf[:n]), "message_start") {
			t.Fatalf("first chunk is not the first event: %q", buf[:n])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first event was held back until the stream ended")
	}
	close(up.release)
	rest, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rest), "message_stop") {
		t.Fatalf("stream truncated: %q", rest)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 || recs[0].Response.Usage == nil || recs[0].Response.Usage.Output != 6 || recs[0].Response.Usage.CacheRead != 300 {
		t.Fatalf("stream usage not recorded: %+v", recs)
	}
	if len(recs[0].Response.Blocks) != 1 || recs[0].Response.Blocks[0].Bytes != len("hello") {
		t.Fatalf("stream blocks not recorded: %+v", recs[0].Response.Blocks)
	}
}

func TestGzipResponseIsForwardedCompressedAndParsed(t *testing.T) {
	up := &upstream{t: t, mode: "gzip"}
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", map[string]string{"Accept-Encoding": "gzip"})
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatal("proxy must not decompress on the client's behalf")
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(zr)
	if err != nil || string(decoded) != messageResponse {
		t.Fatalf("compressed body altered: %v %q", err, decoded)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 || recs[0].Response.Usage == nil || recs[0].Response.Usage.CacheRead != 300 {
		t.Fatalf("gzip usage not recorded: %+v", recs)
	}
}

func TestErrorResponsesPassThroughAndAreRecordedWithoutUsage(t *testing.T) {
	up := &upstream{t: t, mode: "error"}
	base, dir, logs := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status %d", resp.StatusCode)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 || recs[0].Status != 429 || recs[0].Response.Usage != nil {
		t.Fatalf("error record wrong: %+v (logs: %s)", recs, logs.String())
	}
}

func TestBrowserOriginAndTokenChecks(t *testing.T) {
	up := &upstream{t: t}
	base, _, _ := startProxy(t, up, "tok")
	resp := post(t, base, "/v1/messages", map[string]string{"Origin": "https://evil.example"})
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("browser origin accepted: %d", resp.StatusCode)
	}
	resp = post(t, base, "/v1/messages", nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token accepted: %d", resp.StatusCode)
	}
	resp = post(t, base, "/v1/messages", map[string]string{HeaderToken: "tok"})
	_ = resp.Body.Close()
	if n := up.seen().requests; resp.StatusCode != http.StatusOK || n != 1 {
		t.Fatalf("valid token rejected: %d (upstream requests %d)", resp.StatusCode, n)
	}
}

func TestUpstreamDownYieldsBadGateway(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	target, _ := url.Parse(dead.URL)
	dead.Close()
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = srv.ListenAndServe(ctx) }()
	resp := post(t, "http://"+srv.Addr(), "/v1/messages", nil)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), "ANTHROPIC_BASE_URL") {
		t.Fatalf("upstream failure not reported usefully: %d %q", resp.StatusCode, body)
	}
}

func TestCountTokensIsForwardedButNotRecorded(t *testing.T) {
	up := &upstream{t: t}
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages/count_tokens", nil)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || up.seen().requests != 1 {
		t.Fatalf("count_tokens not forwarded: %d", resp.StatusCode)
	}
	// Absence can only be checked after the handler has had time to finish.
	time.Sleep(200 * time.Millisecond)
	if recs := readLedger(t, dir); len(recs) != 0 {
		t.Fatalf("count_tokens must not produce ledger records: %+v", recs)
	}
}

func TestRefusesNonLoopbackListen(t *testing.T) {
	target, _ := url.Parse("https://api.anthropic.com")
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{Listen: "0.0.0.0:4000", Upstream: target, Store: store}); err == nil {
		t.Fatal("binding all interfaces must be refused")
	}
}
