package cachemodel

import (
	"testing"
	"time"
)

// A negotiated rate the operator stated does not reach any number they see.
//
// Rules.AccountDiscount is documented, JSON-configurable, and applied by
// applyDiscount — which is called only from inside PriceAt. PriceAt has no
// production callers. Every dollar Replay prints comes from PriceFor, and
// PriceFor never applies it.
//
// So an operator who states a 50% negotiated rate is charged list, silently,
// in `replay cost`, in the spend cap, and in anything a client is handed. The
// operator most likely to have stated a discount is the one most likely to be
// checking the arithmetic against a real invoice.
//
// This is a guard that cannot fire, on the money path, for a feature the rules
// document advertises.
func TestDC1_AStatedDiscountReachesThePriceTheToolPrints(t *testing.T) {
	r := &Rules{
		Schema: RulesSchema, Version: "test",
		AccountDiscount: 0.5,
		Models: []ModelRule{{
			Match: "claude-opus-5", MinPrefix: 1024,
			InputPerMTok: 10, OutputPerMTok: 100, ReadMult: 0.1, Priced: true,
		}},
	}
	defer Override(r)()

	got, ok := PriceFor("claude-opus-5")
	if !ok {
		t.Fatal("the model is in the installed document and did not price")
	}
	if got.InputPerMTok != 5 {
		t.Errorf("input = $%.2f, want $5.00. The operator stated a 0.5 negotiated rate "+
			"and the tool charged them list", got.InputPerMTok)
	}
	if got.OutputPerMTok != 50 {
		t.Errorf("output = $%.2f, want $50.00", got.OutputPerMTok)
	}
	// The read multiplier is a ratio, not a rate, so the discount must NOT
	// touch it — discounting it twice would understate cached reads.
	if got.ReadMult != 0.1 {
		t.Errorf("readMult = %v, want 0.1 unchanged: it multiplies the already "+
			"discounted input price, and discounting it too applies the rate twice",
			got.ReadMult)
	}
}

// DC2: the two paths agree. Whatever PriceAt says at a time inside the row's
// window, PriceFor says for the same model.
//
// They disagreed by construction: one applied the discount and the other did
// not, and only the one nothing calls was right.
func TestDC2_BothPricePathsAgree(t *testing.T) {
	r := &Rules{
		Schema: RulesSchema, Version: "test",
		AccountDiscount: 0.25,
		Models: []ModelRule{{
			Match: "claude-sonnet-5", MinPrefix: 512,
			InputPerMTok: 8, OutputPerMTok: 40, ReadMult: 0.025, Priced: true,
		}},
	}
	defer Override(r)()

	pf, okf := PriceFor("claude-sonnet-5")
	pa, oka := r.PriceAt("claude-sonnet-5", time.Now())
	if okf != oka {
		t.Fatalf("priced disagrees: PriceFor=%v PriceAt=%v", okf, oka)
	}
	if pf != pa {
		t.Errorf("the two price paths disagree:\n  PriceFor %+v\n  PriceAt  %+v\n"+
			"      Only one of them is called in production and it was the wrong one",
			pf, pa)
	}
}
