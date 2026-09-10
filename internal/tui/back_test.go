package tui

import (
	"bytes"
	"testing"
)

// The footer says "esc back". There was no back.
//
// Escape closed the help overlay and otherwise did nothing, on every screen,
// while the footer advertised it on all ten and the advice detail said "esc
// back to the list" in its own body. A reader who pressed it got no response
// and no explanation — the worst of the three possible outcomes, because a key
// that does nothing is indistinguishable from a terminal that dropped it.

func loopFor(t *testing.T) *Loop {
	t.Helper()
	var out bytes.Buffer
	return &Loop{Out: &out, Keys: make(chan rune), Source: func(k rune, _ int) Frame {
		return Frame{Lines: []string{"  " + string(k)}}
	}}
}

// BK1: escape returns to the screen you came from.
func TestEscapeGoesBackToThePreviousScreen(t *testing.T) {
	l := loopFor(t)
	l.cur = 'c'
	l.press('d')
	if l.Current() != 'd' {
		t.Fatalf("pressing d moved to %q", l.Current())
	}
	l.press(27)
	if got := l.Current(); got != 'c' {
		t.Errorf("escape from d landed on %q, want c. The footer says \"esc back\" "+
			"on every screen", got)
	}
}

// BK2: escape closes help first, and does not also navigate.
//
// Two effects from one keystroke is how a reader loses their place: they press
// escape to dismiss the overlay and find themselves on a different screen.
func TestEscapeClosesHelpWithoutNavigating(t *testing.T) {
	l := loopFor(t)
	l.cur = 'c'
	l.press('d')
	l.press('?')
	l.press(27)
	if l.Helping() {
		t.Error("escape did not close the help overlay")
	}
	if got := l.Current(); got != 'd' {
		t.Errorf("escape closed help and also navigated to %q", got)
	}
	// The next escape is the one that goes back.
	l.press(27)
	if got := l.Current(); got != 'c' {
		t.Errorf("the second escape landed on %q, want c", got)
	}
}

// BK3: escape on the screen you started on does nothing, and does not quit.
//
// A surface that exits on the key people press to back out is a surface people
// lose work in — and the installer opens this one for them.
func TestEscapeOnTheFirstScreenDoesNotQuit(t *testing.T) {
	l := loopFor(t)
	l.cur = 'c'
	l.press(27)
	if got := l.Current(); got != 'c' {
		t.Errorf("escape from the opening screen moved to %q", got)
	}
	// And escape cannot end the loop even in principle: Run owns the exit and
	// press never sees 'q'. Asserted here so the structure stays that way.
	l.press('q')
	if got := l.Current(); got != 'c' {
		t.Errorf("press handled q and moved to %q; the exit belongs to Run, where "+
			"no screen-local handler can swallow it", got)
	}
}

// BK4: back does not stack without bound, and does not loop.
//
// c -> d -> c leaves "the screen you came from" as d. Pressing escape twice
// must not ping-pong forever between two screens; one step back is what the
// footer promises and one step is what it does.
func TestBackIsOneStepNotAHistory(t *testing.T) {
	l := loopFor(t)
	l.cur = 'c'
	l.press('d')
	l.press('c')
	l.press(27)
	if got := l.Current(); got != 'd' {
		t.Errorf("escape landed on %q, want d", got)
	}
	l.press(27)
	if got := l.Current(); got != 'c' {
		t.Errorf("the second escape landed on %q, want c", got)
	}
}

// BK5: a screen-local handler still sees escape first.
//
// The advice detail closes on escape, and it has to close before the loop
// treats the key as navigation — otherwise escape leaves the detail open on a
// different screen.
func TestALocalHandlerClaimsEscapeFirst(t *testing.T) {
	l := loopFor(t)
	l.cur = 'a'
	l.press('d')
	claimed := false
	l.SetLocal(func(k rune) bool {
		if k == 27 {
			claimed = true
			return true
		}
		return false
	})
	l.press(27)
	if !claimed {
		t.Fatal("the screen-local handler never saw escape")
	}
	if got := l.Current(); got != 'd' {
		t.Errorf("escape navigated to %q despite being claimed", got)
	}
}
