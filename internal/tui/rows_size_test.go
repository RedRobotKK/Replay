package tui

import "testing"

// The terminal has a height, and this package never asked.
//
// winsize.go fetches struct{ Row, Col, Xpixel, Ypixel } from the kernel and
// returns only ws.Col. The height was already on the wire and thrown away, one
// field from being useful — the thirteenth built-but-unwired thing found on
// 2026-09-09, and the one that explains the others: the advise screen rendered
// 704 lines into a 24-row terminal because nothing in the package could know
// how tall it was.
//
// Cols has a settled contract — COLUMNS, then the ioctl, then 80 — and Lines
// mirrors it exactly rather than inventing a second convention. Two functions
// that answer the same kind of question differently is how a caller comes to
// trust one and not the other.

// SZ1: LINES is honoured, because that is what a caller sets deliberately.
func TestSZ1_LinesEnvIsHonoured(t *testing.T) {
	t.Setenv("LINES", "50")
	if got := Lines(); got != 50 {
		t.Errorf("Lines() = %d with LINES=50", got)
	}
}

// SZ2: an unusable height falls back rather than laying out for nothing.
//
// A shell can export LINES=0, and a screen laid out for zero rows shows
// nothing at all. Same reasoning as minCols, and the same failure: the reader
// sees an empty frame and concludes the tool is broken.
func TestSZ2_AnUnusableHeightFallsBack(t *testing.T) {
	for _, v := range []string{"0", "-4", "not a number", "1"} {
		t.Setenv("LINES", v)
		if got := Lines(); got < minLines {
			t.Errorf("LINES=%q gave %d, below the %d floor", v, got, minLines)
		}
	}
}

// SZ3: a tall terminal is used, and a very tall one is not capped.
//
// maxCols exists because a 400-cell table row is unreadable. Height has no
// equivalent argument: more rows is strictly more content visible, which is the
// whole reason somebody maximises a window. Capping it would be inventing a
// constraint the terminal did not ask for.
func TestSZ3_HeightIsNotCapped(t *testing.T) {
	t.Setenv("LINES", "200")
	if got := Lines(); got != 200 {
		t.Errorf("Lines() = %d with LINES=200; height must not be capped the way "+
			"width is, because more rows is more content and nothing else", got)
	}
}

// SZ4: with no terminal and no LINES, the default is a real terminal's height.
//
// 24 rather than 0 or 1. A process with no tty still has to lay something out,
// and the number it picks is the one every terminal emulator has defaulted to
// since VT100 — the same reasoning as defaultCols being 80.
func TestSZ4_NoTerminalGivesTheClassicDefault(t *testing.T) {
	t.Setenv("LINES", "")
	if got := Lines(); got < minLines {
		t.Errorf("with no LINES and no tty, Lines() = %d", got)
	}
}

// SZ5: Body is the space a screen actually has for content.
//
// The chrome is not negotiable — a title line, a blank, and the footer the
// reader navigates by — so a screen that lays out for the full height overruns
// by exactly the chrome it forgot. Returning the usable figure means no caller
// has to remember what the frame costs.
func TestSZ5_BodyExcludesTheChrome(t *testing.T) {
	t.Setenv("LINES", "40")
	body := Body()
	if body >= Lines() {
		t.Errorf("Body() = %d with a %d-row terminal; it must leave room for the "+
			"title and the footer", body, Lines())
	}
	if body < 1 {
		t.Errorf("Body() = %d, which lays out for nothing", body)
	}
}

// SZ6: a taller terminal shows more findings.
//
// The point of reading the height. The advise screen showed a hardcoded four
// findings whatever the terminal was — the same four on a 24-row window and on
// a maximised 60-row one, which is a screen ignoring most of what it was given.
//
// Four was itself a fix for rendering 704 lines into 24 rows. The constant was
// right about the problem and wrong to be a constant.
func TestSZ6_ATallerTerminalShowsMore(t *testing.T) {
	many := make([]AdviceRow, 40)
	for i := range many {
		many[i] = AdviceRow{Title: "a finding about prompt tokens",
			Action: "truncate outputs", Sessions: 2, Share: 0.2, PromptTokens: 500}
	}

	t.Setenv("LINES", "24")
	short := len(AdviseScreenAt(many, 100, 0).Lines)
	t.Setenv("LINES", "60")
	tall := len(AdviseScreenAt(many, 100, 0).Lines)

	if tall <= short {
		t.Errorf("a 60-row terminal rendered %d lines and a 24-row one %d: the screen "+
			"ignores the height it was given", tall, short)
	}
	// And it must still fit. Using the height is not licence to overrun it.
	t.Setenv("LINES", "24")
	if got := len(AdviseScreenAt(many, 100, 0).Lines); got > 24 {
		t.Errorf("a 24-row terminal got %d lines", got)
	}
}
