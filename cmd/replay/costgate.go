package main

import (
	"errors"
	"fmt"
	"io"
)

// Failing a build on re-billed spend.
//
// The gate compares one figure `replay cost` already measured against a ceiling
// the caller types. It derives nothing of its own, and that is the whole design:
// a governance check whose numbers come from anywhere but the measurement can be
// confidently right about a corpus nobody ran.
//
// It is opt-in. Without the flag the cost report behaves exactly as before,
// because a report that started failing builds on upgrade would be a breaking
// change delivered as a patch.

// errGate is returned when measured re-billed spend exceeds the ceiling. It is
// an error so a shell gates on the exit code without parsing prose.
var errGate = errors.New("re-billed spend is over the ceiling")

// errNotMeasured is the gate declining to answer rather than finding anything.
//
// It was indistinguishable from errGate to a shell until 2026-09-13: both
// returned 1, so a CI runner with no transcripts failed a merge exactly as
// agents that had wasted money did. Those are opposite situations, and the
// second usually means a path is wrong rather than that anything is expensive.
// See the exit code contract in main.go.
var errNotMeasured = errors.New("NOT MEASURED")

// checkRebilledCeiling enforces --max-rebilled-usd against a measured summary.
//
// The nothing-measured case is the one worth reading. `replay cost` exits 0 on a
// corpus it could not price and explains itself in prose, so a gate that trusted
// the exit code would go green on a CI runner with no transcripts and report a
// clean bill of health nobody earned. Zero priced tasks is refused, loudly,
// because an absence is not a number under a ceiling.
func checkRebilledCeiling(ceiling float64, s costSummary, unpriced, unreadable int, stdout io.Writer) error {
	if ceiling == 0 {
		return nil // not asked for
	}
	if ceiling < 0 {
		return fmt.Errorf("--max-rebilled-usd needs a positive ceiling, got %.2f", ceiling)
	}

	if s.Tasks == 0 {
		_, _ = fmt.Fprintf(stdout,
			"\n  GATE: NOT MEASURED. This corpus priced nothing, so there is no re-billed\n"+
				"  spend to compare against $%.2f. A ceiling met by measuring nothing is not\n"+
				"  a ceiling met.\n", ceiling)
		return fmt.Errorf("refusing to pass a gate over 0 priced %s: %w", s.Unit, errNotMeasured)
	}

	if s.RebilledUSD > ceiling {
		_, _ = fmt.Fprintf(stdout,
			"\n  GATE: re-billed spend $%.2f is over the $%.2f ceiling, across %d %s.\n"+
				"  That is spend nobody chose: context re-billed because a prompt cache broke.\n"+
				"  For the turn it happened on:  replay diff <transcript>\n",
			s.RebilledUSD, ceiling, s.Tasks, s.Unit)
		if unpriced > 0 {
			// The total the ceiling was compared against has holes in it, and a
			// reader deciding whether to trust a failed build needs to know how
			// many. Excluded is not free.
			_, _ = fmt.Fprintf(stdout,
				"  %d transcript(s) were excluded as unpriced, so the real figure is higher.\n", unpriced)
		}
		if unreadable > 0 {
			// Same hole, different cause. A transcript the parser could not
			// read is excluded from the total this ceiling was compared
			// against exactly as completely as an unpriced one, and until
			// this line it was named on no surface at all.
			_, _ = fmt.Fprintf(stdout,
				"  %d transcript(s) could not be read at all, so the real figure is higher still.\n", unreadable)
		}
		return fmt.Errorf("%w: $%.2f over $%.2f", errGate, s.RebilledUSD, ceiling)
	}

	_, _ = fmt.Fprintf(stdout, "\n  GATE: re-billed spend $%.2f is within the $%.2f ceiling.\n",
		s.RebilledUSD, ceiling)
	return nil
}
