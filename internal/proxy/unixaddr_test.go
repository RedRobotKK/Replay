package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

// Addr reports an absolute socket path even when the listen address was
// relative.
//
// ListenAndServe used to derive this by running filepath.Abs over the config
// value, a few lines after listenUnix had already run filepath.Abs over the
// same path and refused to bind if it failed. guard-reachability called that
// recomputation INERT -- neutralising it changed no value, because there was
// no value to change -- and the sibling defect on the metrics listener had
// already been removed for the same reason.
//
// Deleting it leaves the behaviour resting on a property of listenUnix rather
// than on a line in this file, so the property is asserted here. If listenUnix
// ever stops absolutising, this fails and names what to put back.
func TestUnixListenerReportsAnAbsolutePathFromARelativeAddress(t *testing.T) {
	// A short directory: the kernel's sun_path field is 104 bytes on macOS,
	// and the standard temp dir alone can exceed it.
	dir, err := os.MkdirTemp("/tmp", "replay-addr")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, filepath.Join(dir, "s.sock"))
	if err != nil {
		t.Skipf("no relative path from %s: %v", wd, err)
	}
	if filepath.IsAbs(rel) {
		t.Fatalf("%q is absolute; this test needs a relative address to mean anything", rel)
	}

	ln, err := listenUnix(UnixScheme + rel)
	if err != nil {
		t.Fatalf("listenUnix(%q): %v", rel, err)
	}
	defer func() { _ = ln.Close() }()

	got := ln.Addr().String()
	if !filepath.IsAbs(got) {
		t.Fatalf("a listener bound from the relative address %q reports %q, which is not "+
			"absolute. Addr() is the only source of the socket path now, so a relative "+
			"one reaches every caller of Server.Addr", rel, got)
	}
	if want := filepath.Join(dir, "s.sock"); got != want {
		t.Fatalf("Addr() = %q, want %q", got, want)
	}
}
