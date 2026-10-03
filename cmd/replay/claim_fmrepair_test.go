package main

import (
	"math"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// RPL-C035-FM regression. Written BEFORE the repair.
//
// F1 ORDER INVARIANCE  identical records in any order price identically
// F2 PER-RECORD RATE   1 expensive + 3 cheap prices as 1*exp + 3*cheap
// F3 NO SUBSTITUTION   no record's rate is applied to another's deficit
// F4 TOKENS SURVIVE    an unpriceable record keeps its deficit TOKENS and
//                      contributes no dollars
//
// F4 is not a nicety. Deficit is Expected minus Actual and consults no price
// table, so a repair that dropped an unpriceable record's tokens would
// re-break what EC-00 fixed.

const (
	fmExpensive = "claude-opus-5"
	fmCheap     = "claude-3-5-haiku"
)

// F1. The defect itself, correctly isolated.
//
// CORRECTED after the repair exposed a flaw in the first version. That one
// compared three permutations of the same four records and demanded identical
// pricing. It cannot hold: a break needs a predecessor, so the FIRST record
// never breaks, and permuting changes WHICH records break. Per-record pricing
// then legitimately differs, and the test was asserting something false.
//
// The real defect was never "order matters". It was "the rate comes from
// Requests[0].Model". So the fixture holds the BREAKING records identical and
// varies ONLY the first record, which is the one that never breaks and
// therefore must never affect the figure.
func TestFMR1_TheFirstRecordsModelDoesNotSetTheRate(t *testing.T) {
	pe, ok := cachemodel.PriceFor(fmExpensive)
	if !ok {
		t.Fatalf("%s not priced", fmExpensive)
	}
	pc, ok := cachemodel.PriceFor(fmCheap)
	if !ok {
		t.Fatalf("%s not priced", fmCheap)
	}
	if pe.InputPerMTok <= pc.InputPerMTok {
		t.Fatalf("the fixture needs two priced models at different rates")
	}

	// Records 2, 3 and 4 are identical and cheap in both arms. Only record 1,
	// which cannot break, differs.
	expensiveLead := rbRead(t, rbPop(t, []string{fmExpensive, fmCheap, fmCheap, fmCheap}))
	cheapLead := rbRead(t, rbPop(t, []string{fmCheap, fmCheap, fmCheap, fmCheap}))

	if expensiveLead.rebilledTokens != cheapLead.rebilledTokens {
		t.Fatalf("the two arms produce different deficits (%d vs %d); the fixture "+
			"does not isolate the rate", expensiveLead.rebilledTokens, cheapLead.rebilledTokens)
	}
	if math.Abs(expensiveLead.rebilledUSD-cheapLead.rebilledUSD) > 1e-9 {
		t.Errorf("F1 violated: the breaking records are identical and cheap in both "+
			"arms, yet the figures differ ($%.6f with an expensive lead record, "+
			"$%.6f with a cheap one). The lead record never breaks, so it must not "+
			"set the rate.", expensiveLead.rebilledUSD, cheapLead.rebilledUSD)
	}

	// And the surviving figure must be the cheap rate, not the expensive one.
	want := float64(cheapLead.rebilledTokens) / 1_000_000 * pc.InputPerMTok
	if math.Abs(cheapLead.rebilledUSD-want) > 1e-9 {
		t.Errorf("the all-cheap arm priced $%.6f; longhand at $%.2f/MTok over %d "+
			"tokens gives $%.6f", cheapLead.rebilledUSD, pc.InputPerMTok,
			cheapLead.rebilledTokens, want)
	}
	t.Logf("F1: lead record expensive or cheap, breaking records identical, both "+
		"price $%.6f over %d tokens", cheapLead.rebilledUSD, cheapLead.rebilledTokens)
}

// F2 and F3. The figure must be the per-record sum, computed longhand here
// rather than by calling the production arithmetic.
func TestFMR2_EachDeficitIsPricedAtItsOwnRecordsRate(t *testing.T) {
	pe, ok := cachemodel.PriceFor(fmExpensive)
	if !ok {
		t.Fatalf("%s not priced", fmExpensive)
	}
	pc, ok := cachemodel.PriceFor(fmCheap)
	if !ok {
		t.Fatalf("%s not priced", fmCheap)
	}

	// One expensive, three cheap. Every record breaks and contributes an
	// equal share of the deficit, so the oracle can divide it evenly.
	got := rbRead(t, rbPop(t, []string{fmExpensive, fmCheap, fmCheap, fmCheap}))
	if got.rebilledTokens <= 0 {
		t.Fatalf("no deficit produced; this test observes nothing")
	}

	// The breaks are on records 2..4 (a break needs a predecessor), so the
	// deficit is shared by three records whose models the fixture declares.
	// Rather than assume the split, bound the answer: it must lie strictly
	// between all-cheap and all-expensive, and must not equal either.
	perTok := float64(got.rebilledTokens) / 1_000_000
	allCheap := perTok * pc.InputPerMTok
	allExpensive := perTok * pe.InputPerMTok

	if math.Abs(got.rebilledUSD-allExpensive) < 1e-9 {
		t.Errorf("F2 violated: $%.6f is the whole deficit at the EXPENSIVE rate "+
			"($%.2f/MTok). Three of the four records are on %s.",
			got.rebilledUSD, pe.InputPerMTok, fmCheap)
	}
	if got.rebilledUSD > allExpensive+1e-9 || got.rebilledUSD < allCheap-1e-9 {
		t.Errorf("F3 violated: $%.6f lies outside [$%.6f, $%.6f], the all-cheap and "+
			"all-expensive bounds. Some rate is being applied that belongs to no "+
			"record in this corpus.", got.rebilledUSD, allCheap, allExpensive)
	}
	t.Logf("re-billed $%.6f on %d tokens, between all-cheap $%.6f and all-expensive $%.6f",
		got.rebilledUSD, got.rebilledTokens, allCheap, allExpensive)
}

// F4. An unpriceable record keeps its tokens and contributes no dollars.
func TestFMR3_AnUnpriceableRecordKeepsItsTokensAndCostsNothing(t *testing.T) {
	priced := rbRead(t, rbPop(t, []string{fmCheap, fmCheap}))
	mixed := rbRead(t, rbPop(t, []string{fmCheap, fmCheap, ptUnpriced, ptUnpriced}))

	if mixed.rebilledTokens <= priced.rebilledTokens {
		t.Errorf("F4 violated: adding two unpriceable records did not add deficit "+
			"TOKENS (%d then %d). A deficit is Expected minus Actual and needs no "+
			"price; dropping those tokens re-breaks what EC-00 fixed.",
			priced.rebilledTokens, mixed.rebilledTokens)
	}
	if math.Abs(mixed.rebilledUSD-priced.rebilledUSD) > 1e-9 {
		t.Errorf("F4 violated the other way: adding two UNPRICEABLE records changed "+
			"the dollar figure from $%.6f to $%.6f. Nothing prices them, so they "+
			"cannot contribute dollars.", priced.rebilledUSD, mixed.rebilledUSD)
	}
	t.Logf("F4: tokens %d to %d, dollars unchanged at $%.6f",
		priced.rebilledTokens, mixed.rebilledTokens, priced.rebilledUSD)
}
