package tui

import (
	"bytes"
	"strings"
	"testing"
)

// A screen must render the same whether you are looking at it or piping it.
//
// It did not. Every pad helper built exactly BudgetRows lines; the loop then
// trimmed to BudgetRows-1 to make room for the footer it appends. So in a live
// terminal the last body line was cut — and the last body line is
//
//	copy it and you never need this screen again.
//
// the line the design calls the cheapest honesty in it, sitting under `ran
// replay <cmd>`. Down a pipe, `--once` does not trim, so the same screen came
// out one row taller with the line intact. Same screen, two endings, decided by
// how the reader happened to be looking at it, and the committed screen images
// are made through the pipe — so the images showed a line no live user ever saw.

// screens is one of each pad helper's output, named so a failure says which.
func padded(t *testing.T) map[string]Screen {
	t.Helper()
	m := aMachine()
	m.CostReady = true
	m.Tasks, m.TotalUSD, m.MedianUSD, m.P90USD = 3, 12.5, 2.0, 6.0
	m.TaskRows = []Task{{Session: "abcd1234", Model: "opus-5", CostUSD: 9.0, Path: "/c/a.jsonl"}}
	task := m.TaskRows[0]
	return map[string]Screen{
		"doctor": DoctorScreen(m),
		"cost":   CostScreen(m, 0, Selection{Window: 6}),
		"why": WhyScreen(&task, func(string) (string, error) {
			return "  a line of blame output", nil
		}),
	}
}

// RB1: a body leaves the loop room for the footer.
func TestABodyLeavesRoomForTheFooter(t *testing.T) {
	for name, sc := range padded(t) {
		if len(sc.Lines) != BudgetRows-1 {
			t.Errorf("%s is %d rows; the frame is %d and the loop appends the footer, "+
				"so a body of %d loses its last line", name, len(sc.Lines), BudgetRows, BudgetRows)
		}
	}
}

// RB2: and the line it would have lost is still there afterwards.
//
// Asserted through the loop rather than on the Screen, because the Screen is
// what was already correct — the loss happened one layer down, where nothing
// was looking.
func TestTheLastBodyLineSurvivesTheLoop(t *testing.T) {
	for name, sc := range padded(t) {
		last := ""
		for _, l := range sc.Lines {
			if strings.TrimSpace(l) != "" {
				last = strings.TrimSpace(l)
			}
		}
		if last == "" {
			t.Fatalf("%s has no non-blank last line to lose", name)
		}
		var out bytes.Buffer
		l := &Loop{Out: &out, Keys: make(chan rune), Source: func(_ rune, _ int) Frame {
			return Frame{Lines: sc.Lines}
		}}
		l.cur = sc.Key
		l.paint()
		painted := strings.Join(l.Painted(), "\n")
		if !strings.Contains(painted, last) {
			t.Errorf("%s: the loop dropped %q, which the screen ends on:\n%s",
				name, last, painted)
		}
	}
}

// RB3: the painted frame is exactly the budget, and ends with the way out.
func TestThePaintedFrameIsExactlyTheBudget(t *testing.T) {
	for name, sc := range padded(t) {
		var out bytes.Buffer
		l := &Loop{Out: &out, Keys: make(chan rune), Source: func(_ rune, _ int) Frame {
			return Frame{Lines: sc.Lines}
		}}
		l.cur = sc.Key
		l.paint()
		got := l.Painted()
		if len(got) != BudgetRows {
			t.Errorf("%s painted %d rows, budget %d", name, len(got), BudgetRows)
		}
		if !strings.Contains(got[len(got)-1], "q quit") {
			t.Errorf("%s does not end with the footer: %q", name, got[len(got)-1])
		}
	}
}
