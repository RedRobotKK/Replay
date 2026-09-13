package main

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

// PG. Windows is unsupported, and until now that was only true in prose.
//
// The Windows CI job is green, and the reason it is green is that the promise
// it should be checking is switched off there. `internal/ownerdir` reports
// 100% statement coverage on ubuntu and 40% on windows; `modeIsChecked()`
// returns false, so `tighten` returns nil before it chmods or re-stats; and
// twenty-two tests across the ledger, the masking vault, the consent gate and
// the contributor secret skip with "Unix permission bits". That is a blocking
// green check whose greenness is produced by turning off the thing being
// checked, which is precisely what ADR-0014 exists to forbid.
//
// The gap that made it reachable: install.sh refuses Windows and then offers
// two remedies, and both were wrong. There is no Windows release archive,
// because .goreleaser.yaml builds linux and darwin only. And `go install`
// works, because the tree builds and vets clean on Windows on every push. So
// the one sentence a Windows user actually encountered handed them a one line
// path to the exact binary the project says must not ship, which would then
// write a ledger and an encrypted masking vault into a directory it had
// declined to verify was private.
//
// This is the refusal that makes the sentence true in code.

// PG1: on Windows the binary refuses before it can touch anything.
//
// PASS on windows: a refusal, naming the reason.
// PASS elsewhere: no refusal, because the promise is kept there.
// FAIL: a Windows build that proceeds, and writes secrets into a directory
// whose privacy it cannot check.
func TestPG1_WindowsRefusesAndOtherPlatformsDoNot(t *testing.T) {
	got := platformRefusal()

	if runtime.GOOS == "windows" {
		if got == "" {
			t.Fatal("a Windows build did not refuse. The ledger and the masking vault " +
				"would be written into a directory ownerdir declines to verify, and the " +
				"user was told by install.sh that go install was an acceptable route here.")
		}
		for _, want := range []string{"Windows", "ownerdir"} {
			if !strings.Contains(got, want) {
				t.Errorf("the refusal does not mention %q, so a reader cannot tell why: %q", want, got)
			}
		}
		return
	}

	if got != "" {
		t.Fatalf("%s refused, and it should not: %q", runtime.GOOS, got)
	}
}

// PG3: the refusal branch is entered, on the platform the maintainer is
// actually running.
//
// PG2 proved the call site exists. It did not prove the branch behind it is
// ever taken, and `guard reachability` said so out loud on 2026-09-13:
//
//	cmd/replay/main.go:109  if msg := platformRefusal(); msg != "" {
//	      UNREACHED: no test makes this condition true
//
// It was right, and the reason is the whole problem the guard's own comment
// names: `guard reachability` and `frozen mutants` run on ubuntu only, where
// platformRefusal returns "". So the one refusal that decides whether secrets
// get written to an unverified directory was shipping unmutated, which is the
// state this project treats as indistinguishable from having no guard at all.
//
// The fix is a seam rather than a second code path: run() consults a variable
// whose default IS platformRefusal, production never assigns it, and a test on
// any platform can make the condition true and watch what the entry point does.
func TestPG3_TheRefusalBranchIsTakenAndSaysSo(t *testing.T) {
	const notice = "this build refuses because the test said so"
	restore := platformRefusalAtEntry
	t.Cleanup(func() { platformRefusalAtEntry = restore })
	platformRefusalAtEntry = func() string { return notice }

	var out, errOut strings.Builder
	err := run([]string{"version"}, &out, &errOut)

	if err == nil {
		t.Fatal("a refusing build ran the command anyway")
	}
	if !errors.Is(err, errUnsupportedPlatform) {
		t.Errorf("refused with %v, which the exit-code table cannot classify", err)
	}
	if !strings.Contains(errOut.String(), notice) {
		t.Errorf("the reason never reached stderr, so the user is told nothing: %q", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("a refusing build still wrote to stdout, which is somebody's pipe: %q", out.String())
	}
	// Exit 1 rather than 3. ADR: exitGateBreached is the only code that may
	// block a merge, and a platform that cannot be evaluated is not a breach.
	if code := exitCode(err); blocksAMerge(code) {
		t.Errorf("an unsupported platform exits %d, which blocks a merge. A build that "+
			"could not evaluate anything must not read as a measured breach", code)
	}
}

// PG4: the seam's default is the real guard.
//
// PG3 substitutes the variable, so on its own it would pass against a binary
// whose default is a function that never refuses. This is the half that keeps a
// Windows build refusing.
func TestPG4_TheSeamDefaultsToTheRealGuard(t *testing.T) {
	if got, want := platformRefusalAtEntry(), platformRefusal(); got != want {
		t.Errorf("the entry point consults a guard that says %q while the platform's own "+
			"guard says %q", got, want)
	}
	if runtime.GOOS == "windows" && platformRefusalAtEntry() == "" {
		t.Fatal("a Windows build's entry point does not refuse")
	}
}
