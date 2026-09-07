package tui

import (
	"strings"
	"testing"
)

func lines(n int) []Line {
	out := make([]Line, n)
	for i := range out {
		out[i] = Line{Text: "row", Label: "session", Path: "/x.jsonl"}
	}
	return out
}

// Movement clamps rather than wraps.
//
// Holding j past the end would otherwise jump the reader back to the top
// without their asking, and on a long list they will not notice they have gone
// round: they will read the first rows again believing they are the last.
func TestMovementClampsAtBothEnds(t *testing.T) {
	s := Selection{Window: 5}
	for i := 0; i < 50; i++ {
		s.Move('j', 10)
	}
	if s.At != 9 {
		t.Errorf("j past the end landed at %d, want 9. Wrapping loses the reader's "+
			"place silently", s.At)
	}
	for i := 0; i < 50; i++ {
		s.Move('k', 10)
	}
	if s.At != 0 {
		t.Errorf("k past the start landed at %d, want 0", s.At)
	}
}

// g and G go to the ends, which is the whole reason they are worth two keys.
func TestHAndGReachBothEnds(t *testing.T) {
	s := Selection{Window: 5}
	s.Move('G', 40)
	if s.At != 39 {
		t.Errorf("G landed at %d, want 39", s.At)
	}
	s.Move('H', 40)
	if s.At != 0 {
		t.Errorf("H landed at %d, want 0", s.At)
	}
}

// A movement key reports that it was consumed; anything else does not.
//
// Without this a list would swallow every keystroke and the shortcut letters
// would stop working the moment one appeared on screen.
func TestOnlyMovementKeysAreConsumed(t *testing.T) {
	s := Selection{Window: 5}
	for _, k := range []rune{'j', 'k', 'H', 'G'} {
		if !s.Move(k, 10) {
			t.Errorf("%q was not consumed as a movement key", string(k))
		}
	}
	for _, k := range []rune{'c', 'w', 'd', 'g', '?', 'q', 27} {
		if s.Move(k, 10) {
			t.Errorf("%q was swallowed by the list. The shortcut letters must keep "+
				"working when rows are on screen", string(k))
		}
	}
	if s.Move('j', 0) {
		t.Error("a movement key was consumed over an empty list, so pressing j on a " +
			"screen with nothing on it would do something invisible")
	}
}

// The cursor marker never moves the text.
//
// A cursor that indents the line it is on shifts every other line as it
// travels, which is the shear this whole surface is built to avoid.
func TestTheCursorDoesNotIndentTheLineItIsOn(t *testing.T) {
	rendered := RenderRows(lines(4), 2)
	starts := map[int]bool{}
	for _, l := range rendered {
		starts[len(l)-len(strings.TrimLeft(l, "> "))] = true
	}
	if len(starts) != 1 {
		t.Errorf("rows start at %d different columns depending on the cursor: %v",
			len(starts), starts)
	}
	if !strings.HasPrefix(rendered[2], "> ") {
		t.Errorf("the selected row is not marked: %q", rendered[2])
	}
	for i, l := range rendered {
		if i != 2 && strings.HasPrefix(l, "> ") {
			t.Errorf("row %d is marked and is not the cursor", i)
		}
	}
}

// The window follows the cursor and keeps it on screen.
func TestTheVisibleWindowAlwaysContainsTheCursor(t *testing.T) {
	all := lines(100)
	for _, at := range []int{0, 1, 50, 98, 99} {
		s := Selection{At: at, Window: 7}
		vis, cur := s.Visible(all)
		if len(vis) != 7 {
			t.Errorf("at %d the window showed %d rows, want 7", at, len(vis))
		}
		if cur < 0 || cur >= len(vis) {
			t.Errorf("at %d the cursor fell outside the visible window at index %d",
				at, cur)
		}
	}
}

// An overlong row truncates rather than wrapping the list.
func TestALongRowIsTruncatedNotWrapped(t *testing.T) {
	long := []Line{{Text: strings.Repeat("x", 200)}}
	got := RenderRows(long, 0)
	if len(got) != 1 {
		t.Fatalf("a long row produced %d lines; it must stay one", len(got))
	}
	if len(got[0]) > BudgetCols {
		t.Errorf("row is %d columns, budget %d", len(got[0]), BudgetCols)
	}
	if !strings.HasSuffix(got[0], string(truncationMark)) {
		t.Errorf("a truncated row does not say it was cut: %q", got[0])
	}
}
