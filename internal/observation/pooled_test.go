package observation

import (
	"testing"
	"time"
)

// The vendored pool is a copy of something published elsewhere, so the tests
// here are about the ways a copy goes wrong.

// PV-1: a tag is never an identity in anything the CLI emits, and the vendored
// snapshot must not claim otherwise. If this ever flips, every sentence that
// says "machine tags, never people" becomes wrong at once.
func TestPV1_TagsAreNotIdentities(t *testing.T) {
	if Pooled().TagsAreIdentities {
		t.Error("the vendored pool claims its tags are identities; nothing the CLI emits supports that")
	}
}

// PV-2: the snapshot is internally consistent, because a hand-edited figure is
// the failure mode this file exists to catch.
func TestPV2_InternallyConsistent(t *testing.T) {
	p := Pooled()
	if p.Submissions < 1 || p.DistinctTags < 1 {
		t.Fatalf("a pool with %d submission(s) and %d tag(s) is not a pool", p.Submissions, p.DistinctTags)
	}
	if p.DistinctTags > p.Submissions {
		t.Errorf("%d distinct tags across %d submissions is impossible", p.DistinctTags, p.Submissions)
	}
	if p.AvoidableUSD > p.TotalUSD {
		t.Errorf("avoidable $%.2f exceeds total $%.2f", p.AvoidableUSD, p.TotalUSD)
	}
	if p.TotalUSD > 0 {
		want := p.AvoidableUSD / p.TotalUSD
		if d := p.AvoidableShare - want; d > 1e-9 || d < -1e-9 {
			t.Errorf("AvoidableShare is %.9f but avoidable/total is %.9f; one was typed by hand", p.AvoidableShare, want)
		}
	}
	if p.MedianTaskUSDLow > p.MedianTaskUSDHigh {
		t.Errorf("median span runs backwards: $%.4f to $%.4f", p.MedianTaskUSDLow, p.MedianTaskUSDHigh)
	}
}

// PV-3: the dates are real dates, and the pool was not pooled before the prices
// it was priced against were read.
func TestPV3_DatesAreRealAndOrdered(t *testing.T) {
	pooled, err := time.Parse("2006-01-02", PooledAt)
	if err != nil {
		t.Fatalf("PooledAt %q is not a date: %v", PooledAt, err)
	}
	p := Pooled()
	priced, err := time.Parse("2006-01-02", p.PricedAtHigh)
	if err != nil {
		t.Fatalf("PricedAtHigh %q is not a date: %v", p.PricedAtHigh, err)
	}
	if pooled.Before(priced) {
		t.Errorf("pooled %s precedes the price table it used, %s", PooledAt, p.PricedAtHigh)
	}
	if p.PricedAtLow > p.PricedAtHigh {
		t.Errorf("priced-at span runs backwards: %s to %s", p.PricedAtLow, p.PricedAtHigh)
	}
}

// PV-4: the vendored snapshot is asserted by value, not by agreement with a
// constant.
//
// The first version of this compared DistinctTags against BenchmarkMinTags and
// accepted either outcome, so it asserted only that the code agrees with itself.
// It could not catch a wrong floor or a mis-vendored count, while its comment
// claimed it was the thing that would make somebody confirm a change was real.
//
// Asserting the value means the day the pool genuinely grows, this test fails
// and somebody reads the diff. That is the point: the comparison branch is
// unreachable today and its first live execution must not be unreviewed.
func TestPV4_VendoredValuesAreAssertedByValue(t *testing.T) {
	p := Pooled()
	if p.Submissions != 1 || p.DistinctTags != 1 {
		t.Fatalf("the vendored pool is now %d submission(s) and %d tag(s), not 1 and 1.\n"+
			"If that is a real re-vendoring, read the rendered comparison before updating this\n"+
			"test: every branch below BenchmarkMinTags=%d has never run against real data.",
			p.Submissions, p.DistinctTags, BenchmarkMinTags)
	}
	if len(p.MedianTaskUSDs) != p.Submissions {
		t.Errorf("%d submission(s) but %d per-submission median(s); a rank needs one each",
			p.Submissions, len(p.MedianTaskUSDs))
	}
	mine := Benchmark{MedianTaskUSD: usd(0.40), Tasks: 500}
	if mine.Against(p).Comparable {
		t.Error("one machine tag was treated as a population")
	}
}

// PV-5: the vendored medians agree with the bracket they were summarised into.
func TestPV5_MediansAgreeWithTheBracket(t *testing.T) {
	p := Pooled()
	if len(p.MedianTaskUSDs) == 0 {
		t.Skip("no per-submission medians vendored")
	}
	lo, hi := p.MedianTaskUSDs[0], p.MedianTaskUSDs[0]
	for _, v := range p.MedianTaskUSDs {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if lo != p.MedianTaskUSDLow || hi != p.MedianTaskUSDHigh {
		t.Errorf("vendored bracket is $%.7f..$%.7f but the medians run $%.7f..$%.7f; one was typed by hand",
			p.MedianTaskUSDLow, p.MedianTaskUSDHigh, lo, hi)
	}
}
