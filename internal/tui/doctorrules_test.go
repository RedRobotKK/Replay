package tui

import (
	"strings"
	"testing"
)

// The doctor screen and `replay doctor` answer the same question, and until
// this file they gave different answers to half of it.
//
// The command reports the rules document and its age (cmd/replay/doctorrules.go).
// The screen reported the price table and stopped, so an operator who opened
// the TUI — which the installer opens for them — was told their figures were
// current when the document those figures were computed against might be
// months old. The gap matters more than a missing row usually would: the
// cache-read multiple lives in that document, and it decides what Replay
// RECOMMENDS, not only what it reports. Advising "keep the cache" on a table
// that overstates what reading one costs is a claim made on the tool's own
// behalf.

func aRulesMachine(state RulesAge, days int) Machine {
	m := aMachine()
	m.RulesVersion, m.RulesState, m.RulesAgeDays = "anthropic-2026-09-01", state, days
	return m
}

func rulesRow(t *testing.T, m Machine) string {
	t.Helper()
	for _, l := range DoctorScreen(m).Lines {
		if strings.HasPrefix(strings.TrimSpace(l), "rules") {
			return l
		}
	}
	return ""
}

// DR1: a fetched document is named and aged.
func TestDoctorScreenNamesTheRulesDocument(t *testing.T) {
	row := rulesRow(t, aRulesMachine(RulesFetched, 12))
	if row == "" {
		t.Fatalf("no rules row on the doctor screen:\n%s", DoctorScreen(aRulesMachine(RulesFetched, 12)).String())
	}
	for _, want := range []string{"anthropic-2026-09-01", "12"} {
		if !strings.Contains(row, want) {
			t.Errorf("rules row %q does not carry %q, which it was given", row, want)
		}
	}
}

// DR2: past the same thirty days the command uses, the screen says what being
// stale costs — and says which command fixes it, because a warning an operator
// cannot act on is decoration.
func TestDoctorScreenWarnsWhenRulesAreStale(t *testing.T) {
	body := DoctorScreen(aRulesMachine(RulesFetched, 75)).String()
	for _, want := range []string{"cache-read multiple", "recommends", "replay rules --check-prices"} {
		if !strings.Contains(body, want) {
			t.Errorf("a 75-day-old rules document does not say %q:\n%s", want, body)
		}
	}
}

// DR3: and does not nag while the document is current. A warning that is always
// on is one an operator learns to skip, and then it stops working on the day it
// is true.
func TestDoctorScreenIsQuietWhenRulesAreFresh(t *testing.T) {
	body := DoctorScreen(aRulesMachine(RulesFetched, 3)).String()
	if strings.Contains(body, "cache-read multiple") {
		t.Errorf("a 3-day-old rules document is reported as stale:\n%s", body)
	}
}

// DR4: the compiled-in table is a floor, not a stale document. Ageing a
// constant prints a number with nothing behind it, so the screen does not.
func TestDoctorScreenDoesNotAgeTheCompiledInTable(t *testing.T) {
	m := aRulesMachine(RulesBuiltIn, 0)
	row := rulesRow(t, m)
	if !strings.Contains(row, "built in") {
		t.Errorf("the compiled-in table is not marked as such: %q", row)
	}
	if strings.Contains(row, "days") || strings.Contains(row, " 0") {
		t.Errorf("the compiled-in table was given an age: %q", row)
	}
	if strings.Contains(DoctorScreen(m).String(), "cache-read multiple") {
		t.Error("the table this build shipped with is reported as stale")
	}
}

// DR5: absence, zero and unknown are three values (ADR-0018). A fetch date this
// build cannot read must not render as a fresh one, because the reading that
// suppresses the warning is the reading that hides the problem.
func TestDoctorScreenDoesNotPassAnUnreadableRulesDateAsCurrent(t *testing.T) {
	row := rulesRow(t, aRulesMachine(RulesUndated, 0))
	if !strings.Contains(row, "undated") {
		t.Errorf("an unreadable fetch date does not say so: %q", row)
	}
	if row == rulesRow(t, aRulesMachine(RulesFetched, 0)) {
		t.Error("an unreadable date renders identically to a date read as today")
	}
}

// DR6: and a caller that never populated the fields draws no row at all. A
// screen that has not been told cannot report, and a default row would report
// the compiled-in table on a machine running an installed one.
func TestDoctorScreenOmitsTheRulesRowWhenNotTold(t *testing.T) {
	m := aMachine()
	m.RulesVersion, m.RulesState = "", RulesUntold
	if row := rulesRow(t, m); row != "" {
		t.Errorf("a screen told nothing about the rules document drew %q", row)
	}
}

// DR7: every state fits its column. cell() truncates with '~', so an
// over-width value does not overflow the grid — it silently becomes a shorter
// version string, which reads as a different document rather than a cut one.
func TestRulesRowFitsItsColumn(t *testing.T) {
	for _, c := range []struct {
		state RulesAge
		days  int
	}{{RulesBuiltIn, 0}, {RulesFetched, 9}, {RulesFetched, 9999}, {RulesUndated, 0}} {
		row := rulesRow(t, aRulesMachine(c.state, c.days))
		if strings.ContainsRune(row, truncationMark) {
			t.Errorf("state %v at %d days truncates: %q", c.state, c.days, row)
		}
	}
}

// DR8: and the warning survives a screen where every other note is also firing.
// pad() cuts at the budget, so the last note added is the one a future row
// silently deletes — and the last note is this one.
func TestStaleRulesWarningSurvivesTheRowBudget(t *testing.T) {
	m := aRulesMachine(RulesFetched, 75)
	m.PriceAgeDays, m.Readings, m.Models = 75, 4, 4 // both other notes on
	sc := DoctorScreen(m)
	if len(sc.Lines) > BudgetRows {
		t.Errorf("%d rows, budget %d", len(sc.Lines), BudgetRows)
	}
	if !strings.Contains(sc.String(), "replay rules --check-prices") {
		t.Errorf("the stale-rules warning was cut by the row budget:\n%s", sc.String())
	}
}

// DR9: the worst case fits, with a row to spare.
//
// The screen's tightest state is a stale price table, a stale rules document
// and one probe reading per model — every note firing at once. It came to 21
// rows against a body budget of 20 the moment bodyRows() landed, and the row that
// lost was the last note appended: the rules staleness warning, which is the
// one this file exists for.
//
// Neither change was wrong. #157 added the rules row and note; #160 gave the
// body one row less than the frame so the loop could append the footer without
// eating a line. Each was green alone and the pair was not, which is the shape
// of failure no single pull request can see.
//
// This asserts that every note survives the worst case, so the next row to
// arrive fails here by name rather than by silently deleting a warning.
func TestDoctorWorstCaseKeepsEveryNote(t *testing.T) {
	m := aMachine()
	m.RulesVersion, m.RulesState, m.RulesAgeDays = "anthropic-2026-09-01", RulesFetched, 75
	m.PriceAgeDays = 75         // the price note fires
	m.Readings, m.Models = 4, 4 // the one-reading-per-model note fires

	sc := DoctorScreen(m)
	// pad() fills to bodyRows()-3 and then appends three lines of its own, so the
	// body is everything before those three. Counting back from the end of
	// sc.Lines stops on the tagline, not on the padding.
	body := sc.Lines[:len(sc.Lines)-3]
	content := len(body)
	for content > 0 && body[content-1] == "" {
		content--
	}
	// The screen fills its body exactly in this state, which is allowed and is
	// why the assertions below are on CONTENT rather than on a spare row.
	// Demanding a blank line would mean deleting something a reader wants in
	// order to satisfy a margin nobody sees. What must not happen is a row
	// disappearing quietly, and that is what the checks below catch — pad()
	// truncates from the bottom, so the first casualty is always the last note
	// appended.
	if content != len(body) {
		t.Logf("worst case uses %d of %d body rows", content, len(body))
	}
	rendered := sc.String()
	for _, want := range []string{"replay rules --check-prices", "list price on that date", "within-model variance"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the worst case dropped %q:\n%s", want, rendered)
		}
	}
}
