package transcript

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// truncateLabelReference is the implementation TruncateLabel had before it
// stopped converting the whole string to runes. It is kept here as the
// oracle: the fast form has to agree with it on every input, byte for byte,
// including the inputs nobody thought about when writing either one.
func truncateLabelReference(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}

// TestTL1MatchesTheReferenceOnEveryInput is the whole net for the rewrite.
// The cut is now found by walking byte offsets rather than by indexing a rune
// slice, and the ways that goes wrong — off by one rune, cutting inside a
// multi-byte character, returning the ellipsis when the label fitted, losing
// the n == 1 and empty-string corners — all produce a string that is merely
// plausible. Only comparison against the old code catches them.
func TestTL1MatchesTheReferenceOnEveryInput(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"ab",
		"abc",
		"short",
		strings.Repeat("プ", 10), // 3-byte runes
		"héllo wörld",
		"áé",                                // combining marks: runes, not graphemes
		"\U0001f469\u200d\U0001f4bb together", // ZWJ sequence: 4-byte runes joined
		"…",                                   // the ellipsis itself
		strings.Repeat("…", 5),                // a label that already ends in one
		strings.Repeat("x", 300),
		strings.Repeat("プx", 150),
		"\x00\x01mixedcontrol",
		"tab\there",
	}
	// Widths either side of every boundary that matters, plus the negative
	// and zero cases the function refuses to act on.
	widths := []int{-3, -1, 0, 1, 2, 3, 4, 5, 9, 10, 11, 60, 299, 300, 301}

	checked := 0
	for _, s := range inputs {
		for _, n := range widths {
			want := truncateLabelReference(s, n)
			got := TruncateLabel(s, n)
			if got != want {
				t.Fatalf("TruncateLabel(%q, %d) = %q, reference = %q", s, n, got, want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("TruncateLabel(%q, %d) = %q, not valid UTF-8", s, n, got)
			}
			if n > 0 && utf8.RuneCountInString(got) > n {
				t.Fatalf("TruncateLabel(%q, %d) = %q, %d runes exceeds the width",
					s, n, got, utf8.RuneCountInString(got))
			}
			checked++
		}
	}
	if checked != len(inputs)*len(widths) {
		t.Fatalf("checked %d pairs, want %d", checked, len(inputs)*len(widths))
	}
	t.Logf("%d input/width pairs agree with the reference", checked)
}

// TestTL2LongLabelsDoNotCopyThemselvesToTruncate pins the point of the
// rewrite. The old form converted the entire string to a []rune to keep the
// first few of them, so the cost of shortening a label to 60 characters grew
// with the length of the label being thrown away. One allocation is the
// result string; anything proportional to the input is the defect returning.
func TestTL2LongLabelsDoNotCopyThemselvesToTruncate(t *testing.T) {
	// 150 KB of label, cut to 60 runes. The reference form allocates about
	// 800 KB for the rune slice alone.
	long := strings.Repeat("プ", 50_000)
	const width = 60

	allocs := testing.AllocsPerRun(50, func() {
		truncateSink = TruncateLabel(long, width)
	})
	if allocs > 2 {
		t.Fatalf("TruncateLabel allocates %.0f times on a %d-byte label, want at most 2:\n"+
			"the whole string is being converted to runes again", allocs, len(long))
	}
	if utf8.RuneCountInString(truncateSink) != width {
		t.Fatalf("result = %d runes, want %d", utf8.RuneCountInString(truncateSink), width)
	}
	t.Logf("%.0f allocations to cut %d bytes to %d runes", allocs, len(long), width)
}

var truncateSink string
