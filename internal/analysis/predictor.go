package analysis

// Pre-flight deficit policy.
//
// The provider's rate-limit headers cannot drive this. Measured 2026-09-06
// across a 30-lane randomized trial: 142 responses carried
// anthropic-ratelimit-unified-*, and 3,778,706 re-billed tokens moved the 5h
// utilization figure from 0.10 to 0.15. That is five steps at the header's
// published 0.01 resolution, each step spanning ~28 requests across both arms.
// A 140,000-token full-prefix re-lay does not move the counter by one tick, so
// the header can be a denominator and never a trigger.
//
// What is available before the wire is the prefix comparison the calibrator
// already does: whether this request re-lays content an earlier request in the
// same lane had established. That is a request-pass fact, unlike a cache hit,
// which the provider only reports afterwards in cache_read_input_tokens.
//
// The default is to observe and say so. Blocking is opt-in because a refused
// request is indistinguishable from a network failure to the agent that sent
// it, and because ADR-0011 admits rewriting only with consent.

// PolicyState is the user's stated preference for pre-flight deficits.
//
// The zero value warns and never blocks, which is the intended default: a
// ceiling nobody set must not refuse anybody's request.
type PolicyState struct {
	// CeilingTokens is the estimated deficit above which a diverged request
	// is refused. It is compared against an estimate, not a measurement;
	// see EstimateError.
	CeilingTokens int64
	// OptInActive gates refusal. False observes and reports only.
	OptInActive bool
}

// EvaluatePreFlightPolicy reports whether this request should be refused.
//
// Three terms, and each one is load-bearing:
//
//   - optInActive, because refusing without consent is the ADR-0011 breach;
//   - isDiverged, because a request that matches the established prefix costs
//     the cache-read rate however large it is, and refusing it saves nothing;
//   - estimatedTokens > ceiling, strictly greater, so a request landing
//     exactly on the ceiling passes. A ceiling is the largest deficit the
//     user accepts, not the smallest one they refuse.
func EvaluatePreFlightPolicy(isDiverged bool, estimatedTokens, ceiling int64, optInActive bool) bool {
	if !optInActive {
		return false
	}
	if isDiverged && estimatedTokens > ceiling {
		return true
	}
	return false
}

// EstimateError is the fraction by which a pre-flight token estimate may be
// wrong in either direction.
//
// The estimate comes from bytes through the fitted tokens-per-byte ratio, and
// Fit reports it is estimating whenever it had no comparable turns to fit. A
// ceiling compared against that estimate fires early or late in proportion to
// the error, which is why PreFlightDeficit carries the band rather than
// returning a bare number a caller would read as measured.
const EstimateError = 0.15

// PreFlightDeficit is an estimated deficit with its uncertainty attached.
type PreFlightDeficit struct {
	// Tokens is the point estimate.
	Tokens int64
	// Low and High bound it at EstimateError.
	Low, High int64
	// Diverged reports whether the prefix comparison found this request
	// re-laying established content.
	Diverged bool
	// Fitted reports whether Tokens came from a fitted ratio or from the
	// prose default. An unfitted estimate is a guess with a stated shape,
	// and the caller must be able to tell the difference.
	Fitted bool
}

// WouldRefuse applies a policy to an estimate.
func (d PreFlightDeficit) WouldRefuse(p PolicyState) bool {
	return EvaluatePreFlightPolicy(d.Diverged, d.Tokens, p.CeilingTokens, p.OptInActive)
}

// Straddles reports that the ceiling falls inside the estimate's error band,
// so the refusal decision is not supported by the measurement's precision.
//
// A caller that refuses here is refusing on noise. The honest move is to warn
// and let the request through, which is what the proxy does.
func (d PreFlightDeficit) Straddles(p PolicyState) bool {
	if !p.OptInActive || !d.Diverged {
		return false
	}
	return d.Low <= p.CeilingTokens && p.CeilingTokens <= d.High
}

// NewPreFlightDeficit builds an estimate and its band from a point estimate.
func NewPreFlightDeficit(tokens int64, diverged, fitted bool) PreFlightDeficit {
	margin := int64(float64(tokens) * EstimateError)
	return PreFlightDeficit{
		Tokens:   tokens,
		Low:      tokens - margin,
		High:     tokens + margin,
		Diverged: diverged,
		Fitted:   fitted,
	}
}
