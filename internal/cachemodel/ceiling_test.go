package cachemodel

import (
	"math"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// What a cache-blind budget stops you at.
//
// The measured case: an agent platform's budget collapses the provider's usage
// fields into one token count and bills it at one flat rate. Every cache READ —
// which the provider charges at a fraction of the input price — is billed as
// fresh input. On this repository's own corpus that arithmetic runs 8.1x high
// aggregate and 16.3x on one model, and ignoring the cache is 101% of the error
// while the flat rate contributes -1.1% (docs/evidence/qm-budget-2026-09-08.md).
//
// The consequence that matters is not a bill. It is THROTTLING: a budget with a
// ceiling stops execution when its own arithmetic says the ceiling is reached,
// so a $500/day ceiling computed this way halts agents at roughly $62 of real
// spend. That is an availability defect, and it is what this measures.
//
// Nothing here claims anyone was overcharged. Both sides are arithmetic over
// the same observed token counts, which is why the ratio holds regardless of
// who was billed or whether anyone was.

func usage(in, write, read, out int) transcript.Usage {
	return transcript.Usage{Input: in, CacheCreation: write, CacheRead: read, Output: out, Create5m: write}
}

// CE1: cache reads are the error.
//
// PASS: a corpus that is mostly cache reads prices far higher blind than
// correct.
// FAIL: the two agree, which would mean the read multiplier is not being
// applied and the whole finding is arithmetic noise.
func TestCE1_CacheReadsAreTheError(t *testing.T) {
	p, ok := PriceFor("claude-opus-5")
	if !ok {
		t.Fatal("claude-opus-5 is not in the price table")
	}
	// One request, almost all of it served from cache.
	u := usage(1_000, 0, 100_000, 500)

	correct := CostUSD(u, p)
	blind := BlindCostUSD(u, p.InputPerMTok)
	if blind <= correct {
		t.Fatalf("blind %.4f is not above correct %.4f; a cache read billed as fresh input "+
			"must cost more", blind, correct)
	}
	// The read multiplier is 0.10, so 100k reads cost a tenth of 100k input.
	// Blind pricing should land near ten times the read component.
	if ratio := blind / correct; ratio < 3 {
		t.Errorf("ratio is %.2fx on a corpus that is 99%% cache reads; the published "+
			"aggregate over a mixed corpus was 8.1x, so a nearly-all-reads request "+
			"should be higher than this", ratio)
	}
}

// CE2: with no cache activity the two agree.
//
// The control. If blind and correct differ on a request that never touched the
// cache, the difference is coming from somewhere other than cache accounting
// and every other number here is suspect.
func TestCE2_NoCacheMeansNoDifference(t *testing.T) {
	p, _ := PriceFor("claude-opus-5")
	u := usage(10_000, 0, 0, 0)

	correct := CostUSD(u, p)
	blind := BlindCostUSD(u, p.InputPerMTok)
	if math.Abs(blind-correct) > 1e-9 {
		t.Errorf("blind %.6f != correct %.6f with no cache activity", blind, correct)
	}
}

// CE3: nothing measured is not a ratio of one.
//
// An empty corpus has no ratio. Returning 1.0 would say "your budget is
// accurate" to somebody who measured nothing, which is the defect this
// repository names most often.
func TestCE3_AnEmptyCorpusHasNoRatio(t *testing.T) {
	var e CeilingEffect
	if e.Measured() {
		t.Error("a zero-value CeilingEffect reports itself as measured")
	}
	if r, ok := e.Ratio(); ok {
		t.Errorf("an empty corpus produced a ratio of %v", r)
	}
	if _, ok := e.StopsAt(500); ok {
		t.Error("an empty corpus answered where a $500 ceiling stops")
	}
}

// CE4: the ceiling divides by the ratio.
//
// This is the sentence the whole command exists to print. A ceiling of $500,
// computed by arithmetic that runs 8.1x high, halts execution at $500/8.1 of
// real spend.
func TestCE4_TheCeilingDividesByTheRatio(t *testing.T) {
	e := CeilingEffect{Requests: 10, CorrectUSD: 100, BlindUSD: 810}
	r, ok := e.Ratio()
	if !ok {
		t.Fatal("a measured corpus reports no ratio")
	}
	if math.Abs(r-8.1) > 1e-9 {
		t.Errorf("ratio %.4f, want 8.1", r)
	}
	stop, ok := e.StopsAt(500)
	if !ok {
		t.Fatal("no answer for a $500 ceiling")
	}
	if math.Abs(stop-500.0/8.1) > 1e-9 {
		t.Errorf("a $500 ceiling stops at $%.2f, want $%.2f", stop, 500.0/8.1)
	}
}

// CE5: a ceiling of zero is off, not a division.
//
// Zero means no ceiling, the convention `serve --max-day-usd` and
// `cost --max-avoidable-usd` already use. Dividing it would answer $0.00 and
// read as "your agents stop immediately".
func TestCE5_AZeroCeilingIsOff(t *testing.T) {
	e := CeilingEffect{Requests: 10, CorrectUSD: 100, BlindUSD: 810}
	if _, ok := e.StopsAt(0); ok {
		t.Error("a ceiling of zero was treated as a limit rather than as off")
	}
	if _, ok := e.StopsAt(-5); ok {
		t.Error("a negative ceiling produced an answer")
	}
}

// CE6: the flat rate is nearly harmless; the cache is the error.
//
// The published decomposition: cache-blindness is 101% of the error and the
// flat rate is -1.1%. So pricing the same tokens blind at the model's OWN rate
// must still land close to the blind figure at a generic rate — if swapping the
// rate moved the answer much, the headline would be about rates and it is not.
func TestCE6_TheRateIsNotWhereTheErrorLives(t *testing.T) {
	p, _ := PriceFor("claude-opus-5")
	u := usage(5_000, 2_000, 80_000, 1_000)

	atModelRate := BlindCostUSD(u, p.InputPerMTok)
	atFlatRate := BlindCostUSD(u, 5.0)
	correct := CostUSD(u, p)

	cacheError := atModelRate - correct
	rateError := atFlatRate - atModelRate
	if math.Abs(rateError) >= math.Abs(cacheError) {
		t.Errorf("swapping the rate moved the answer by %.4f and ignoring the cache moved it "+
			"by %.4f; the finding says the cache is the error and the rate is nearly "+
			"harmless", rateError, cacheError)
	}
}

// CE7: a subscriber is never handed a bill.
//
// `replay cost` already refuses to let a subscriber read list prices as money.
// A ceiling report that ignored the basis would tell a Max seat they are losing
// dollars they never spend, which is the single most likely way this feature
// becomes dishonest.
func TestCE7_ASubscriberIsNeverHandedABill(t *testing.T) {
	e := CeilingEffect{Requests: 10, CorrectUSD: 100, BlindUSD: 794}
	e.AddTokens(usage(5_000, 2_000, 80_000, 1_000))

	sub := e.Note(BasisSubscription, 500)
	if strings.Contains(sub, "halts execution at $") {
		t.Errorf("a subscription seat was told where a dollar ceiling stops it:\n%s", sub)
	}
	if !strings.Contains(sub, "NOT MEASURED") {
		t.Errorf("the allowance note does not name what is unmeasured:\n%s", sub)
	}
	if !strings.Contains(sub, "none of the dollar figures are money") ||
		!strings.Contains(sub, "list price for somebody who is billed per token") {
		t.Errorf("the note does not tell a subscriber the dollars are not theirs:\n%s", sub)
	}

	metered := e.Note(BasisMetered, 500)
	if !strings.Contains(metered, "halts execution at $") {
		t.Errorf("a metered reader was not told where the ceiling stops them:\n%s", metered)
	}
}

// CE8: an unstated basis is refused, not guessed.
//
// classifyRoute already documents why: a bare first-party model id is emitted
// by an API key and by a subscription alike, so nothing in a transcript settles
// this. Guessing would put a bill in front of half the readers who get it wrong.
func TestCE8_AnUnstatedBasisIsRefused(t *testing.T) {
	e := CeilingEffect{Requests: 10, CorrectUSD: 100, BlindUSD: 794}
	note := e.Note(BasisUnknown, 500)
	if !strings.Contains(note, "NOT MEASURED") {
		t.Errorf("an unstated basis produced an answer:\n%s", note)
	}
	if strings.Contains(note, "$62") || strings.Contains(note, "halts execution") {
		t.Errorf("an unstated basis was priced anyway:\n%s", note)
	}
}

// CE9: allowance tokens count unpriced models too.
//
// A model missing from the price table still consumed a subscriber's
// allowance. Dropping it, as the dollar path must, would under-report the one
// figure that is actually theirs.
func TestCE9_AllowanceCountsUnpricedModels(t *testing.T) {
	var e CeilingEffect
	e.Add("no-such-model-anywhere", usage(1_000, 2_000, 3_000, 400), 5.0)
	e.AddTokens(usage(1_000, 2_000, 3_000, 400))

	if e.Unpriced != 1 {
		t.Errorf("Unpriced = %d, want 1", e.Unpriced)
	}
	if e.CorrectUSD != 0 || e.BlindUSD != 0 {
		t.Error("an unpriced model contributed to a dollar total")
	}
	if got := e.Tokens.Total(); got != 6_400 {
		t.Errorf("allowance tokens = %d, want 6400; an unpriced model still spent them", got)
	}
	if !strings.Contains(e.AllowanceNote(), "2000 written to cache") {
		t.Errorf("the allowance note does not report the unpriced model's tokens:\n%s", e.AllowanceNote())
	}
}
