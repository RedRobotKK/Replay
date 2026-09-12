package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// errAfter reads n bytes and then fails, the way a connection dropped
// mid-upload does. A body that fails on the first read would not prove the
// guard sits after the limit reader.
type errAfter struct {
	data []byte
	n    int
}

func (e *errAfter) Read(p []byte) (int, error) {
	if e.n >= len(e.data) {
		return 0, errors.New("connection reset by peer")
	}
	c := copy(p, e.data[e.n:])
	e.n += c
	return c, nil
}
func (e *errAfter) Close() error { return nil }

// localReq builds a request that clears the host guard. httptest.NewRequest
// sets Host to example.com, which notLocal refuses with 403 before handle
// reaches anything this file is about -- which is exactly how the first
// version of these tests passed while proving nothing.
func localReq(body io.ReadCloser) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", body)
	r.Host = "127.0.0.1:8080"
	return r
}

func testServer(t *testing.T) *Server {
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

// A request body that dies mid-read is answered 400, not carried forward as
// though it were the whole body.
//
// passthrough.go reads the body under a LimitReader and checks the error. No
// test made that error happen, so nothing established what a truncated upload
// produces -- and the failure mode if the guard went missing is worse than a
// wrong status: a partial body would be forwarded upstream and recorded as the
// request the agent made.
func TestRequestBodyThatFailsMidReadIsRefused(t *testing.T) {
	srv := testServer(t)
	r := localReq(&errAfter{data: []byte(`{"model":"x",`)})
	w := httptest.NewRecorder()

	srv.handle(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("a body that failed mid-read was answered %d, want %d. A partial body "+
			"must not reach the provider or the ledger as if it were complete",
			w.Code, http.StatusBadRequest)
	}
}

// The half-open probe is given back when the request never reaches an outcome.
//
// The breaker lets exactly one request through after the cooldown. If that
// request dies before the provider is consulted, and the slot is not released,
// `probing` stays true and EVERY later request is refused until the process
// restarts -- the circuit never closes again. That is the bug the guard
// prevents, and it is the one this asserts: not that the branch ran, but that
// a second probe is still available afterwards.
func TestAbortedProbeGivesTheSlotBack(t *testing.T) {
	srv := testServer(t)
	b := NewBreaker(BreakerSettings{Failures: 1, Cooldown: time.Minute})
	srv.cfg.Breaker = b

	// Open the circuit, then move past the cooldown so the next Allow is the
	// half-open probe.
	b.Observe(true)
	base := time.Now()
	b.now = func() time.Time { return base.Add(2 * time.Minute) }

	// A request that fails before any provider outcome. The handler itself
	// must take the half-open slot and give it back; nothing here releases it
	// on the handler's behalf.
	w := httptest.NewRecorder()
	srv.handle(w, localReq(&errAfter{data: []byte(`{"m":`)}))

	// Assert the request actually got past the host guard and into the body
	// read. Without this, a 403 from notLocal would leave the breaker
	// untouched and the check below would pass for the wrong reason.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("the request was answered %d, not %d: it never reached the body read, "+
			"so this test is not exercising the probe-release path at all", w.Code, http.StatusBadRequest)
	}

	ok2, probe2, wait := b.Allow()
	if !ok2 || !probe2 {
		t.Fatalf("after a probe that never reached the provider, the next request got "+
			"ok=%v probe=%v wait=%v. The slot was not given back, so the circuit can "+
			"never close and every request is refused until restart", ok2, probe2, wait)
	}
}

// ListenAndServe reports a bind failure instead of pretending it is serving.
//
// New refuses some addresses, but a port that is free at New and taken by the
// time ListenAndServe runs is an ordinary race, and the error return is how the
// caller finds out. Nothing exercised it.
func TestListenAndServeReportsABindFailure(t *testing.T) {
	// Hold a port so the server's bind is guaranteed to fail.
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Close() }()

	target, _ := url.Parse("https://api.anthropic.com")
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{Listen: held.Addr().String(), Upstream: target, Store: store})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = srv.ListenAndServe(ctx)
	if err == nil {
		t.Fatal("binding a port already held returned no error; a caller would treat " +
			"a server that never bound as running")
	}

	// Addr must be unblocked even on the failed path, or a caller that waits
	// on it deadlocks on a server that is never going to serve.
	done := make(chan string, 1)
	go func() { done <- srv.Addr() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Addr blocked after a failed bind: the deferred markReady did not run")
	}
}
