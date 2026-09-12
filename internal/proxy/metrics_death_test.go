package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// A metrics listener that dies mid-run.
//
// ListenAndServe answers this by stopping the whole proxy and returning
// "metrics listener stopped: ...". The reasoning in the code is that a scrape
// target which quietly stops being scraped is discovered days later, from a gap
// in a dashboard, and read as an outage.
//
// As of this audit that arm had never executed. MT1-MT7 cover binding, refusing
// and shutting down on a cancelled context; none of them kills the listener.
// A TCP listener the server opened cannot be closed from outside the process,
// which is why bindMetrics exists as a seam.
//
// The claim being tested is not "an error is returned". It is that the PROXY
// stops — a metrics failure that logged and carried on would leave the operator
// with exactly the silent gap the comment says it must not.

// killableListener is a real loopback listener the test keeps a handle on.
type killableListener struct {
	net.Listener
}

// MD1: a metrics listener that dies takes the proxy down, loudly.
//
// PASS: ListenAndServe returns an error naming the metrics listener, and the
// proxy's own port stops answering.
// FAIL: ListenAndServe blocks (the death was swallowed and the operator is
// unscraped and does not know), or it returns nil (same, without even an exit
// code), or the proxy keeps serving on its own port while nothing collects its
// counters.
func TestMD1_AMetricsListenerThatDiesStopsTheProxy(t *testing.T) {
	var held *killableListener
	restore := bindMetrics
	bindMetrics = func(_ string) (net.Listener, error) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		held = &killableListener{Listener: ln}
		return held, nil
	}
	t.Cleanup(func() { bindMetrics = restore })

	srv, _, done, cancel := metricsServer(t, "127.0.0.1:0", "127.0.0.1:0")
	t.Cleanup(cancel)
	base := srv.Addr()
	if held == nil {
		t.Fatal("the seam was not used; ListenAndServe did not bind through bindMetrics")
	}
	// The proxy answers before the metrics listener dies, so a failure below
	// cannot be "it was never up".
	if resp, err := http.Get("http://" + base + HealthPath); err != nil {
		t.Fatalf("the proxy was not serving before the metrics listener died: %v", err)
	} else {
		_ = resp.Body.Close()
	}

	_ = held.Close()

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the proxy carried on with a dead metrics listener; the failure is silent")
	}
	if err == nil {
		t.Fatal("a metrics listener that died must not be reported as a clean exit")
	}
	if !strings.Contains(err.Error(), "metrics listener stopped") {
		t.Fatalf("the error must name what died: %v", err)
	}
	if _, gerr := http.Get("http://" + base + HealthPath); gerr == nil {
		t.Fatal("the proxy is still serving; the metrics failure did not stop it")
	}
}

// MD2: no metrics listener means the arm never fires.
//
// The nil-channel trick in ListenAndServe exists for this: receiving from a nil
// channel blocks forever, so the "stopped" arm cannot be selected when there is
// nothing to serve. Without it serveMetrics returns nil immediately and the
// proxy shuts itself down the moment it starts.
//
// PASS: with no --metrics-listen, the proxy is still serving after a beat and
// exits cleanly on cancel.
// FAIL: an immediate exit, which is the whole proxy killed by a feature nobody
// turned on.
func TestMD2_NoMetricsListenerDoesNotStopTheProxy(t *testing.T) {
	srv, _, done, cancel := metricsServer(t, "127.0.0.1:0", "")
	base := srv.Addr()

	select {
	case err := <-done:
		t.Fatalf("the proxy exited with no metrics listener configured: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	resp, err := http.Get("http://" + base + HealthPath)
	if err != nil {
		t.Fatalf("the proxy is not serving: %v", err)
	}
	_ = resp.Body.Close()

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("clean shutdown reported an error: %v", err)
	}
}

// MD3: serveMetrics reports a listener failure rather than swallowing it.
//
// This is the half MD1 depends on. A closed listener makes http.Server.Serve
// return a non-ErrServerClosed error, and serveMetrics must pass it up; the
// ErrServerClosed case is an ordinary shutdown and must come back as nil.
//
// PASS: a dead listener yields a non-nil error; nothing to serve yields nil.
// FAIL: nil for the dead listener, which would leave MD1 depending on an arm
// that receives nil and returns nil — a proxy that stops without saying why.
func TestMD3_ServeMetricsReportsADeadListener(t *testing.T) {
	s := &Server{cfg: Config{}, stats: newStats()}

	if err := s.serveMetrics(context.Background(), nil); err != nil {
		t.Fatalf("with nothing to serve there is nothing to report: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
	err = s.serveMetrics(context.Background(), ln)
	if err == nil {
		t.Fatal("a listener that cannot accept must be reported, not tolerated in silence")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("a dead listener is not an ordinary shutdown: %v", err)
	}
}

// MD4: while its context is live, serveMetrics reports a listener failure as a
// failure — never as a clean stop.
//
// This is the probe behind the note on ListenAndServe's "metrics listener
// stopped" arm, and it does NOT satisfy that guard's UNREACHED verdict.
// Nothing here makes `err == nil` true over there; what it does is hold the
// premise the note reasons from, so the note is measured rather than argued.
//
// The note says mdone can only carry a non-nil error while the context is
// live, because serveMetrics turns a live-context failure into an error on
// every path. MD3 shows that for one input, a real listener that was closed.
// This shows it for the class: whatever Accept reports, the answer is an
// error. If that stops being true, the note is wrong and this goes red before
// anyone reads it.
//
// PASS: every hostile listener yields a non-nil error.
// FAIL: one of them comes back nil, which is the metrics listener dying and
// ListenAndServe being told the scrape target stopped on purpose.

// acceptFails is a listener whose every Accept reports the same failure.
//
// The errors below are chosen to be ones http.Server returns rather than
// retries: Serve loops on an Accept error only when it satisfies net.Error
// with Temporary() true, and none of these does — net.ErrClosed's Temporary()
// is false by construction (internal/poll errNetClosing), and the rest are not
// net.Errors at all. A retried error would hang this test rather than fail it,
// so the bound below is not decoration.
type acceptFails struct {
	err error
}

func (l *acceptFails) Accept() (net.Conn, error) { return nil, l.err }
func (l *acceptFails) Close() error              { return nil }
func (l *acceptFails) Addr() net.Addr            { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0} }

func TestMD4_ALiveMetricsFailureIsNeverACleanStop(t *testing.T) {
	s := &Server{cfg: Config{}, stats: newStats()}
	for _, c := range []struct {
		name string
		err  error
	}{
		{"the listener was closed underneath it", net.ErrClosed},
		{"the same, wrapped the way the net package wraps it", fmt.Errorf("accept tcp 127.0.0.1:9: %w", net.ErrClosed)},
		{"end of file", io.EOF},
		{"a truncated accept", io.ErrUnexpectedEOF},
		{"a failure with no type at all", errors.New("accept: the listener is broken")},
		{"a cancelled accept", context.Canceled},
	} {
		t.Run(c.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			got := make(chan error, 1)
			go func() { got <- s.serveMetrics(ctx, &acceptFails{err: c.err}) }()
			select {
			case err := <-got:
				if err == nil {
					t.Fatalf("Accept reported %v and serveMetrics called it a clean stop. "+
						"ListenAndServe reads nil on that channel as the metrics listener "+
						"having been asked to stop, so the proxy exits 0 and the operator "+
						"is unscraped with nothing to read", c.err)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("serveMetrics never returned for %v; Serve is retrying an error it "+
					"should have reported, and the metrics listener is neither serving nor "+
					"stopped", c.err)
			}
		})
	}

	// The exception, named rather than left for someone to find.
	//
	// http.ErrServerClosed out of Accept IS mapped to nil, and that nil is the
	// only value that reaches the `err == nil` arm in ListenAndServe. No
	// listener in the net package produces it — Serve itself returns it, for a
	// server someone called Close or Shutdown on — which is why that arm sits
	// unreached and why the note there explains itself instead of carrying a
	// test that would have to fabricate this listener to exist.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.serveMetrics(ctx, &acceptFails{err: http.ErrServerClosed}); err != nil {
		t.Fatalf("an orderly server close came back as %v. Every clean metrics shutdown "+
			"produces ErrServerClosed, and passing it up makes the proxy exit non-zero on "+
			"a stop that was asked for", err)
	}
}
