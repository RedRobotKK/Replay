package analysis

import (
	"math"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// Tokens is a token count that remembers how much of it was measured from
// provider usage and how much was estimated through the byte-to-token fit.
// Arithmetic keeps the two apart so reports can state uncertainty on the
// estimated part only.
type Tokens struct {
	Measured  int
	Estimated int
}

// Measured constructs a count the provider reported.
func Measured(n int) Tokens { return Tokens{Measured: n} }

// Estimated constructs a count derived from the byte-to-token fit.
func Estimated(n int) Tokens { return Tokens{Estimated: n} }

// Total is the whole count regardless of provenance.
func (t Tokens) Total() int { return t.Measured + t.Estimated }

// Add sums two counts.
func (t Tokens) Add(o Tokens) Tokens {
	return Tokens{Measured: t.Measured + o.Measured, Estimated: t.Estimated + o.Estimated}
}

// Times scales a count.
func (t Tokens) Times(n int) Tokens {
	return Tokens{Measured: t.Measured * n, Estimated: t.Estimated * n}
}

// Figure is a token figure ready to print: the value and the uncertainty
// of its estimated part.
type Figure struct {
	Value int
	Error int
}

// TokenFit is the session's observed relationship between user-side content
// bytes (tool results, user text) and provider tokens. Assistant output is
// never fitted: the provider reports its token count and replays exactly
// those tokens, so it is measured.
type TokenFit struct {
	// TokensPerByte is the pooled ratio across fitted turns.
	TokensPerByte float64
	// RelativeError is the byte-weighted standard deviation of per-turn
	// ratios divided by the pooled ratio. Estimates carry this uncertainty.
	RelativeError float64
	// Turns is how many turns contributed.
	Turns int
	// UnseenPrefix is the shared prefix ahead of the first message (system
	// prompt and tool definitions) when the transcript does not show it:
	// measured from the first request's cache read when that request found
	// a warm cache, estimated from bytes otherwise, zero when visible.
	UnseenPrefix Tokens
	// Injected is content the client sent with the first request that the
	// transcript does not show (attachments, injected reminders).
	Injected Tokens
}

// defaultTokensPerByte is used only when a session offers no turn to fit
// on. It is a coarse prose average and is reported as such.
const defaultTokensPerByte = 0.25

// minFitBytes excludes turns whose new user content is so small that the
// client's fixed per-message overhead, not the content, decides the ratio.
const minFitBytes = 512

// turnContent splits what a request's cache write covered into the
// previous output (measured), new user-side content (estimated from bytes),
// and history re-written because the prefix broke (measured).
type turnContent struct {
	userBlocks   []transcript.Block
	userBytes    int
	userTokens   int
	rebillTokens int
}

func splitTurn(t Turn) turnContent {
	prev, cur := t.Previous, t.Request
	var tc turnContent
	for _, m := range cur.Context[min(len(prev.Context), len(cur.Context)):] {
		if m.Role == transcript.RoleUser {
			tc.userBlocks = append(tc.userBlocks, m.Blocks...)
			tc.userBytes += m.Bytes()
		}
	}
	// The cache write covers the previous request's uncached tail (which is
	// subtracted to isolate new content), the new content, and, on a broken
	// turn, the history that had to be re-written. On an exceeded turn a
	// sibling request already wrote part of the new content, so the write
	// undercounts it by the excess read.
	written := cur.Usage.CacheCreation + cur.Usage.Input - prev.Usage.Input
	if t.Outcome == cachemodel.ReadBroken {
		tc.rebillTokens = min(t.Expected-t.Actual, written)
	}
	newTokens := written - tc.rebillTokens + max(t.Actual-t.Expected, 0)
	tc.userTokens = max(newTokens-prev.Usage.Output, 0)
	return tc
}

// sample is one turn's ratio, weighted by the bytes behind it so a large
// tool result counts for more than a one-line acknowledgement.
type sample struct {
	ratio, weight float64
}

// Fit computes the byte-to-token relationship for a calibrated lane.
// prefixVisible says the system prompt and tools are in the context (ledger
// data), so nothing ahead of the first message needs estimating.
func Fit(cal *Calibration, prefixVisible bool) TokenFit {
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
	if len(cal.Lane.Requests) > 0 && !prefixVisible {
		first := cal.Lane.Requests[0]
		seen := 0
		for _, m := range first.Context {
			seen += m.Bytes()
		}
		visible := fit.EstimateTokens(seen)
		if first.Usage.CacheRead > 0 {
			fit.UnseenPrefix = Measured(first.Usage.CacheRead)
			fit.Injected = Estimated(max(first.Usage.PromptTotal()-first.Usage.CacheRead-visible, 0))
		} else {
			fit.UnseenPrefix = Estimated(max(first.Usage.PromptTotal()-visible, 0))
		}
	}
	return fit
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

// Figure turns a count into a printable figure with its uncertainty.
func (f TokenFit) Figure(t Tokens) Figure {
	return Figure{Value: t.Total(), Error: int(math.Round(float64(t.Estimated) * f.RelativeError))}
}

// Labels for content the transcript does not show.
const (
	unseenPrefixLabel = "system prompt and tool definitions (shared prefix, not in transcript)"
	injectedLabel     = "client-injected content on the first turn (attachments, reminders; not in transcript)"
)
