package analysis

import "math"

// FitSample is one session's fit, ready to be pooled with others.
type FitSample struct {
	// TokensPerByte is that session's own fitted ratio.
	TokensPerByte float64
	// Turns is how many turns it was fitted on, and its pooling weight.
	Turns int
	// RelativeError is the session's own internal spread. It is the only
	// uncertainty available when there is a single session and nothing to
	// compare it against.
	RelativeError float64
}

// PoolFits combines per-session fits into one estimate and states how uncertain
// that estimate is.
//
// The uncertainty is the part that was wrong. It used to be a turn-weighted
// mean of each session's own relative error, which is an average of within
// session noise and answers a different question entirely. An average does not
// fall as samples accumulate, so the pooled figure reported the same band over
// two sessions as over two hundred, and no amount of corpus could sharpen it.
//
// A null test exposed it: comparing a model against itself over 17,560 turns
// per side returned a ratio of exactly 1.0000, correct by construction, with a
// band of plus or minus 85.10%. A quantity known to be exactly 1 cannot carry
// an 85% band. The number was the estimator describing itself.
//
// What is reported now is the standard error of the pooled ratio: how much the
// sessions disagree with each other, divided by how many independent sessions
// there effectively are. Sessions that agree narrow it, sessions that disagree
// widen it, and more of them narrow it further, which is what a reader assumes
// an error bar means.
func PoolFits(samples []FitSample) TokenFit {
	var sumW, sumRW float64
	turns := 0
	for _, s := range samples {
		w := float64(s.Turns)
		if w <= 0 {
			continue
		}
		sumW += w
		sumRW += s.TokensPerByte * w
		turns += s.Turns
	}
	if sumW == 0 {
		return TokenFit{RelativeError: 1}
	}
	mean := sumRW / sumW
	fit := TokenFit{TokensPerByte: mean, Turns: turns}

	// One session has nothing to disagree with. Its own internal error is the
	// only honest figure, and claiming a between-session spread that was never
	// measured would be the same error in the opposite direction.
	if countWeighted(samples) < 2 {
		fit.RelativeError = firstError(samples)
		if fit.RelativeError == 0 {
			fit.RelativeError = 1
		}
		return fit
	}

	// Weighted spread of the session ratios about the pooled mean. This is the
	// population spread: how much sessions differ from one another.
	var ss float64
	for _, s := range samples {
		w := float64(s.Turns)
		if w <= 0 {
			continue
		}
		d := s.TokensPerByte - mean
		ss += w * d * d
	}
	if mean == 0 {
		fit.RelativeError = 1
		return fit
	}
	spread := math.Sqrt(ss/sumW) / mean

	// Kish effective sample size, not the raw session count: one enormous
	// session next to twenty tiny ones is not twenty-one independent
	// observations, and treating it as such would understate the band.
	var sumW2 float64
	for _, s := range samples {
		w := float64(s.Turns)
		if w > 0 {
			sumW2 += w * w
		}
	}
	nEff := sumW * sumW / sumW2
	if nEff < 1 {
		nEff = 1
	}
	fit.RelativeError = spread / math.Sqrt(nEff)
	return fit
}

// countWeighted is how many samples carry any weight at all.
func countWeighted(samples []FitSample) int {
	n := 0
	for _, s := range samples {
		if s.Turns > 0 {
			n++
		}
	}
	return n
}

// firstError returns the sole weighted sample's own error.
func firstError(samples []FitSample) float64 {
	for _, s := range samples {
		if s.Turns > 0 {
			return s.RelativeError
		}
	}
	return 0
}
