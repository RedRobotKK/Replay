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

// rebilledFigureCaveats names every way the figure a ceiling was compared
// against falls short of the work it appears to describe.
//
// One helper rather than a sentence per branch, because the gate's PASS verdict
// used to say nothing while its FAIL verdict named two of these, and a reader
// deciding whether to trust a GREEN build needs them more than one deciding
// whether to trust a red one: a pass is exactly the verdict excluded spend
// could flip.
//
// Four holes, deliberately kept apart rather than summed. They have different
// causes and different remedies, and a single "coverage" number would say which
// of them nobody could act on.
func rebilledFigureCaveats(s costSummary, unpriced, unreadable int) []string {
	var out []string
	if unpriced > 0 {
		out = append(out, fmt.Sprintf(
			"%d transcript(s) were excluded as unpriced, so the real figure is higher.", unpriced))
	}
	if unreadable > 0 {
		out = append(out, fmt.Sprintf(
			"%d transcript(s) could not be read at all, so the real figure is higher still.", unreadable))
	}
	// Record granularity, which the two above do not reach: these are records
	// INSIDE transcripts that did price. RPL-C035.
	if n := s.PricedRequests + s.UnpricedRequests; s.UnpricedRequests > 0 && n > 0 {
		out = append(out, fmt.Sprintf(
			"%d of %d requests read priced nothing, so this figure covers part of the work.",
			s.UnpricedRequests, n))
	}
	// Break granularity, which none of the above reach: these are re-billed
	// TOKENS outside the re-billed DOLLARS. RPL-C037.
	if s.UnpricedRebilledTokens > 0 && s.RebilledTokens > 0 {
		out = append(out, fmt.Sprintf(
			"%d of %d re-billed tokens are outside this figure, on a model no price table carries.",
			s.UnpricedRebilledTokens, s.RebilledTokens))
	}
	return out
}

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
		for _, c := range rebilledFigureCaveats(s, unpriced, unreadable) {
			_, _ = fmt.Fprintf(stdout, "  %s\n", c)
		}
		return fmt.Errorf("%w: $%.2f over $%.2f", errGate, s.RebilledUSD, ceiling)
	}

	_, _ = fmt.Fprintf(stdout, "\n  GATE: re-billed spend $%.2f is within the $%.2f ceiling.\n",
		s.RebilledUSD, ceiling)
	// The same holes the failing verdict names. A pass over a partial figure is
	// a pass the excluded part could overturn, so it is reported here rather
	// than only where the build was going to fail anyway.
	if caveats := rebilledFigureCaveats(s, unpriced, unreadable); len(caveats) > 0 {
		_, _ = fmt.Fprintf(stdout, "  The figure it was compared against is not complete:\n")
		for _, c := range caveats {
			_, _ = fmt.Fprintf(stdout, "    %s\n", c)
		}
	}
	return nil
}
