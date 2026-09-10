package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The test suite must not write to the machine it runs on.
//
// It did. `go test ./cmd/replay/` rewrote the operator's own
// ~/.replay/advice.json, because TestS1 runs every value-delivering command —
// `advise` among them — against a fixture corpus with HOME left alone, and
// `advise` persists its findings to ~/.replay by design.
//
// Three things go wrong at once, and only the third is loud:
//
//   - the reader's saved advice, including which suggestions they had marked
//     applied, is replaced by findings from a two-session test fixture
//   - a later test in the same package reads that file back, so the suite's
//     result depends on what an earlier test left in a directory outside the
//     repository
//   - which is how ▸ and — reached CI: TestTC1 and TestTW1 render the advise
//     screen, find real findings written by TestS1 rather than the empty
//     corpus they assume, and only then reach the rows that draw a marker.
//     Both passed when run alone. That is why this was invisible for four
//     commits and arrived as four simultaneously red CI jobs.
//
// The fix is TestMain, so isolation is the default for the package rather than
// a line each test has to remember. Tests that want a particular HOME still
// call t.Setenv and are unaffected.

// realHome is the environment's HOME as it was before TestMain replaced it.
// Captured so H1 can assert the substitution actually happened; nothing else
// may use it, and nothing may write to it.
var realHome string

func TestMain(m *testing.M) {
	realHome = os.Getenv("HOME")
	if realHome == "" {
		realHome = os.Getenv("USERPROFILE")
	}

	// One directory for the package, removed when it exits. Not t.TempDir,
	// which needs a *testing.T and would not cover code running before the
	// first test.
	dir, err := os.MkdirTemp("", "replay-suite-home-")
	if err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	// Checked, not fire-and-forget. A Setenv that failed quietly would leave
	// the suite pointed at the real home with nothing to say so, which is the
	// defect this function exists to remove.
	if err := os.Setenv("HOME", dir); err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	// USERPROFILE too: os.UserHomeDir reads that on Windows and ignores HOME,
	// so pinning only HOME would leave the Windows job writing to the runner's
	// real profile — the same defect, hidden on the one platform nobody here
	// develops on.
	if err := os.Setenv("USERPROFILE", dir); err != nil {
		panic("isolating USERPROFILE for the test suite: " + err.Error())
	}

	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		// Worth a line on stderr and not worth failing a green suite over: the
		// directory is under the system temp root and the OS will reclaim it.
		fmt.Fprintf(os.Stderr, "removing the suite's temporary home: %v\n", err)
	}
	os.Exit(code)
}

// H1: the suite's HOME is not the machine's HOME.
//
// PASS: commands that persist state write inside a temporary directory.
// FAIL: a test run mutates the reader's ~/.replay, and later tests in the same
// package read back what earlier ones left there.
func TestH1_TheSuiteDoesNotWriteToTheRealHome(t *testing.T) {
	if realHome == "" {
		t.Skip("no HOME in the environment to be protected from")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolving the home directory: %v", err)
	}
	if home == realHome {
		t.Fatalf("the suite runs against the real home directory %q, so any command "+
			"that persists state overwrites the reader's own files", home)
	}
	if _, err := os.Stat(filepath.Join(home, ".replay")); err == nil {
		t.Logf("state was written to the isolated home %q, as intended", home)
	}
}

// H2: running the commands does not touch the real advice file.
//
// H1 checks the mechanism; this checks the consequence, because the mechanism
// could be right and a command could still build its path from something other
// than os.UserHomeDir.
func TestH2_TheRealAdviceFileIsUntouched(t *testing.T) {
	if realHome == "" {
		t.Skip("no HOME in the environment to be protected from")
	}
	theirs := filepath.Join(realHome, ".replay", adviceFileName)

	// Absence is a state to defend, not a reason to skip.
	//
	// This test used to t.Skipf when the file did not exist, which is every CI
	// runner — so on the only machine where nobody is watching, the guard
	// asserted nothing. An audit built the case that proves it matters: a
	// variant of `advise` resolving ~ at package-init, before TestMain can
	// redirect it, leaves H1 passing, writes to the runner's real home, and
	// this test skipped past it with the package green.
	//
	// So there are two properties, not one, and the second is the CI one:
	// a file that exists must be unchanged, and a file that does not exist
	// must not be created.
	before, err := os.Stat(theirs)
	existed := err == nil

	dir := filepath.Join("..", "..", "internal", "transcript", "testdata")
	var out, errb discard
	_ = run([]string{"advise", dir}, &out, &errb)

	after, err := os.Stat(theirs)
	switch {
	case !existed:
		if err == nil {
			t.Errorf("running `advise` in a test CREATED %s (%d bytes). The suite writes "+
				"to the machine's real home, and on a runner with no prior file this is "+
				"the only assertion that can see it.", theirs, after.Size())
		}
	case err != nil:
		t.Fatalf("running advise removed the reader's advice file: %v", err)
	case !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size():
		t.Errorf("running `advise` in a test rewrote %s\n  was %d bytes at %s\n  now %d bytes at %s",
			theirs, before.Size(), before.ModTime(), after.Size(), after.ModTime())
	}
}

// discard is an io.Writer that keeps nothing; these tests are about a file on
// disk, not about what was printed.
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
