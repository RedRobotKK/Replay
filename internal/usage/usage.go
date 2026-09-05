// Package usage holds the engine's own shape for what a request cost, and the
// adapters that map a provider's reporting into it.
//
// Replay modelled one provider's mechanism and called it prompt caching. That
// assumption sits in three layers, not one: the cost model, the ledger schema,
// and the analysis that reads it. The three caching families are not variations
// on a theme, they are different products with different failure modes, and the
// one that breaks the current engine is rented cache, where caching is a bet:
// you pay storage per unit time, and underusing the cache before it expires
// leaves you worse off than not caching. Replay's advice assumes more caching is
// better, which is true in the first two families and false in the third.
//
// There is deliberately no Provider interface here. An interface with one
// implementation is a guess about the second one, and this design's own argument
// is that documented behaviour and real behaviour diverge. The second provider's
// awkwardness should dictate the interface, not this file. What ships now is the
// part that is not a guess: one normalised shape, one concrete adapter, and the
// provider's own payload kept verbatim.
package usage

import (
	"encoding/json"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Caching mechanism families. Which one a provider belongs to decides what
// advice is even meaningful: placement for the first, prefix hygiene for the
// second, utilisation for the third.
const (
	// MechanismExplicitBreakpoint: the client marks what to cache and pays a
	// premium on the write. You lose money by writing a cache you never read.
	MechanismExplicitBreakpoint = "explicit_breakpoint"
	// MechanismImplicitPrefix: nobody marks anything, matching just happens.
	// You cannot lose money, you can only fail to save it, and the only lever
	// is not perturbing the prefix.
	MechanismImplicitPrefix = "implicit_prefix"
	// MechanismRentedCache: you create a cache object and pay rent per unit
	// time. This is the family where caching more can cost more.
	MechanismRentedCache = "rented_cache"
)

// Provider names. One entry, because one is what has been observed.
const ProviderAnthropic = "anthropic"

// Record is one request's cost in the engine's own vocabulary.
//
// The field names are deliberately not any provider's. Every provider reports
// cache hits in its own shape under its own names, and an engine that reads one
// vendor's spelling has that vendor's model baked into it whether or not anyone
// intended that.
type Record struct {
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	At       time.Time `json:"at"`
	// Mechanism is the caching family this provider belongs to. It decides
	// which advice applies, so it travels with the measurement rather than
	// being looked up later from a table that may have moved on.
	Mechanism string `json:"mechanism"`

	// Prompt is every input token the provider processed, cached or not, and
	// always equals Fresh + CachedRead + CachedWrite.
	Prompt int `json:"prompt"`
	// Fresh is prompt tokens that neither hit the cache nor were written to it.
	Fresh int `json:"fresh"`
	// CachedRead is prompt tokens served from cache, billed at the provider's
	// read multiple.
	CachedRead int `json:"cached_read"`
	// CachedWrite is prompt tokens written to cache, billed at a premium.
	CachedWrite int `json:"cached_write"`
	// CachedWrite5m and CachedWrite1h split the write by the TTL it was made
	// with, because the write multiplier depends on it and that difference,
	// 1.25x against 2.0x, is the largest single lever in the cost model. Both
	// are zero when the provider did not report the breakdown.
	CachedWrite5m int `json:"cached_write_5m,omitempty"`
	CachedWrite1h int `json:"cached_write_1h,omitempty"`

	Output int `json:"output"`
	// Reasoning is the share of Output spent on reasoning, replayed as input
	// when the block is sent back.
	Reasoning int `json:"reasoning,omitempty"`

	// RentUSD is cache storage billed per unit time, the rented-cache family's
	// meter. No adapter here sets it, because no provider observed so far
	// charges it. It exists so that a rented-cache adapter has somewhere
	// truthful to put the number instead of forcing it into a token count,
	// and it must stay zero until one does: a plausible default here would be
	// indistinguishable from a measurement downstream.
	RentUSD float64 `json:"rent_usd,omitempty"`

	// Raw is the provider's own usage object, verbatim and unparsed.
	//
	// Normalising is lossy by construction: it keeps the fields this build
	// knows are load-bearing. A field nobody knew mattered is exactly what a
	// later calibration needs, and it can only come from a payload that was
	// stored before anyone knew to ask.
	Raw json.RawMessage `json:"raw,omitempty"`
}

// FromAnthropic maps one provider's reporting into the engine's shape.
//
// It computes nothing and fills nothing in. Every field is either copied or is
// a sum of copied fields, so a Record built from a measured usage stays
// measured, which is the distinction every truth tier downstream rests on.
func FromAnthropic(u transcript.Usage, model string, at time.Time, raw json.RawMessage) Record {
	return Record{
		Provider:      ProviderAnthropic,
		Model:         model,
		At:            at,
		Mechanism:     MechanismExplicitBreakpoint,
		Prompt:        u.PromptTotal(),
		Fresh:         u.Input,
		CachedRead:    u.CacheRead,
		CachedWrite:   u.CacheCreation,
		CachedWrite5m: u.Create5m,
		CachedWrite1h: u.Create1h,
		Output:        u.Output,
		Reasoning:     u.ThinkingTokens,
		Raw:           raw,
	}
}

// ToAnthropic maps back, exactly.
//
// Every reader in the tree still speaks the provider's shape, and this is what
// lets the normalised record be introduced without a flag day. A lossy hop here
// would silently change figures the ledger reports as measured.
func (r Record) ToAnthropic() transcript.Usage {
	return transcript.Usage{
		Input:          r.Fresh,
		CacheCreation:  r.CachedWrite,
		CacheRead:      r.CachedRead,
		Output:         r.Output,
		ThinkingTokens: r.Reasoning,
		Create5m:       r.CachedWrite5m,
		Create1h:       r.CachedWrite1h,
	}
}

// CachedShare is cache reads over the whole prompt, the scale-free figure that
// survives a change of tokenizer and so can be compared across providers.
func (r Record) CachedShare() float64 {
	if r.Prompt <= 0 {
		return 0
	}
	return float64(r.CachedRead) / float64(r.Prompt)
}
