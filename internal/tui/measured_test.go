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

func aCountedMachine() Machine {
	m := aMachine()
	m.CostReady = true
	m.Tasks, m.TotalUSD, m.MedianUSD, m.P90USD = 1599, 3102.84, 0.63, 2.14
	m.AvoidableUSD, m.AvoidableShare, m.AvoidableTokens = 152.88, 0.05, 41_200_000
	m.PriceDate, m.CorpusFiles = "2026-06-24", 1614
	// Task rows, because without them the screen takes the no-breakdown path
	// and the assertions below measure a branch nobody sees.
	m.TaskRows = []Task{
		{Session: "facfd32e", Model: "claude-opus-5", CostUSD: 286.59, Breaks: 4,
			Path: "/p/facfd32e.jsonl"},
		{Session: "fee79714", Model: "claude-opus-5", CostUSD: 236.26, Breaks: 6,
			Path: "/p/fee79714.jsonl"},
		{Session: "eec05948", Model: "claude-opus-5", CostUSD: 221.53, Breaks: 1},
	}
	return m
}

// Counting is not the same as nothing, and the screen says which.
//
// A corpus of a thousand transcripts takes seconds to walk. A surface showing
// zeros until it finished would be showing a wrong number rather than a pending
// one, and zero dollars is a number somebody will believe.
func TestCostSaysItIsStillCountingRatherThanShowingZero(t *testing.T) {
	m := aMachine() // Found, but not counted yet
	sc := CostScreen(m, 0, Selection{Window: 6})
	body := strings.Join(sc.Lines, "\n")
	if strings.Contains(body, "$0.00") {
		t.Errorf("showed a zero total while still counting. That is a wrong figure, "+
			"not a pending one:\n%s", body)
	}
	if !strings.Contains(body, "waiting") {
		t.Errorf("did not mark the fields as waiting:\n%s", body)
	}
	if sc.From == Measured {
		t.Error("a screen that has not finished counting declared itself measured")
	}
}

// The cue moves while it counts, or the screen is indistinguishable from stuck.
func TestCostScreenIsAliveWhileItCounts(t *testing.T) {
	m := aMachine()
	a := strings.Join(CostScreen(m, 0, Selection{Window: 6}).Lines, "\n")
	b := strings.Join(CostScreen(m, 1, Selection{Window: 6}).Lines, "\n")
	if a == b {
		t.Error("the counting screen renders identically at two consecutive ticks. " +
			"Somebody waiting on a slow walk cannot tell it from a hang")
	}
}

// Once counted it is measured, carries no notice, and shows the real figures.
func TestCostScreenIsMeasuredOnceCounted(t *testing.T) {
	sc := CostScreen(aCountedMachine(), 0, Selection{Window: 6})
	if sc.From != Measured {
		t.Errorf("a counted corpus declares itself %v", sc.From)
	}
	if Marked(sc.Lines) {
		t.Error("a measured cost screen carries a provenance notice")
	}
	body := strings.Join(sc.Lines, "\n")
	for _, want := range []string{"$3102.84", "$0.63", "$2.14", "$152.88", "2026-06-24"} {
		if !strings.Contains(body, want) {
			t.Errorf("does not show %q, which it was given:\n%s", want, body)
		}
	}
}

// The dollars are not the reader's bill, and the screen says so.
//
// costSummary's own comment: the dollar figure is meaningless to a flat-seat
// subscriber, who is not billed per token and is most of the readership. The
// tokens are what they actually lost. A screen that leads with dollars and
// never says that is telling most of its readers a number about somebody else.
func TestCostScreenSaysDollarsAreNotYourBill(t *testing.T) {
	body := strings.Join(CostScreen(aCountedMachine(), 0, Selection{Window: 6}).Lines, "\n")
	if !strings.Contains(body, "not your bill") {
		t.Errorf("does not say the dollars are list price rather than an invoice:\n%s", body)
	}
	if !strings.Contains(body, "41,200,000") {
		t.Errorf("does not give the waste in tokens, which is what a flat-seat reader "+
			"actually lost:\n%s", body)
	}
}

func TestCostScreenFitsTheBudget(t *testing.T) {
	for _, m := range []Machine{aCountedMachine(), aMachine(), {ProjectsDir: "~/x"}} {
		sc := CostScreen(m, 0, Selection{Window: 6})
		if len(sc.Lines) > BudgetRows {
			t.Errorf("%d rows, budget %d", len(sc.Lines), BudgetRows)
		}
		for i, l := range sc.Lines {
			if len(l) > BudgetCols {
				t.Errorf("line %d is %d columns:\n%s", i, len(l), l)
			}
		}
	}
}
