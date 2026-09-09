package tui

import (
	"fmt"
	"os"
	"testing"
)

// This package resolves a home directory, so its tests must not run against
// the real one.
//
// measured.go:219 calls os.UserHomeDir to decide how to describe a path on
// screen. Nothing here writes to it today, and that is not the guarantee worth
// having: cmd/replay had no write to a home directory either, until TestS1 ran
// `advise` and rewrote the operator's own ~/.replay/advice.json.
//
// The rule is in internal/regression/home_isolation_test.go and computes which
// packages can reach a home directory rather than assuming. This is one of the
// two that can.
//
// USERPROFILE as well as HOME: os.UserHomeDir reads that on Windows and ignores
// HOME, so pinning only HOME would leave the Windows job resolving the runner's
// real profile — the same exposure, hidden on the one platform nobody here
// develops on.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "replay-tui-home-")
	if err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	// Checked, not fire-and-forget. A Setenv that failed quietly would leave
	// the run pointed at the real home with nothing to say so.
	if err := os.Setenv("HOME", dir); err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	if err := os.Setenv("USERPROFILE", dir); err != nil {
		panic("isolating USERPROFILE for the test suite: " + err.Error())
	}

	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		// Worth a line and not worth failing a green suite over: the directory
		// is under the system temp root and the OS will reclaim it.
		fmt.Fprintf(os.Stderr, "removing the suite's temporary home: %v\n", err)
	}
	os.Exit(code)
}
