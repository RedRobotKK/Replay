package consent

import (
	"runtime"
	"testing"
)

// requireUnixModeBits skips a test that asserts the ownership gate refuses
// something.
//
// The gate reads Unix permission bits, and on Windows there are none: Go
// synthesises 0666 for any writable file and 0444 for a read-only one. A test
// asserting refusal there was not checking that an unsafe file is caught. It
// was passing because EVERY file tripped the check, including the ones a user
// had just written safely, which is the defect this branch fixes.
//
// Skipping is the honest outcome. Loosening the assertion so it passes on both
// platforms would produce a test that runs everywhere and distinguishes
// nothing.
func requireUnixModeBits(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("no Unix mode bits on this platform; the ownership gate does not run here")
	}
}
