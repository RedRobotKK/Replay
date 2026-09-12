package tui

import (
	"strings"
	"testing"
)

// The screen behind "Am I about to blow a budget?" must answer that question.
//
// It did not. The key declares `serve --max-day-usd` and promises "Every guard,
// whether it is armed, and whether it can fire". What it rendered was `advise
// --guards`: caps SUGGESTED from the reader's own spread, print-only, written
// nowhere. Useful, and a different question — what a cap could be, not what a
// cap is doing.
//
// The gap that matters is proxy.Status.SpendCapNotEnforced: a dollar cap is
// configured and some traffic could not be priced, so that traffic is not
// capped at all. The operator believes they have a limit they do not have,
// which is worse than having set none — and the one screen named for the
// question could not say it. `replay doctor` could, and does, loudly.

func liveGuards() GuardState {
	return GuardState{
		Reachable: true, Addr: "127.0.0.1:4000",
		Caps:     Caps{DayUSD: true, SessionTokens: true},
		Refused:  4,
		Refusals: map[string]int{"spend_cap": 3, "loop": 1},
		CostUSD:  12.34, DayCostUSD: 2.10,
	}
}

func someAdvice() []string {
	return []string{
		"caps from your own spread, Tukey's upper fence (Q3 + 1.5*IQR):",
		"  --max-session-usd 12.34",
		"    Q1 $1.00, median $2.00, Q3 $6.00, IQR $5.00, over 20 sessions",
		"Above the fence is an outlier against your own history, not a rule.",
		"Nothing is written: pass these yourself if you want them.",
		"  replay serve --max-session-usd 12.34",
	}
}

// G1: what is armed is reported as armed.
func TestGuardsReportsWhichCapsAreSet(t *testing.T) {
	body := GuardsScreen(liveGuards(), someAdvice(), 20).String()
	for _, want := range []string{"day $", "session tokens"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not say %q is set:\n%s", want, body)
		}
	}
}

// G2: a cap that cannot fire is the loudest thing on the screen.
func TestGuardsSaysWhenADollarCapIsNotBeingEnforced(t *testing.T) {
	g := liveGuards()
	g.SpendCapNotEnforced = true
	body := GuardsScreen(g, someAdvice(), 20).String()
	if !strings.Contains(body, "NOT enforced") {
		t.Errorf("a dollar cap that is not being applied is not reported:\n%s", body)
	}
	if !Urgent(GuardsScreen(g, someAdvice(), 20).Lines) {
		t.Errorf("the warning is not marked urgent, so it reads as information:\n%s", body)
	}
	// And it must say WHY, because "not enforced" without the mechanism is a
	// status a reader cannot act on.
	if !strings.Contains(body, "priced") {
		t.Errorf("the warning does not name the mechanism (unpriced traffic):\n%s", body)
	}
}

// G3: with no proxy answering, the screen does not report "no guards".
//
// Absence, zero and unknown are three values (ADR-0018). "Nothing has been
// refused" and "nothing was asked" are different sentences, and on this screen
// the first one is the dangerous one to print by mistake.
func TestGuardsDoesNotReadSilenceAsSafety(t *testing.T) {
	body := GuardsScreen(GuardState{}, someAdvice(), 20).String()
	if strings.Contains(body, "0 refused") || strings.Contains(body, "no request has been refused") {
		t.Errorf("an unreachable proxy was reported as a clean record:\n%s", body)
	}
	if !strings.Contains(body, "replay serve") {
		t.Errorf("the screen does not say how to get a guard running:\n%s", body)
	}
}

// G4: refusals are named by guard.
//
// A count says something is happening; the name says which limit the agent is
// hitting, and that is the part an operator changes.
func TestGuardsNamesWhichGuardRefused(t *testing.T) {
	body := GuardsScreen(liveGuards(), someAdvice(), 20).String()
	for _, want := range []string{"spend_cap", "loop"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen counts refusals without naming %q:\n%s", want, body)
		}
	}
}

// G5: a suggestion is never presented as a setting.
//
// Both belong on this screen and they are opposite claims: one is what the
// proxy is doing, the other is what it could do. A reader who confuses them
// believes they are protected by a number nobody applied.
func TestGuardsSeparatesSuggestionsFromSettings(t *testing.T) {
	sc := GuardsScreen(liveGuards(), someAdvice(), 20)
	body := sc.String()
	low := strings.ToLower(body)
	iEnforced := strings.Index(low, "enforcing")
	iSuggested := strings.Index(low, "suggested")
	if iEnforced < 0 || iSuggested < 0 {
		t.Fatalf("the screen does not label both halves:\n%s", body)
	}
	if iSuggested < iEnforced {
		t.Error("the suggestion is printed above what is actually enforced")
	}
	if !strings.Contains(body, "Nothing here is written") {
		t.Errorf("the screen does not say the suggestion is not applied:\n%s", body)
	}
}

// G6: and it fits, in every state.
func TestGuardsFitsTheBudget(t *testing.T) {
	loud := liveGuards()
	loud.SpendCapNotEnforced = true
	for name, g := range map[string]GuardState{
		"live": liveGuards(), "loud": loud, "unreachable": {},
	} {
		sc := GuardsScreen(g, someAdvice(), 20)
		if len(sc.Lines) > bodyRows() {
			t.Errorf("%s: %d rows, body budget %d", name, len(sc.Lines), bodyRows())
		}
	}
}

// G7: no corpus is still no corpus. The suggestion needs a spread.
func TestGuardsWithNoSessionsStillReportsLiveState(t *testing.T) {
	body := GuardsScreen(liveGuards(), nil, 0).String()
	if !strings.Contains(body, "refused") {
		t.Errorf("live guard state was dropped because no transcripts were read; "+
			"the two halves have different prerequisites:\n%s", body)
	}
}
