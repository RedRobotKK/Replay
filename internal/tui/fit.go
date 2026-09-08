package tui

import "strings"

// Making prose fit a narrow terminal.
//
// After the width became a measurement, 72 lines across the nine screens still
// exceeded a 60-column terminal. Counted across both locales: 46 prose, 22
// table rows, 4 rules. This handles the prose, the largest class and the only
// one where wrapping is the right answer.
//
// Table rows are left alone on purpose. Wrapping a row destroys the alignment
// that makes it a table, and truncating one silently drops a column's value —
// which is the shape of defect this project keeps finding. storyboard.go scene
// 25 already specifies the remedy, columns dropped in a fixed order, and that
// belongs where the table is built. This boundary sees strings and cannot know
// which column could go.

// Fit breaks one rendered line so it sits inside cols.
//
// A line that already fits, or that looks like a column layout, is returned
// untouched. Anything else is wrapped on whitespace with the original indent
// carried onto each continuation.
func Fit(line string, cols int) []string {
	if cols <= 0 || VisibleLen(line) <= cols || looksTabular(line) {
		return []string{line}
	}

	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	words := strings.Fields(strings.TrimLeft(line, " "))
	if len(words) == 0 {
		return []string{line}
	}

	var out []string
	cur := indent
	for _, w := range words {
		switch {
		case strings.TrimSpace(cur) == "":
			// First word on a line goes on whatever the width, because a token
			// longer than the terminal has nowhere to break. Emitting it over
			// width loses nothing; dropping it loses the path the reader
			// needed. The audit counts what that costs.
			cur += w
		case VisibleLen(cur)+1+VisibleLen(w) <= cols:
			cur += " " + w
		default:
			out = append(out, cur)
			cur = indent + w
		}
	}
	return append(out, cur)
}

// looksTabular reports whether a line is a column layout rather than a sentence.
//
// Two or more spaces between two non-space runs is what separates columns here:
// every table in these screens is built by padding cells, and no prose in them
// contains a double space. It is a heuristic, and it is deliberately the
// conservative direction — a sentence mistaken for a table stays too long and
// the audit counts it, whereas a table mistaken for a sentence is silently
// mangled and nothing notices.
func looksTabular(line string) bool {
	s := StripSGR(line)
	t := strings.TrimLeft(s, " ")
	if strings.HasPrefix(strings.TrimSpace(t), "---") || strings.HasPrefix(strings.TrimSpace(t), "===") {
		return true // a rule belongs to the table above it
	}
	gap := 0
	seen := false
	for _, r := range t {
		if r == ' ' {
			gap++
			continue
		}
		if gap >= 2 && seen {
			return true
		}
		gap, seen = 0, true
	}
	return false
}
