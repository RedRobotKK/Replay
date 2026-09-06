package tui

import (
	"strings"
	"testing"
	"unicode"
)

// Every character a frame is built from must occupy exactly one terminal cell
// in every locale.
//
// This is not a style rule. Box-drawing characters and most symbol glyphs are
// East Asian Ambiguous: one cell in a Western locale, two in a CJK one. A frame
// drawn from them is 80 columns on one machine and 160 on another, and the
// second is not a frame. A previous design of this surface used 98 ambiguous
// characters in three lines of grid and stated as an invariant that its borders
// could never shear.
//
// Restricting frame elements to printable ASCII makes the question unaskable
// rather than answered carefully, which is the only version of this that
// survives somebody adding a row later.
func TestFrameElementsArePrintableASCII(t *testing.T) {
	for name, s := range frameElements() {
		for i, r := range s {
			if r > unicode.MaxASCII {
				t.Errorf("%s contains a non-ASCII rune %q at %d. Frame elements must be "+
					"one cell wide in every locale; anything outside ASCII risks being "+
					"two in a CJK terminal", name, r, i)
			}
			if r != ' ' && !unicode.IsPrint(r) {
				t.Errorf("%s contains a non-printable rune %q at %d", name, r, i)
			}
		}
	}
}

// Every row of a table must be the same display width as its header.
//
// The failure this catches is a single missing pad space, which is invisible in
// review and shears the whole column. It is what went wrong in the storyboard
// that claimed it could not happen: data rows measured 78 against a 79-column
// header.
func TestEveryRowMatchesTheHeaderWidth(t *testing.T) {
	cols := []Column{{"time", 8}, {"surface", 9}, {"endpoint", 23}, {"status", 9}}
	header := Row(cols, "time", "surface", "endpoint", "status")
	want := len(header)

	cases := [][]string{
		{"15:06:44", "anthropic", "api.anthropic.com", "parsed"},
		{"15:05:58", "grok", "cli-chat-proxy.grok.com", "forwarded"},
		// Overlong content must truncate, never widen the row.
		{"15:05:44", "a-very-long-surface-name", "an.endpoint.far.past.the.column.width", "stub"},
		// An empty slot still holds the grid open.
		{"", "", "", ""},
	}
	for _, c := range cases {
		got := Row(cols, c...)
		if len(got) != want {
			t.Errorf("row %q is %d wide, header is %d. A row that does not match its "+
				"header shears the column for every row after it", got, len(got), want)
		}
		if strings.ContainsRune(got, '\n') {
			t.Errorf("row %q wrapped. Wrapping breaks the grid; truncate instead", got)
		}
	}
}

// Truncation must be visible, or a shortened value reads as the whole value.
func TestTruncationIsMarked(t *testing.T) {
	cols := []Column{{"endpoint", 10}}
	got := Row(cols, "cli-chat-proxy.grok.com")
	if !strings.Contains(got, "~") {
		t.Errorf("a truncated cell must say it was truncated, or the reader takes the "+
			"prefix for the value: %q", got)
	}
}
