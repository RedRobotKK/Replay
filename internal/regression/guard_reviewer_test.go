package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reviewer has to run, or it is another catalogue nobody executes.
//
// scripts/guard-reachability neutralises each conditional a pull request
// touches and reports the ones no test observes. It exists because of a
// measured gap rather than a theory: on 2026-09-09 this repository merged 23
// pull requests in fourteen hours, all authored and merged by one agent with CI
// green throughout, and three carried defects. Every one was caught by
// disabling a guard and watching the suite stay green. None by reading a diff.
//
// The frozen mutant catalogue taught the other half of this lesson the hard
// way. It sat behind a build tag for its whole life with no CI job passing the
// tag, so 72 mutants never ran once. A tool that is not wired up is a tool that
// does not exist, and the wiring needs its own cheap guard in the normal suite.
//
// GR2 is the one that matters. A checker that reports nothing is
// indistinguishable from a clean tree, so the script must exit non-zero on a
// survivor rather than printing and passing.

func ciFile(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading the CI workflow: %v", err)
	}
	return string(b)
}

// GR1: a job runs the reviewer.
func TestGR1_CIRunsTheGuardReviewer(t *testing.T) {
	ci := ciFile(t)
	if !strings.Contains(ci, "scripts/guard-reachability/main.go") {
		t.Error("no CI job runs scripts/guard-reachability, so every conditional a pull " +
			"request adds goes unchecked for whether any test can observe it")
	}
	// It diffs against the base branch, which a shallow clone does not have.
	if !strings.Contains(ci, "fetch-depth: 0") {
		t.Error("the workflow never requests full history; the reviewer diffs against the " +
			"base ref and a shallow clone has neither history nor base")
	}
}

// GR2: the script fails the build on a survivor.
//
// Without this it can report a survivor and exit 0, which is a reviewer that
// notices and says nothing — worse than no reviewer, because the green tick
// implies somebody looked.
func TestGR2_TheReviewerFailsOnASurvivor(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "guard-reachability", "main.go"))
	if err != nil {
		t.Fatalf("reading the reviewer: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "os.Exit(1)") {
		t.Error("the reviewer never exits non-zero, so a survivor is printed into a log " +
			"nobody reads and the build stays green")
	}
	// And it must refuse a red baseline, or every neutralisation looks caught.
	if !strings.Contains(s, "the baseline is red") {
		t.Error("the reviewer does not refuse a red baseline; against one, every mutant " +
			"appears caught and the run asserts nothing (ADR-0014)")
	}
	// A stillborn mutant is not a caught mutant.
	if !strings.Contains(s, "stillborn") {
		t.Error("the reviewer does not distinguish a mutant the compiler rejected from one " +
			"the tests killed, which is how a score is inflated")
	}
}
