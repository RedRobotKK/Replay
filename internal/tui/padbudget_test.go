package tui

import (
	"strings"
	"testing"
)

// Every pad helper trims an over-long body, and until now nothing watched it do
// that.
//
// The guard-reachability check neutralises every conditional a change touches
// and reports the ones no test enters. It named the truncation branch in all
// four pad helpers: screens are built well inside the budget, so the `if
// len(lines) > bodyRows()-3` arm never ran under test, and a screen that
// overflowed would have been trimmed by code nobody had ever seen work.
//
// That branch is the one that decides WHICH line disappears when a screen grows
// — and a screen growing past its budget is exactly the condition under which
// somebody is about to lose a row without being told. It is worth a test.

func overlong() []string {
	lines := make([]string, bodyRows()*2)
	for i := range lines {
		lines[i] = "  row"
	}
	return lines
}

func TestEveryPadTrimsAnOverlongBody(t *testing.T) {
	for name, got := range map[string][]string{
		"pad":      pad(overlong()),
		"padCost":  padCost(overlong()),
		"padWhy":   padWhy(overlong()),
		"padShare": padShare(overlong(), ShareState{}),
	} {
		if len(got) != bodyRows() {
			t.Errorf("%s returned %d rows for a %d-row body; the body budget is %d",
				name, len(got), bodyRows()*2, bodyRows())
		}
	}
}

// And the trim keeps the footer lines each helper appends, rather than cutting
// them off with the overflow. Those carry the "ran replay <cmd>" provenance
// line, which the design calls the cheapest honesty in it.
func TestTrimmingKeepsTheProvenanceLine(t *testing.T) {
	body := strings.Join(pad(overlong()), "\n")
	if !strings.Contains(body, "ran   replay doctor") {
		t.Errorf("an over-long doctor screen lost the line naming what it ran:\n%s", body)
	}
	cost := strings.Join(padCost(overlong()), "\n")
	if !strings.Contains(cost, "ran   replay cost") {
		t.Errorf("an over-long cost screen lost its provenance line:\n%s", cost)
	}
}

// A body already at the budget is returned unchanged, so the trim cannot be a
// branch that fires on every screen and quietly removes a row from all of them.
func TestAnExactlyFittingBodyIsNotTrimmed(t *testing.T) {
	exact := make([]string, bodyRows()-3)
	for i := range exact {
		exact[i] = "  row"
	}
	got := pad(exact)
	if len(got) != bodyRows() {
		t.Fatalf("an exactly fitting body produced %d rows, want %d", len(got), bodyRows())
	}
	for i := range exact {
		if got[i] != exact[i] {
			t.Errorf("row %d changed from %q to %q on a body that fit", i, exact[i], got[i])
		}
	}
}
