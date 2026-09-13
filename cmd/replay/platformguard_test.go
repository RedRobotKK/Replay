package main

import (
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

// PG2: the refusal is reachable from the entry point rather than defined and
// forgotten.
//
// A guard nothing calls is the same defect one level up. This asserts the
// string the entry point would print is the one platformRefusal produces, so
// deleting the call site fails here and not only on a Windows runner nobody
// watches.
func TestPG2_TheEntryPointConsultsTheGuard(t *testing.T) {
	// Drive the real entry point, with the cheapest subcommand there is, so the
	// assertion is about the code path a user takes rather than about a variable
	// this test set itself.
	refusalCheckedAtEntry = nil
	var out, errOut strings.Builder
	_ = run([]string{"version"}, &out, &errOut)

	if refusalCheckedAtEntry == nil {
		t.Fatal("run() did not consult platformRefusal, so the refusal cannot fire and a " +
			"Windows build would proceed to write a ledger and a masking vault into a " +
			"directory ownerdir declines to verify")
	}
	if got, want := refusalCheckedAtEntry(), platformRefusal(); got != want {
		t.Errorf("the entry point sees %q and the guard says %q", got, want)
	}
}
