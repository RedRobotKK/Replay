package advisor

import (
	"math"
	"math/rand"
	"testing"
)

// Conformance for the surfaces that are statistical rather than deterministic.
//
// The wire-parsing suites assert values: 9630 inclusive of 9600 cached leaves
// 30 fresh, and anything else is a defect. Nothing here can be asserted that
// way. Sigma, the byte-to-token fit and the spend fence are estimates over
// samples, and their correct output changes with the sample.
//
// So the pass/fail conditions are about the DECISION PROCEDURE, not the number:
// when it must refuse, what it must never do regardless of the draw, and how it
// behaves as evidence accumulates. Those are as falsifiable as an equality and
// they are what actually protects a user, because the failure mode of a
// statistical surface is not a wrong digit — it is a confident answer built on
// nothing.
//
//	P1  refusal is monotone in evidence: below the gate it refuses, at and
//	    above it answers, never the reverse
//	P2  a refusal says what was missing, in numbers
//	P3  degenerate input refuses rather than emitting a confident point
//	P4  the same input gives the same answer, every time
//	P5  the estimate tracks a known ground truth on synthetic data
//	P6  uncertainty grows as the sample shrinks; it never shrinks to zero
//	P7  no output is silently zero — refusal and "measured as none" are
//	    distinguishable by the caller

// ---------------------------------------------------------------- the fence

// P1. UpperFence takes a floor. Below it, refusal; at it, an answer. A gate
// that lets one sample through is not a gate.
func TestFenceRefusalIsMonotoneInSampleCount(t *testing.T) {
	const floor = 8
	// A spread that is genuinely usable, so only the count is under test.
	pool := []float64{0.10, 0.25, 0.40, 0.55, 0.70, 0.95, 1.30, 2.10, 3.40, 5.00}

	firstOK := -1
	for n := 0; n <= len(pool); n++ {
		_, ok := UpperFence(pool[:n], floor)
		if ok && firstOK < 0 {
			firstOK = n
		}
		if !ok && firstOK >= 0 {
			t.Fatalf("P1 refusal is not monotone: answered at n=%d then refused at n=%d. "+
				"More evidence must never make the tool less willing", firstOK, n)
		}
	}
	if firstOK < 0 {
		t.Fatal("P1 never answered, even at full sample")
	}
	if firstOK < floor {
		t.Errorf("P1 answered at n=%d with a floor of %d: the gate does not hold", firstOK, floor)
	}
}

// P3. The subtle one, and the comment in guards.go says why: a fence over
// identical sessions sits exactly on the typical session, so a cap derived
// from it would refuse ordinary work while looking like it came from evidence.
func TestFenceRefusesZeroSpread(t *testing.T) {
	same := make([]float64, 40)
	for i := range same {
		same[i] = 0.65
	}
	if f, ok := UpperFence(same, 4); ok {
		t.Errorf("P3 forty identical samples produced a fence at %.4f. A cap there "+
			"lands on the typical session and refuses ordinary work, while "+
			"looking like it was derived from data", f.Upper)
	}
}

// P3. Values that are not observations must not reach a quartile. A negative
// cost or a NaN is an upstream bug, and letting either in moves a threshold
// that refuses live traffic.
func TestFenceIgnoresImpossibleValues(t *testing.T) {
	clean := []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.9, 1.4, 2.2}
	dirty := append(append([]float64{}, clean...),
		math.NaN(), math.Inf(1), math.Inf(-1), -3.0)

	a, aok := UpperFence(clean, 4)
	b, bok := UpperFence(dirty, 4)
	if !aok || !bok {
		t.Fatal("both samples should produce a fence")
	}
	if a.Upper != b.Upper || a.N != b.N {
		t.Errorf("P3 impossible values moved the fence: clean upper %.6f over n=%d, "+
			"with NaN/Inf/negative present %.6f over n=%d", a.Upper, a.N, b.Upper, b.N)
	}
}

// P4. Determinism. A threshold that moves between runs on identical input
// cannot be reasoned about, and Go's map iteration order makes this a live
// hazard rather than a theoretical one.
func TestFenceIsDeterministic(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	xs := make([]float64, 200)
	for i := range xs {
		xs[i] = rng.ExpFloat64() * 0.8
	}
	first, ok := UpperFence(xs, 8)
	if !ok {
		t.Fatal("expected a fence")
	}
	for i := 0; i < 50; i++ {
		got, ok := UpperFence(xs, 8)
		if !ok || got != first {
			t.Fatalf("P4 run %d differed: %+v then %+v", i, first, got)
		}
	}
}

// P5. Against a distribution whose quartiles are known by construction, the
// fence must land where hand arithmetic puts it. This is the one place a
// statistical surface can be checked against ground truth.
func TestFenceMatchesHandArithmetic(t *testing.T) {
	// 1..9: Q1 = 3, Q3 = 7, IQR = 4, upper fence = 7 + 6 = 13.
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}
	f, ok := UpperFence(xs, 4)
	if !ok {
		t.Fatal("expected a fence")
	}
	for _, c := range []struct {
		name string
		got  float64
		want float64
	}{{"Q1", f.Q1, 3}, {"Q3", f.Q3, 7}, {"IQR", f.IQR, 4}, {"median", f.Median, 5}, {"upper", f.Upper, 13}} {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("P5 %s: got %.6f, want %.6f (type-7 quantiles, what a "+
				"spreadsheet gives)", c.name, c.got, c.want)
		}
	}
}

// P6. The fence must widen as the spread widens. A threshold that ignores
// dispersion is a constant wearing a statistic's clothes.
func TestFenceWidensWithSpread(t *testing.T) {
	tight := []float64{1.0, 1.05, 1.1, 1.15, 1.2, 1.25, 1.3, 1.35}
	wide := []float64{1.0, 1.5, 2.0, 3.0, 4.0, 6.0, 9.0, 14.0}
	a, aok := UpperFence(tight, 4)
	b, bok := UpperFence(wide, 4)
	if !aok || !bok {
		t.Fatal("both should produce a fence")
	}
	if !(b.Upper > a.Upper) {
		t.Errorf("P6 a wider spread did not widen the fence: tight %.4f, wide %.4f",
			a.Upper, b.Upper)
	}
}
