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

// SZ7: the loop opens on the screen it was given.
//
// `--screen advise` is documented as "which question to open on" and was
// honoured only by --once. StartWith built the Loop with no initial key, so the
// interactive path fell to Shortcuts()[0] — cost — and the flag was silently
// dropped.
//
// Found by recording a demo. The tape typed `tui --screen advise`, waited, and
// captured the cost screen counting zero transcripts; two renders were blamed
// on the wrong binary before the flag itself was suspected. A demo is a test
// that watches the product the way a person does, which is why it caught what
// nine unit tests over this package did not.
func TestSZ7_TheLoopOpensOnTheGivenScreen(t *testing.T) {
	want := rune(0)
	for _, s := range Shortcuts() {
		if s.Label == "advise" {
			want = s.Key
		}
	}
	if want == 0 {
		t.Fatal("no advise shortcut")
	}

	var got rune
	l := &Loop{Start: want, Source: func(k rune, _ int) Frame {
		got = k
		return Frame{Key: k, Lines: []string{"x"}}
	}}
	l.first()
	if got != want {
		t.Errorf("the loop opened on %q with Start=%q; the --screen flag is dropped "+
			"on the interactive path and honoured only by --once", got, want)
	}
}

// SZ8: with no Start, the loop still opens on the first shortcut.
//
// The fallback has to survive, or every caller that does not care about the
// opening screen gets a blank one.
func TestSZ8_NoStartFallsBackToTheFirstShortcut(t *testing.T) {
	var got rune
	l := &Loop{Source: func(k rune, _ int) Frame {
		got = k
		return Frame{Key: k, Lines: []string{"x"}}
	}}
	l.first()
	if got != Shortcuts()[0].Key {
		t.Errorf("with no Start the loop opened on %q, want %q",
			got, Shortcuts()[0].Key)
	}
}

// SZ9: the screen never renders more lines than the terminal has.
//
// The height fix estimated six lines per finding and stopped there. Six is
// right when the action wraps to two lines and wrong when it wraps to one, so
// the screen overran on some inputs and under-filled on others — the recorded
// demo shows a fourth finding clipped by the footer.
//
// An estimate that has to be right for every combination of width, action
// length and terminal height is a constant pretending to be a measurement.
// This asserts the property instead: whatever the terminal, whatever the
// content, the frame fits.
func TestSZ9_TheScreenNeverOverrunsTheTerminal(t *testing.T) {
	mk := func(n int, action string) []AdviceRow {
		out := make([]AdviceRow, n)
		for i := range out {
			out[i] = AdviceRow{
				Title:  "a finding about prompt tokens that runs on a bit",
				Action: action, Sessions: 3, Share: 0.3,
				PromptTokens: 900000, PredictedShare: 0.15, Status: "pending",
			}
		}
		return out
	}
	actions := map[string]string{
		"short":   "trim it",
		"onewrap": "truncate outputs before they enter the conversation with limits",
		"twowrap": "truncate outputs before they enter the conversation: head, tail, " +
			"grep with limits, or a summarizing wrapper that keeps the shape",
	}
	for _, lines := range []string{"14", "24", "40", "60", "100"} {
		for name, action := range actions {
			for _, cols := range []string{"60", "80", "120"} {
				t.Setenv("LINES", lines)
				t.Setenv("COLUMNS", cols)
				got := len(AdviseScreenAt(mk(40, action), 1738, 0).Lines)
				if got > Body() {
					t.Errorf("LINES=%s COLUMNS=%s action=%s: %d lines for a body of %d — "+
						"the frame overruns and the last finding is clipped by the footer",
						lines, cols, name, got, Body())
				}
			}
		}
	}
}
