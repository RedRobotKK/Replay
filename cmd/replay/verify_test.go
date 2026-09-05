package main

import (
	"math"
	"strings"
	"testing"
	"time"
)

// The one experiment that would reopen the business question: apply the advice,
// then check whether the next period's cost actually moved by what was
// predicted. Everything else either measures the instrument against itself or
// asks people what they think.
//
// The trap the whole thing turns on: total spend falling proves nothing,
// because it also falls when you do less work. The unit has to be cost per
// task, and the volume on each side has to be printed so a reader can see the
// mix for themselves.

func TestComparePricesPerTaskNotInTotal(t *testing.T) {
	// Half the work, same efficiency. Total spend halves; nothing improved.
	var before, after []costUnit
	for i := 0; i < 20; i++ {
		before = append(before, costUnit{CostUSD: 1})
	}
	for i := 0; i < 10; i++ {
		after = append(after, costUnit{CostUSD: 1})
	}

	c := compare(before, after)
	if math.Abs(c.MedianDelta) > 0.001 {
		t.Fatalf("doing half as much work reported a %.1f%% saving", c.MedianDelta*100)
	}
	if !strings.Contains(renderCompare(c, 0), "50%") {
		t.Fatalf("the volume change must be visible:\n%s", renderCompare(c, 0))
	}
}

func TestCompareDetectsARealPerTaskImprovement(t *testing.T) {
	var before, after []costUnit
	for i := 0; i < 12; i++ {
		before = append(before, costUnit{CostUSD: 1.00})
		after = append(after, costUnit{CostUSD: 0.80})
	}

	c := compare(before, after)
	if math.Abs(c.MedianDelta-(-0.20)) > 0.001 {
		t.Fatalf("want a 20%% fall in cost per task, got %.1f%%", c.MedianDelta*100)
	}
}

// A prediction is only confirmed when the realised move lands near it. The
// band is wide on purpose: this is an estimate against a real bill, and a tool
// that declares itself right on a 2% coincidence is worth nothing.
func TestCompareJudgesAPredictionHonestly(t *testing.T) {
	cases := []struct {
		name              string
		predicted, actual float64
		want              string
	}{
		{"close enough", -0.20, -0.17, "confirmed"},
		{"predicted far too much", -0.20, -0.05, "not confirmed"},
		{"went the wrong way", -0.20, 0.10, "not confirmed"},
		{"beat the prediction", -0.20, -0.45, "not confirmed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := judgePrediction(c.predicted, c.actual)
			if !strings.Contains(got, c.want) {
				t.Fatalf("predicted %.0f%% actual %.0f%%: got %q, want it to say %q",
					c.predicted*100, c.actual*100, got, c.want)
			}
		})
	}
}

// Two sessions on one side is an anecdote. Refuse rather than publish a median
// of noise, because this is the number the whole argument would rest on.
func TestCompareRefusesTooLittleEvidence(t *testing.T) {
	c := compare([]costUnit{{CostUSD: 1}}, []costUnit{{CostUSD: 1}})
	out := renderCompare(c, 0)
	if !strings.Contains(strings.ToLower(out), "too few") {
		t.Fatalf("want a refusal on thin evidence, got:\n%s", out)
	}
	if strings.Contains(out, "%") && !strings.Contains(strings.ToLower(out), "too few") {
		t.Fatal("it must not print a percentage it cannot stand behind")
	}
}

func TestSplitByDatePutsEachSessionOnOneSide(t *testing.T) {
	cut := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	units := []costUnit{
		{CostUSD: 1, At: cut.Add(-48 * time.Hour)},
		{CostUSD: 1, At: cut.Add(-1 * time.Hour)},
		{CostUSD: 1, At: cut},
		{CostUSD: 1, At: cut.Add(72 * time.Hour)},
	}
	before, after := splitAt(units, cut)
	if len(before) != 2 || len(after) != 2 {
		t.Fatalf("split %d before / %d after, want 2/2", len(before), len(after))
	}
}
