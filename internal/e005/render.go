package e005

import "fmt"

// Rendering, kept in its own file and out of score.go on purpose.
//
// The frozen table shows 13,482 for a median whose value is 13,482.5, and
// shows 93% for a bound of 932/1006 that truncates to 92. Both differences are
// Python's "%.0f" rounding half to even, which Go's "%.0f" matches.
//
// Mixing that with the computation is how a reader comes to believe the
// underlying median IS 13,482. Nothing in this file is called by Score or
// Bounds; it only turns their output into the strings the published table
// shows, and the tests assert both layers separately.

// RenderMedian formats a computed median the way the frozen table printed it.
func RenderMedian(v float64) string { return fmt.Sprintf("%.0f", v) }

// Pct renders a proportion the way the frozen table printed it.
//
// Integer truncation is NOT equivalent: 100*932/1006 is 92.64, which truncates
// to 92 where the frozen table shows 93. Truncating would silently move four
// of the eight published bounds.
func Pct(part, whole int) string {
	if whole == 0 {
		return "0%"
	}
	return RenderMedian(100*float64(part)/float64(whole)) + "%"
}
