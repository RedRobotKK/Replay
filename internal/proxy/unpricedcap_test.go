package proxy

import (
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
)

// An unpriced model must not spend nothing against a dollar cap.
//
// TOKEN-PRICES.md names the two defensible answers — refuse, or price at the
// most expensive known row and label it an upper bound — and says counting it
// as zero is neither. Zero fails OPEN: an operator who asked to stop at $20 has
// no cap at all on the traffic most likely to be expensive, because an unknown
// model is usually a new one, and new models have not historically been
// cheaper.
//
// #261 records the decision; ADR-0022 is the same argument one field over, for
// the read multiple.

func heavyUsage() ledger.Usage {
	return ledger.Usage{Input: 1_000_000, CacheCreation: 200_000, CacheRead: 800_000, Output: 50_000}
}

// The headline: an unknown model costs something.
func TestUnpricedCap_AnUnknownModelIsNotFree(t *testing.T) {
	got := listCost(heavyUsage(), "claude-opus-99-does-not-exist")
	if got <= 0 {
		t.Fatalf("an unknown model priced at %v. A dollar cap accumulating zero never reaches "+
			"its limit, so an operator who asked to stop at $20 has no cap on exactly the "+
			"traffic most likely to be expensive", got)
	}
}

// And it is an UPPER bound, not a guess: at least what the dearest known model
// would have cost for the same usage.
func TestUnpricedCap_TheEstimateIsAnUpperBound(t *testing.T) {
	u := heavyUsage()
	unknown := listCost(u, "claude-opus-99-does-not-exist")

	dearest, ok := cachemodel.DearestPrice()
	if !ok {
		t.Fatal("the table holds no priced row; this test compares against nothing")
	}
	want := cachemodel.CostUSD(u, dearest)
	if unknown != want {
		t.Fatalf("an unknown model priced at %v, the dearest known row at %v. The figure is "+
			"only defensible as an upper bound, and a bound below the dearest row is not one",
			unknown, want)
	}

	// Against every priced model, the unknown figure is at least as large.
	// That is what "upper bound" means and it is the property, not the number.
	for _, m := range []string{"claude-opus-5", "claude-sonnet-5", "claude-haiku-4-5"} {
		p, pok := cachemodel.PriceFor(m)
		if !pok {
			t.Fatalf("%s is unpriced; this fixture no longer tests what it claims", m)
		}
		if known := cachemodel.CostUSD(u, p); unknown < known {
			t.Errorf("unknown priced at %v, below %s at %v — not an upper bound", unknown, m, known)
		}
	}
}

// A known model is unaffected. The fallback is for the unknown case only, and
// a change that quietly re-priced everything would be far worse than the bug
// it fixed.
func TestUnpricedCap_AKnownModelIsUnchanged(t *testing.T) {
	u := heavyUsage()
	p, ok := cachemodel.PriceFor("claude-haiku-4-5")
	if !ok {
		t.Fatal("haiku-4-5 is unpriced; this fixture is wrong")
	}
	want := cachemodel.CostUSD(u, p)
	if got := listCost(u, "claude-haiku-4-5"); got != want {
		t.Fatalf("a known model priced at %v, want its own row's %v", got, want)
	}
	// The premise: haiku really is cheaper than the bound, or this test would
	// pass even if every model got the dearest price.
	if dearest, dok := cachemodel.DearestPrice(); dok {
		if cachemodel.CostUSD(u, dearest) <= want {
			t.Fatal("the dearest row is no dearer than haiku; this test cannot tell the " +
				"fallback from the real price")
		}
	}
}

// DearestPrice is the dearest, derived rather than asserted.
func TestUnpricedCap_DearestPriceIsTheDearest(t *testing.T) {
	dearest, ok := cachemodel.DearestPrice()
	if !ok {
		t.Fatal("no priced row")
	}
	u := heavyUsage()
	bound := cachemodel.CostUSD(u, dearest)
	for _, m := range []string{"claude-opus-5", "claude-opus-4-1", "claude-sonnet-5", "claude-haiku-4-5", "claude-3-5-haiku"} {
		p, pok := cachemodel.PriceFor(m)
		if !pok {
			continue
		}
		if c := cachemodel.CostUSD(u, p); c > bound {
			t.Errorf("%s costs %v, above the supposed dearest %v", m, c, bound)
		}
	}
}
