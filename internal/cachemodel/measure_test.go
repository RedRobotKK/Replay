package cachemodel

import "testing"

// Measuring what a model's minimum cacheable prefix actually is.
//
// This is the content of the maintained feed. The free table carries what the
// provider documents; this carries what replaying real traffic witnessed, and
// the two are different kinds of statement. On 2026-09-05 a price check found
// zero disagreement across ten models against an independent database, 73 days
// after the table was dated — so freshness is not the product. What the
// provider does not publish, and what costs real API spend to establish, is
// where the caching floor actually sits.
//
// The evidence is asymmetric and the tests encode that:
//
//   - A request that WROTE a cache entry proves the floor is at or below its
//     prefix size. Smallest such request is the tightest upper bound.
//   - A request that carried a breakpoint and wrote NOTHING proves the floor is
//     above its prefix size. Largest such request is the tightest lower bound.
//   - A request with no breakpoint proves nothing either way: it was never
//     going to cache, so its silence is not evidence.
//
//   V1  no evidence produces no claim, not a confident one
//   V2  a write bounds the floor from above, and the tightest wins
//   V3  a marked request that did not write bounds it from below
//   V4  an unmarked request is not evidence
//   V5  one counterexample contradicts, whatever the sample size
//   V6  breadth is reported, because agreement over a wide range is worth more
//   V7  incoherent evidence is reported as such rather than averaged away

func ev(model string, prefix int, wrote, marked bool, session, machine string) PrefixEvidence {
	return PrefixEvidence{Model: model, PrefixTokens: prefix, Wrote: wrote, Marked: marked, Session: session, Machine: machine}
}

func TestV1_NoEvidenceProducesNoClaim(t *testing.T) {
	// PASS: untested, and no Observation invented.
	// FAIL: a claim with no evidence behind it, which is the thing this whole
	// schema exists to prevent.
	got := MeasureClaims(nil)
	if len(got) != 0 {
		t.Fatalf("measured %d claims from no evidence", len(got))
	}
	got = MeasureClaims([]PrefixEvidence{ev("claude-opus-5", 5000, false, false, "s1", "m1")})
	c, ok := got["claude-opus-5"]
	if ok && c.Observed != nil && (c.Observed.UpperBound != nil || c.Observed.LowerBound != nil) {
		t.Error("an unmarked request produced a bound; it is not evidence")
	}
	if ok && c.Status() != StatusUntested {
		t.Errorf("status = %v, want untested", c.Status())
	}
}

func TestV2_AWriteBoundsFromAbove(t *testing.T) {
	// PASS: the smallest writing request sets the upper bound.
	// FAIL: any other value — a loose bound understates what was learned, and
	// a bound below the smallest write is not supported by the evidence.
	got := MeasureClaims([]PrefixEvidence{
		ev("claude-opus-5", 9000, true, true, "s1", "m1"),
		ev("claude-opus-5", 2048, true, true, "s1", "m1"),
		ev("claude-opus-5", 6000, true, true, "s2", "m1"),
	})
	c := got["claude-opus-5"]
	if c.Observed == nil || c.Observed.UpperBound == nil {
		t.Fatal("a cache write must bound the floor from above")
	}
	if *c.Observed.UpperBound != 2048 {
		t.Errorf("upperBound = %d, want 2048 (the smallest prompt seen to cache)", *c.Observed.UpperBound)
	}
	if c.Observed.LowerBound != nil {
		t.Error("no marked-and-silent request was seen, so there is no lower bound")
	}
}

func TestV3_AMarkedSilentRequestBoundsFromBelow(t *testing.T) {
	// PASS: the largest marked request that wrote nothing sets the lower bound.
	// FAIL: a smaller one, which throws away evidence.
	got := MeasureClaims([]PrefixEvidence{
		ev("claude-sonnet-5", 100, false, true, "s1", "m1"),
		ev("claude-sonnet-5", 900, false, true, "s1", "m1"),
		ev("claude-sonnet-5", 400, false, true, "s1", "m1"),
	})
	c := got["claude-sonnet-5"]
	if c.Observed == nil || c.Observed.LowerBound == nil {
		t.Fatal("a marked request that did not cache must bound the floor from below")
	}
	if *c.Observed.LowerBound != 900 {
		t.Errorf("lowerBound = %d, want 900 (the largest prompt seen NOT to cache)", *c.Observed.LowerBound)
	}
}

func TestV4_AnUnmarkedRequestIsNotEvidence(t *testing.T) {
	// PASS: unmarked silence is ignored entirely.
	// FAIL: treating it as a lower bound. A request with no breakpoint was
	// never going to cache, so it says nothing about where the floor is — and
	// counting it would push the reported floor arbitrarily high.
	got := MeasureClaims([]PrefixEvidence{
		ev("claude-opus-5", 500_000, false, false, "s1", "m1"),
		ev("claude-opus-5", 300, false, true, "s1", "m1"),
	})
	c := got["claude-opus-5"]
	if c.Observed == nil || c.Observed.LowerBound == nil {
		t.Fatal("the marked request is evidence and must produce a bound")
	}
	if *c.Observed.LowerBound != 300 {
		t.Errorf("lowerBound = %d, want 300; the 500,000-token unmarked request must be ignored", *c.Observed.LowerBound)
	}
}

func TestV5_OneCounterexampleContradicts(t *testing.T) {
	// PASS: a single write below the documented floor contradicts it,
	// regardless of how much evidence agrees.
	// FAIL: requiring a sample size for a refutation. Falsification is
	// asymmetric: one machine caching below the published minimum refutes the
	// figure, while a thousand agreeing with it prove very little.
	docs := DocumentedMinPrefix("claude-opus-5")
	if docs == 0 {
		t.Skip("no documented figure to contradict")
	}
	got := MeasureClaims([]PrefixEvidence{
		ev("claude-opus-5", docs-1, true, true, "s1", "m1"),
	})
	if s := got["claude-opus-5"].Status(); s != StatusContradicted {
		t.Errorf("status = %v, want contradicted: a prompt cached below the documented floor refutes it", s)
	}
}

func TestV6_BreadthIsReported(t *testing.T) {
	// PASS: distinct sessions and machines are counted.
	// FAIL: counting requests instead, which makes one chatty session look
	// like broad evidence. Agreement is only as good as its sampling, and the
	// number that matters is how varied the sources were.
	got := MeasureClaims([]PrefixEvidence{
		ev("claude-opus-5", 3000, true, true, "s1", "m1"),
		ev("claude-opus-5", 3000, true, true, "s1", "m1"),
		ev("claude-opus-5", 4000, true, true, "s2", "m1"),
		ev("claude-opus-5", 5000, true, true, "s3", "m2"),
	})
	o := got["claude-opus-5"].Observed
	if o.Sessions != 3 {
		t.Errorf("sessions = %d, want 3 distinct", o.Sessions)
	}
	if o.Machines != 2 {
		t.Errorf("machines = %d, want 2 distinct", o.Machines)
	}
}

func TestV7_IncoherentEvidenceIsNotAveragedAway(t *testing.T) {
	// A lower bound at or above the upper bound cannot both be true: some
	// request cached at a size another request failed to cache at. That is
	// real and worth surfacing — a per-model floor is not the whole story, and
	// pretending otherwise hides the interesting part.
	// PASS: both bounds preserved, and the incoherence detectable.
	// FAIL: silently dropping one, which manufactures a clean answer from
	// dirty data.
	got := MeasureClaims([]PrefixEvidence{
		ev("claude-opus-5", 1000, true, true, "s1", "m1"),
		ev("claude-opus-5", 4000, false, true, "s2", "m2"),
	})
	o := got["claude-opus-5"].Observed
	if o == nil || o.UpperBound == nil || o.LowerBound == nil {
		t.Fatal("both bounds must survive; dropping one hides the contradiction")
	}
	if *o.LowerBound < *o.UpperBound {
		t.Fatalf("this fixture is meant to be incoherent: lower %d upper %d", *o.LowerBound, *o.UpperBound)
	}
	if !Incoherent(*got["claude-opus-5"].Observed) {
		t.Error("evidence where the floor is both below 1000 and above 4000 must be reported as incoherent")
	}
}

// V8: a model nobody has documented is still measurable.
//
// This is the case that makes the measurement worth paying for. A compiled
// table can only contain models that existed when the binary shipped. A new
// model appears and the table has nothing to say about it — no price, no
// caching floor, no verdict — while traffic against it starts immediately.
//
// The measurement does not need the table. It observes where caching actually
// began, so an unknown model gets a real bound on its first day, and the
// documented figure is reported as absent rather than as zero.
//
// PASS: bounds computed, documented reported as unknown, status untested
// against a figure that does not exist.
// FAIL: skipping unknown models, which is precisely when a reader most needs
// this — or reporting documented as 0, which reads as "caches from the first
// token" and is the most misleading possible answer.
func TestV8_AnUndocumentedModelIsStillMeasured(t *testing.T) {
	const brandNew = "claude-something-6-preview"
	if DocumentedMinPrefix(brandNew) != 0 {
		t.Skipf("%s is unexpectedly in the compiled table", brandNew)
	}
	got := MeasureClaims([]PrefixEvidence{
		ev(brandNew, 800, false, true, "s1", "m1"),
		ev(brandNew, 1600, true, true, "s1", "m1"),
		ev(brandNew, 4096, true, true, "s2", "m2"),
	})
	c, ok := got[brandNew]
	if !ok {
		t.Fatal("a model absent from the table must still be measured; that is when measurement is worth most")
	}
	if c.Observed == nil || c.Observed.UpperBound == nil || c.Observed.LowerBound == nil {
		t.Fatal("both bounds are available from this evidence")
	}
	if *c.Observed.UpperBound != 1600 || *c.Observed.LowerBound != 800 {
		t.Errorf("bounds = (%d, %d], want (800, 1600]", *c.Observed.LowerBound, *c.Observed.UpperBound)
	}
	if c.Documented != 0 {
		t.Errorf("documented = %d; an undocumented model has no published figure", c.Documented)
	}
	// With no documented figure there is nothing to agree or disagree with.
	// Reporting "consistent" here would be a verdict on a claim nobody made.
	if s := c.Status(); s == StatusConsistent || s == StatusContradicted {
		t.Errorf("status = %v; there is no documented figure to be consistent with or to contradict", s)
	}
}
