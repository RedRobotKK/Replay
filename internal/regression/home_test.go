package regression

import (
	"fmt"
	"os"
	"testing"
)

// This package can reach a home directory, so its tests must not run against
// the real one.
//
// It was exposed the whole time and HI1 said otherwise, for the most direct
// reason available: HI1 looked for the strings "func TestMain(" and
// `Setenv("HOME"` anywhere in a package's source, and its own source contained
// both — as the needles it was searching for. The guard reported this package
// isolated on the strength of its own grep literals.
//
// Parsing the AST instead removed the self-satisfaction, and the gap it had
// been hiding showed up on the first run.
//
// USERPROFILE as well as HOME: os.UserHomeDir reads that on Windows and ignores
// HOME, so pinning only HOME would leave the Windows job resolving the runner's
// real profile.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "replay-regression-home-")
	if err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	if err := os.Setenv("HOME", dir); err != nil {
		panic("isolating HOME for the test suite: " + err.Error())
	}
	if err := os.Setenv("USERPROFILE", dir); err != nil {
		panic("isolating USERPROFILE for the test suite: " + err.Error())
	}

	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(os.Stderr, "removing the suite's temporary home: %v\n", err)
	}
	os.Exit(code)
}
