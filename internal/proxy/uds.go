package proxy

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The Unix domain socket transport.
//
// A loopback TCP port is bound to an address, and an address is not an
// authorization: every user and every process on the machine can connect to
// 127.0.0.1:4000. On a personal laptop that is nobody. On a shared build box,
// a jump host, or a machine running a service as another user, it is anyone —
// and the proxy holds the API key it was configured with, so whatever can
// dial it can spend against that key and read whatever it sends back.
//
// A socket file is an ordinary filesystem object, so the kernel answers the
// question instead, with the permission bits the user can see and audit. That
// property is the only reason to offer this transport, which is why binding
// refuses rather than proceeds whenever the property cannot be delivered.
//
// It is opt-in. ANTHROPIC_BASE_URL takes an http:// URL and the provider SDKs
// cannot dial a socket path from one, so a default of unix:// would leave
// every ordinary user unable to reach their own proxy. TCP stays the default;
// this is for people who know they need it.

// UnixScheme marks a listen address as a socket path rather than host:port.
const UnixScheme = "unix://"

// socketMode is set explicitly rather than left to the process umask, which
// is inherited from whatever shell or supervisor started the proxy and is
// routinely permissive.
const socketMode = 0o600

// maxSocketPath is the shortest sun_path any supported platform offers, less
// the terminating NUL: 104 on macOS, 108 on Linux. Using the smaller keeps a
// path that works on one from failing on the other.
const maxSocketPath = 103

// isUnixAddr reports whether a listen address asks for a socket.
func isUnixAddr(addr string) bool { return strings.HasPrefix(addr, UnixScheme) }

// socketPath is the filesystem path in a unix:// listen address.
func socketPath(addr string) string { return strings.TrimPrefix(addr, UnixScheme) }

// listenUnix binds a socket, having first established that the socket can
// actually carry the guarantee the transport exists to make.
//
// The checks are the feature. A socket at 0666, or one sitting in a directory
// anyone can write to, is strictly worse than the TCP port it replaced,
// because it looks like a security measure while providing nothing.
func listenUnix(addr string) (net.Listener, error) {
	path := socketPath(addr)
	if path == "" {
		return nil, errors.New("a unix:// listen address needs a socket path")
	}
	if runtime.GOOS == "windows" {
		// Windows has AF_UNIX, but not the permission semantics this
		// transport is for. Offering it there would be offering the name of
		// a guarantee without the guarantee.
		return nil, errors.New("the unix socket transport relies on filesystem permissions " +
			"that Windows does not provide here; use the loopback address instead")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// The kernel stores the path in a fixed-size struct field: sun_path is
	// 104 bytes on macOS and 108 on Linux, NUL included. Exceed it and bind
	// fails with "invalid argument", which says nothing about the cause and
	// sends people looking at permissions. A deep home directory reaches this
	// in ordinary use, so the limit is checked here and named.
	if len(abs) > maxSocketPath {
		return nil, fmt.Errorf("socket path is %d bytes and the kernel allows %d: %s\n"+
			"Unix sockets store the path in a fixed-size field. Use a shorter path, "+
			"such as ~/.replay/p.sock", len(abs), maxSocketPath, abs)
	}
	if err := checkSocketDir(filepath.Dir(abs)); err != nil {
		return nil, err
	}
	if err := clearSocketPath(abs); err != nil {
		return nil, err
	}

	ln, err := net.Listen("unix", abs)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", abs, err)
	}
	// Bind first, then narrow. The socket exists between these two calls with
	// whatever the umask allowed, so the directory check above is what covers
	// that window: nobody else can reach into an owner-only directory to
	// connect in the meantime.
	if err := os.Chmod(abs, socketMode); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("cannot restrict %s to its owner: %w", abs, err)
	}
	return ln, nil
}

// checkSocketDir refuses a directory whose contents someone else can replace.
//
// Socket permissions alone are not enough. If the containing directory is
// writable by others, another user can unlink the socket and bind their own in
// its place; the agent then sends its API key to them, over a path the user was
// told was private. Putting the socket in /tmp is the ordinary way to get this
// wrong.
func checkSocketDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("socket directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if perm := info.Mode().Perm(); perm&0o022 != 0 {
		return fmt.Errorf("socket directory %s is writable by group or other (%04o); "+
			"anyone who can write there can replace the socket and receive the API key. "+
			"Put it somewhere only you can write, such as ~/.replay", dir, perm)
	}
	// A sticky world-writable parent (/tmp) still allows creating names, so
	// the check above is the one that matters; this is the same test applied
	// to the symlink case at the socket path itself, in clearSocketPath.
	return nil
}

// clearSocketPath removes a socket left behind by a killed proxy, and refuses
// anything else.
//
// The two cases have to be told apart. A stale file must go, or an ordinary
// SIGKILL becomes an outage nobody can clear without a manual rm. A socket
// something is still listening on must NOT go: unlinking it would silently
// take every future request away from a proxy that is running and working.
// Dialling it is the only reliable way to know which it is.
func clearSocketPath(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink; refusing to bind a socket through a redirected path", path)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s exists and is not a socket; refusing to delete it", path)
	}
	if c, derr := net.Dial("unix", path); derr == nil {
		_ = c.Close()
		return fmt.Errorf("a proxy is already listening on %s; refusing to take over its socket", path)
	}
	// Nothing answered, so the file is the remains of a process that died.
	return os.Remove(path)
}
