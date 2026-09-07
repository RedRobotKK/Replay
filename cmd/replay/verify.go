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
	// Unit is what one row on each side is: a session, or an agent lane under
	// --per-lane. It is carried rather than assumed because this report says
	// "task" five times, and a report that says "task" over a lane count is
	// the defect that put one session id on 1014 rows, reappearing in the one
	// output somebody quotes at somebody else.
	Unit string
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

func compare(before, after []costUnit, unit string) comparison {
	c := comparison{BeforeTasks: len(before), AfterTasks: len(after), Unit: unit}
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
	// One noun, and it is the one the rows actually are. Under --per-lane the
	// rows are agent lanes, and a session that fanned out contributes hundreds
	// of them, so "1528 tasks, median $0.63" would overstate the work by an
	// order of magnitude in the one report a reader might take to a meeting.
	noun, nouns := "task", "tasks"
	if c.Unit == unitLane {
		noun, nouns = "agent lane", "agent lanes"
	}
	if !c.Enough {
		fmt.Fprintf(&b, "Too few %s to compare: %d before, %d after, and each side needs at least %d.\n"+
			"No figure is printed rather than a median of noise.\n", nouns, c.BeforeTasks, c.AfterTasks, minSideTasks)
		return b.String()
	}
	fmt.Fprintf(&b, "Cost per %s, before and after.\n\n", noun)
	fmt.Fprintf(&b, "  before   %d %s, median $%.2f\n", c.BeforeTasks, nouns, c.BeforeMedian)
	fmt.Fprintf(&b, "  after    %d %s, median $%.2f\n", c.AfterTasks, nouns, c.AfterMedian)
	fmt.Fprintf(&b, "  change   %+.0f%% per %s, on %+.0f%% %s volume\n", c.MedianDelta*100, noun, c.VolumeDelta*100, noun)
	if math.Abs(c.VolumeDelta) > 0.4 {
		fmt.Fprintf(&b, "\nVolume moved by more than 40%%, so the two periods are not comparable work.\nTreat the per-%s figure with suspicion.\n", noun)
	}
	if predicted != 0 {
		fmt.Fprintf(&b, "\n%s\n", judgePrediction(predicted, c.MedianDelta))
	}
	fmt.Fprintf(&b, "\nThis is list price against transcripts, not your invoice. It is the closest\nhonest proxy available offline; the invoice is the only thing that settles it.\n")
	return b.String()
}
