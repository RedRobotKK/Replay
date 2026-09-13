package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-WR. The Windows job has to check the thing that is true on Windows.
//
// Until 2026-09-13 the Windows leg ran `go test ./...` and was green, and the
// reason it was green is that the promise it should have been checking is
// switched off there: `internal/ownerdir` reports 40% statement coverage on
// Windows because `modeIsChecked()` returns false, and twenty-two tests skip
// with "Unix permission bits". A green check produced by turning off the thing
// being checked is what ADR-0014 exists to forbid.
//
// The binary now refuses to run on Windows, which is the honest version of that
// situation. The consequence is that every cmd/replay test calling run() fails
// there, and it fails CORRECTLY: those tests assert behaviour the binary has
// just declared it will not perform on that platform. Demanding they pass would
// be demanding Windows do the thing the refusal exists to prevent.
//
// So the Windows leg checks what is actually true there: the tree compiles, the
// packages with no platform refusal pass, and THE BINARY REFUSES. That last one
// is the only end-to-end evidence anywhere that the refusal reaches a user, and
// before this it was checked by nothing at all.
//
// PASS: the Windows leg builds the binary and asserts it refuses.
// FAIL: somebody restored `go test ./...` there to get a green tick back, which
// would fail immediately and then be "fixed" by removing the refusal.
func TestFCWR_TheWindowsLegChecksTheRefusal(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	ci := string(raw)

	if !strings.Contains(ci, "windows-latest") {
		t.Fatal("ci.yml no longer runs anything on Windows. The refusal is the only thing " +
			"standing between a Windows user and a ledger written into a directory whose " +
			"privacy the binary declined to check, and nothing would now observe it.")
	}

	// The refusal has to be exercised against the built artifact, not asserted
	// about the source. The whole reason this section exists is that a check
	// against the tree passed for weeks while the artifact was unexamined.
	if !strings.Contains(ci, "Windows refuses") {
		t.Error("ci.yml has no step named \"Windows refuses\". The Windows leg must run the " +
			"built binary and assert it declines, or the refusal ships with no end-to-end " +
			"evidence that it reaches anybody.")
	}

	// And the suite must not be restored wholesale on Windows. If it is, every
	// run() test fails, and the cheapest way to make CI green again is to delete
	// the refusal, which is precisely the wrong repair.
	windowsRunsWholeSuite := strings.Contains(ci, "matrix.os != 'windows-latest'") ||
		strings.Contains(ci, "runner.os != 'Windows'")
	if !windowsRunsWholeSuite {
		t.Error("the Test step is not scoped away from Windows. `go test ./...` there fails " +
			"on every test that calls run(), because run() refuses on Windows by design, " +
			"and the cheapest way to make that green is to delete the refusal.")
	}
}

// FC-WR2: the refusal text is checkable from a machine that is not Windows.
//
// Scoping the Windows leg away from `go test ./...` had a cost, and this is the
// payment. TestPG1's Windows branch asserts the message names Windows and
// ownerdir, and that branch now executes nowhere: the file it lives in is only
// compiled on Windows, and Windows no longer runs cmd/replay's tests.
//
// The CI step "Windows refuses" checks the real artifact, which is stronger
// evidence than PG1 ever produced. But it only runs in CI, so a maintainer who
// guts the message locally finds out after pushing. This reads the source from
// any platform so `go test ./...` on a laptop says it first.
func TestFCWR2_TheWindowsRefusalStillSaysWhy(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "cmd", "replay", "platformguard_windows.go"))
	if err != nil {
		t.Fatalf("reading platformguard_windows.go: %v", err)
	}
	src := string(raw)

	// The two the CI step greps for. If these move, that step starts failing on
	// a runner rather than here, which is the slower half of the same answer.
	for _, want := range []string{"Windows", "ownerdir"} {
		if !strings.Contains(src, want) {
			t.Errorf("the Windows refusal no longer says %q. The CI step greps for it, so "+
				"this becomes a red Windows runner instead of a local failure.", want)
		}
	}
	// A refusal that does not say where to go instead is a dead end. install.sh
	// used to offer `go install`, which handed the user the exact binary the
	// project says must not ship; naming the real routes is what replaced it.
	routed := false
	for _, route := range []string{"WSL2", "WSL", "Linux", "macOS"} {
		if strings.Contains(src, route) {
			routed = true
		}
	}
	if !routed {
		t.Error("the refusal names no platform the user could move to. A refusal with no " +
			"route is how somebody ends up running `go install` and getting the binary " +
			"this refusal exists to keep off their machine.")
	}
	if !strings.Contains(src, "//go:build windows") {
		t.Error("platformguard_windows.go lost its build tag, so this refusal is either " +
			"compiled everywhere or nowhere")
	}
}
