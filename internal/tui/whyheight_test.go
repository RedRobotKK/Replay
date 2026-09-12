package tui

import (
	"strings"
	"testing"
)

// The why screen shows more blame on a taller terminal.
//
// blameBody capped at BudgetRows-11, a constant, so it kept thirteen lines
// however tall the window was and padWhy filled the rest with blanks. The
// output it had to show was sitting truncated while the screen was mostly
// empty.
//
// Measured with a session that HAS blame output. The demo screen has none, so
// it is blank at any height for an honest reason, and measuring that instead
// is how this looked like it had not worked.
func TestWhy_MoreOfTheBlameFitsInATallerWindow(t *testing.T) {
	task := Task{Session: "9f2c5be1", Model: "claude-opus-5", CostUSD: 412.88}
	long := strings.Repeat("  a line of blame output\n", 60)
	body := func(lines string) (int, int) {
		t.Setenv("LINES", lines)
		sc := WhyScreen(&task, func(string) (string, error) { return long, nil })
		blank := 0
		for _, l := range sc.Lines {
			if strings.TrimSpace(l) == "" {
				blank++
			}
		}
		return sc.BodyRows, blank
	}
	short, shortBlank := body("24")
	tall, tallBlank := body("50")

	if tall <= short {
		t.Fatalf("the why screen built %d body lines at LINES=24 and %d at LINES=50. "+
			"The blame output is capped by a constant again, so a taller window shows "+
			"the same thirteen lines and more blanks.", short, tall)
	}
	if tallBlank > shortBlank+2 {
		t.Errorf("blank rows went from %d to %d as the window grew. The extra height "+
			"is being padded rather than filled with the blame that was truncated.",
			shortBlank, tallBlank)
	}
}
