package proxy

import (
	"context"
	"errors"
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
