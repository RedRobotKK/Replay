package proxy

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The Unix domain socket transport.
//
// A loopback TCP port is reachable by every user and every process on the
// machine. That is fine on a single-user laptop and wrong on a shared build
// box, a jump host, or anywhere a process you did not start is running as
// another user: the proxy carries the API key it was configured with, and
// anything that can dial it can spend against that key.
//
// A socket file is an ordinary filesystem object, so the kernel's own
// permission check answers the question instead. That property is the entire
// reason to offer this transport, and these tests are what stop it being
// claimed rather than delivered — a socket at 0666, or inside a directory
// anyone can write to, provides exactly nothing over the TCP port.
//
// It is opt-in. `ANTHROPIC_BASE_URL` takes an http:// URL, and the provider
// SDKs cannot dial a socket path from one, so making this the default would
// disconnect every ordinary user from their agent.

// shortDir is an owner-only directory with a short path. macOS temp
// directories are long enough on their own to exceed sun_path, so a test
// using t.TempDir() would exercise the length limit rather than the thing it
// is about.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "rpx")
	if err != nil {
		t.Skipf("cannot create a short temp directory: %v", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func udsServer(t *testing.T, sock string) (*Server, func()) {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		Listen: sock, Upstream: target, Store: store,
		Logger: log.New(&syncBuffer{}, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv, up.Close
}

// serveUDS starts the server and returns the error it exits with.
func serveUDS(t *testing.T, srv *Server) (context.CancelFunc, chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	return cancel, done
}

// refusalFrom waits for a bind to fail, bounded.
//
// Every test below expects the server to REFUSE. If a change lets it bind
// instead, an unbounded receive blocks until the whole package times out, and
// the run reports a hang rather than the failure it is — which is what
// happened: removing the socket-directory check left the frozen-mutant harness
// stuck for seven minutes on a mutant that should have died in five seconds.
// A mutant that removes a stop condition must fail, not hang.
func refusalFrom(t *testing.T, cancel context.CancelFunc, done chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		cancel()
		<-done
		t.Fatal("the server bound instead of refusing; the guard under test is gone")
		return nil
	}
}

func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix domain sockets with filesystem permissions")
	}
}

// U1: a unix:// address binds a socket and serves over it.
//
// PASS: a request over the socket reaches upstream, and Addr names the path.
// FAIL: TCP semantics leaking through — a socket that is not there, or an
// address that is a host:port.
func TestU1_UnixAddressServes(t *testing.T) {
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
		t.Errorf("Addr() = %q, want the socket path %q", got, sock)
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health over the socket = %d", resp.StatusCode)
	}
}

// U2: the socket is owner-only.
//
// This is the whole property. A socket at 0666 is reachable by every local
// user, which is the situation the TCP port already had, so the transport
// would be ceremony.
//
// PASS: 0600.
// FAIL: any group or other bit, including the umask-dependent default, which
// is why the mode is set explicitly rather than left to the process umask.
func TestU2_TheSocketIsOwnerOnly(t *testing.T) {
	requireUnix(t)
	// A permissive umask is the realistic hostile case: it is inherited from
	// whatever shell or supervisor started the proxy.
	old := syscallUmask(0)
	defer syscallUmask(old)

	sock := filepath.Join(shortDir(t), "p.sock")
	srv, _ := udsServer(t, "unix://"+sock)
	cancel, done := serveUDS(t, srv)
	defer func() { cancel(); <-done }()
	srv.Addr()

	info, err := os.Lstat(sock)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("socket is %04o, want 0600: anything that can dial this can spend "+
			"against the key the proxy holds", perm)
	}
}

// U3: a directory anyone can write to cannot hold the socket.
//
// Socket permissions are not enough on their own. If the containing directory
// is writable by others, another user can unlink the socket and bind their own
// in its place, and the agent then sends its key to them. The classic case is
// putting it in /tmp.
//
// PASS: refused, and the message names the directory.
// FAIL: bound anyway, which delivers a worse property than TCP while claiming
// a better one.
func TestU3_ARefusedSocketDirectory(t *testing.T) {
	requireUnix(t)
	dir := shortDir(t)
	shared := filepath.Join(dir, "shared")
	if err := os.Mkdir(shared, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o777); err != nil {
		t.Skipf("cannot create a world-writable directory here: %v", err)
	}
	srv, _ := udsServer(t, "unix://"+filepath.Join(shared, "p.sock"))
	cancel, done := serveUDS(t, srv)
	err := refusalFrom(t, cancel, done)
	if err == nil {
		t.Fatal("bound a socket in a world-writable directory; another user can replace it " +
			"and receive the API key")
	}
	if !strings.Contains(err.Error(), "writable") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// U4: a symlink at the socket path is refused.
//
// Binding through a link writes the socket wherever the link points, which is
// somebody else's choice of location rather than the user's.
//
// PASS: refused.
// FAIL: followed.
func TestU4_ASymlinkAtTheSocketPathIsRefused(t *testing.T) {
	requireUnix(t)
	dir := shortDir(t)
	sock := filepath.Join(dir, "p.sock")
	if err := os.Symlink(filepath.Join(dir, "e.sock"), sock); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	srv, _ := udsServer(t, "unix://"+sock)
	cancel, done := serveUDS(t, srv)
	if err := refusalFrom(t, cancel, done); err == nil {
		t.Fatal("bound through a symlink")
	} else if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// U5: a live proxy is never displaced.
//
// A stale socket must be cleaned up, but a socket something is still
// listening on must not be: unlinking it would silently steal every future
// request from a proxy that is running and working.
//
// PASS: the second bind refuses and the first still answers.
// FAIL: the second wins, which is data loss disguised as a restart.
func TestU5_ALiveProxyIsNotDisplaced(t *testing.T) {
	requireUnix(t)
	sock := filepath.Join(shortDir(t), "p.sock")
	first, _ := udsServer(t, "unix://"+sock)
	cancel, done := serveUDS(t, first)
	defer func() { cancel(); <-done }()
	first.Addr()

	second, _ := udsServer(t, "unix://"+sock)
	cancel2, done2 := serveUDS(t, second)
	// Bounded. A refusal returns at once; a mutant that lets the second bind
	// would otherwise leave this blocked until the whole package times out,
	// which reports as a hang rather than as the failure it is.
	select {
	case err := <-done2:
		if err == nil {
			t.Fatal("a second proxy took over a live socket")
		}
		if !strings.Contains(err.Error(), "already") {
			t.Errorf("the refusal does not say a proxy is already there: %v", err)
		}
	case <-time.After(3 * time.Second):
		cancel2()
		<-done2
		t.Fatal("the second proxy bound over a socket the first is still serving; " +
			"every future request would go to it instead")
	}

	client := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sock)
		},
	}}
	resp, err := client.Get("http://replay" + HealthPath)
	if err != nil {
		t.Fatalf("the original proxy stopped answering: %v", err)
	}
	resp.Body.Close()
}

// U6: a stale socket is cleaned up.
//
// A proxy killed with SIGKILL leaves the file behind. Refusing to start until
// somebody deletes it by hand would make an ordinary crash into an outage.
//
// PASS: binds over a socket nothing is listening on.
// FAIL: refuses, or clobbers a live one (U5 covers the other direction).
func TestU6_AStaleSocketIsReplaced(t *testing.T) {
	requireUnix(t)
	sock := filepath.Join(shortDir(t), "p.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	// Close the listener but leave the file, which is what a killed process
	// does.
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = ln.Close()
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("the stale socket did not survive the close: %v", err)
	}

	srv, _ := udsServer(t, "unix://"+sock)
	cancel, done := serveUDS(t, srv)
	defer func() { cancel(); <-done }()
	if got := srv.Addr(); got != sock {
		t.Fatalf("did not bind over the stale socket, Addr() = %q", got)
	}
}

// U7: the socket is removed on shutdown.
//
// PASS: gone after a clean exit, so the next start is not a stale-socket case.
// FAIL: left behind, which turns every restart into U6.
func TestU7_TheSocketIsRemovedOnShutdown(t *testing.T) {
	requireUnix(t)
	sock := filepath.Join(shortDir(t), "p.sock")
	srv, _ := udsServer(t, "unix://"+sock)
	cancel, done := serveUDS(t, srv)
	srv.Addr()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("server exit: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Lstat(sock); errors.Is(err, os.ErrNotExist) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("the socket file outlived a clean shutdown")
}

// U8: TCP is untouched and is still the default.
//
// The provider SDKs take an http:// base URL and cannot dial a socket path,
// so a default of unix:// would disconnect every ordinary user from their
// agent. This asserts the existing transport is unchanged by the new one.
//
// PASS: a host:port address still binds TCP and serves.
// FAIL: anything that makes the ordinary path depend on the new one.
func TestU8_TCPRemainsTheDefaultTransport(t *testing.T) {
	srv, _ := udsServer(t, "127.0.0.1:0")
	cancel, done := serveUDS(t, srv)
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("server exit: %v", err)
		}
	}()
	addr := srv.Addr()
	if !strings.Contains(addr, "127.0.0.1:") {
		t.Fatalf("Addr() = %q, want a loopback host:port", addr)
	}
	resp, err := http.Get("http://" + addr + HealthPath)
	if err != nil {
		t.Fatalf("request over TCP: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health over TCP = %d", resp.StatusCode)
	}
}

// U9: a path too long for the kernel is refused by name, not by errno.
//
// sun_path is a fixed-size field — 104 bytes on macOS, 108 on Linux. Exceed it
// and bind returns "invalid argument", which names nothing and sends people to
// look at permissions. This is reachable in ordinary use: a deep home
// directory or a sandboxed temp path gets there without trying, and it is how
// the first version of these tests failed.
//
// PASS: refused before binding, with the limit and the actual length stated.
// FAIL: the kernel's errno reaching the user, or a silent truncation.
func TestU9_APathTooLongForTheKernelIsRefusedByName(t *testing.T) {
	requireUnix(t)
	dir := shortDir(t)
	// Build past the limit with nested directories, each owner-only so the
	// directory check is not what refuses.
	for len(dir) <= maxSocketPath {
		dir = filepath.Join(dir, "0123456789")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	sock := filepath.Join(dir, "p.sock")
	srv, _ := udsServer(t, "unix://"+sock)
	cancel, done := serveUDS(t, srv)
	err := refusalFrom(t, cancel, done)
	if err == nil {
		t.Fatal("bound a socket at a path longer than the kernel accepts")
	}
	msg := err.Error()
	if strings.Contains(msg, "invalid argument") {
		t.Errorf("the kernel's errno reached the user unexplained: %v", err)
	}
	if !strings.Contains(msg, "kernel allows") {
		t.Errorf("the refusal does not say what the limit is: %v", err)
	}
}

// U10: a file that is not a socket is never deleted.
//
// Clearing a stale socket means unlinking a file, and the path is
// user-supplied. A typo that points at something real — a key, a config, a
// database — must not cost the user that file. Only a socket is ever removed.
//
// PASS: refused, and the file is byte-for-byte intact.
// FAIL: the file removed, which is data loss caused by a typo.
func TestU10_ANonSocketAtThePathIsNeverDeleted(t *testing.T) {
	requireUnix(t)
	path := filepath.Join(shortDir(t), "p.sock")
	const precious = "not a socket: someone's actual file\n"
	if err := os.WriteFile(path, []byte(precious), 0o600); err != nil {
		t.Fatal(err)
	}
	srv, _ := udsServer(t, "unix://"+path)
	cancel, done := serveUDS(t, srv)
	if err := refusalFrom(t, cancel, done); err == nil {
		t.Fatal("bound over a regular file")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file was destroyed: %v", err)
	}
	if string(body) != precious {
		t.Errorf("the file was modified:\n got %q\nwant %q", body, precious)
	}
}
