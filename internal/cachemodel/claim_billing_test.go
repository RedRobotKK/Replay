package cachemodel

import (
	"math"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RPL-C011. The boundary between a figure Replay calculates and a figure a
// provider charged.
//
// The oracle here is deliberately NOT CostLegsUSD. It is the arithmetic
// written out longhand in the test, from the definition, so that a defect
// shared between implementation and oracle cannot hide. If the two ever
// disagree, one of them is wrong and the test says which numbers it used.

const tol = 1e-9

// handCost is the independent reference implementation. Four legs, per the
// published definition of the pricing: uncached input at list, cache writes
// at a TTL-dependent premium over list, cache reads at a multiple of list,
// output at its own rate. Nothing here imports the production calculation.
func handCost(inputTok, write5m, write1h, readTok, outputTok int, inPerM, outPerM, readMult float64) float64 {
	const perMillion = 1_000_000.0
	in := inPerM / perMillion
	uncached := float64(inputTok) * in
	// 5-minute writes bill at 1.25x input, 1-hour writes at 2x. These
	// multipliers are the provider's published ones, written here from the
	// specification rather than read from the code under test.
	write := (float64(write5m)*1.25 + float64(write1h)*2.0) * in
	read := float64(readTok) * readMult * in
	out := float64(outputTok) * outPerM / perMillion
	return uncached + write + read + out
}

// POSITIVE CONTROL for the oracle itself: the production calculation and the
// independent one must agree on a case computed by hand. If they do not, the
// rest of this file is measuring nothing.
func TestC011_IndependentArithmeticAgreesWithProduction(t *testing.T) {
	p := Price{InputPerMTok: 3.0, OutputPerMTok: 15.0, ReadMult: 0.1}
	u := transcript.Usage{Input: 1000, CacheCreation: 2000, CacheRead: 8000, Output: 500}

	got := CostUSD(u, p)
	want := handCost(1000, 2000, 0, 8000, 500, 3.0, 15.0, 0.1)

	if math.Abs(got-want) > tol {
		t.Fatalf("production and independent arithmetic disagree:\n  production  %.12f\n  independent %.12f\n"+
			"One of them is wrong. The independent figure is uncached %.9f + write %.9f + read %.9f + output %.9f.",
			got, want,
			1000*3.0/1e6, 2000*1.25*3.0/1e6, 8000*0.1*3.0/1e6, 500*15.0/1e6)
	}
}

// RPL-C011 and RPL-C012, the claim itself.
//
// The dollar figure is a function of a LOCAL, EDITABLE constant. That is the
// whole proof that it is not a bill: a bill does not change when you edit
// your own copy of the price list, and this figure does.
//
// This is a mutation test on the evidence rather than on the code. The
// mutation is applied to an input the operator controls, and the output
// moves with it, which is exactly the property an authoritative figure must
// not have.
func TestC011_MeasuredUsageTimesLocalPriceIsNotABill(t *testing.T) {
	// Usage held constant. Imagine every token here came from the provider's
	// own counters through the proxy, which is the strongest evidence Replay
	// can have: the "measured" truth tier.
	u := transcript.Usage{Input: 1000, CacheCreation: 2000, CacheRead: 8000, Output: 500}

	listed := Price{InputPerMTok: 3.0, OutputPerMTok: 15.0, ReadMult: 0.1}
	negotiated := Price{InputPerMTok: 2.4, OutputPerMTok: 12.0, ReadMult: 0.1} // a 20% account discount

	atList := CostUSD(u, listed)
	atNegotiated := CostUSD(u, negotiated)

	if math.Abs(atList-atNegotiated) <= tol {
		t.Fatal("the dollar figure did not move when the price table moved. Either " +
			"the price is not an input, or this test is not exercising it, and in " +
			"either case the mutation below proves nothing.")
	}

	// The two figures differ by 20% on the input-priced legs while the usage,
	// the provider and the account are all identical. A figure that can take
	// two values for one set of provider-reported tokens is a reconstruction,
	// not a charge.
	t.Logf("identical provider-reported usage prices at $%.6f on the list table and "+
		"$%.6f on a negotiated one. The usage is measured; the dollar figure is not.",
		atList, atNegotiated)

	// And the direction of the boundary: knowing the usage exactly does not
	// narrow the charge at all unless the price basis is also known to be the
	// one the account was billed on, which Replay has no way to check.
	if atNegotiated >= atList {
		t.Fatalf("expected the discounted basis to produce a lower figure; got %.6f >= %.6f",
			atNegotiated, atList)
	}
}

// The price table is local, dated, and declares its own staleness. That is
// the honest version of the claim and it is worth pinning: the version and
// the checked-at date are what let a reader decide whether to believe the
// dollar figure at all.
func TestC011_ThePriceBasisDeclaresItsOwnProvenanceAndAge(t *testing.T) {
	if PriceTableVersion == "" {
		t.Error("the price table carries no version, so a reader cannot tell which " +
			"basis produced a figure")
	}
	if PriceTableCheckedAt == "" {
		t.Error("the price table carries no checked-at date, so a reader cannot tell " +
			"how stale the basis is")
	}
	// The staleness warning must be REACHABLE, or the declaration is
	// decorative. Positive control: a clock far past the checked-at date must
	// produce a note. Negative control: the checked-at date itself must not.
	checked, err := time.Parse("2006-01-02", PriceTableCheckedAt)
	if err != nil {
		t.Fatalf("checked-at date %q does not parse, so staleness cannot be computed: %v",
			PriceTableCheckedAt, err)
	}
	if note := PriceTableAgeNote(checked.AddDate(2, 0, 0)); note == "" {
		t.Error("a price table two years past its check date emits no staleness note. " +
			"A dollar figure computed on it would carry no warning at all.")
	}
	if note := PriceTableAgeNote(checked); note != "" {
		t.Errorf("a freshly checked table emits a staleness note %q; the warning "+
			"cannot distinguish stale from current", note)
	}
}
