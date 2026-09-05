package main

import (
	"encoding/json"
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

// Telling somebody to add a token cap they already have is noise, and noise in
// a warning is how a warning stops being read. When a dollar cap is blind but a
// token cap is running, the token cap is already catching the runaway loop the
// warning is about, and the honest advice is different.
func TestTheWarningKnowsWhenATokenCapAlreadyCoversIt(t *testing.T) {
	blindWithTokens := proxy.Status{
		Requests:            map[string]int{"2xx": 10},
		SpendCapNotEnforced: true,
		Caps:                proxy.CapStatus{DayUSD: true, DayTokens: true},
	}
	got := strings.Join(guardLines(blindWithTokens), "\n")
	if !strings.Contains(got, "WARNING") {
		t.Fatalf("the dollar cap is still blind, so it still warns:\n%s", got)
	}
	if strings.Contains(got, "Cap tokens as well") {
		t.Fatalf("a token cap is already running; do not tell them to add one:\n%s", got)
	}
	if !strings.Contains(got, "already") {
		t.Fatalf("it must say the token cap is covering this:\n%s", got)
	}

	// And with no token cap, the ten-second fix is still the first thing said.
	blindNoTokens := proxy.Status{
		Requests:            map[string]int{"2xx": 10},
		SpendCapNotEnforced: true,
		Caps:                proxy.CapStatus{DayUSD: true},
	}
	got = strings.Join(guardLines(blindNoTokens), "\n")
	if !strings.Contains(got, "--max-day-tokens") {
		t.Fatalf("with no token cap the flag must be named:\n%s", got)
	}
}

// Which caps are configured has to reach the doctor, which reads the status
// endpoint over HTTP and cannot see serve's flags.
func TestStatusReportsWhichCapsAreConfigured(t *testing.T) {
	var st proxy.Status
	if err := json.Unmarshal([]byte(`{"caps":{"day_usd":true,"session_tokens":true}}`), &st); err != nil {
		t.Fatal(err)
	}
	if !st.Caps.DayUSD || !st.Caps.SessionTokens {
		t.Fatalf("caps did not survive the wire: %+v", st.Caps)
	}
	if st.Caps.DayTokens || st.Caps.SessionUSD {
		t.Fatalf("unset caps must stay false: %+v", st.Caps)
	}
}
