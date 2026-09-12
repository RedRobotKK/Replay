package proxy

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// Tests for lifecycle.go: binding refuses a non-loopback address, and a
// shutdown closes what carries no turn without cutting one that does.
//
// They are here rather than in server_test.go because that is where the
// code they drive now lives, and a survivor report from
// scripts/guard-reachability is a claim about a guard's own package: the
// cheapest way to keep it honest is for the test to sit next to the guard
// it observes. Nothing in these tests changed in the move.

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

// A client that opened a connection and sent nothing must not hold the
// proxy open: agents keep pooled connections exactly like that, and
// waiting for them turned Ctrl-C into a five-second hang and a non-zero
// exit. In-flight work still gets the grace period.
func TestShutdownClosesConnectionsThatNeverSentARequest(t *testing.T) {
	upSrv := httptest.NewServer(&upstream{t: t})
	t.Cleanup(upSrv.Close)
	target, err := url.Parse(upSrv.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store, Logger: log.New(logs, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	const grace = 100 * time.Millisecond
	srv.shutdownGrace = grace
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	// An accepted connection that never sends a request never becomes
	// idle, so a graceful shutdown alone would wait for ReadHeaderTimeout.
	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	waitFor(t, "the server to accept the connection", func() bool {
		srv.idleMu.Lock()
		defer srv.idleMu.Unlock()
		return len(srv.idleConns) == 1
	})

	cancel()
	start := time.Now()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown must not fail because a client held an unused connection: %v", err)
		}
	case <-time.After(ReadHeaderTimeout):
		t.Fatal("shutdown hung on a connection that never sent a request")
	}
	// The connection carries no turn, so it is closed at once rather than
	// waited on for the grace period.
	if took := time.Since(start); took > grace {
		t.Fatalf("shutdown waited %v on a connection with no request in flight", took)
	}
	if !strings.Contains(logs.String(), "closed 1 connection(s) with no request in flight") {
		t.Fatalf("the close must be logged:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), "turns still running") {
		t.Fatalf("nothing was in flight, so nothing should be forced:\n%s", logs.String())
	}
}

// A shutdown with nothing open returns cleanly and logs nothing about
// forcing connections closed.
func TestShutdownIsCleanWhenNothingIsOpen(t *testing.T) {
	up := &upstream{t: t}
	base, _, logs := startProxyWith(t, up, Config{})
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-shutdown"})
	http.DefaultClient.CloseIdleConnections()
	// startProxyWith's cleanup asserts ListenAndServe returned nil.
	t.Cleanup(func() {
		if strings.Contains(logs.String(), "turns still running") {
			t.Errorf("a clean shutdown must not force a turn closed:\n%s", logs.String())
		}
	})
}

// A turn still running at shutdown gets the grace period and is then
// closed, so a wedged provider cannot hold the proxy open forever.
func TestShutdownForcesATurnThatOutlastsTheGrace(t *testing.T) {
	release := make(chan struct{})
	var inFlight atomic.Bool
	upSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		inFlight.Store(true)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	// Cleanups run last registered first: the handler must be released
	// before the upstream server's Close waits for it.
	t.Cleanup(upSrv.Close)
	t.Cleanup(func() { close(release) })
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
	srv, err := New(Config{Listen: "127.0.0.1:0", Upstream: target, Store: store, Logger: log.New(logs, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	const grace = 200 * time.Millisecond
	srv.shutdownGrace = grace
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	base := "http://" + srv.Addr()
	started := make(chan struct{})
	go func() {
		close(started)
		req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(requestBody))
		if err != nil {
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	// Let the request reach the upstream handler, which then blocks.
	waitFor(t, "the turn to reach the provider", inFlight.Load)

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a forced close is not a failure: %v", err)
		}
	case <-time.After(ReadHeaderTimeout):
		t.Fatal("shutdown hung on a running turn")
	}
	if !strings.Contains(logs.String(), "turns still running after") {
		t.Fatalf("the forced close must be logged:\n%s", logs.String())
	}
	// The provider may already have billed the turn Replay cut short, so it
	// is still recorded. Waiting for the record also lets the handler
	// finish before the test's temporary directory goes away.
	if recs := waitLedger(t, dir, 1); recs[0].Path != "/v1/messages" {
		t.Fatalf("a turn cut short by shutdown must still be recorded: %+v", recs[0])
	}
}
