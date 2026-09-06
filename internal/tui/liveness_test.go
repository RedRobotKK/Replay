package tui

import (
	"strings"
	"testing"
)

// A live screen must never look frozen.
//
// This is the rule, stated mechanically: render the same live state at two
// consecutive repaint ticks and the two frames must differ. If they do not, an
// operator cannot tell a working proxy waiting quietly from a process that
// died three minutes ago, and the screen answers the only question they have
// wrongly and with confidence.
//
// It is the same defect as a check that cannot fail, moved from the test suite
// to the display: a signal that cannot vary carries no information.
func TestALiveScreenNeverLooksFrozen(t *testing.T) {
	for _, sc := range LiveScenes() {
		a := strings.Join(sc.Render(0), "\n")
		b := strings.Join(sc.Render(1), "\n")
		if a == b {
			t.Errorf("%s renders identically at tick 0 and tick 1. A screen with nothing "+
				"moving on it cannot be told apart from a hung process:\n%s", sc.Name, a)
		}
	}
}

// Every liveness element must move on its own, not be carried by a neighbour.
//
// The scene-level test above passed with the heartbeat frozen, because the
// pinwheel beside it was still turning. That is the composite-signal problem:
// the screen as a whole varied, so the check was satisfied while one of its two
// liveness cues was dead. An operator watching the heartbeat specifically would
// have been told the proxy had stopped.
//
// So each cue is asserted individually, and a frozen one fails here whatever
// else on the screen happens to be moving.
func TestEachLivenessCueAdvancesOnItsOwn(t *testing.T) {
	cues := []struct {
		name string
		at   func(int) string
	}{
		{"pinwheel", Pinwheel},
		{"heartbeat", Heartbeat},
		{"awaiting field", func(tick int) string { return Awaiting("upstream", tick) }},
		{"in-flight row", func(tick int) string {
			return LiveRow("15:06:44", "anthropic", "api.anthropic.com", "messages", tick)
		}},
	}
	for _, c := range cues {
		seen := map[string]bool{}
		for tick := 0; tick < 8; tick++ {
			seen[c.at(tick)] = true
		}
		if len(seen) < 2 {
			t.Errorf("the %s renders the same string at every tick, so it is a still "+
				"picture of a moving thing. Another cue on the same screen moving does "+
				"not make this one alive: %q", c.name, c.at(0))
		}
	}
}

// The cue must not move the column it sits in.
//
// A spinner that changes width reflows the field beside it on every tick, which
// draws the eye to the layout instead of to the data. Braille and box-drawing
// spinners do exactly this in a CJK terminal, which is why the cue is ASCII.
func TestPinwheelKeepsItsWidth(t *testing.T) {
	for tick := -4; tick < 12; tick++ {
		if got := len(Pinwheel(tick)); got != 1 {
			t.Errorf("Pinwheel(%d) is %d cells wide, want 1", tick, got)
		}
		if got := len(Heartbeat(tick)); got != heartbeatTrack+2 {
			t.Errorf("Heartbeat(%d) is %d cells, want %d", tick, got, heartbeatTrack+2)
		}
	}
	// And a field carrying the cue keeps its own width across a full rotation.
	w := len(Awaiting("upstream", 0))
	for tick := 1; tick < 8; tick++ {
		if got := len(Awaiting("upstream", tick)); got != w {
			t.Errorf("Awaiting at tick %d is %d cells, tick 0 was %d. A field that reflows "+
				"as it spins draws the eye for the wrong reason", tick, got, w)
		}
	}
}

// The cue must actually rotate rather than sit on one character.
func TestPinwheelRotatesThroughEveryPosition(t *testing.T) {
	seen := map[string]bool{}
	for tick := 0; tick < len(pinwheel); tick++ {
		seen[Pinwheel(tick)] = true
	}
	if len(seen) != len(pinwheel) {
		t.Errorf("the cue showed %d of %d positions in a full turn: %v", len(seen), len(pinwheel), seen)
	}
	// A negative tick is a clock that went backwards, not a panic.
	if Pinwheel(-1) == "" {
		t.Error("Pinwheel(-1) returned empty; a tick counter that wraps must not break the frame")
	}
}
