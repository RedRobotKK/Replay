package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/proxy"
)

// The guards are the proxy's strongest unsold feature: the engineering exists,
// is tested, and `doctor` never mentioned it. A user could not find out from
// the tool that a guard had fired, or that one they configured was not being
// applied.
func TestDoctorNamesTheGuardsThatFired(t *testing.T) {
	got := strings.Join(guardLines(proxy.Status{
		Requests: map[string]int{"2xx": 400, "refused": 12},
		Refusals: map[string]int{"spend_cap": 8, "loop": 4},
	}), "\n")
	for _, want := range []string{"12", "spend_cap", "8", "loop", "4"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

// CapNotEnforced is good design that nothing printed. It means a dollar cap is
// set and some traffic could not be priced, so that traffic is uncapped: the
// user believes they have a limit they do not have. It has to be loud.
func TestDoctorShoutsWhenADollarCapIsNotEnforced(t *testing.T) {
	got := strings.Join(guardLines(proxy.Status{
		Requests:            map[string]int{"2xx": 10},
		SpendCapNotEnforced: true,
	}), "\n")
	if !strings.Contains(got, "WARNING") {
		t.Fatalf("an unenforced cap must be loud, got:\n%s", got)
	}
	if !strings.Contains(got, "not being applied") {
		t.Fatalf("the warning must say what is actually wrong, got:\n%s", got)
	}
	// Naming the flag beats describing it. The reader is being told their
	// protection is not running; the next thing they need is the exact thing
	// to type, not a category of thing to look for.
	for _, want := range []string{"--max-day-tokens", "--max-session-tokens", "replay rules --update"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the warning must name %q so it can be acted on, got:\n%s", want, got)
		}
	}
	// And it must say why tokens work where dollars do not, or the advice
	// reads as an arbitrary substitution.
	if !strings.Contains(got, "whether or not") && !strings.Contains(got, "regardless") {
		t.Fatalf("the warning must say why a token cap still applies, got:\n%s", got)
	}
}

// Silence is a report too: a user who has configured guards wants to know they
// are on and quiet, not to wonder whether the block is missing.
func TestDoctorSaysSoWhenNoGuardHasFired(t *testing.T) {
	got := strings.Join(guardLines(proxy.Status{Requests: map[string]int{"2xx": 10}}), "\n")
	if got == "" {
		t.Fatal("the guards block must print even when nothing has fired")
	}
	if strings.Contains(got, "WARNING") {
		t.Fatalf("nothing is wrong here, so nothing should shout:\n%s", got)
	}
}

// A guard that fired but has no per-kind breakdown must still be reported
// rather than dropped for lacking detail.
func TestRefusalsWithoutABreakdownAreStillReported(t *testing.T) {
	got := strings.Join(guardLines(proxy.Status{Requests: map[string]int{"refused": 3}}), "\n")
	if !strings.Contains(got, "3") {
		t.Fatalf("a refusal count with no breakdown was dropped:\n%s", got)
	}
}
