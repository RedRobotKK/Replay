package tui

import (
	"strings"
	"testing"
	"time"
)

// Refresh rates must be sane for a screen that is on all day.
//
// The failure mode is not a wrong number, it is a plausible one chosen once and
// never questioned: a repaint fast enough to feel responsive while somebody is
// looking, left running for eight hours while nobody is.
func TestRefreshCadencesAreDeliberate(t *testing.T) {
	if TickLiveness < 100*time.Millisecond {
		t.Errorf("liveness at %v is animation, not information, and costs battery to "+
			"tell a room nobody is in that a process is alive", TickLiveness)
	}
	if TickLiveness > TickTraffic || TickTraffic > TickTotals {
		t.Errorf("the cadences are not ordered: liveness %v, traffic %v, totals %v. "+
			"The cue must move faster than the rows, and the rows faster than the "+
			"figures somebody is trying to read", TickLiveness, TickTraffic, TickTotals)
	}
	if TickAmbient == 0 {
		t.Error("ambient repaint is zero, so a screen with nothing happening stops " +
			"redrawing entirely and is indistinguishable from a dead one")
	}
	if TickTotals < 2*time.Second {
		t.Errorf("totals redraw every %v. A figure that changes faster than somebody "+
			"can read it is a figure nobody reads", TickTotals)
	}
}

// The glance line has to survive being read badly.
//
// Two metres, off-axis, on a monitor with the contrast turned down. Colour is a
// hint at that distance; the word is the message. A line that needs colour to
// say "act" says nothing to somebody who is not looking directly at it.
func TestGlanceReadsWithoutColour(t *testing.T) {
	cases := []struct {
		a    Attention
		want string
	}{
		{Calm, "OK"}, {Notice, "NOTE"}, {Act, "CRIT"},
	}
	seen := map[string]bool{}
	for _, c := range cases {
		got := Glance(c.a, "the day cap cannot be enforced")
		if !strings.Contains(got, c.want) {
			t.Errorf("attention %v does not carry the word %q, so it depends on colour "+
				"to be understood: %q", c.a, c.want, got)
		}
		if len(got) > BudgetCols {
			t.Errorf("the glance line is %d columns, budget %d: %q", len(got), BudgetCols, got)
		}
		seen[strings.TrimSpace(got[:6])] = true
	}
	// The markers must differ in SHAPE, not only in the letters inside them.
	// A reader who cannot separate the colours, or who is two metres away and
	// squinting, is reading the outline before the word.
	shapes := map[string]bool{}
	for _, c := range cases {
		shapes[Glance(c.a, "x")[:6]] = true
	}
	if len(shapes) != 3 {
		t.Errorf("the three states share %d distinct markers, not 3. Fixed width "+
			"stops the line reflowing; it does not make the states tellable apart "+
			"at a glance: %v", len(shapes), shapes)
	}
	for _, c := range cases {
		m := Glance(c.a, "x")[:6]
		if m[0] != '[' || m[5] != ']' {
			t.Errorf("marker %q has no bracket outline. The brackets are the "+
				"structural tell that survives bad colour", m)
		}
	}

	if len(seen) != 3 {
		t.Errorf("the three attention levels do not produce three distinct markers: %v. "+
			"A screen that looks the same when it needs you is a screen you stop "+
			"looking at", seen)
	}
	// The marker must be a fixed width or the message beside it shifts.
	w := len(Glance(Calm, "x")) - 1
	for _, c := range cases {
		if got := len(Glance(c.a, "x")) - 1; got != w {
			t.Errorf("the %v marker is %d cells and calm is %d. The message after it "+
				"would move as the state changes, which is the one thing a glanceable "+
				"line must not do", c.a, got, w)
		}
	}
}

// Coming back after a while must be answered, not punished.
//
// A monitor that only shows now is a monitor that penalises the thing its owner
// will certainly do, which is look away.
func TestSinceAnswersWhatWasMissed(t *testing.T) {
	got := strings.Join(Since(3*time.Hour, 412, 3, []string{"day cap"}), "\n")
	for _, want := range []string{"3 hours", "412", "day cap"} {
		if !strings.Contains(got, want) {
			t.Errorf("the away summary does not carry %q:\n%s", want, got)
		}
	}
	quiet := strings.Join(Since(20*time.Minute, 8, 0, nil), "\n")
	if !strings.Contains(quiet, "none") {
		t.Errorf("a quiet period must say so explicitly. An empty field reads as a "+
			"missing measurement rather than an uneventful one:\n%s", quiet)
	}
	if !strings.Contains(quiet, "20 minutes") {
		t.Errorf("the window is not stated in words somebody says out loud:\n%s", quiet)
	}
}

// The eye anchor must not move when data arrives.
//
// On a screen being read, a row arriving at the top is fine. On a screen being
// glanced at, it means the row the eye was trained on is now one lower, and the
// reader re-reads instead of recognising.
func TestArrivingDataDoesNotMoveTheReadersEye(t *testing.T) {
	before := []string{Empty(), Empty(), Empty(), Empty(), Empty()}
	after := []string{
		Traffic("15:06:44", "anthropic", "api.anthropic.com", "messages", "parsed"),
		Empty(), Empty(), Empty(), Empty(),
	}
	if !Anchored(before, after) {
		t.Error("the traffic window changed height when a request arrived. Every row " +
			"below it moves, and so does the glance line and the footer")
	}
	if len(before[0]) != len(after[0]) {
		t.Errorf("an occupied row is %d cells and an empty one is %d. The grid would "+
			"breathe as traffic arrives", len(after[0]), len(before[0]))
	}

	// And it must say no when the anchor really moves. Without these, the check
	// only ever sees frames that agree, and a version returning true
	// unconditionally passes: a check that cannot fail, inside the file that
	// argues against them.
	if Anchored(before, append(after, Empty())) {
		t.Error("a window that grew by a row was reported as anchored. Everything " +
			"below it, including the glance line and the footer, has moved")
	}
	widened := make([]string, len(after))
	copy(widened, after)
	widened[0] += " "
	if Anchored(after, widened) {
		t.Error("a row that changed width was reported as anchored. It pushes nothing " +
			"vertically and reflows the column beside it, which moves the figure the " +
			"reader had already located")
	}
}
