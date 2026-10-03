package main

import "testing"

// RELEASE BLOCKER, 1.0 hardening campaign.
//
// `replay verify` is the one figure in this product meant to face an invoice:
// it compares the median task cost before and after a change. cost.go:447-456
// already states the governing rule for its own median, in this same binary on
// this same struct:
//
//	"Admitting an unpriced row's zero would move the median and the p90 by
//	 asserting that a session nobody could price cost nothing, which is the
//	 same false zero one column over."
//
// compare() does not apply it. A session nobody could price enters the median
// as $0.00 and counts toward the minSideTasks evidence gate.
//
// The predicate used here is PricedRequests == 0, not the Unpriced flag. Q01
// established that the flag on a FOLDED row is inherited from whichever lane
// was seen first, so gating on it would make this figure depend on lane
// ordering and cache state. PricedRequests is summed in the fold and is immune
// to both.

// vUnit builds one row: cost, and how many of its records could be priced.
func vUnit(cost float64, priced, unpriced int) costUnit {
	return costUnit{CostUSD: cost, PricedRequests: priced, UnpricedRequests: unpriced}
}

// Ten priceable rows at $1.00, so the median of the priceable population is
// unambiguously $1.00 on both sides.
func vTen(cost float64) []costUnit {
	out := make([]costUnit, 0, 10)
	for i := 0; i < 10; i++ {
		out = append(out, vUnit(cost, 3, 0))
	}
	return out
}

// V1. An unpriceable row must not drag the median toward zero.
func TestV1_AnUnpriceableRowIsNotAFreeRow(t *testing.T) {
	clean := compare(vTen(1.00), vTen(1.00), unitSession)
	if clean.BeforeMedian != 1.00 || clean.AfterMedian != 1.00 {
		t.Fatalf("control: a corpus of ten $1.00 rows must have median $1.00 on both "+
			"sides; got before $%.4f after $%.4f", clean.BeforeMedian, clean.AfterMedian)
	}
	if clean.MedianDelta != 0 {
		t.Fatalf("control: identical sides must show no movement; got %.4f", clean.MedianDelta)
	}

	// Same corpus, plus ELEVEN rows on the AFTER side that nobody could price.
	// They cost an unknown amount, not nothing.
	//
	// Eleven, not five. The first version of this test added five, which put
	// the median of the 15 sorted values at index 7 and left it on $1.00: the
	// test passed without the defect ever reaching the figure it was written to
	// protect. Eleven makes the zeros the majority, so a false zero that is
	// admitted lands on the median and the assertion can fail.
	after := vTen(1.00)
	for i := 0; i < 11; i++ {
		after = append(after, vUnit(0, 0, 4))
	}
	got := compare(vTen(1.00), after, unitSession)

	if got.AfterMedian != 1.00 {
		t.Errorf("the after-median is $%.4f. Five rows nobody could price were "+
			"admitted at $0.00 and pulled it off $1.00. cost.go:447-456 forbids "+
			"exactly this for its own median: an unpriced row's zero asserts that a "+
			"session nobody could price cost nothing.", got.AfterMedian)
	}
	if got.MedianDelta != 0 {
		t.Errorf("verify reports a %.1f%% median move between two sides whose "+
			"priceable rows are identical. That is a cost claim manufactured out of "+
			"missing evidence, and verify is the figure meant to face an invoice.",
			got.MedianDelta*100)
	}
}

// V2. The evidence gate must not be satisfied by rows that can contribute no
// cost. Ten unpriceable rows are not ten measurements.
func TestV2_UnpriceableRowsDoNotSatisfyTheEvidenceGate(t *testing.T) {
	before := vTen(1.00)
	after := make([]costUnit, 0, 10)
	for i := 0; i < 10; i++ {
		after = append(after, vUnit(0, 0, 4))
	}
	got := compare(before, after, unitSession)
	if got.Enough {
		t.Errorf("verify reports Enough=true with %d after-rows, none of which could "+
			"be priced. minSideTasks exists because two sessions is an anecdote; ten "+
			"rows that contribute no cost are not ten measurements.", got.AfterTasks)
	}
}

// V3. NEGATIVE CONTROL. A genuine $0.00 that WAS priced is a measurement and
// must still count, or the repair has replaced one false zero with a missing
// true one.
func TestV3_AGenuineZeroStillCounts(t *testing.T) {
	before := vTen(1.00)
	after := make([]costUnit, 0, 10)
	for i := 0; i < 10; i++ {
		after = append(after, vUnit(0, 3, 0)) // priced, and genuinely cost nothing
	}
	got := compare(before, after, unitSession)
	if !got.Enough {
		t.Errorf("ten priced rows that genuinely cost $0.00 are ten measurements and " +
			"must satisfy the gate; Enough=false")
	}
	if got.AfterMedian != 0 {
		t.Errorf("a priced $0.00 row is a measured zero and must enter the median; "+
			"after-median $%.4f", got.AfterMedian)
	}
}
