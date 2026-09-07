package tui

import (
	"strings"
	"testing"
)

func aMachine() Machine {
	return Machine{
		Transcripts: 1599, Lanes: 214, Projects: 12,
		ProjectsDir: "~/.claude/projects",
		LedgerDir:   "~/.replay/ledger", LedgerWritable: true,
		PriceTableDate: "2026-06-24", PriceAgeDays: 75,
		Readings: 4, Models: 4,
		Found: true,
	}
}

// A measured screen carries no notice, and every figure on it is one it was
// given.
//
// The rule this file exists to keep: Measured means every number came from
// somewhere a reader could go and check. A screen that measures three and
// invents a fourth is an example screen, because the notice is what tells a
// reader how much to trust and a partial one tells them wrong.
func TestDoctorScreenIsMeasuredAndUnmarked(t *testing.T) {
	sc := DoctorScreen(aMachine())
	if sc.From != Measured {
		t.Errorf("a screen built from a real walk declares itself %v", sc.From)
	}
	if Marked(sc.Lines) {
		t.Error("a measured screen carries a provenance notice. Stamping real data " +
			"teaches the reader to skim the stamp, and then it stops working where " +
			"it matters")
	}
	body := strings.Join(sc.Lines, "\n")
	for _, want := range []string{"1,599", "12", "214", "2026-06-24"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not show %q, which it was given:\n%s", want, body)
		}
	}
}

// Nothing readable is a different answer from zero, and is rendered as one.
//
// A doctor screen reporting "0 transcripts" on a machine it could not read is
// the null-is-not-zero defect wearing a number, and this screen is the one
// somebody opens precisely when nothing is working.
func TestDoctorSaysNotFoundRatherThanZero(t *testing.T) {
	sc := DoctorScreen(Machine{ProjectsDir: "~/.claude/projects"})
	if sc.From != Unavailable {
		t.Errorf("an unreadable machine declares itself %v, want Unavailable", sc.From)
	}
	if !Marked(sc.Lines) {
		t.Error("an unreadable machine does not say so on screen")
	}
	body := strings.Join(sc.Lines, "\n")
	if strings.Contains(body, "0 transcripts") || strings.Contains(body, "0 files") {
		t.Errorf("reported nothing-found as a count of zero:\n%s", body)
	}
	if !strings.Contains(body, "REPLAY_TRANSCRIPTS") {
		t.Errorf("did not say how to point it at a corpus, which is the one thing "+
			"somebody on this screen needs:\n%s", body)
	}
}

// The caveats fire on the conditions they describe, not always.
//
// A note that is always present is a note nobody reads, which is how the
// warning that matters gets skipped.
func TestDoctorCaveatsAreConditional(t *testing.T) {
	fresh := aMachine()
	fresh.PriceAgeDays = 3
	fresh.Readings, fresh.Models = 40, 4
	body := strings.Join(DoctorScreen(fresh).Lines, "\n")
	if strings.Contains(body, "days old: figures are list price") {
		t.Error("a three-day-old price table was flagged as stale")
	}
	if strings.Contains(body, "no within-model variance") {
		t.Error("forty readings across four models was reported as one reading each")
	}

	stale := aMachine()
	body = strings.Join(DoctorScreen(stale).Lines, "\n")
	if !strings.Contains(body, "no within-model variance") {
		t.Error("four readings across four models is one each, and the screen did not " +
			"say so. That is the whole reason the routing bands cannot narrow")
	}
}

// It fits, like everything else.
func TestDoctorScreenFitsTheBudget(t *testing.T) {
	for _, m := range []Machine{aMachine(), {ProjectsDir: "~/.claude/projects"}} {
		sc := DoctorScreen(m)
		if len(sc.Lines) > BudgetRows {
			t.Errorf("%d rows, budget %d", len(sc.Lines), BudgetRows)
		}
		for i, l := range sc.Lines {
			if len(l) > BudgetCols {
				t.Errorf("line %d is %d columns, budget %d:\n%s", i, len(l), BudgetCols, l)
			}
		}
	}
}
