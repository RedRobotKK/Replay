package cachemodel

import (
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Cache semantics that differ by provider.
//
// This file exists because anthropic.go's numbers were package constants, and
// a constant is a claim that every provider agrees. They do not. Anthropic
// expires a default entry after five minutes; OpenAI documents thirty for the
// Astra tier. Anthropic invalidates the prefix when the thinking setting
// changes; OpenAI documents an input item that changes reasoning effort and
// keeps the prefix. Carrying one provider's rule across both does not widen an
// error bar, it reports a cause that did not happen, which is the one thing
// this tool is supposed to refuse.
//
// The seam is a value rather than a build tag or an env var: the rules are
// chosen by the code that knows which provider the request went to, and a test
// can hold two providers side by side in one process.

// CacheRules is one provider's published cache behaviour.
//
// Every field here is a CLAIM read from the provider's own documentation on a
// date, not something this package measured. Where replaying real traffic has
// bounded a number, the bound belongs in the dated rules document's
// MinPrefixClaim, next to what was published. See rules.go.
type CacheRules struct {
	// Provider names whose published numbers these are, for the provenance
	// line on any report built from them.
	Provider string

	// TTL is the lifetime of a cache entry written under this provider's
	// default terms.
	TTL time.Duration

	// TTLFrom reads the TTL actually bought back off a request, for providers
	// that sell more than one. Nil means the provider offers a single TTL and
	// TTL is it.
	//
	// Anthropic sells a five minute and a one hour entry and reports the split
	// in the usage breakdown, so the TTL a gap must be measured against is a
	// property of the request that wrote the entry rather than of the vendor.
	TTLFrom func(transcript.Usage) time.Duration

	// MinPrefix is the smallest visible prefix the provider will cache at all.
	// Zero means the provider publishes no single floor and the per-model rows
	// in the dated rules document decide, which is Anthropic's case: the floor
	// moves by model tier.
	MinPrefix int

	// EffortChangeBreaks reports whether changing the reasoning or thinking
	// setting invalidates the cached prefix.
	EffortChangeBreaks bool
}

// ttlFor is the TTL a gap must be measured against for the request that wrote
// the entry.
func (r CacheRules) ttlFor(u transcript.Usage) time.Duration {
	if r.TTLFrom != nil {
		return r.TTLFrom(u)
	}
	return r.TTL
}

// AnthropicRules is what this package assumed globally before providers were
// told apart. Behaviour is unchanged: the TTL still comes from the usage
// breakdown, and the prefix floor still comes from the per-model rows.
func AnthropicRules() CacheRules {
	return CacheRules{
		Provider:           "anthropic",
		TTL:                TTLShort,
		TTLFrom:            TTLOf,
		MinPrefix:          0,
		EffortChangeBreaks: true,
	}
}

// Astra's published cache terms, read from OpenAI's prompt-caching guide on
// 2026-09-15.
const (
	// TTLAstra is the documented lifetime of a cache entry.
	//
	// It is set per request through prompt_cache_options.ttl rather than being
	// a property of the vendor, which is why TTLFrom stays a real seam here
	// rather than a quirk of Anthropic's. What makes the flat value correct
	// today is narrower than it looks: "30m" is the ONLY supported value and
	// is also the default, so every request has the same deadline until
	// OpenAI ships a second one. Checked against the prompt-caching guide on
	// 2026-09-15, not against a summary of it.
	//
	// Retention is documented as best effort and MAY exceed this, so a gap
	// inside it is evidence of nothing and a gap beyond it is the provider's
	// own stated deadline rather than a guarantee of eviction.
	TTLAstra = 30 * time.Minute

	// MinPrefixAstra is the smallest visible prefix documented as cacheable.
	// It is inclusive: a prompt of exactly this many tokens is eligible.
	MinPrefixAstra = 1024
)

// AstraRules is OpenAI's published behaviour for the GPT-6 Astra tier.
func AstraRules() CacheRules {
	return CacheRules{
		Provider:  "openai",
		TTL:       TTLAstra,
		TTLFrom:   nil,
		MinPrefix: MinPrefixAstra,
		// OpenAI documents a configuration_update input item that changes
		// reasoning effort between responses while leaving the cached prefix
		// intact. An agent that instead rewrites the top-level reasoning
		// setting pays a full write, and the transcript of the two looks
		// almost identical. That difference is the thing worth naming on a
		// report, and it is why this flag is not simply inherited.
		EffortChangeBreaks: false,
	}
}

// CauseBelowMinPrefix joins the BreakCause vocabulary declared in anthropic.go.
//
// It is ADR-0018 applied to the prefix floor. A prompt too small to be cached
// and a prompt whose cache broke are different states, and only one of them is
// worth an operator's attention. Before this existed, a sub-floor request on a
// provider with a floor read as CausePrefixChange, which invents a cause for a
// request that behaved exactly as documented, and would have put a fabricated
// break in front of anyone running short prompts.
const CauseBelowMinPrefix BreakCause = "prompt below the provider's minimum cacheable prefix (nothing was cached, so nothing broke)"

// ClassifyBreakWith decides the causes that usage and timing alone can settle,
// under one provider's rules. ok is false when only the message history can
// tell.
//
// prefixTokens is the visible prefix the provider was asked to match, used
// only against the floor. Pass zero when it is not known: the floor test is
// then skipped rather than guessed, because a request whose prefix size was
// never observed is not a request known to be under the floor.
func ClassifyBreakWith(r CacheRules, prev, cur transcript.Usage, prevModel, model string, gap time.Duration, prefixTokens int) (BreakCause, bool) {
	switch {
	case r.MinPrefix > 0 && prefixTokens > 0 && prefixTokens < r.MinPrefix:
		return CauseBelowMinPrefix, true
	case gap > r.ttlFor(prev):
		return CauseTTLExpired, true
	case prevModel != model:
		return CauseModelChanged, true
	case cur.CacheRead == 0:
		return CausePrefixChange, true
	default:
		return "", false
	}
}
