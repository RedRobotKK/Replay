package tui

import (
	"strings"
	"testing"
)

// The screen uses the terminal it was given.
//
// bodyRows was BudgetRows-1, a constant, so every screen laid out for 24 rows
// whatever the window was. Measured before the change: LINES=50 rendered 24
// rows and the remaining twenty-six were dead. A reader who makes their
// terminal taller got more nothing.
//
// This fails if the budget ever goes back to being fixed.
func TestWindow_TheBodyGrowsWithTheTerminal(t *testing.T) {
	short := withLines(t, 24, bodyRows)
	tall := withLines(t, 50, bodyRows)
	if tall <= short {
		t.Fatalf("bodyRows is %d at LINES=24 and %d at LINES=50. The budget is not "+
			"reading the terminal, so a taller window buys blank rows rather than "+
			"more of the answer.", short, tall)
	}
	if want := 23; short != want {
		t.Errorf("bodyRows at LINES=24 is %d, want %d: the piped default must stay "+
			"what it was, or every committed screen capture moves", short, want)
	}
}

// The list is what grows. Reserving the trailer first and giving the rows the
// remainder is the whole point; if the extra height went to padding instead,
// the frame would be taller and just as empty.
func TestWindow_TheExtraRowsCarryContentNotPadding(t *testing.T) {
	blanks := func(lines int) (total, blank int) {
		sc := withLinesScreen(t, lines)
		for _, l := range sc.Lines {
			total++
			if strings.TrimSpace(l) == "" {
				blank++
			}
		}
		return
	}
	t24, b24 := blanks(24)
	t50, b50 := blanks(50)
	if t50 <= t24 {
		t.Fatalf("the screen is %d lines at LINES=50 and %d at LINES=24; it did not grow", t50, t24)
	}
	if b50 > b24+2 {
		t.Errorf("blank rows went from %d to %d as the window grew from %d to %d lines. "+
			"The extra height is being padded, not used.", b24, b50, t24, t50)
	}
}

func withLines(t *testing.T, n int, f func() int) int {
	t.Helper()
	t.Setenv("LINES", itoa(n))
	return f()
}

func withLinesScreen(t *testing.T, n int) Screen {
	t.Helper()
	t.Setenv("LINES", itoa(n))
	m := fullMachine()
	task := Task{Session: "9f2c5be1", Model: "claude-opus-5", CostUSD: 412.88}
	// Enough rows that the list can grow into a taller window. With a short
	// list there is nothing to grow and the test would pass on an empty frame.
	for i := 0; i < 60; i++ {
		m.TaskRows = append(m.TaskRows, task)
	}
	m.CostReady = true
	m.Tasks, m.TotalUSD, m.MedianUSD, m.P90USD = len(m.TaskRows), 3_302.57, 88.12, 411.90
	return CostScreen(m, 0, Selection{})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// A short terminal gets a bounded list, not the whole corpus.
//
// maxRows is what the window has left after the lines that must not be
// dropped, and on a small terminal that goes negative. Selection.Visible reads
// a non-positive window as "show all of them", so without a floor the screen
// asks for every task it has and lets padCost cut the result off at the
// bottom, taking the selected line and the provenance with it.
//
// Measured, not assumed: the first version of this test asserted three rows
// survive at LINES=10 and failed, because a ten-line terminal cannot hold this
// screen's header and a list at all. That is a real limit and it is stated
// here rather than papered over. Rows begin to survive at LINES=12.
func TestWindow_AShortTerminalGetsABoundedList(t *testing.T) {
	const many = 60
	sc := withLinesScreen(t, 14)

	// BodyRows is the length BEFORE padCost fits the screen to the frame, and
	// it is the only place the difference shows: padCost trims the rendered
	// Lines either way, so an unbounded list and a bounded one look identical
	// on screen while the screen that built 70 lines has silently dropped its
	// selected line and its provenance.
	if sc.BodyRows >= many {
		t.Fatalf("a 14-line terminal built a body of %d lines from %d rows. The window "+
			"budget went non-positive and Visible read that as 'all of them', so "+
			"everything below the list is now trimmed off by padCost rather than "+
			"laid out.", sc.BodyRows, many)
	}
	rows := 0
	for _, l := range sc.Lines {
		if strings.Contains(l, "9f2c5be1") {
			rows++
		}
	}
	if rows == 0 {
		t.Fatal("a 14-line terminal shows no task rows at all")
	}
}
