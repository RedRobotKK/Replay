package cachemodel

import (
	"fmt"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// sprintf is fmt.Sprintf, named locally so the notes below read as prose.
var sprintf = fmt.Sprintf

// What a cache-blind budget stops you at.
//
// An agent platform with a spend ceiling enforces it against its own
// arithmetic. When that arithmetic collapses the provider's usage fields into
// one token count and prices it at one flat rate, every cache READ is billed as
// fresh input — and the provider charges a fraction of the input price for a
// read. Measured against this repository's corpus the result runs 8.1x high
// aggregate and 16.3x on one model, with cache-blindness accounting for 101% of
// the error and the flat rate for -1.1%
// (docs/evidence/qm-budget-2026-09-08.md).
//
// The consequence worth acting on is not a bill. It is THROTTLING: the ceiling
// is reached when the budget's own arithmetic says so, so a $500/day ceiling
// computed this way halts execution at roughly $62 of real spend. Agents stop
// at an eighth of the budget somebody approved, and nothing in the platform
// reports why.
//
// Nothing here is a claim that anyone was overcharged. Both figures are
// arithmetic over the same observed token counts, which is why the ratio holds
// regardless of who was billed or whether anyone was — it would hold on a free
// tier. Quoting it as money recovered would be false, and the evidence file
// says so at more length.

// BlindCostUSD prices usage the way a cache-blind budget does: every token in
// the usage object at one flat rate, with no read multiplier and no write
// premium.
//
// It models the DEFECT CLASS rather than any one implementation. A budget that
// collapses the fields differently will land on a different number; what they
// share is that a cache read costs the same as fresh input, and that is the
// term this isolates.
func BlindCostUSD(u transcript.Usage, ratePerMTok float64) float64 {
	tokens := float64(u.Input + u.CacheCreation + u.CacheRead)
	return (tokens*ratePerMTok + float64(u.Output)*ratePerMTok) / tokensPerMillion
}

// CeilingEffect is what a corpus says about a cache-blind ceiling.
//
// Both totals are sums over the same requests, so the ratio between them is a
// property of the arithmetic and not of the billing relationship.
type CeilingEffect struct {
	// Requests is how many priced requests are behind these figures. It is the
	// n, and it travels because a ratio without one is not checkable.
	Requests int
	// CorrectUSD prices every request at published rates with the cache
	// accounted for.
	CorrectUSD float64
	// BlindUSD prices the same requests with every prompt token at one flat
	// input rate.
	BlindUSD float64
	// Tokens is what the provider counted, priced or not. On a subscription
	// seat this is the finding and the dollars are somebody else's.
	Tokens Tokens
	// Unpriced counts requests left out because their model is not in the
	// price table. Excluded rather than counted as free, and reported so the
	// ratio can say what it does not cover.
	Unpriced int
}

// Measured reports whether there is anything here to state a ratio about.
//
// A corpus that priced nothing has no ratio, and this is the guard that stops
// one being invented. Correct spend of zero is the decisive test rather than
// the request count: requests that all priced to nothing give a denominator of
// zero, and "your budget is 0x wrong" is not a sentence about anything.
func (e CeilingEffect) Measured() bool {
	return e.Requests > 0 && e.CorrectUSD > 0
}

// Ratio is how many times higher the blind arithmetic runs.
//
// The bool is the point. An unmeasured corpus returns false rather than 1.0,
// because 1.0 reads as "your budget is accurate" and would be told to somebody
// who measured nothing.
func (e CeilingEffect) Ratio() (float64, bool) {
	if !e.Measured() {
		return 0, false
	}
	return e.BlindUSD / e.CorrectUSD, true
}

// StopsAt answers where a ceiling set under blind arithmetic actually halts
// execution, in real spend.
//
// A ceiling of zero or less is OFF, the convention `serve --max-day-usd` and
// `cost --max-avoidable-usd` already use. Dividing zero would answer $0.00 and
// read as "your agents stop immediately", which is the opposite of what a
// disabled ceiling means.
func (e CeilingEffect) StopsAt(ceilingUSD float64) (float64, bool) {
	if ceilingUSD <= 0 {
		return 0, false
	}
	r, ok := e.Ratio()
	if !ok {
		return 0, false
	}
	return ceilingUSD / r, true
}

// Add folds one request's usage into the effect.
//
// An unpriced model is counted and skipped on BOTH sides. Pricing it on one
// side only would move the ratio by an amount that has nothing to do with cache
// accounting, which is the single thing this measures.
func (e *CeilingEffect) Add(model string, u transcript.Usage, blindRatePerMTok float64) {
	p, ok := PriceFor(model)
	if !ok {
		e.Unpriced++
		return
	}
	e.Requests++
	e.CorrectUSD += CostUSD(u, p)
	e.BlindUSD += BlindCostUSD(u, blindRatePerMTok)
}

// Basis is how the reader is billed, and it decides which figures mean
// anything to them.
//
// This distinction is not a nicety. `replay cost` already refuses to let a
// subscriber read list prices as money — "you are not billed per token, so the
// dollars above are list price for someone who is. The tokens are still yours"
// — and a ceiling report that ignored it would be telling a Max subscriber they
// are losing dollars they never spend.
type Basis int

const (
	// BasisUnknown is the honest default. A bare first-party model id is
	// emitted by an API key and by a subscription alike, so nothing in a
	// transcript settles this; the reader has to say.
	BasisUnknown Basis = iota
	// BasisMetered is billed per token: the dollar figures are the reader's.
	BasisMetered
	// BasisSubscription is a Pro, Max, Team or Enterprise seat. Dollar figures
	// are list-price equivalents for somebody else, and the currency that
	// matters is the usage allowance.
	BasisSubscription
)

// Tokens carried alongside the money, because on a subscription seat the tokens
// ARE the finding and the dollars are somebody else's.
type Tokens struct {
	Input, CacheWrite, CacheRead, Output int
}

// Total is every token the provider counted for these requests.
func (t Tokens) Total() int { return t.Input + t.CacheWrite + t.CacheRead + t.Output }

// AddTokens folds one request's counts in, unpriced or not.
//
// Deliberately independent of the price table: a model missing from the table
// still consumed a subscriber's allowance, and dropping it here would under-
// report the one figure that is theirs.
func (e *CeilingEffect) AddTokens(u transcript.Usage) {
	e.Tokens.Input += u.Input
	e.Tokens.CacheWrite += u.CacheCreation
	e.Tokens.CacheRead += u.CacheRead
	e.Tokens.Output += u.Output
}

// MeteredNote and AllowanceNote are the sentences each basis earns.

// MeteredNote states what the ratio costs a reader billed per token.
func (e CeilingEffect) MeteredNote(ceilingUSD float64) string {
	r, ok := e.Ratio()
	if !ok {
		return "NOT MEASURED: nothing here priced, so there is no ratio to state."
	}
	if stop, ok := e.StopsAt(ceilingUSD); ok {
		return sprintf("A ceiling set at $%.2f under cache-blind arithmetic halts execution at "+
			"$%.2f of real spend: %.2fx high, so agents stop at %.0f%% of the budget that was "+
			"approved.", ceilingUSD, stop, r, 100/r)
	}
	return sprintf("Cache-blind arithmetic runs %.2fx high over these %d requests. No ceiling "+
		"was given, so where it would halt is not computed.", r, e.Requests)
}

// AllowanceNote states what is true for a subscription seat, and — more
// importantly — what is not measured.
//
// The temptation is to convert re-billed tokens into a fraction of a quota and
// print a percentage. This repository measured that question and retracted the
// answer in full: whether cache reads carry the 0.10x discount against the
// ALLOWANCE as they do against the bill is NOT MEASURED, and it is the
// parameter any such figure would be most sensitive to. So this reports the
// tokens, names the unknown, and stops.
func (e CeilingEffect) AllowanceNote() string {
	t := e.Tokens
	if t.Total() == 0 {
		return "NOT MEASURED: no tokens were read, so there is nothing to say about allowance."
	}
	return sprintf("On a subscription seat none of the dollar figures are money — they are list "+
		"price for somebody who is billed per token. What is yours is the tokens: %d written to "+
		"cache and %d read back, out of %d prompt tokens. A cache write that could have been a "+
		"read spends allowance twice. How much: NOT MEASURED. Whether reads weigh against the "+
		"allowance at the same 0.10x discount they get against a bill has never been measured "+
		"here, and every percentage anyone quotes turns on it.",
		t.CacheWrite, t.CacheRead, t.Input+t.CacheWrite+t.CacheRead)
}

// Note dispatches on basis, so a caller cannot hand a subscriber a bill.
func (e CeilingEffect) Note(b Basis, ceilingUSD float64) string {
	switch b {
	case BasisMetered:
		return e.MeteredNote(ceilingUSD)
	case BasisSubscription:
		return e.AllowanceNote()
	default:
		return "NOT MEASURED: the billing basis was not given. A first-party model id is " +
			"emitted by an API key and by a subscription alike, so nothing in a transcript " +
			"settles it. Pass --metered or --subscription."
	}
}
