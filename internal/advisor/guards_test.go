package advisor

import (
	"math"
	"testing"
)

func closeTo(t *testing.T, got, want float64, what string) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s: got %.9f, want %.9f", what, got, want)
	}
}

// A cap should come from the user's own spread, not from a number somebody
// picked. Tukey's upper fence, Q3 + 1.5*IQR, is the threshold above which a
// session is an outlier against that user's own history.
func TestUpperFenceOnAKnownSample(t *testing.T) {
	// 1..10, quartiles by linear interpolation (R type 7).
	// Q1 at index 2.25 -> 3.25; Q3 at index 6.75 -> 7.75; IQR 4.5.
	f, ok := UpperFence([]float64{5, 3, 9, 1, 7, 2, 8, 4, 10, 6}, 10)
	if !ok {
		t.Fatal("ten samples is above any sane floor")
	}
	closeTo(t, f.Q1, 3.25, "Q1")
	closeTo(t, f.Q3, 7.75, "Q3")
	closeTo(t, f.IQR, 4.5, "IQR")
	closeTo(t, f.Upper, 7.75+1.5*4.5, "upper fence")
	if f.N != 10 {
		t.Fatalf("N must be reported: %d", f.N)
	}
}

// The input arrives in whatever order the ledger yields. A fence that depends
// on that order is not a statistic.
func TestFenceDoesNotDependOnInputOrder(t *testing.T) {
	a, _ := UpperFence([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 10)
	b, _ := UpperFence([]float64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}, 10)
	closeTo(t, a.Upper, b.Upper, "fence under reordering")
}

// The PRD's own pass condition: an empty ledger and a three-session ledger
// both produce nothing. A threshold derived from three sessions is a threshold
// derived from noise, and it would be presented with the same confidence as a
// real one.
func TestFenceRefusesBelowTheFloor(t *testing.T) {
	for _, n := range []int{0, 1, 3, MinGuardSessions - 1} {
		xs := make([]float64, n)
		for i := range xs {
			xs[i] = float64(i + 1)
		}
		if f, ok := UpperFence(xs, MinGuardSessions); ok {
			t.Fatalf("%d samples produced a fence of %.4f", n, f.Upper)
		}
	}
	xs := make([]float64, MinGuardSessions)
	for i := range xs {
		xs[i] = float64(i + 1)
	}
	if _, ok := UpperFence(xs, MinGuardSessions); !ok {
		t.Fatalf("exactly %d samples is the floor and must pass", MinGuardSessions)
	}
}

// A user whose sessions all cost the same has no spread, so the fence sits on
// the value itself. That is a real answer, not a degenerate one, but a cap
// equal to the typical session would refuse ordinary work, so it is refused.
func TestNoSpreadProducesNoRecommendation(t *testing.T) {
	xs := make([]float64, MinGuardSessions)
	for i := range xs {
		xs[i] = 4.20
	}
	if f, ok := UpperFence(xs, MinGuardSessions); ok {
		t.Fatalf("a zero-IQR sample must not yield a cap: %.4f", f.Upper)
	}
}

// Negative or non-finite samples are a bug upstream, not data.
func TestFenceIgnoresUnusableSamples(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, math.NaN(), math.Inf(1), -3}
	f, ok := UpperFence(xs, 10)
	if !ok {
		t.Fatal("ten usable samples remain")
	}
	if f.N != 10 {
		t.Fatalf("unusable samples were counted: N=%d", f.N)
	}
}
