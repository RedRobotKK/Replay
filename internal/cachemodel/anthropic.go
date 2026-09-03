// Package cachemodel encodes the provider's prompt-caching rules and list
// prices as named, versioned values and the invariants the analysis relies
// on.
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

// PriceTableVersion dates the price table. Every dollar figure Buffy prints
// cites it, because prices change and the figure is only as current as the
// table.
const PriceTableVersion = "2026-06-24"

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
	// readMultiplierNewest applies to the Fable and Mythos 5.1 tier.
	readMultiplierNewest = 0.025
)

// Minimum cacheable prefix, in tokens, by model family. A prefix shorter than
// this silently does not cache.
const (
	minPrefixNewest   = 512
	minPrefixStandard = 1024
	minPrefixOpus47   = 2048
	minPrefixLegacy   = 4096
)

// tokensPerMillion converts per-million prices to per-token.
const tokensPerMillion = 1_000_000

// Price is a model's first-party API list price in US dollars per million
// tokens, plus its cache-read multiple.
type Price struct {
	InputPerMTok  float64
	OutputPerMTok float64
	ReadMult      float64
}

// modelRow holds everything the rules know about one model family. Rows
// are matched by substring in order, so more specific ids come first, and
// one table decides both prices and caching floors.
type modelRow struct {
	match     string
	minPrefix int
	price     Price
	priced    bool
}

var modelTable = []modelRow{
	{"fable-5-1", minPrefixNewest, Price{10, 50, readMultiplierNewest}, true},
	{"mythos-5-1", minPrefixNewest, Price{10, 50, readMultiplierNewest}, true},
	{"fable-5", minPrefixNewest, Price{10, 50, ReadMultiplier}, true},
	{"mythos-5", minPrefixNewest, Price{10, 50, ReadMultiplier}, true},
	{"opus-5", minPrefixNewest, Price{5, 25, ReadMultiplier}, true},
	{"opus-4-8", minPrefixStandard, Price{5, 25, ReadMultiplier}, true},
	{"opus-4-7", minPrefixOpus47, Price{5, 25, ReadMultiplier}, true},
	{"opus-4-6", minPrefixLegacy, Price{5, 25, ReadMultiplier}, true},
	{"opus-4-5", minPrefixLegacy, Price{}, false},
	{"haiku-4-5", minPrefixLegacy, Price{1, 5, ReadMultiplier}, true},
	{"sonnet-5", minPrefixStandard, Price{2, 10, ReadMultiplier}, true},
	{"sonnet-4-6", minPrefixStandard, Price{3, 15, ReadMultiplier}, true},
	{"sonnet", minPrefixStandard, Price{}, false},
	{"opus-4", minPrefixStandard, Price{}, false},
	{"haiku", minPrefixStandard, Price{}, false},
}

var unknownModel = modelRow{minPrefix: minPrefixStandard, price: Price{ReadMult: ReadMultiplier}}

func lookup(model string) modelRow {
	m := strings.ToLower(model)
	for _, row := range modelTable {
		if strings.Contains(m, row.match) {
			if row.price.ReadMult == 0 {
				row.price.ReadMult = ReadMultiplier
			}
			return row
		}
	}
	return unknownModel
}

// MinCacheablePrefix returns the minimum cacheable prefix for a model id.
func MinCacheablePrefix(model string) int {
	return lookup(model).minPrefix
}

// PriceFor returns the list price for a model id, and false when the model
// is not in the table. Callers print no dollar figure in that case.
func PriceFor(model string) (Price, bool) {
	row := lookup(model)
	return row.price, row.priced
}

// ReadMultiplierFor returns the cache-read multiple for a model, falling
// back to the standard multiple for unknown models.
func ReadMultiplierFor(model string) float64 {
	return lookup(model).price.ReadMult
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

// BreakCause classifies why a cached prefix stopped matching. The values
// are the one vocabulary used by live logs, ledger records, metrics, and
// offline reports.
type BreakCause string

// Causes, from most to least specific. The first three can be decided from
// usage and timing alone; the rest need the message history.
const (
	CauseTTLExpired   BreakCause = "cache expired (gap longer than the TTL)"
	CauseModelChanged BreakCause = "model changed between requests"
	CausePrefixChange BreakCause = "system prompt or tool definitions changed"
	CauseEffortChange BreakCause = "effort or thinking setting changed"
	CauseHistoryEdit  BreakCause = "an earlier message was edited or removed"
	CauseRerendered   BreakCause = "client re-rendered history after the system prefix (no edit visible in transcript)"
	CauseUnknown      BreakCause = "prefix diverged inside the message history at an unknown block"
)

// ClassifyBreak decides the causes that usage and timing alone can settle.
// ok is false when only the message history can tell.
func ClassifyBreak(prev, cur transcript.Usage, prevModel, model string, gap time.Duration) (BreakCause, bool) {
	switch {
	case gap > TTLOf(prev):
		return CauseTTLExpired, true
	case prevModel != model:
		return CauseModelChanged, true
	case cur.CacheRead == 0:
		return CausePrefixChange, true
	default:
		return "", false
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

// WriteMultiplier returns the write multiplier for a TTL.
func WriteMultiplier(ttl time.Duration) float64 {
	if ttl >= TTLLong {
		return WriteMultiplierLong
	}
	return WriteMultiplierShort
}

// writeEquivalent prices cache writes in base-input-token equivalents by
// their TTL split, or at the short multiplier when no split was reported.
func writeEquivalent(u transcript.Usage) float64 {
	if u.Create5m == 0 && u.Create1h == 0 {
		return float64(u.CacheCreation) * WriteMultiplierShort
	}
	return float64(u.Create5m)*WriteMultiplierShort + float64(u.Create1h)*WriteMultiplierLong
}

// EffectiveTokens prices a request's prompt in base-input-token equivalents:
// uncached input at 1x, writes at their TTL multiplier, reads at the
// model's read multiplier. It is a relative measure for comparing layouts,
// not a bill.
func EffectiveTokens(u transcript.Usage, model string) float64 {
	return float64(u.Input) + writeEquivalent(u) + float64(u.CacheRead)*ReadMultiplierFor(model)
}

// CostUSD prices one request's usage at first-party list rates: prompt
// tokens as EffectiveTokens at the input price, output at the output price.
func CostUSD(u transcript.Usage, p Price) float64 {
	input := (float64(u.Input) + writeEquivalent(u) + float64(u.CacheRead)*p.ReadMult) * p.InputPerMTok
	output := float64(u.Output) * p.OutputPerMTok
	return (input + output) / tokensPerMillion
}

// SimulatedUsage builds the usage a simulated request would report, so
// simulated prompts are priced by the same functions as real ones.
func SimulatedUsage(tail, write, read, output int, ttl time.Duration) transcript.Usage {
	u := transcript.Usage{Input: tail, CacheCreation: write, CacheRead: read, Output: output}
	if ttl >= TTLLong {
		u.Create1h = write
	} else {
		u.Create5m = write
	}
	return u
}

// CachedShare is cache reads divided by prompt tokens, zero for an empty
// prompt.
func CachedShare(reads, prompt int) float64 {
	if prompt <= 0 {
		return 0
	}
	return float64(reads) / float64(prompt)
}
