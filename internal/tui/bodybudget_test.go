package tui

import (
	"strings"
	"testing"
)

// The budget is checked against the body a screen BUILT, not the body it
// shipped, and the difference is the whole point of this file.
//
// pad(), padCost(), padWhy() and padShare() each fill a short body and trim a
// long one, so Screen.Lines is bodyRows()-3 whatever went in. RB1 in
// rowbudget_test.go asserts on exactly that length. It is a true statement
// about a value that is constant by construction: a screen that overflowed and
// silently lost its last row passes it identically to one that fitted with
// room to spare. ADR-0014 calls that a check that cannot fail, and this one sat
// in the file named for the budget.
//
// It cost a red main. #157 and #160 each added one row to the doctor screen.
// Each respected the budget alone. Together the screen needed 21 rows against
// 20, pad() cut the last one, every test stayed green, and the row that went
// out of frame was noticed by a person looking at a terminal.
//
// Screen.BodyRows records what was built. This file compares it against the
// frame.

// fullMachine is a doctor screen with every optional row present at once.
//
// That is not a pessimistic invention: it is the shape #157 and #160 produced
// between them. Each added one conditional row, each tested its own row, and
// nothing built a machine that carried both. A worst case assembled by hand is
// the only thing that would have caught it, so it is assembled here and the
// comment says why, because a later reader will otherwise trim it back toward
// "realistic".
func fullMachine() Machine {
	m := aMachine()
	// Every count wide enough to wrap nothing but long enough to be real.
	m.Sessions, m.Transcripts, m.Lanes, m.Projects = 148_302, 1_982_114, 21_408, 1_284
	m.ProjectsDir = "~/Library/Application Support/some-client/projects/deeply/nested"
	m.LedgerDir, m.LedgerWritable = "~/Library/Application Support/replay/ledger", false
	// A stale price table and a stale rules document both add a note.
	m.PriceTableDate, m.PriceAgeDays = "2025-11-02", 313
	m.RulesVersion, m.RulesState, m.RulesAgeDays = "2025-10-14", RulesFetched, 331
	m.Readings, m.Models = 1, 1
	m.Found = true
	return m
}

// TestBB1_NoScreenBuildsMoreBodyThanTheFrameHolds is the guard the row budget
// needed.
//
// It fails when a screen's body exceeds what pad can hold, which is the moment
// a row starts disappearing — not later, when somebody notices one is missing.
func TestBB1_NoScreenBuildsMoreBodyThanTheFrameHolds(t *testing.T) {
	m := fullMachine()
	task := Task{
		Session: "9f2c5be1", Model: "claude-opus-5", CostUSD: 412.88,
		Path: "~/Library/Application Support/some-client/projects/a/very/long/transcript.jsonl",
	}
	m.TaskRows = []Task{task, task, task, task, task, task, task, task}
	m.CostReady = true
	m.Tasks, m.TotalUSD, m.MedianUSD, m.P90USD = len(m.TaskRows), 3_302.57, 88.12, 411.90

	screens := map[string]Screen{
		"doctor": DoctorScreen(m),
		"cost":   CostScreen(m, 0, Selection{Window: 6}),
		"why": WhyScreen(&task, func(string) (string, error) {
			return strings.Repeat("  a line of blame output\n", 40), nil
		}),
	}

	frame := bodyRows() - 3
	for name, sc := range screens {
		if sc.BodyRows == 0 {
			t.Errorf("%s reports no body length, so this test is not looking at it. "+
				"Every padded Screen literal sets BodyRows: len(lines).", name)
			continue
		}
		if sc.BodyRows > frame {
			t.Errorf("the %s screen built %d body rows and the frame holds %d, so %d "+
				"row(s) were cut from the bottom in silence.\n"+
				"This is what #157 and #160 did together. Take a row out, or raise "+
				"the budget deliberately — do not let the trim decide.",
				name, sc.BodyRows, frame, sc.BodyRows-frame)
		}
	}
}

// TestBB2_TheFrameIsStillTheFrame keeps BB1 honest.
//
// BB1 compares a built body against bodyRows()-3. If a later change altered how
// much pad actually keeps, BB1 would go on comparing against a number that no
// longer describes the frame, and would pass while rows vanished again. This
// asserts the two agree by measuring what pad returns for a body of exactly
// the size BB1 permits.
func TestBB2_TheFrameIsStillTheFrame(t *testing.T) {
	frame := bodyRows() - 3
	body := make([]string, frame)
	for i := range body {
		body[i] = "  row"
	}
	got := pad(body)

	kept := 0
	for _, l := range got {
		if l == "  row" {
			kept++
		}
	}
	if kept != frame {
		t.Fatalf("pad kept %d of %d rows that BB1 treats as fitting, so BB1 is "+
			"comparing against the wrong number", kept, frame)
	}
}
