package main

import (
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// buildRoute priced the observed side and the projected side of the SAME
// comparison with different cache-write multipliers.
//
// The observed usage comes off the wire carrying its TTL split: every one of
// 59,348 creation-bearing usage objects on this machine has the nested
// cache_creation object, and in all of them 5m + 1h equals the total. So
// `observed` prices a one-hour write at WriteMultiplierLong, 2.0.
//
// `scaled` was rebuilt field by field — Input, CacheCreation, CacheRead,
// Output — and dropped Create5m and Create1h. With no split,
// cachemodel.writeEquivalent falls back to WriteMultiplierShort, 1.25. So
// `projected` priced the same writes 37.5% cheaper than `observed` did, for
// no reason a reader could see.
//
// Both halves of the error point the same way: the destination model looks
// cheaper than it is, and the switch looks better than it is. That is the
// figure this command exists to produce.
//
// The fixture needs no magic number. fable-5 and mythos-5 carry identical
// price rows, so at a measured sigma of 1.0 the projection is the identity:
// the same tokens, the same rates. Anything other than equality is the bug.
func fitAt(tpb float64, turns int) analysis.TokenFit {
	return analysis.TokenFit{TokensPerByte: tpb, RelativeError: 0.05, Turns: turns}
}

func TestRT1_TheProjectionKeepsTheTTLSplit(t *testing.T) {
	const from, to = "fable-5", "mythos-5"

	pf, okf := cachemodel.PriceFor(from)
	pt, okt := cachemodel.PriceFor(to)
	if !okf || !okt {
		t.Fatalf("both models must be priced for this fixture: %v %v", okf, okt)
	}
	if pf != pt {
		t.Fatalf("this test relies on %s and %s carrying IDENTICAL price rows, so "+
			"that a sigma of 1.0 makes the projection an identity. They now differ: "+
			"%+v vs %+v. Pick another pair or the invariant below is not one.", from, to, pf, pt)
	}

	// A one-hour write is the normal case for Claude Code on this machine,
	// and it is the case the dropped split mis-prices.
	u := transcript.Usage{
		Input:         10_000,
		CacheCreation: 1_000_000,
		Create1h:      1_000_000,
		CacheRead:     500_000,
		Output:        2_000,
	}
	c := modelCorpus{
		fits:  map[string]analysis.TokenFit{from: fitAt(0.25, 100), to: fitAt(0.25, 100)},
		turns: map[string]int{from: 100, to: 100},
		usage: map[string]transcript.Usage{from: u},
		hits:  80,
		total: 100,
	}

	r := buildRoute(from, to, c)
	if !r.Dilation.Measured {
		t.Fatalf("sigma is not measured for this fixture, so the projection branch "+
			"never ran and this test asserts nothing: %+v", r.Dilation)
	}
	if r.Dilation.Sigma != 1.0 {
		t.Fatalf("sigma = %v, want exactly 1.0 — equal fits must give an identity "+
			"projection or the invariant below does not hold", r.Dilation.Sigma)
	}
	if r.Observed == nil || r.Dollars == nil {
		t.Fatal("the projection produced no figures")
	}

	if *r.Dollars != *r.Observed {
		t.Errorf("projected $%.6f != observed $%.6f at sigma 1.0 between two models "+
			"with identical price rows.\n"+
			"The only difference between the two sides is that the projected usage "+
			"lost its TTL split, so a 1h write (x%.2f) was priced as a 5m write "+
			"(x%.2f) — %.1f%% cheap, in the direction that flatters the switch.",
			*r.Dollars, *r.Observed,
			cachemodel.WriteMultiplierLong, cachemodel.WriteMultiplierShort,
			100*(1-cachemodel.WriteMultiplierShort/cachemodel.WriteMultiplierLong))
	}
}

// The switch cost is the other half. The prefix the destination must write
// once was built as transcript.Usage{CacheCreation: prefix} with no TTL at
// all, so it too was priced at the short rate — making the payback look
// faster than it is.
func TestRT2_TheSwitchCostUsesTheObservedTTL(t *testing.T) {
	const from, to = "fable-5", "mythos-5"
	u := transcript.Usage{
		Input:         10_000,
		CacheCreation: 1_000_000,
		Create1h:      1_000_000,
		CacheRead:     500_000,
		Output:        2_000,
	}
	c := modelCorpus{
		fits:  map[string]analysis.TokenFit{from: fitAt(0.25, 100), to: fitAt(0.25, 100)},
		turns: map[string]int{from: 100, to: 100},
		usage: map[string]transcript.Usage{from: u},
		hits:  80,
		total: 100,
	}
	r := buildRoute(from, to, c)
	if r.Switch == nil {
		t.Fatal("no switch figure, so this asserts nothing")
	}

	// The prefix write, priced both ways. The session wrote 1h exclusively,
	// so the destination writing that prefix writes 1h too.
	pt, _ := cachemodel.PriceFor(to)
	prefix := scaleTokens(u.CacheRead/c.total, r.Dilation.Sigma)
	short := cachemodel.CostUSD(transcript.Usage{CacheCreation: prefix}, pt)
	long := cachemodel.CostUSD(
		transcript.Usage{CacheCreation: prefix, Create1h: prefix}, pt)
	if short == long {
		t.Fatal("the two TTLs price the same here, so this fixture cannot tell " +
			"them apart and the assertion below proves nothing")
	}

	// Recompute what the report should have used.
	want := analysis.Payback(long, *r.Observed, *r.Dollars, c.total)
	if r.Switch.CostUSD != want.CostUSD {
		t.Errorf("switch cost $%.6f, want $%.6f.\nThe prefix write was priced at the "+
			"5m rate on a session that wrote 1h exclusively, so the payback reads "+
			"faster than it is.", r.Switch.CostUSD, want.CostUSD)
	}
}

// scaleUsage directly, at a sigma that is NOT a whole number — which is the
// only case where the three counts can disagree. Scaling the total and the
// two parts independently lets total != 5m + 1h, an inconsistency the wire
// never produces: all 59,348 creation-bearing records on this machine hold
// total == 5m + 1h exactly.
func TestRT3_ScalingKeepsTheTotalEqualToItsParts(t *testing.T) {
	// Both parts round UP while the total rounds DOWN — 3 and 3 at sigma 0.5
	// give 2 + 2 = 4 against a scaled total of 3. Chosen deliberately: a
	// fixture whose roundings happen to agree cannot tell whether the
	// recomputation ran, and my first one did exactly that.
	u := transcript.Usage{CacheCreation: 6, Create5m: 3, Create1h: 3}
	for _, sigma := range []float64{0.5, 1.5, 0.45} {
		got := scaleUsage(u, sigma)
		if got.CacheCreation != got.Create5m+got.Create1h {
			t.Errorf("sigma %v: CacheCreation %d != 5m %d + 1h %d. A Usage whose "+
				"total disagrees with its split is one the wire never produces, "+
				"and CostUSD reads the two independently.",
				sigma, got.CacheCreation, got.Create5m, got.Create1h)
		}
	}
}

// The other arm: with no split observed there is nothing to recompute from,
// and the scaled total must survive. Deleting the conditional would zero it.
func TestRT4_WithNoSplitTheScaledTotalSurvives(t *testing.T) {
	u := transcript.Usage{CacheCreation: 1000}
	got := scaleUsage(u, 2.0)
	if got.CacheCreation == 0 {
		t.Fatal("a creation with no TTL split scaled to zero: the total is the " +
			"only count there is in that case and it was dropped")
	}
	if got.Create5m != 0 || got.Create1h != 0 {
		t.Errorf("scaling invented a TTL split that was never reported: 5m %d, 1h %d. "+
			"Absence is not zero and it is not a guess.", got.Create5m, got.Create1h)
	}
}
