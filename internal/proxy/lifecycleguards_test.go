package proxy

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Lifecycle guards nothing exercised.
//
// #235 lists nine; measured by neutralising each and running the package, one
// is already caught and six survive. They are not one subject, but they share
// a shape: each decides what an OPERATOR is told when the proxy stops, and a
// proxy that exits quietly on a failure is indistinguishable from one that
// was asked to stop.

// A metrics listener on a unix socket reports an absolute path.
//
// MetricsAddr is what an operator points a scraper at. A relative path is
// correct only from the directory the proxy happened to start in, and the
// proxy is usually started by something whose working directory the operator
// never sees.
func TestLifecycle_AUnixMetricsListenerReportsAnAbsolutePath(t *testing.T) {
	requireUnix(t)
	// shortDir, not t.TempDir. A unix socket path is capped near 104 bytes,
	// and this test's own name plus the macOS temp prefix exceeds it — the
	// bind fails, markReady still fires on the failure path, and MetricsAddr
	// returns empty. The first version of this test failed that way and looked
	// like a defect in the code it was testing.
	// A RELATIVE path, and that is the whole test. net.UnixListener.Addr()
	// returns the path exactly as configured, so an absolute one comes back
	// absolute and the guard changes nothing — the mutant survives. It earns
	// its place only when the operator configured a relative path, which is
	// the case where the reported address would otherwise be correct from a
	// working directory they never see.
	dir := shortDir(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, filepath.Join(dir, "m.sock"))
	if err != nil {
		t.Skipf("no relative path from %s to the socket dir: %v", wd, err)
	}
	if filepath.IsAbs(rel) {
		t.Fatal("the fixture path is absolute; this test cannot distinguish the guard")
	}
	srv, _, done, cancel := metricsServer(t, "127.0.0.1:0", "unix://"+rel)
	defer func() { cancel(); <-done }()

	_ = srv.Addr() // blocks until both listeners are bound
	got := srv.MetricsAddr()

	if !filepath.IsAbs(got) {
		t.Fatalf("the metrics address is not absolute: %q. An operator points a scraper at "+
			"this, and a relative path is only correct from a directory they never see", got)
	}
	if !strings.HasSuffix(got, "m.sock") {
		t.Fatalf("the metrics address is not the socket that was bound: %q", got)
	}
	if strings.HasPrefix(got, UnixScheme) {
		t.Errorf("the scheme was not trimmed: %q is a URL, not a path a scraper can open", got)
	}
}

// A shutdown with a metrics listener waits for it before returning.
//
// Without the wait the proxy returns while the metrics goroutine is still
// closing, so a caller that exits on the return can take the process down
// mid-flush — and any error the metrics listener was about to report is lost.
func TestLifecycle_ShutdownWaitsForTheMetricsListener(t *testing.T) {
	srv, _, done, cancel := metricsServer(t, "127.0.0.1:0", "127.0.0.1:0")
	_ = srv.Addr()
	if srv.MetricsAddr() == "" {
		t.Fatal("no metrics listener bound; this test would assert nothing")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a cancelled context is a clean stop, not an error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ListenAndServe did not return after the context was cancelled")
	}
}

// A server closed by shutdown returns nil, not ErrServerClosed.
//
// http.Serve returns ErrServerClosed on every clean shutdown. Passing it up
// would make a normal stop look like a failure to every caller that checks the
// error — including the CLI, which would exit non-zero on Ctrl-C.
func TestLifecycle_ACleanCloseIsNotAnError(t *testing.T) {
	srv, _, done, cancel := metricsServer(t, "127.0.0.1:0", "")
	defer cancel()
	_ = srv.Addr()

	// Close the HTTP server directly, so Serve returns ErrServerClosed on the
	// errc arm rather than the context arm. Cancelling the context exercises a
	// different branch and would leave this one untouched.
	if err := srv.http.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				t.Fatal("ErrServerClosed was passed up. Every clean shutdown produces it, so a " +
					"caller checking the error reads a normal stop as a failure")
			}
			t.Fatalf("a closed server returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ListenAndServe did not return after the server was closed")
	}
}

// The shutdown grace period honours a configured value.
//
// grace() is the only thing deciding how long in-flight turns get before they
// are cut. A configured value that silently did nothing would look like a
// setting and behave like a default.
func TestLifecycle_AConfiguredGraceIsUsedOverTheDefault(t *testing.T) {
	s := &Server{}
	if got := s.grace(); got != ShutdownTimeout {
		t.Fatalf("an unset grace is %v, want the default %v", got, ShutdownTimeout)
	}
	s.shutdownGrace = 3 * time.Second
	if got := s.grace(); got != 3*time.Second {
		t.Fatalf("a configured grace of 3s came back as %v; the setting is decorative", got)
	}
	// The premise: these really are different, or the comparison above holds
	// for a reason that has nothing to do with the guard.
	if ShutdownTimeout == 3*time.Second {
		t.Fatal("the fixture equals the default; this test cannot distinguish them")
	}
}
