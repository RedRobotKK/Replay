package guardcheck

import (
	"errors"
	"strings"
	"testing"
)

// The reviewer's most important property is the one nothing was watching.
//
// PR #242 taught the reviewer to exempt a survivor the base tree already had.
// That is an exemption mechanism, and every exemption mechanism has a failure
// mode: granting itself when it has measured nothing. If the base worktree
// cannot be created, or the base suite is red, or the base sources will not
// parse, then the base has certified nothing and NOTHING may be exempted —
// every survivor is this change's until proven otherwise, and the run exits 1.
//
// That was correct on 2026-09-11 by reading, and only by reading. The decision
// lived in scripts/guard-reachability/main.go, which carries //go:build ignore,
// so `go test ./...` could not reach a line of it. A silent turn from
// fail-closed to fail-open there would not have gone red anywhere: the
// reviewer would have kept printing its summary and stopped failing builds,
// which is worse than not having a reviewer, because the summary is read as
// evidence and there would be none behind it.
//
// These tests are the watcher. ClassifySurvivors and ExitCode are the decision
// itself, extracted here so it can be put to a suite; main.go now only feeds
// them.

// errBaseSilent stands for every way the base tree fails to answer. The
// decision does not read the error, only whether there is one.
var errBaseSilent = errors.New("checking origin/main out into a worktree: exit status 128")

// funcsOf names a set of guards by enclosing function.
//
// A failure here has to say WHICH guard went to the wrong side, and printing
// the Guard structs says it in four lines of temp-directory path per guard —
// evidence nobody reads is not evidence.
func funcsOf(gs []Guard) string {
	names := make([]string, 0, len(gs))
	for _, g := range gs {
		names = append(names, g.Func)
	}
	return strings.Join(names, ", ")
}

// twoSurvivors parses two guards with distinct identities, for tests that need
// real identities rather than hand-built ones.
func twoSurvivors(t *testing.T) (moved, fresh Guard) {
	t.Helper()
	dir := t.TempDir()
	path := writeGoIn(t, dir, "pair.go", `package p

func handle(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func report(n int) int {
	if n == 7 {
		return 1
	}
	return n
}
`)
	moved, fresh = only(t, path, 4), only(t, path, 11)
	if moved.Identity() == fresh.Identity() {
		t.Fatal("the fixture's two guards were meant to be distinguishable")
	}
	return moved, fresh
}

// FC1: a base tree that could not answer exempts nothing, whatever counts came
// back with the failure.
//
// The counts here would exempt both survivors if the error were ignored, which
// is the point: the error dominates the measurement. A future baseSurvivorCounts
// that returned partial counts alongside a late failure — five identities
// measured, the sixth unparseable — must not leak those five into an exemption.
// The base either answered or it did not.
func TestFC1_ABaseThatCouldNotAnswerExemptsNothing(t *testing.T) {
	moved, fresh := twoSurvivors(t)
	survivors := []Guard{moved, fresh}
	generous := map[Identity]int{moved.Identity(): 1, fresh.Identity(): 1}

	pre, introduced := ClassifySurvivors(survivors, generous, errBaseSilent)

	if len(pre) != 0 {
		t.Errorf("the base failed and %d survivor(s) were exempted anyway; "+
			"an unanswerable base certifies nothing", len(pre))
	}
	if len(introduced) != len(survivors) {
		t.Fatalf("the base failed and %d of %d survivors were reported as introduced; "+
			"want all of them", len(introduced), len(survivors))
	}
	for i, g := range introduced {
		if g.Identity() != survivors[i].Identity() {
			t.Errorf("survivor %d came back as %v, want %v", i, g.Identity(), survivors[i].Identity())
		}
	}
}

// FC2: a base tree that could not answer still fails the run.
//
// FC1 pins the classification; this pins what the classification costs. The
// two are separate because a reviewer could classify every survivor as
// introduced and still exit 0, and that is precisely the silent fail-open this
// file exists to catch.
func TestFC2_ABaseThatCouldNotAnswerStillFailsTheRun(t *testing.T) {
	moved, fresh := twoSurvivors(t)
	generous := map[Identity]int{moved.Identity(): 1, fresh.Identity(): 1}

	_, introduced := ClassifySurvivors([]Guard{moved, fresh}, generous, errBaseSilent)

	if got := ExitCode(introduced, nil, nil); got != 1 {
		t.Fatalf("the base could not answer, two survivors went unexempted, and the run "+
			"exited %d; want 1", got)
	}
}

// FC3: when the base does answer, only the guards it measured are exempted.
//
// The exemption is earned per guard. One identity the base measured as
// surviving there, one it did not: the first is pre-existing, the second is
// this change's. PairSurvivors already owns the pairing arithmetic and its
// edges (PX5, PX6); this checks only that ClassifySurvivors hands the answer to
// it rather than inventing one.
func TestFC3_OnlyTheGuardsTheBaseMeasuredAreExempted(t *testing.T) {
	moved, fresh := twoSurvivors(t)

	pre, introduced := ClassifySurvivors([]Guard{moved, fresh},
		map[Identity]int{moved.Identity(): 1}, nil)

	if len(pre) != 1 || pre[0].Identity() != moved.Identity() {
		t.Fatalf("the base measured one survivor and %d were exempted (%s); "+
			"want just %s", len(pre), funcsOf(pre), moved.Func)
	}
	if len(introduced) != 1 || introduced[0].Identity() != fresh.Identity() {
		t.Fatalf("the base measured nothing for %s and %d were introduced (%s); "+
			"want just %s", fresh.Func, len(introduced), funcsOf(introduced), fresh.Func)
	}
}

// FC4: a run with nothing left unmeasured is the only run that passes.
//
// The zero case has to be reachable, or ExitCode is a constant 1 and the tests
// above prove nothing. This is the other half of the assertion.
func TestFC4_ARunWithNothingLeftUnmeasuredPasses(t *testing.T) {
	if got := ExitCode(nil, nil, nil); got != 0 {
		t.Fatalf("nothing introduced, nothing unchecked, nothing unbuilt, and the run "+
			"exited %d; want 0", got)
	}
}

// FC5: a guard nothing could measure fails the run on its own.
//
// UNCHECKED is a mutant the compiler rejected and UNBUILT is a file this host
// does not compile. Neither is a pass; both are absences of measurement, and a
// run that exited 0 on them would print a clean line over guards nothing looked
// at. Each category is tried alone, because an exit rule that only consulted
// `introduced` would still satisfy FC2.
func TestFC5_AGuardNothingCouldMeasureFailsTheRunOnItsOwn(t *testing.T) {
	moved, _ := twoSurvivors(t)
	for name, code := range map[string]int{
		"a mutant the compiler rejected":    ExitCode(nil, []Guard{moved}, nil),
		"a file this host does not compile": ExitCode(nil, nil, []Guard{moved}),
	} {
		if code != 1 {
			t.Errorf("%s alone exited %d; want 1", name, code)
		}
	}
}
