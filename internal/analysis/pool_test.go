package analysis

import (
	"math"
	"testing"
)

// Pooling many sessions must produce an estimate whose uncertainty reflects how
// much the sessions actually disagree, and how many of them there are.
//
// What it did instead was carry a turn-weighted MEAN of each session's own
// relative error. A mean does not shrink with sample size, so the pooled
// estimate reported the same uncertainty over two sessions as over two hundred,
// and no quantity of corpus could ever sharpen it.
//
// This was caught by a null test rather than by reading the code. Comparing
// claude-opus-5 against itself over 17,560 turns per side returned a ratio of
// exactly 1.0000, which is correct by construction, with a band of plus or
// minus 85.10%. A quantity known to be exactly 1 cannot honestly carry an 85%
// band; that number was the estimator describing itself.
func TestPoolFits_UncertaintyShrinksAsAgreeingSessionsAccumulate(t *testing.T) {
	// Sessions scattered around a common ratio, the way real sessions are.
	// Each also carries its own within-session noise, which is what the old
	// code averaged and reported as the pooled band.
	//
	// The scatter is deterministic and identical in both runs, so the only
	// thing that differs between them is how many sessions there are.
	session := func(n int) []FitSample {
		out := make([]FitSample, 0, n)
		for i := 0; i < n; i++ {
			// Alternating offsets: same spread whatever n is.
			offset := 0.05
			if i%2 == 1 {
				offset = -0.05
			}
			out = append(out, FitSample{
				TokensPerByte: 0.50 + offset,
				Turns:         100,
				RelativeError: 0.60,
			})
		}
		return out
	}

	few := PoolFits(session(2))
	many := PoolFits(session(200))

	if few.RelativeError <= 0 {
		t.Fatalf("a pooled estimate must carry some uncertainty: %v", few.RelativeError)
	}
	if many.RelativeError >= few.RelativeError {
		t.Errorf("uncertainty did not fall as sessions accumulated: 2 sessions gave "+
			"%.4f, 200 gave %.4f. If more agreeing data does not sharpen the estimate, "+
			"the figure is describing the estimator and not the corpus",
			few.RelativeError, many.RelativeError)
	}
	// The pooled ratio itself must be unchanged by the fix.
	if math.Abs(many.TokensPerByte-0.50) > 1e-9 {
		t.Errorf("pooled ratio moved: %v, want 0.50", many.TokensPerByte)
	}
}

// The null case: sessions that agree exactly must not manufacture a band out of
// their own internal noise.
func TestPoolFits_PerfectAgreementIsNotReportedAsWideUncertainty(t *testing.T) {
	same := []FitSample{
		{TokensPerByte: 0.42, Turns: 5000, RelativeError: 0.85},
		{TokensPerByte: 0.42, Turns: 5000, RelativeError: 0.85},
		{TokensPerByte: 0.42, Turns: 5000, RelativeError: 0.85},
	}
	got := PoolFits(same)
	if got.RelativeError > 0.10 {
		t.Errorf("three sessions agreeing exactly reported %.4f uncertainty. The old "+
			"code would report 0.85 here, which is the average of their internal noise "+
			"rather than any statement about the pooled figure", got.RelativeError)
	}
}

// Disagreement must still be reported. A fix that always returns a small number
// would pass the two tests above and be worse than the bug.
func TestPoolFits_GenuineDisagreementIsStillReported(t *testing.T) {
	spread := []FitSample{
		{TokensPerByte: 0.20, Turns: 1000, RelativeError: 0.05},
		{TokensPerByte: 0.80, Turns: 1000, RelativeError: 0.05},
	}
	got := PoolFits(spread)
	if got.RelativeError < 0.20 {
		t.Errorf("two sessions differing by 4x reported %.4f uncertainty. Sessions that "+
			"disagree must widen the band, or the estimate is confident about a number "+
			"its own inputs do not support", got.RelativeError)
	}
}

// One session cannot have a spread, and must not claim one it does not have.
func TestPoolFits_ASingleSessionFallsBackToItsOwnError(t *testing.T) {
	one := []FitSample{{TokensPerByte: 0.33, Turns: 900, RelativeError: 0.44}}
	got := PoolFits(one)
	if got.RelativeError != 0.44 {
		t.Errorf("a single session has no between-session spread to measure, so its own "+
			"error is the only honest figure: got %.4f, want 0.44", got.RelativeError)
	}
	if got.Turns != 900 {
		t.Errorf("turns lost: %d", got.Turns)
	}
}
