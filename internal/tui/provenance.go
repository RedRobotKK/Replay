package tui

import "strings"

// Where a screen's figures came from.
//
// The surface shipped with nine hardcoded answers and forty hardcoded rows, and
// nothing on screen said so. A user running `replay tui` saw "$2.41 of $5.00,
// NOT ENFORCED" and had no way to tell it was not their machine.
//
// That is the defect this project exists to refuse, arriving from the inside. A
// figure with nothing behind it is the thing the installer was corrected for
// twice tonight, and putting one on a screen is worse than putting it in a
// document, because a screen looks like an instrument.
//
// So every frame declares its provenance and the screen says it out loud.

// Provenance is how much a screen's numbers are worth.
type Provenance int

const (
	// Measured means the figures came from this machine.
	Measured Provenance = iota
	// Example means they are illustrative and describe nobody.
	Example
	// Unavailable means the screen would be measured and the source is not
	// there: no ledger, no running proxy, no transcripts.
	Unavailable
)

// Banner is the line a screen carries when its numbers are not measured.
//
// Above the figures rather than below them, because a caveat under a table is a
// caveat read after the number has already been believed. Empty for measured
// screens: a surface that stamps "real" on real data teaches the reader to skim
// the stamp.
func Banner(p Provenance, what string) string {
	switch p {
	case Example:
		return "  [NOTE] example data: describes nobody, shows only the shape."
	case Unavailable:
		s := "  [NOTE] not measured here: " + what
		if len(s) > BudgetCols {
			s = s[:BudgetCols-1] + string(truncationMark)
		}
		return s
	default:
		return ""
	}
}

// WithBanner puts the provenance line above a frame's body, under its title.
func WithBanner(lines []string, p Provenance, what string) []string {
	b := Banner(p, what)
	if b == "" {
		return lines
	}
	// After the title row and its blank, so the screen still opens with what it
	// is before it says what its numbers are worth.
	at := 2
	if len(lines) < at {
		at = len(lines)
	}
	out := make([]string, 0, len(lines)+2)
	out = append(out, lines[:at]...)
	out = append(out, b, "")
	out = append(out, lines[at:]...)
	return out
}

// Marked reports whether a rendered frame declares unmeasured figures, which is
// what the test asserts rather than trusting the caller to have called Banner.
func Marked(lines []string) bool {
	for _, l := range lines {
		if strings.Contains(l, "[NOTE] example data") ||
			strings.Contains(l, "[NOTE] not measured here") {
			return true
		}
	}
	return false
}
