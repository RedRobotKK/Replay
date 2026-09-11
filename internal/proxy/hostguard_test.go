package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Finding 6 of the 2026-09-04 adversarial security review, closed here.
//
// What was open, verbatim from docs/evidence/security-review-2026-09-04.md:
//
//	`/replay/status` and `/replay/metrics` are unauthenticated by default;
//	`/replay/healthz` has no guard; there is no `Host` validation
//
// and, from the same file's "what threat model you are adopting" table:
//
//	No `Host` validation; the anti-rebinding defence rests on `Origin` being
//	present. `/replay/healthz` answers cross-origin, so a page can fingerprint
//	that Replay is running
//
// Two of the three are closed by the tests below and the third is stated
// rather than closed:
//
//   - Host validation now exists on every TCP-served route, so a page at
//     evil.example whose DNS answers 127.0.0.1 is refused on the Host header
//     alone. That is the defence that does not depend on the browser sending
//     Origin, which was the reviewer's actual objection: Origin is absent on
//     a plain <form> or <img> and on any non-browser client, so a guard that
//     rests on it is a guard with a hole in it.
//   - `/replay/healthz` now carries the browser guard, so a page cannot use
//     it to fingerprint that Replay is running.
//   - The token remains OPTIONAL on the read endpoints, and healthz answers
//     without one on purpose. `replay doctor` probes healthz with no token to
//     tell an operator why their agent is failing (cmd/replay/doctor.go:260),
//     and it has no way to learn a token it did not start the proxy with.
//     Requiring one there would break the one command whose job is to explain
//     a broken setup. What healthz discloses to a non-browser local process
//     is "something answers ok here", which that process could learn from
//     connect(2) anyway.
//
// The Host guard is deliberately confined to TCP. A Unix socket has no DNS
// and no browser reach, and clients address it through a placeholder
// authority — `http://replay/…` in TestU1 — so enforcing a loopback name
// there would refuse the transport that is already the more isolated one.

// hostProbe sends a GET to path with an explicit Host header, dialling addr
// directly so the Host header and the destination can disagree — which is
// exactly the shape of a DNS-rebinding request.
func hostProbe(t *testing.T, base, path, host string, headers map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("probe %s (Host %q): %v", path, host, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// S6a: a Host header naming anything but this machine is refused, on every
// route the TCP listener serves — including the passthrough, which is the one
// that spends money.
//
// PASS: 403 for a rebinding-shaped Host on all four routes.
// FAIL: any status that means the request was served.
func TestS6a_HostHeaderMustNameThisMachine(t *testing.T) {
	up := &upstream{t: t}
	base, _, _ := startProxy(t, up, "")

	hostile := []string{
		"evil.example",
		"evil.example:8080",
		// A subdomain of localhost is RFC 6761 loopback, but a name that
		// merely ENDS in the string is not; the check must not be a suffix
		// match.
		"notlocalhost",
		"localhost.evil.example",
		// A public address literal cannot reach a loopback listener directly,
		// but it can arrive as a Host on a connection that did.
		"203.0.113.4",
	}
	for _, path := range []string{HealthPath, StatusPath, MetricsPath, "/v1/messages"} {
		for _, h := range hostile {
			if got := hostProbe(t, base, path, h, nil); got != http.StatusForbidden {
				t.Errorf("GET %s with Host %q = %d, want 403", path, h, got)
			}
		}
	}
	if n := up.seen().requests; n != 0 {
		t.Errorf("upstream saw %d request(s); a refused Host must not reach the provider", n)
	}
}

// S6b: the names a real client actually sends still work, so the guard is not
// closed by refusing everything.
//
// PASS: 200 on healthz for each loopback spelling.
// FAIL: a refusal, which would break `ANTHROPIC_BASE_URL=http://localhost:PORT`.
func TestS6b_LoopbackNamesAreAccepted(t *testing.T) {
	base, _, _ := startProxy(t, &upstream{t: t}, "")
	_, port, err := net.SplitHostPort(base[len("http://"):])
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{
		"localhost:" + port,
		"localhost",
		"127.0.0.1:" + port,
		"127.0.0.1",
		"[::1]:" + port,
		// RFC 6761 reserves the whole .localhost tree for loopback.
		"replay.localhost:" + port,
	} {
		if got := hostProbe(t, base, HealthPath, h, nil); got != http.StatusOK {
			t.Errorf("GET %s with Host %q = %d, want 200", HealthPath, h, got)
		}
	}
}

// S6c: healthz refuses a browser-originated request, so a page cannot use it
// to fingerprint that Replay is running.
//
// This is the finding's own wording — "`/replay/healthz` answers
// cross-origin" — and it answered 200 to both of these before this change.
//
// PASS: 403 for Origin and for Sec-Fetch-Mode.
// FAIL: 200, which is a page learning the proxy is up.
func TestS6c_HealthzRefusesBrowserOrigins(t *testing.T) {
	base, _, _ := startProxy(t, &upstream{t: t}, "")
	for _, h := range []map[string]string{
		{"Origin": "https://evil.example"},
		{"Sec-Fetch-Mode": "no-cors"},
	} {
		if got := hostProbe(t, base, HealthPath, "", h); got != http.StatusForbidden {
			t.Errorf("GET %s with %v = %d, want 403", HealthPath, h, got)
		}
	}
}

// S6d: healthz still answers the doctor probe, which sends no token and no
// browser headers.
//
// The compensating assertion for the paragraph above: the reason healthz is
// not behind the token is that `replay doctor` needs it, so a change that
// puts it behind one has to fail here first.
//
// PASS: 200 with a token configured and none presented.
// FAIL: 401, which would break `replay doctor` against a tokened proxy.
func TestS6d_HealthzAnswersTheDoctorProbeWithoutAToken(t *testing.T) {
	base, _, _ := startProxy(t, &upstream{t: t}, "a-configured-token")
	if got := hostProbe(t, base, HealthPath, "", nil); got != http.StatusOK {
		t.Errorf("GET %s with no token = %d, want 200", HealthPath, got)
	}
	// The read endpoints, by contrast, do require it when one is set.
	for _, path := range []string{StatusPath, MetricsPath} {
		if got := hostProbe(t, base, path, "", nil); got != http.StatusUnauthorized {
			t.Errorf("GET %s with no token = %d, want 401", path, got)
		}
	}
}

// S6e: the metrics listener carries the same Host guard.
//
// It is a separate mux by design (metrics_listener.go), which is exactly why
// a guard added to the main one can miss it. This is the test that notices.
//
// PASS: 403 on the metrics listener for a rebinding Host.
// FAIL: a served response, which is the whole guard bypassed by choosing the
// other port.
func TestS6e_MetricsListenerCarriesTheHostGuard(t *testing.T) {
	srv, _, done, cancel := metricsServer(t, "127.0.0.1:0", "127.0.0.1:0")
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("server exit: %v", err)
		}
	}()
	srv.Addr()
	base := "http://" + srv.MetricsAddr()
	for _, path := range []string{HealthPath, StatusPath, MetricsPath} {
		if got := hostProbe(t, base, path, "evil.example", nil); got != http.StatusForbidden {
			t.Errorf("metrics listener GET %s with a hostile Host = %d, want 403", path, got)
		}
		if got := hostProbe(t, base, path, "", nil); got != http.StatusOK {
			t.Errorf("metrics listener GET %s with an honest Host = %d, want 200", path, got)
		}
	}
}

// S6f: a Unix socket is exempt, because it has no DNS and clients address it
// through a placeholder authority.
//
// PASS: `http://replay/replay/healthz` over the socket answers 200.
// FAIL: 403, which would break the transport TestU1 documents.
func TestS6f_UnixSocketIsExemptFromTheHostGuard(t *testing.T) {
	requireUnix(t)
	sock := filepath.Join(shortDir(t), "p.sock")
	srv, _ := udsServer(t, "unix://"+sock)
	cancel, done := serveUDS(t, srv)
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("server exit: %v", err)
		}
	}()
	if got := srv.Addr(); got != sock {
		t.Fatalf("Addr() = %q, want the socket path %q", got, sock)
	}
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("no socket was created: %v", err)
	}
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}}
	resp, err := client.Get("http://replay" + HealthPath)
	if err != nil {
		t.Fatalf("request over the socket: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health over the socket = %d, want 200", resp.StatusCode)
	}
}

// S6g: the Host allowlist is not vacuous.
//
// hostIsLocal is the whole guard, so a unit test over it pins the boundary
// cases the end-to-end tests above cannot cheaply reach, and would fail if the
// function were replaced by `return true`.
func TestS6g_HostAllowlistIsNotVacuous(t *testing.T) {
	local := []string{"", "localhost", "localhost:1", "LOCALHOST:8080", "127.0.0.1:8080",
		"127.9.9.9", "[::1]:8080", "::1", "x.localhost", "X.LocalHost:9"}
	foreign := []string{"evil.example", "evil.example:80", "notlocalhost", "localhost.evil.example",
		"203.0.113.4", "0.0.0.0:8080", "10.0.0.1", "[2001:db8::1]:80", "localhostx", "..localhost"}
	for _, h := range local {
		if !hostIsLocal(h) {
			t.Errorf("hostIsLocal(%q) = false, want true", h)
		}
	}
	for _, h := range foreign {
		if hostIsLocal(h) {
			t.Errorf("hostIsLocal(%q) = true, want false", h)
		}
	}
}

// S6h: a request that carries no local address is treated as TCP.
//
// http.Server sets LocalAddrContextKey on every request it serves, so this is
// the case of a handler invoked directly — an httptest.NewRecorder call, a
// middleware test, a future in-process caller. The default has to be the
// strict one: an unknown transport must not be granted the Unix socket's
// exemption from the Host check, because "I could not tell" is not evidence
// that it was a socket.
//
// guard-reachability reported this branch as UNREACHED before this test, which
// is the only reason it is worth a test of its own: the default that fires
// when the environment is unfamiliar is exactly the one that never runs in the
// environment you tested.
func TestS6h_ARequestWithNoLocalAddressIsTreatedAsTCP(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://evil.example"+HealthPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !overTCP(req) {
		t.Error("a request with no local address in its context must default to TCP, and so to the Host check")
	}

	// End to end through the guard, so the default is pinned where it matters
	// rather than only in the helper.
	srv := &Server{}
	rec := httptest.NewRecorder()
	if !srv.notLocal(rec, req) {
		t.Errorf("notLocal accepted Host %q on a request with no known transport", req.Host)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}

// S6i: the refusal explains itself.
//
// The guard has one false positive that a real operator will hit: a name in
// /etc/hosts pointed at 127.0.0.1, which from inside the process is
// byte-for-byte a rebinding attempt. Refusing it is right; refusing it with
// "403 forbidden" and nothing else turns a two-minute fix into a bug report
// about Replay being broken.
//
// PASS: the body names the offending Host and the forms that work.
// FAIL: a bare refusal.
func TestS6i_TheHostRefusalNamesTheHeaderAndTheFix(t *testing.T) {
	base, _, _ := startProxy(t, &upstream{t: t}, "")
	req, err := http.NewRequest(http.MethodGet, base+HealthPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "replay.internal"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"replay.internal", "localhost", "ANTHROPIC_BASE_URL"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the refusal does not mention %q:\n%s", want, body)
		}
	}
}
