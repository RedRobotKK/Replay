// Package tui lays out the live `replay serve` surface.
//
// Everything here is pure: values in, a string out, no terminal and no I/O.
// The rendering loop that owns stdout lives at the boundary, so the layout can
// be tested by reading what it produced rather than by driving a terminal.
package tui

import "strings"

// gutter is the leading indent on every line, and the gap between columns.
// Two spaces, not a rule, because a drawn border buys nothing here and costs
// the locale safety the whole package is built on.
const gutter = "  "

// truncationMark says a cell was cut. Without it a shortened endpoint reads as
// the whole endpoint, which is a wrong value rather than a missing one.
const truncationMark = '~'

// Column is one field of the traffic table: its heading and its fixed width.
type Column struct {
	Name  string
	Width int
}

// Row lays values into columns and returns one line of the table.
//
// The line is always exactly the same width for a given set of columns,
// whatever it is given: short values pad, long values truncate, and nothing
// wraps. A wrapped cell breaks the grid for every row after it, and a row one
// space short does the same thing while being invisible in review.
//
// Values beyond the column count are dropped and missing values render empty,
// so a caller cannot shear the table by miscounting.
func Row(cols []Column, values ...string) string {
	var b strings.Builder
	b.WriteString(gutter)
	for i, c := range cols {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		if i > 0 {
			b.WriteString(gutter)
		}
		b.WriteString(cell(v, c.Width))
	}
	return b.String()
}

// cell fits one value to one width.
func cell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	// Count runes, not bytes: a multibyte value must occupy its rune count in
	// cells, and every rune reaching here is ASCII by the package's own rule.
	r := []rune(s)
	if len(r) > w {
		if w == 1 {
			return string(truncationMark)
		}
		return string(r[:w-1]) + string(truncationMark)
	}
	return s + strings.Repeat(" ", w-len(r))
}

// frameElements is every fixed string the layout draws itself from, named so a
// failure says which one is wrong. The ASCII test walks this, so adding a frame
// element without adding it here is the one way to get an unchecked glyph on
// screen.
func frameElements() map[string]string {
	return map[string]string{
		"gutter":         gutter,
		"truncationMark": string(truncationMark),
		"rule":           strings.Repeat("-", 8),
		"noteWarning":    "! ",
		"noteInfo":       "- ",
	}
}
