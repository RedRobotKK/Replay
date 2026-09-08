package main

import (
	"errors"
	"fmt"
	"io"
)

// Failing a build on avoidable spend.
//
// The gate compares one figure `replay cost` already measured against a ceiling
// the caller types. It derives nothing of its own, and that is the whole design:
// a governance check whose numbers come from anywhere but the measurement can be
// confidently right about a corpus nobody ran.
//
// It is opt-in. Without the flag the cost report behaves exactly as before,
// because a report that started failing builds on upgrade would be a breaking
// change delivered as a patch.

// errGate is returned when measured avoidable spend exceeds the ceiling. It is
// an error so a shell gates on the exit code without parsing prose.
var errGate = errors.New("avoidable spend is over the ceiling")

// checkAvoidableCeiling enforces --max-avoidable-usd against a measured summary.
//
// The nothing-measured case is the one worth reading. `replay cost` exits 0 on a
// corpus it could not price and explains itself in prose, so a gate that trusted
// the exit code would go green on a CI runner with no transcripts and report a
// clean bill of health nobody earned. Zero priced tasks is refused, loudly,
// because an absence is not a number under a ceiling.
func checkAvoidableCeiling(ceiling float64, s costSummary, unpriced int, stdout io.Writer) error {
	if ceiling == 0 {
		return nil // not asked for
	}
	if ceiling < 0 {
		return fmt.Errorf("--max-avoidable-usd needs a positive ceiling, got %.2f", ceiling)
	}

	if s.Tasks == 0 {
		_, _ = fmt.Fprintf(stdout,
			"\n  GATE: NOT MEASURED. This corpus priced nothing, so there is no avoidable\n"+
				"  spend to compare against $%.2f. A ceiling met by measuring nothing is not\n"+
				"  a ceiling met.\n", ceiling)
		return fmt.Errorf("refusing to pass a gate over 0 priced %s: NOT MEASURED", s.Unit)
	}

	if s.AvoidableUSD > ceiling {
		_, _ = fmt.Fprintf(stdout,
			"\n  GATE: avoidable spend $%.2f is over the $%.2f ceiling, across %d %s.\n"+
				"  That is spend nobody chose: context re-billed because a prompt cache broke.\n"+
				"  For the turn it happened on:  replay diff <transcript>\n",
			s.AvoidableUSD, ceiling, s.Tasks, s.Unit)
		if unpriced > 0 {
			// The total the ceiling was compared against has holes in it, and a
			// reader deciding whether to trust a failed build needs to know how
			// many. Excluded is not free.
			_, _ = fmt.Fprintf(stdout,
				"  %d transcript(s) were excluded as unpriced, so the real figure is higher.\n", unpriced)
		}
		return fmt.Errorf("%w: $%.2f over $%.2f", errGate, s.AvoidableUSD, ceiling)
	}

	_, _ = fmt.Fprintf(stdout, "\n  GATE: avoidable spend $%.2f is within the $%.2f ceiling.\n",
		s.AvoidableUSD, ceiling)
	return nil
}
