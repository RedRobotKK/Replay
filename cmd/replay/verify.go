package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// The experiment that would settle whether any of this is worth money: apply
// the advice, wait, and check whether cost per task actually moved by what was
// predicted.
//
// Everything else available today either measures the engine against its own
// model, or asks people what they believe. Neither is evidence. A provider
// invoice is evidence, and the closest honest proxy to one is what the same
// work costs before and after.
//
// The trap this is built around: total spend falling proves nothing, because
// it also falls when you simply do less. So the unit is cost per task and the
// volume on both sides is printed, so a reader can judge the mix themselves
// rather than take the headline on trust.

type comparison struct {
	BeforeTasks  int
	AfterTasks   int
	BeforeMedian float64
	AfterMedian  float64
	// MedianDelta is the fractional change in cost per task. Negative is a
	// saving.
	MedianDelta float64
	// VolumeDelta is the fractional change in how many tasks were run, which
	// is the first thing a sceptic should look at.
	VolumeDelta float64
	Enough      bool
}

// minSideTasks is the fewest tasks either side may have. Two sessions is an
// anecdote, and this is the figure an argument would rest on.
const minSideTasks = 10

func splitAt(units []costUnit, cut time.Time) (before, after []costUnit) {
	for _, u := range units {
		if u.At.Before(cut) {
			before = append(before, u)
			continue
		}
		after = append(after, u)
	}
	return before, after
}

func compare(before, after []costUnit) comparison {
	c := comparison{BeforeTasks: len(before), AfterTasks: len(after)}
	c.Enough = len(before) >= minSideTasks && len(after) >= minSideTasks

	med := func(us []costUnit) float64 {
		if len(us) == 0 {
			return 0
		}
		costs := make([]float64, 0, len(us))
		for _, u := range us {
			costs = append(costs, u.CostUSD)
		}
		sort.Float64s(costs)
		return percentile(costs, 0.5)
	}
	c.BeforeMedian, c.AfterMedian = med(before), med(after)
	if c.BeforeMedian > 0 {
		c.MedianDelta = (c.AfterMedian - c.BeforeMedian) / c.BeforeMedian
	}
	if len(before) > 0 {
		c.VolumeDelta = (float64(len(after)) - float64(len(before))) / float64(len(before))
	}
	return c
}

// judgePrediction says whether a realised move confirms a predicted one.
//
// The band is deliberately wide and symmetric. Wide, because this is an
// estimate compared against real spend and a tool that claims to be right on a
// two percent coincidence is worth nothing. Symmetric, because beating a
// prediction badly is also a failed prediction: it means the model does not
// understand the effect it is claiming.
func judgePrediction(predicted, actual float64) string {
	if predicted == 0 {
		return "no prediction was made, so there is nothing to confirm"
	}
	ratio := actual / predicted
	switch {
	case ratio >= 0.7 && ratio <= 1.3:
		return fmt.Sprintf("confirmed: predicted %.0f%%, realised %.0f%%", predicted*100, actual*100)
	case actual*predicted <= 0:
		return fmt.Sprintf("not confirmed: predicted %.0f%%, cost moved the other way (%.0f%%)", predicted*100, actual*100)
	default:
		return fmt.Sprintf("not confirmed: predicted %.0f%%, realised %.0f%%", predicted*100, actual*100)
	}
}

func renderCompare(c comparison, predicted float64) string {
	var b strings.Builder
	if !c.Enough {
		fmt.Fprintf(&b, "Too few tasks to compare: %d before, %d after, and each side needs at least %d.\n"+
			"No figure is printed rather than a median of noise.\n", c.BeforeTasks, c.AfterTasks, minSideTasks)
		return b.String()
	}
	fmt.Fprintf(&b, "Cost per task, before and after.\n\n")
	fmt.Fprintf(&b, "  before   %d tasks, median $%.2f\n", c.BeforeTasks, c.BeforeMedian)
	fmt.Fprintf(&b, "  after    %d tasks, median $%.2f\n", c.AfterTasks, c.AfterMedian)
	fmt.Fprintf(&b, "  change   %+.0f%% per task, on %+.0f%% task volume\n", c.MedianDelta*100, c.VolumeDelta*100)
	if math.Abs(c.VolumeDelta) > 0.4 {
		fmt.Fprintf(&b, "\nVolume moved by more than 40%%, so the two periods are not comparable work.\nTreat the per-task figure with suspicion.\n")
	}
	if predicted != 0 {
		fmt.Fprintf(&b, "\n%s\n", judgePrediction(predicted, c.MedianDelta))
	}
	fmt.Fprintf(&b, "\nThis is list price against transcripts, not your invoice. It is the closest\nhonest proxy available offline; the invoice is the only thing that settles it.\n")
	return b.String()
}
