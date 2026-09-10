package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The surface harness must actually run, and must be able to fail.
//
// scripts/surface-drift/drift.py exercises every command and flag in
// docs/CLI.md and compares each exit code against what that surface is supposed
// to do. It sat in the repository unrun, and wiring it up needed two defects
// fixed first — both of them instances of classes this project has already
// frozen elsewhere:
//
//   - It ended `return 0` unconditionally, so DRIFT, FLAKY and TIMEOUT were
//     printed and then discarded. A job running it would have been a check that
//     could not fail. See ADR-0014 and TestFrozenFD-series.
//   - `serve` was exercised blind. It is a long-running listener: it either
//     binds and blocks to the 45s timeout, or reports the port already in use
//     on a machine where a proxy is running. Eight serve surfaces times four
//     invocations is roughly twenty-four minutes of a job measuring the runner
//     rather than the product, which is why the first attempt to run this
//     harness was killed rather than read.
//
// These tests are cheap and run in the normal suite. The expensive job is what
// exercises the surfaces; this is what proves the expensive job is still wired
// up and still capable of reporting a failure.

func driftScript(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "surface-drift", "drift.py"))
	if err != nil {
		t.Fatalf("reading the surface-drift harness: %v", err)
	}
	return string(b)
}

// SD-1: a CI job runs the harness.
func TestSD1_SurfaceDriftRunsInCI(t *testing.T) {
	for _, line := range ciCommands(t) {
		if strings.Contains(line, "surface-drift/drift.py") {
			return
		}
	}
	t.Error("no executable line in .github/workflows/ci.yml runs " +
		"scripts/surface-drift/drift.py. The harness exercises every surface in " +
		"docs/CLI.md and is worth nothing unrun — it already sat here unrun once, " +
		"which is the whole reason this guard exists")
}

// SD-2: the harness can report a failure.
//
// A run that reports DRIFT and exits 0 is indistinguishable from a clean run to
// everything downstream of it, which is what CI is.
func TestSD2_SurfaceDriftCanFail(t *testing.T) {
	body := driftScript(t)
	if !strings.Contains(body, "return 1") {
		t.Error("scripts/surface-drift/drift.py never returns non-zero. It printed " +
			"DRIFT, FLAKY and TIMEOUT counts and then exited 0, so a CI job running it " +
			"could not fail. Restore the exit path rather than deleting this guard")
	}
	for _, verdict := range []string{"DRIFT", "FLAKY", "TIMEOUT"} {
		if !strings.Contains(body, `"`+verdict+`"`) {
			t.Errorf("the harness no longer classifies %s; the exit path keys on these "+
				"three verdicts and silently stops covering one that disappears", verdict)
		}
	}
}

// SD-3: the CI invocation keeps the JSON record off the working tree.
//
// scripts/surface-drift/last-run.json is tracked. A job that writes the default
// path leaves the tree dirty, and a dirty tree is how an unrelated check starts
// failing for a reason nobody can find.
func TestSD3_SurfaceDriftWritesOutsideTheTree(t *testing.T) {
	for _, line := range ciCommands(t) {
		if !strings.Contains(line, "surface-drift/drift.py") {
			continue
		}
		if !strings.Contains(line, "--out") {
			t.Errorf("CI runs the harness without --out, so it writes "+
				"scripts/surface-drift/last-run.json, which is tracked:\n  %s",
				strings.TrimSpace(line))
		}
		return
	}
}
