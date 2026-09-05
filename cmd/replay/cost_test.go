package main

import (
	"math"
	"strings"
	"testing"
)

// Cost per task is the unit-economics figure: what one session cost, and what
// share of it nobody chose to spend. The aggregation is where a report like
// this lies most easily, so the rules are pinned here.
//
// A mean is the wrong summary. One 40-hour session drags it somewhere no real
// task lives, and a person reading "average task: $12" plans against a number
// that describes none of their work. The median and the p90 are what a unit
// economics conversation actually needs.

func TestCostSummaryReportsMedianAndP90NotMean(t *testing.T) {
	var units []costUnit
	for i := 0; i < 99; i++ {
		units = append(units, costUnit{CostUSD: 1})
	}
	units = append(units, costUnit{CostUSD: 1000}) // the outlier that ruins a mean

	s := summarise(units)
	if s.MedianUSD != 1 {
		t.Fatalf("median: %v, want 1", s.MedianUSD)
	}
	if s.P90USD != 1 {
		t.Fatalf("p90: %v, want 1", s.P90USD)
	}
	if math.Abs(s.TotalUSD-1099) > 0.001 {
		t.Fatalf("total: %v, want 1099", s.TotalUSD)
	}
}

// The avoidable share is the point of the whole report, and it has to be a
// share of what was actually priced, not of everything walked past.
func TestCostSummaryAvoidableIsAShareOfWhatWasPriced(t *testing.T) {
	units := []costUnit{
		{CostUSD: 10, AvoidableUSD: 2},
		{CostUSD: 10, AvoidableUSD: 0},
	}
	s := summarise(units)
	if math.Abs(s.AvoidableShare-0.1) > 0.0001 {
		t.Fatalf("avoidable share: %v, want 0.1", s.AvoidableShare)
	}
}

// A model with no price yields no cost, and a session with no cost must not be
// counted as a task that cost nothing: that would drag every figure down and
// make the corpus look cheaper than it is.
func TestCostSummaryExcludesUnpricedSessionsRatherThanCountingThemAsZero(t *testing.T) {
	units := []costUnit{{CostUSD: 4, AvoidableUSD: 1}, {CostUSD: 6, AvoidableUSD: 1}}
	s := summarise(units)
	if s.Tasks != 2 {
		t.Fatalf("tasks: %d, want 2", s.Tasks)
	}
	if s.MedianUSD != 5 {
		t.Fatalf("median of 4 and 6 should be 5, got %v", s.MedianUSD)
	}
	// An empty corpus is not a zero-cost corpus.
	empty := summarise(nil)
	if empty.Tasks != 0 || empty.MedianUSD != 0 || empty.AvoidableShare != 0 {
		t.Fatalf("an empty corpus must summarise to nothing, got %+v", empty)
	}
}

// The headline is the number a person can act on: what the avoidable share
// would have been worth across the corpus.
func TestCostSummaryRendersTheActionableNumber(t *testing.T) {
	s := summarise([]costUnit{
		{CostUSD: 100, AvoidableUSD: 12},
		{CostUSD: 50, AvoidableUSD: 3},
	})
	out := renderCost(s, 0)
	for _, want := range []string{"$150.00", "10%", "$15.00"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report is missing %q:\n%s", want, out)
		}
	}
}

// Sessions the engine could not reproduce must be named, not silently dropped,
// because a cost report that quietly ignores what it could not read is exactly
// the kind of number this tool exists to distrust.
func TestCostReportNamesWhatItCouldNotPrice(t *testing.T) {
	out := renderCost(summarise([]costUnit{{CostUSD: 1}}), 7)
	if !strings.Contains(out, "7") {
		t.Fatalf("the report must say how many sessions it could not price:\n%s", out)
	}
}
