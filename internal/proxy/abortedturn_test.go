package proxy

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// A turn the user interrupted is still a turn the provider billed.
//
// When the client goes away mid-stream the reverse proxy aborts the handler
// with a panic. The bookkeeping is deferred precisely so it still runs, and
// the log line carries `aborted=client-disconnected` so an operator reading
// the log can tell an interrupted turn from a completed one. guard-reachability
// reported the note INERT (#238): it ran, and nothing depended on whether it
// did, so the log could have lost the marker silently.
//
// The distinction matters for reading spend: an interrupted turn that is not
// marked looks like a short, cheap, successful turn.
func TestAnInterruptedTurnIsRecordedAndMarked(t *testing.T) {
	// An upstream that sends headers, then keeps streaming, so the client can
	// leave while the response is still open.
	release := make(chan struct{})
	up := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f, _ := w.(http.Flusher)
		for i := 0; i < 200; i++ {
			if _, err := fmt.Fprintf(w, "data: {\"chunk\":%d}\n\n", i); err != nil {
				break
			}
			if f != nil {
				f.Flush()
			}
			select {
			case <-release:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	})
	base, dir, logs := startProxy(t, up, "")
	defer close(release)

	addr := strings.TrimPrefix(base, "http://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"model":"claude-sonnet-5","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req := "POST /v1/messages HTTP/1.1\r\nHost: 127.0.0.1\r\nContent-Type: application/json\r\n" +
		fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)) + body
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}

	// Read just enough to know the stream started, then walk away, which is
	// what an agent does when the user interrupts a turn.
	br := bufio.NewReader(conn)
	if _, err := br.ReadString('\n'); err != nil {
		t.Fatalf("no status line from the proxy: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "data:") {
			break
		}
	}
	_ = conn.Close()

	// The turn must still reach the ledger. The provider billed it.
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("a turn the client interrupted wrote no ledger record. The provider " +
			"billed it, so it must still be counted; otherwise interrupting a turn is " +
			"a way to spend money Replay never sees")
	}

	// And the log must say it was interrupted.
	const marker = "aborted=client-disconnected"
	waitFor(t, "the interrupted-turn marker in the log", func() bool {
		return strings.Contains(logs.String(), marker)
	})
	if !strings.Contains(logs.String(), marker) {
		t.Fatalf("the log never carried %q. An interrupted turn that is not marked reads "+
			"as a short, cheap, successful one.\n--- log ---\n%s", marker, logs.String())
	}
}

// A panic that is not a client disconnect is re-raised, not eaten.
//
// The bookkeeping deferred in handle calls recover(), and recover() catches
// EVERY panic, not only the http.ErrAbortHandler the reverse proxy raises when
// a client leaves. Re-raising is what keeps a genuine bug in the proxy a
// crash the operator sees instead of a request that quietly logged and
// returned 200.
//
// guard-reachability called the re-raise INERT and it was right in a narrow
// sense: on the abort path nothing downstream can tell the difference, because
// the client is already gone. Neutralising it leaves the whole package green.
// That is precisely why this test drives the other case.
func TestAPanicThatIsNotAnAbortIsNotSwallowed(t *testing.T) {
	srv := testServerForPanic(t)

	r := localReqForPanic(`{"model":"claude-sonnet-5","messages":[{"role":"user","content":"hi"}]}`)
	w := &panicOnWrite{panicWith: "a genuine bug, not a disconnect"}

	var got any
	func() {
		defer func() { got = recover() }()
		srv.handle(w, r)
	}()

	if got == nil {
		t.Fatal("handle returned normally after a panic inside it. The deferred " +
			"bookkeeping recovers every panic, so without the re-raise a real bug in " +
			"the proxy is swallowed: no crash, no stack, and the operator sees a " +
			"request that looks like it worked")
	}
	if s, ok := got.(string); !ok || s != "a genuine bug, not a disconnect" {
		t.Fatalf("a different value came back out: %#v", got)
	}
}

// panicOnWrite fails the way a bug does: partway through serving.
type panicOnWrite struct {
	hdr       http.Header
	panicWith string
}

func (p *panicOnWrite) Header() http.Header {
	if p.hdr == nil {
		p.hdr = http.Header{}
	}
	return p.hdr
}
func (p *panicOnWrite) Write([]byte) (int, error) { panic(p.panicWith) }
func (p *panicOnWrite) WriteHeader(int)           { panic(p.panicWith) }

func testServerForPanic(t *testing.T) *Server {
	t.Helper()
	target, err := url.Parse("https://api.anthropic.com")
	if err != nil {
		t.Fatal(err)
	}
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func localReqForPanic(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	r.Host = "127.0.0.1:8080"
	return r
}
