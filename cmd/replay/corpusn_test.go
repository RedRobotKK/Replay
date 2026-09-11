package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
)

// A rate without its denominator is not a measurement anyone can weigh.
//
// The per-model table reported "87.5% recent exact" for claude-haiku-4-5 and
// "93.3%" for claude-opus-5, and marked both calibrated. The first is seven of
// EIGHT turns. The second is 539. Nothing in the row let a reader tell them
// apart, and 87.5% is exactly 7/8 — a shape a reviewer spotted from the number
// alone, which is the point: the table was inviting that guess instead of
// answering it.
//
// This came from a review challenge on the exact/match split, and the
// challenge was right that the split alone does not fix it:
// CalibrationThreshold has no minimum sample size, so a lane clears 95% on
// eight turns as easily as on eight hundred. Splitting one unweighable number
// into two leaves both unweighable. Showing n is the part that can be done
// without changing which lanes pass.
func TestCN1_ThePerModelTableShowsItsDenominator(t *testing.T) {
	models := []analysis.ModelCalibration{{
		Model:          "claude-haiku-4-5-20251001",
		Sessions:       42,
		Compared:       900,
		Matched:        891,
		Exact:          867,
		RecentSessions: 5,
		RecentCompared: 8,
		RecentMatched:  8,
		RecentExact:    7,
	}}
	var sb strings.Builder
	if err := writeCorpus(&sb, nil, models, nil); err != nil {
		t.Fatal(err)
	}
	got := sb.String()

	if !strings.Contains(got, "Recent turns") {
		t.Errorf("the per-model table has no column for the recent window's size:\n%s", got)
	}
	// The number itself, not just the heading.
	if !strings.Contains(got, "| 5 | 8 |") {
		t.Errorf("the recent turn count is not printed. A reader cannot tell 7 of 8 from "+
			"700 of 800, and both render as a rate near 87.5%%:\n%s", got)
	}
}
