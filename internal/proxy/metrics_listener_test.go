package proxy

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The read-only metrics listener.
//
// Prometheus cannot scrape a Unix socket. So a user who moves the proxy onto a
// socket for the isolation it gives has no way to be scraped, and a user who
// wants to be scraped has to leave the proxy on a TCP port every local process
// can reach. This listener is the way out: the proxy stays on the socket, and
// a second, separate listener serves the counters.
//
// Its single load-bearing property is that it CANNOT proxy. If a request for
// /v1/messages on this port reached upstream, the port would be a complete
// bypass of the transport it exists to complement — anyone who can open a
// loopback connection could spend against the key, which is precisely what
// moving to a socket was meant to stop. Nearly every test here is that one
// property approached from a different direction, because it is the only thing
// standing between "metrics endpoint" and "unauthenticated proxy".

// metricsServer starts a proxy with a metrics listener and counts upstream hits.
func metricsServer(t *testing.T, listen, metricsListen string) (*Server, *atomic.Int64, chan error, context.CancelFunc) {
	t.Helper()
	var upstreamHits atomic.Int64
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(up.Close)
	target, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	store, err := ledger.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(Config{
		Listen: listen, MetricsListen: metricsListen, Upstream: target,
		Store: store, Logger: log.New(&syncBuffer{}, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	return srv, &upstreamHits, done, cancel
}

// MT1: the counters are reachable on the metrics listener.
//
// PASS: Prometheus text on the second port while the proxy is on a socket.
// FAIL: not served, which leaves a socket user with no way to be scraped and
// makes the whole listener pointless.
func TestMT1_MetricsAreServedOnTheSecondListener(t *testing.T) {
	requireUnix(t)
	sock := filepath.Join(shortDir(t), "p.sock")
	srv, _, done, cancel := metricsServer(t, "unix://"+sock, "127.0.0.1:0")
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("server exit: %v", err)
		}
	}()
	srv.Addr()

	resp, err := http.Get("http://" + srv.MetricsAddr() + MetricsPath)
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics = %d", resp.StatusCode)
	}
	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	if !strings.Contains(string(body[:n]), "# TYPE replay_requests_total counter") {
		t.Errorf("not Prometheus text:\n%s", body[:n])
	}
}

// MT2: the metrics listener cannot proxy. This is the whole point.
//
// A request for the provider API on this port must not reach upstream, must
// not be answered as though it had, and must not consume the API key the
// proxy holds. If it did, opening a loopback socket would be enough to spend
// somebody else's money, and the Unix socket transport would be decorative.
//
// PASS: refused, and upstream saw nothing.
// FAIL: any upstream hit at all — the count is the assertion, because a
// refusal returned to the caller after the request was already forwarded is
// still a request that was forwarded.
func TestMT2_TheMetricsListenerCannotProxy(t *testing.T) {
	requireUnix(t)
	sock := filepath.Join(shortDir(t), "p.sock")
	srv, hits, done, cancel := metricsServer(t, "unix://"+sock, "127.0.0.1:0")
	defer func() { cancel(); <-done }()
	srv.Addr()
	base := "http://" + srv.MetricsAddr()

	for _, path := range []string{"/v1/messages", "/v1/complete", "/", "/v1/messages?beta=true"} {
		req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(requestBody))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", secret)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue // a refused connection is also a refusal
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("POST %s on the metrics listener returned 200; this port is a proxy bypass", path)
		}
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("upstream was reached %d time(s) through the metrics listener. Anyone who can "+
			"open a loopback connection can spend against the key, which is exactly what "+
			"moving the proxy to a socket was meant to prevent", got)
	}
}

// MT3: only the read endpoints exist on it.
//
// PASS: health, status and metrics answer; nothing else does.
// FAIL: a catch-all, which is how MT2's property gets lost later — a route
// added to the shared mux would silently appear here too.
func TestMT3_OnlyReadEndpointsAreExposed(t *testing.T) {
	requireUnix(t)
	sock := filepath.Join(shortDir(t), "p.sock")
	srv, hits, done, cancel := metricsServer(t, "unix://"+sock, "127.0.0.1:0")
	defer func() { cancel(); <-done }()
	srv.Addr()
	base := "http://" + srv.MetricsAddr()

	for _, p := range []string{HealthPath, StatusPath, MetricsPath} {
		resp, err := http.Get(base + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, resp.StatusCode)
		}
	}
	for _, p := range []string{"/", "/replay/", "/anything", "/replay/metrics/../../v1/messages"} {
		resp, err := http.Get(base + p)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("GET %s = 200 on the metrics listener; only the read endpoints belong here", p)
		}
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("upstream reached %d time(s)", got)
	}
}

// MT4: it is off unless asked for.
//
// A metrics port that appears by default would hand every local process the
// counters, and would do it to users who never asked to be scraped.
//
// PASS: no metrics listener, and MetricsAddr says so.
// FAIL: a port bound anyway.
func TestMT4_NoMetricsListenerByDefault(t *testing.T) {
	srv, _, done, cancel := metricsServer(t, "127.0.0.1:0", "")
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("server exit: %v", err)
		}
	}()
	srv.Addr()
	if got := srv.MetricsAddr(); got != "" {
		t.Errorf("MetricsAddr() = %q with no --metrics-listen; nothing should be bound", got)
	}
}

// MT5: it binds loopback only.
//
// The counters name repositories and token spend. Binding a routable address
// publishes a department's usage to the network.
//
// PASS: refused before anything is served.
// FAIL: bound, which turns an internal diagnostic into a public endpoint.
func TestMT5_TheMetricsListenerIsLoopbackOnly(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:0", "192.168.1.10:0", ":0"} {
		_, _, done, cancel := metricsServer(t, "127.0.0.1:0", addr)
		err := refusalFrom(t, cancel, done)
		if err == nil {
			t.Errorf("bound the metrics listener to %q", addr)
			continue
		}
		if !strings.Contains(err.Error(), "loopback") {
			t.Errorf("the refusal for %q does not say why: %v", addr, err)
		}
	}
}

// MT6: it may itself be a socket.
//
// Someone who moved the proxy to a socket for isolation should not be forced
// onto a TCP port to read their own counters. A scraper that can dial a socket
// then needs no TCP port at all.
//
// PASS: unix:// works and carries the same owner-only guarantees.
// FAIL: TCP-only, which makes the isolation choice all-or-nothing.
func TestMT6_TheMetricsListenerMayBeASocket(t *testing.T) {
	requireUnix(t)
	dir := shortDir(t)
	sock := filepath.Join(dir, "p.sock")
	msock := filepath.Join(dir, "m.sock")
	srv, _, done, cancel := metricsServer(t, "unix://"+sock, "unix://"+msock)
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("server exit: %v", err)
		}
	}()
	srv.Addr()

	info, err := os.Lstat(msock)
	if err != nil {
		t.Fatalf("no metrics socket: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("metrics socket is %04o, want 0600", perm)
	}
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", msock)
		},
	}}
	resp, err := client.Get("http://replay" + MetricsPath)
	if err != nil {
		t.Fatalf("scrape over the socket: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics over the socket = %d", resp.StatusCode)
	}
}

// MT7: both listeners stop together.
//
// PASS: the metrics port is closed after shutdown.
// FAIL: a listener outliving the proxy, still answering with counters that
// have stopped moving.
func TestMT7_BothListenersShutDownTogether(t *testing.T) {
	srv, _, done, cancel := metricsServer(t, "127.0.0.1:0", "127.0.0.1:0")
	srv.Addr()
	addr := srv.MetricsAddr()
	if addr == "" {
		t.Fatal("no metrics listener was bound")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("server exit: %v", err)
	}
	if _, err := http.Get("http://" + addr + MetricsPath); err == nil {
		t.Error("the metrics listener is still answering after shutdown")
	}
}
