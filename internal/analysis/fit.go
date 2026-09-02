package analysis

import (
	"math"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// TokenFit is the session's observed relationship between user-side content
// bytes (tool results, user text) and provider tokens. Assistant output is
// never fitted: its token count is reported by the provider and replayed
// verbatim, so it is measured, not estimated.
type TokenFit struct {
	// TokensPerByte is the pooled ratio across fitted turns.
	TokensPerByte float64
	// RelativeError is the standard deviation of per-turn ratios divided by
	// the pooled ratio. Estimates carry this as their uncertainty.
	RelativeError float64
	// Turns is how many turns contributed.
	Turns int
	// UnseenPrefixTokens is the shared prefix ahead of the first message:
	// system prompt and tool definitions. It is measured from the first
	// request's cache read when that request found a warm cache, and
	// estimated from bytes otherwise.
	UnseenPrefixTokens int
	// UnseenPrefixMeasured says which of the two applied.
	UnseenPrefixMeasured bool
	// InjectedTokens is content the client sent with the first request that
	// the transcript does not show (attachments, injected reminders).
	InjectedTokens int
}

// defaultTokensPerByte is used only when a session offers no turn to fit
// on. It is a coarse prose average and is reported as such.
const defaultTokensPerByte = 0.25

// minFitBytes excludes turns whose new user content is so small that the
// client's fixed per-message overhead, not the content, decides the ratio.
const minFitBytes = 512

// turnContent splits what a request added beyond the previous cached
// prefix into the previous output (measured) and new user-side messages
// (estimated from bytes).
type turnContent struct {
	outputTokens int
	userBytes    int
	userTokens   int
}

func splitTurn(t Turn) turnContent {
	prev, cur := t.Previous, t.Request
	tc := turnContent{outputTokens: prev.Usage.Output}
	for _, m := range cur.Context[min(len(prev.Context), len(cur.Context)):] {
		if m.Role == transcript.RoleUser {
			tc.userBytes += m.Bytes()
		}
	}
	// The previous request's uncached tail is re-processed inside this
	// request's cache write, so it is subtracted to isolate new content.
	newTokens := cur.Usage.CacheCreation + cur.Usage.Input - prev.Usage.Input
	tc.userTokens = max(newTokens-tc.outputTokens, 0)
	return tc
}

// Fit computes the byte-to-token relationship for a calibrated lane.
func Fit(cal *Calibration) TokenFit {
	var sumBytes, sumTokens float64
	var samples []sample
	for _, t := range cal.Turns {
		if t.Outcome != cachemodel.ReadReproduced {
			continue
		}
		tc := splitTurn(t)
		if tc.userBytes < minFitBytes || tc.userTokens <= 0 {
			continue
		}
		sumBytes += float64(tc.userBytes)
		sumTokens += float64(tc.userTokens)
		samples = append(samples, sample{ratio: float64(tc.userTokens) / float64(tc.userBytes), weight: float64(tc.userBytes)})
	}
	fit := TokenFit{Turns: len(samples)}
	if sumBytes == 0 {
		fit.TokensPerByte = defaultTokensPerByte
		fit.RelativeError = 1
	} else {
		fit.TokensPerByte = sumTokens / sumBytes
		fit.RelativeError = weightedSpread(samples, fit.TokensPerByte)
	}
	if len(cal.Lane.Requests) > 0 {
		first := cal.Lane.Requests[0]
		seen := 0
		for _, m := range first.Context {
			seen += m.Bytes()
		}
		visible := fit.EstimateTokens(seen)
		if first.Usage.CacheRead > 0 {
			fit.UnseenPrefixTokens = first.Usage.CacheRead
			fit.UnseenPrefixMeasured = true
			fit.InjectedTokens = max(first.Usage.PromptTotal()-first.Usage.CacheRead-visible, 0)
		} else {
			fit.UnseenPrefixTokens = max(first.Usage.PromptTotal()-visible, 0)
		}
	}
	return fit
}

// sample is one turn's ratio, weighted by the bytes behind it so a large
// tool result counts for more than a one-line acknowledgement.
type sample struct {
	ratio, weight float64
}

// weightedSpread is the byte-weighted standard deviation of per-turn
// ratios relative to the pooled ratio.
func weightedSpread(samples []sample, mean float64) float64 {
	if len(samples) < 2 || mean == 0 {
		return 1
	}
	var ss, sw float64
	for _, s := range samples {
		d := s.ratio - mean
		ss += s.weight * d * d
		sw += s.weight
	}
	if sw == 0 {
		return 1
	}
	return math.Sqrt(ss/sw) / mean
}

// EstimateTokens converts bytes to tokens using the fit.
func (f TokenFit) EstimateTokens(bytes int) int {
	return int(math.Round(float64(bytes) * f.TokensPerByte))
}

// Estimated is a token figure with the part of it that carries uncertainty.
type Estimated struct {
	Value int
	// Error is the fit's relative error applied to the estimated portion
	// only; measured tokens contribute no error.
	Error int
}

// Estimate builds a figure from measured and estimated token counts.
func (f TokenFit) Estimate(measured, estimated int) Estimated {
	return Estimated{Value: measured + estimated, Error: int(math.Round(float64(estimated) * f.RelativeError))}
}

// Labels for content the transcript does not show.
const (
	unseenPrefixLabel = "system prompt and tool definitions (shared prefix, not in transcript)"
	injectedLabel     = "client-injected content on the first turn (attachments, reminders; not in transcript)"
)
