package tui

import (
	"strings"
	"testing"
)

// The footer is a floor, not a manual.
//
// The first draft put all eight questions there, which is the wall-of-keys
// problem progressive disclosure exists to prevent. Three to five, and the rest
// behind "?". A footer that lists actions has stopped being a floor.
func TestFooterShowsAFloorNotEveryKey(t *testing.T) {
	if n := len(footerKeys()); n < 3 || n > 5 {
		t.Errorf("the footer offers %d keys; the floor is three to five. More than "+
			"that and a first-time user reads a manual before they read an answer", n)
	}
	f := Footer('c')
	if len(f) > BudgetCols {
		t.Errorf("the footer is %d columns, budget %d:\n%s", len(f), BudgetCols, f)
	}
	// The questions must NOT be in the footer any more.
	loud := 0
	for _, s := range Shortcuts() {
		if strings.Contains(f, " "+string(s.Key)+" "+s.Label) {
			loud++
		}
	}
	if loud > 1 {
		t.Errorf("%d question keys are still in the footer. They belong behind ?, "+
			"which is the whole point of the tier:\n%s", loud, f)
	}
	if !strings.Contains(f, "? keys") {
		t.Errorf("the footer does not offer the way to the rest of the vocabulary:\n%s", f)
	}
}

// The floor must be reachable without knowing vim.
//
// L1 is the terminal lingua franca and L0 is for everyone else. A surface with
// only vim motion is a surface that turns away the beginners it was built for;
// one with only arrows asks fluent users to unlearn what they know.
func TestBothL0AndL1AreOffered(t *testing.T) {
	var l0, l1 int
	for _, b := range Bindings() {
		switch b.Layer {
		case L0:
			l0++
		case L1:
			l1++
		}
	}
	if l0 == 0 {
		t.Error("no L0 binding. Arrows, Enter, Esc and q are how somebody who has " +
			"never used a TUI gets anywhere")
	}
	if l1 == 0 {
		t.Error("no L1 binding. j/k, /, ? and g/G are the vocabulary terminal users " +
			"already have, and omitting them asks them to unlearn it")
	}
	want := []string{"j k", "/", "?", "g G", "esc", "q"}
	have := map[string]bool{}
	for _, b := range Bindings() {
		have[b.Keys] = true
	}
	for _, k := range want {
		if !have[k] {
			t.Errorf("%q is not bound. It is part of the six-keystroke vocabulary that "+
				"covers most terminal navigation", k)
		}
	}
}

// Every question stays reachable, just one tier down.
func TestHelpCarriesEveryQuestion(t *testing.T) {
	h := strings.Join(Help(), "\n")
	for _, s := range Shortcuts() {
		if !strings.Contains(h, s.Question) {
			t.Errorf("? does not offer %q. Moving a key off the footer must not make it "+
				"undiscoverable, or progressive disclosure has become hiding", s.Question)
		}
	}
	for _, l := range Help() {
		if len(l) > BudgetCols {
			t.Errorf("the help overlay is %d columns, budget %d:\n%s", len(l), BudgetCols, l)
		}
	}
	if len(Help()) > BudgetRows {
		t.Errorf("the help overlay is %d rows and does not fit %d. The overlay that "+
			"explains the keys cannot itself need scrolling to read", len(Help()), BudgetRows)
	}
}

// The screen says where you are.
//
// Contextual intelligence: the status bar reflects current state. A footer
// identical on every screen tells the user nothing about which one they are on.
func TestFooterNamesTheCurrentScreen(t *testing.T) {
	for _, s := range Shortcuts() {
		if !strings.Contains(Footer(s.Key), s.Label) {
			t.Errorf("the footer on the %q screen does not name it", s.Label)
		}
	}
}

// The meter is a flourish and must never move a column.
func TestMeterKeepsItsWidth(t *testing.T) {
	for _, f := range []float64{-1, 0, 0.01, 0.5, 0.999, 1, 2} {
		if got := len(Meter(f, 20)); got != 20 {
			t.Errorf("Meter(%v) is %d cells, want 20. A meter that changes width is a "+
				"flourish that broke the frame it sits in", f, got)
		}
	}
	if Meter(0, 10) == Meter(1, 10) {
		t.Error("the meter looks the same empty and full, so it shows nothing")
	}
	for _, r := range Meter(0.5, 20) {
		if r > 127 {
			t.Errorf("the meter used a non-ASCII glyph %q. It is a flourish over an "+
				"ASCII frame and must degrade rather than shear", r)
		}
	}
}

// The glyph table must match what the runes actually are.
//
// Written because the intuition is backwards: Braille is Neutral width and safe,
// while the full block every terminal chart is drawn from is Ambiguous and
// doubles in a CJK locale. A sparkline of Braille is safer than one of blocks.
func TestGlyphSafetyMatchesMeasurement(t *testing.T) {
	safe := []string{"|/-\\", "#.", "[]<>", "⠐⢿"}
	unsafe := []string{"█", "▏", "─", "│", "§"}
	for _, s := range safe {
		if !SafeInAFrame(s) {
			t.Errorf("%q is measured safe and the code rejects it", s)
		}
	}
	for _, s := range unsafe {
		if SafeInAFrame(s) {
			t.Errorf("%q is East Asian Ambiguous and would double in a CJK terminal, "+
				"and the code accepts it in a frame", s)
		}
	}
}
