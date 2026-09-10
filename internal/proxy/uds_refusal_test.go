package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Four socket-transport refusals that nothing executed.
//
// The socket transport exists for one reason: the kernel's permission check
// replaces "anyone on this machine can dial 127.0.0.1". listenUnix therefore
// refuses rather than proceeds whenever it cannot deliver that. U1-U10 cover
// the refusals that involve an existing socket file. These four did not run at
// all as of this audit: the empty path, the non-directory parent, the missing
// parent, and the Windows platform refusal.
//
// The Windows one is the interesting case. It is behind runtime.GOOS, so no
// test on a developer machine or on a Linux CI runner could ever reach it, and
// the only alternative on offer was to grep the source for the string — which
// is the first defect ADR-0014 catalogues. Instead the platform is read from a
// package variable, so the decision is a function a test can call with
// "windows" as its input.

// UR1: unix:// with nothing after it is refused, by name.
//
// PASS: an error naming the missing socket path.
// FAIL: no error, which means net.Listen("unix", "") decides instead and the
// operator gets "invalid argument" with nothing pointing at the address.
func TestUR1_AUnixAddressWithNoPathIsRefused(t *testing.T) {
	ln, err := listenUnix(UnixScheme)
	if ln != nil {
		_ = ln.Close()
		t.Fatal("unix:// with no path must never bind")
	}
	if err == nil {
		t.Fatal("unix:// with no path must be refused")
	}
	if !strings.Contains(err.Error(), "socket path") {
		t.Fatalf("the refusal must name what is missing: %v", err)
	}
}

// bindWrapper is the prefix listenUnix puts on an error that came back from
// net.Listen. Its presence means the kernel refused, not this package.
//
// It matters because the kernel's own messages read the same as the checks'.
// The first version of UR2 asserted only that the error said "not a directory"
// — and deleting checkSocketDir's IsDir test left it green, because the bind
// then failed with ENOTDIR and the string appeared anyway. That is ADR-0014's
// grep-standing-in-for-a-guarantee, reproduced while writing a test against it.
const bindWrapper = "listen on "

// UR2: a socket whose parent is a regular file is refused before any bind.
//
// checkSocketDir stats the parent and requires a directory. Reaching net.Listen
// at all means the directory checks did not run, and the one that matters — the
// group/other-writable test that stops another user replacing the socket and
// receiving the API key — is in the same function.
//
// PASS: checkSocketDir names the parent as not a directory, and listenUnix
// refuses without the bind wrapper.
// FAIL: no error, or an error carrying "listen on ", which means the kernel
// decided and the permission checks were skipped.
func TestUR2_ASocketUnderARegularFileIsRefusedBeforeBinding(t *testing.T) {
	requireUnix(t)
	dir := shortDir(t)
	notADir := filepath.Join(dir, "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := checkSocketDir(notADir)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("checkSocketDir must refuse a regular file by name, got: %v", err)
	}

	ln, err := listenUnix(UnixScheme + filepath.Join(notADir, "p.sock"))
	if ln != nil {
		_ = ln.Close()
		t.Fatal("a socket cannot live under a regular file and must not appear to")
	}
	if err == nil {
		t.Fatal("a socket under a regular file must be refused")
	}
	if strings.Contains(err.Error(), bindWrapper) {
		t.Fatalf("the kernel refused, not the directory checks; they did not run: %v", err)
	}
}

// UR3: a socket whose parent does not exist is refused before any bind.
//
// PASS: checkSocketDir names the missing directory, and listenUnix refuses
// without the bind wrapper.
// FAIL: as UR2. A stat error swallowed rather than refused skips the
// writable-by-others check for every path whose parent it could not read.
func TestUR3_AMissingSocketDirectoryIsRefusedBeforeBinding(t *testing.T) {
	requireUnix(t)
	missing := filepath.Join(shortDir(t), "nope")

	err := checkSocketDir(missing)
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("checkSocketDir must name the directory it could not read, got: %v", err)
	}

	ln, err := listenUnix(UnixScheme + filepath.Join(missing, "p.sock"))
	if ln != nil {
		_ = ln.Close()
		t.Fatal("a socket in a directory that does not exist must not bind")
	}
	if err == nil {
		t.Fatal("a missing socket directory must be refused")
	}
	if strings.Contains(err.Error(), bindWrapper) {
		t.Fatalf("the kernel refused, not the directory checks; they did not run: %v", err)
	}
}

// UR4: Windows is refused, and the refusal says why rather than failing later.
//
// Windows has AF_UNIX but not the permission semantics the transport exists
// for. Binding there would deliver the name of a guarantee without the
// guarantee, which is worse than not offering it: the operator would have
// moved off TCP believing they had gained something.
//
// PASS: on a platform reported as windows, listenUnix refuses without touching
// the filesystem, and the message names Windows and points at loopback.
// FAIL: a listener, or an error that does not explain the platform. Either
// leaves a Windows user with a socket they think is owner-only.
func TestUR4_TheSocketTransportRefusesOnWindows(t *testing.T) {
	requireUnix(t)
	dir := shortDir(t)
	sock := filepath.Join(dir, "p.sock")

	restore := socketGOOS
	socketGOOS = "windows"
	t.Cleanup(func() { socketGOOS = restore })

	ln, err := listenUnix(UnixScheme + sock)
	if ln != nil {
		_ = ln.Close()
		t.Fatal("the socket transport must not bind on windows")
	}
	if err == nil {
		t.Fatal("windows must be refused, not silently accepted")
	}
	for _, want := range []string{"Windows", "loopback"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the refusal must name %q so the operator knows what to do instead: %v", want, err)
		}
	}
	if _, statErr := os.Stat(sock); statErr == nil {
		t.Fatal("the refusal must come before anything is created on disk")
	}
}

// UR4b: the platform check refuses only Windows.
//
// Without this, UR4 is satisfied by a listenUnix that refuses everywhere, and
// every other test in the socket suite would have to be the thing that notices
// — which is precisely how the shadowed-guard defect in ADR-0014 survived.
//
// PASS: with the platform reported as this machine's real one, the same
// address binds.
// FAIL: a refusal, meaning the platform guard rejects the supported platforms
// too.
func TestUR4b_TheSameAddressBindsOnASupportedPlatform(t *testing.T) {
	requireUnix(t)
	sock := filepath.Join(shortDir(t), "p.sock")

	ln, err := listenUnix(UnixScheme + sock)
	if err != nil {
		t.Fatalf("the platform guard must not refuse a supported platform: %v", err)
	}
	_ = ln.Close()
}
