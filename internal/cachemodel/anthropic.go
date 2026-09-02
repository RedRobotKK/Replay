// Package cachemodel encodes the provider's prompt-caching rules as named,
// versioned constants and the invariants the analysis relies on.
//
// Every value here is a documented provider rule, not a measurement. When
// the provider changes a rule, this file changes and RulesVersion moves.
package cachemodel

import (
	"strings"
	"time"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// RulesVersion is printed on every report so a reader knows which rules a
// figure was computed under.
const RulesVersion = "anthropic-2026-09-01"

// Cache TTLs offered by the provider.
const (
	TTLShort = 5 * time.Minute
	TTLLong  = time.Hour
)

// Price multipliers relative to the base input token price.
const (
	WriteMultiplierShort = 1.25
	WriteMultiplierLong  = 2.0
	ReadMultiplier       = 0.10
)

// Structural limits.
const (
	MaxBreakpoints    = 4
	LookbackPositions = 20
)

// Minimum cacheable prefix, in tokens, by model family. A prefix shorter than
// this silently does not cache. The default applies to unknown models.
const (
	minPrefixNewest   = 512
	minPrefixStandard = 1024
	minPrefixOpus47   = 2048
	minPrefixLegacy   = 4096
	minPrefixDefault  = minPrefixStandard
)

// MinCacheablePrefix returns the minimum cacheable prefix for a model id.
func MinCacheablePrefix(model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "fable"), strings.Contains(m, "mythos"), strings.Contains(m, "opus-5"):
		return minPrefixNewest
	case strings.Contains(m, "opus-4-7"):
		return minPrefixOpus47
	case strings.Contains(m, "opus-4-6"), strings.Contains(m, "opus-4-5"), strings.Contains(m, "haiku-4-5"):
		return minPrefixLegacy
	case strings.Contains(m, "sonnet"), strings.Contains(m, "opus-4"), strings.Contains(m, "haiku"):
		return minPrefixStandard
	default:
		return minPrefixDefault
	}
}

// ExpectedRead is the cache read the next request should report if nothing
// in the prefix changed and the cache is still warm: everything the previous
// request processed except its uncached tail (the content after its last
// breakpoint, reported as input_tokens).
func ExpectedRead(prev transcript.Usage) int {
	return prev.PromptTotal() - prev.Input
}

// ReadOutcome classifies one request's cache read against the expectation.
type ReadOutcome int

const (
	// ReadFirst is the first request in a lane; nothing to compare against.
	ReadFirst ReadOutcome = iota
	// ReadReproduced means the provider read exactly the expected prefix.
	ReadReproduced
	// ReadExceeded means the provider read more than the previous request
	// wrote. Another request sharing the prefix extended the cache.
	ReadExceeded
	// ReadBroken means the provider read less than expected: the prefix
	// diverged or the cache expired.
	ReadBroken
)

func (o ReadOutcome) String() string {
	switch o {
	case ReadFirst:
		return "first"
	case ReadReproduced:
		return "reproduced"
	case ReadExceeded:
		return "exceeded"
	case ReadBroken:
		return "broken"
	default:
		return "unknown"
	}
}

// ClassifyRead compares a request's reported cache read with the expectation
// derived from the previous request in the same lane.
func ClassifyRead(prev, cur transcript.Usage) (ReadOutcome, int) {
	expected := ExpectedRead(prev)
	switch {
	case cur.CacheRead == expected:
		return ReadReproduced, expected
	case cur.CacheRead > expected:
		return ReadExceeded, expected
	default:
		return ReadBroken, expected
	}
}

// TTLOf returns the TTL the request's cache writes used, inferred from the
// creation breakdown. Without a breakdown the provider default applies.
func TTLOf(u transcript.Usage) time.Duration {
	if u.Create1h > 0 && u.Create5m == 0 {
		return TTLLong
	}
	return TTLShort
}

// EffectiveTokens prices a request's prompt in base-input-token equivalents:
// uncached input at 1x, writes at their TTL multiplier, reads at the read
// multiplier. It is a relative measure for comparing layouts, not a bill.
func EffectiveTokens(u transcript.Usage) float64 {
	writes := float64(u.Create5m)*WriteMultiplierShort + float64(u.Create1h)*WriteMultiplierLong
	if u.Create5m == 0 && u.Create1h == 0 {
		writes = float64(u.CacheCreation) * WriteMultiplierShort
	}
	return float64(u.Input) + writes + float64(u.CacheRead)*ReadMultiplier
}

// WriteMultiplier returns the write multiplier for a TTL.
func WriteMultiplier(ttl time.Duration) float64 {
	if ttl >= TTLLong {
		return WriteMultiplierLong
	}
	return WriteMultiplierShort
}
