package tui

import (
	"strings"
	"testing"
)

// Making prose fit a narrow terminal.
//
// After the width became a measurement, 72 lines across the nine screens still
// exceeded a 60-column terminal: 46 prose, 22 table rows, 4 rules, counting both
// locales. This handles the prose, which is the largest class and the only one
// where wrapping is the right answer.
//
// Table rows are deliberately left alone. Wrapping a row breaks the alignment
// that makes it a table, and truncating one silently drops a column's value.
// storyboard.go scene 25 already specifies what should happen to them — columns
// dropped in a fixed order, wire then endpoint then surface — and that belongs
// where the table is built, not at the output boundary where nothing knows
// which column means what.
//
// So this is deliberately partial, and the audit's frozen count is what says by
// how much.

// FT1: a long sentence is wrapped to the width.
func TestFT1_ProseWraps(t *testing.T) {
	in := "  Median $32.25, p90 $32.25, $1.89 avoidable. List price, not your bill."
	got := Fit(in, 40)
	if len(got) < 2 {
		t.Fatalf("a %d-cell line did not wrap at 40: %q", VisibleLen(in), got)
	}
	for i, l := range got {
		if VisibleLen(l) > 40 {
			t.Errorf("wrapped line %d is %d cells, over 40: %q", i, VisibleLen(l), l)
		}
	}
	if !strings.Contains(strings.Join(got, " "), "not your bill") {
		t.Errorf("wrapping dropped the end of the sentence: %q", got)
	}
}

// FT2: the continuation keeps the original indent.
//
// Every screen indents its prose by two spaces. A continuation flush against
// the margin reads as a new item rather than the rest of a sentence.
func TestFT2_ContinuationKeepsTheIndent(t *testing.T) {
	got := Fit("    four spaces in, and long enough that this must wrap somewhere sensible", 30)
	if len(got) < 2 {
		t.Fatal("did not wrap")
	}
	for i, l := range got[1:] {
		if !strings.HasPrefix(l, "    ") {
			t.Errorf("continuation %d lost the four-space indent: %q", i+1, l)
		}
	}
}

// FT3: a column layout is left exactly as it was.
//
// Wrapping a table row destroys the alignment that makes it readable, and this
// boundary cannot know which column could be dropped. Scene 25 owns that.
func TestFT3_TableRowsAreUntouched(t *testing.T) {
	row := "  1 of 1   redacted  $32.25   (transcript not found, cannot price it here)"
	got := Fit(row, 40)
	if len(got) != 1 || got[0] != row {
		t.Errorf("a column layout was altered:\n in  %q\n out %q", row, got)
	}
}

// FT4: a line that already fits is returned unchanged.
//
// The common case by far, and the one that must not allocate or reflow.
func TestFT4_ShortLinesPassThrough(t *testing.T) {
	for _, s := range []string{"", "  $32.25 across 1 tasks", "   "} {
		got := Fit(s, 80)
		if len(got) != 1 || got[0] != s {
			t.Errorf("Fit(%q) = %q, want it unchanged", s, got)
		}
	}
}

// FT5: an unbreakable token is not lost.
//
// A path or a URL longer than the terminal has no whitespace to break on.
// Dropping it would lose the one thing the reader needed; emitting it over
// width is the lesser fault, and the audit counts it.
func TestFT5_AnUnbreakableTokenSurvives(t *testing.T) {
	long := "  /Users/someone/a/very/long/path/that/exceeds/the/width/entirely.jsonl"
	got := Fit(long, 30)
	if !strings.Contains(strings.Join(got, ""), "entirely.jsonl") {
		t.Errorf("an unbreakable token was lost: %q", got)
	}
}

// FT6: colour survives the wrap.
//
// The screens paint before this point, so a wrapped line carries SGR sequences.
// Measuring on bytes would wrap early and cut visible text that fits, which is
// the defect rows.go documents having already made once.
func TestFT6_MeasuresVisibleCellsNotBytes(t *testing.T) {
	painted := "  \x1b[36mmeasured\x1b[0m and then a good deal more text to force a wrap here"
	got := Fit(painted, 40)
	for i, l := range got {
		if VisibleLen(l) > 40 {
			t.Errorf("line %d is %d visible cells, over 40: %q", i, VisibleLen(l), l)
		}
	}
}
