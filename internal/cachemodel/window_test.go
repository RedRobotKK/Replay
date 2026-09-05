package cachemodel

import (
	"testing"
	"time"
)

// Dated prices, and a discount only the account holder can know.
//
// A vendor promotion is a pricing event, not a fact about traffic. The ledger
// records quantities — tokens, reads, writes, timings — and those do not change
// because someone ran a sale. So a promotion is expressed as a rules row with a
// validity window, and a request is priced by the rules in effect at ITS OWN
// timestamp rather than at the time of the report.
//
// That distinction is the whole feature. Without it a report spanning the end
// of a promotion prices the whole period at one rate, and is wrong on one side
// of the boundary no matter which rate it picks.
//
//	W1  an undated row applies at any time, as it always has
//	W2  a dated row applies only inside its window
//	W3  inside the window the promotion wins over the base rate
//	W4  outside it, the base rate returns
//	W5  a backwards window is refused at load
//	W6  overlapping windows for one model are refused as ambiguous
//	W7  an account discount is applied and labelled declared, never measured
//	W8  an implausible discount is refused rather than believed

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func rules(models ...ModelRule) *Rules {
	return &Rules{Schema: RulesSchema, Version: "test", Models: models}
}

func TestW1_AnUndatedRowAppliesAtAnyTime(t *testing.T) {
	// PASS: no window means always, which is every row written before this
	// feature existed.
	// FAIL: an undated row silently ceasing to apply, which would break every
	// existing rules document.
	r := rules(ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true})
	for _, when := range []string{"2020-01-01T00:00:00Z", "2026-09-05T00:00:00Z", "2099-01-01T00:00:00Z"} {
		got, ok := r.PriceAt("claude-opus-5", at(when))
		if !ok || got.InputPerMTok != 5 {
			t.Errorf("%s: got %v ok=%v, want the undated row", when, got.InputPerMTok, ok)
		}
	}
}

func TestW2_ADatedRowAppliesOnlyInsideItsWindow(t *testing.T) {
	// PASS: applies from the start instant through the end instant, and not
	// outside.
	// FAIL: an off-by-one at either edge. A promotion that appears a day early
	// or lingers a day late misprices every request in that day.
	r := rules(ModelRule{
		Match: "opus-5", MinPrefix: 512, InputPerMTok: 2.5, OutputPerMTok: 12.5, ReadMult: 0.1, Priced: true,
		EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30",
	})
	inside := []string{"2026-09-01T00:00:00Z", "2026-09-15T12:00:00Z", "2026-09-30T23:59:59Z"}
	for _, when := range inside {
		if _, ok := r.PriceAt("claude-opus-5", at(when)); !ok {
			t.Errorf("%s is inside the window and must be priced", when)
		}
	}
	for _, when := range []string{"2026-08-31T23:59:59Z", "2026-10-01T00:00:00Z"} {
		if _, ok := r.PriceAt("claude-opus-5", at(when)); ok {
			t.Errorf("%s is outside the window and must not match this row", when)
		}
	}
}

func TestW3_InsideTheWindowThePromotionWins(t *testing.T) {
	// PASS: the dated row beats the undated one while it is in effect.
	// FAIL: file order deciding it. A promotion that loses to the base rate is
	// a promotion that does nothing, and one that wins outside its window
	// understates spend forever.
	r := rules(
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true},
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 2.5, OutputPerMTok: 12.5, ReadMult: 0.1, Priced: true,
			EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30"},
	)
	got, ok := r.PriceAt("claude-opus-5", at("2026-09-15T00:00:00Z"))
	if !ok || got.InputPerMTok != 2.5 {
		t.Errorf("inside the window: input = %v, want the promotional 2.5", got.InputPerMTok)
	}
}

func TestW4_OutsideTheWindowTheBaseRateReturns(t *testing.T) {
	// PASS: the undated row again, at full price.
	// FAIL: the promotion persisting, which understates every later report.
	r := rules(
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true},
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 2.5, OutputPerMTok: 12.5, ReadMult: 0.1, Priced: true,
			EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30"},
	)
	got, ok := r.PriceAt("claude-opus-5", at("2026-10-02T00:00:00Z"))
	if !ok || got.InputPerMTok != 5 {
		t.Errorf("after the window: input = %v, want the base 5", got.InputPerMTok)
	}
}

func TestW5_ABackwardsWindowIsRefused(t *testing.T) {
	// PASS: refused at load with a reason.
	// FAIL: accepting a window that can never contain anything, which prices
	// nothing and looks like a working promotion.
	r := rules(ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true,
		EffectiveFrom: "2026-09-30", EffectiveUntil: "2026-09-01"})
	if err := r.validate(); err == nil {
		t.Error("a window ending before it starts must be refused")
	}
	bad := rules(ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true,
		EffectiveFrom: "not-a-date"})
	if err := bad.validate(); err == nil {
		t.Error("an unparseable date must be refused rather than ignored")
	}
}

func TestW6_OverlappingWindowsAreRefused(t *testing.T) {
	// PASS: refused. Two promotions covering the same instant for the same
	// model make the price depend on file order, and a figure that depends on
	// which line came first is not a figure anyone should act on.
	// FAIL: silently picking one.
	r := rules(
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 2.5, OutputPerMTok: 12, ReadMult: 0.1, Priced: true,
			EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30"},
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 3, OutputPerMTok: 15, ReadMult: 0.1, Priced: true,
			EffectiveFrom: "2026-09-15", EffectiveUntil: "2026-10-15"},
	)
	if err := r.validate(); err == nil {
		t.Error("two windows covering the same instant for one model must be refused as ambiguous")
	}
}

func TestW7_ADeclaredDiscountIsAppliedAndLabelled(t *testing.T) {
	// An account-level rate is negotiated, private, and invisible on the wire.
	// Replay cannot measure it and must not pretend to: it is applied only
	// when the operator states it, and it is labelled `declared` so no reader
	// mistakes it for something observed.
	// PASS: applied, and the tier says declared.
	// FAIL: a discounted figure wearing the measured or estimated label.
	r := rules(ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true})
	r.AccountDiscount = 0.8

	got, ok := r.PriceAt("claude-opus-5", at("2026-09-05T00:00:00Z"))
	if !ok {
		t.Fatal("the row must still price")
	}
	if got.InputPerMTok != 8 {
		t.Errorf("input = %v, want 8 (10 at a declared 0.8)", got.InputPerMTok)
	}
	if got.OutputPerMTok != 40 {
		t.Errorf("output = %v, want 40", got.OutputPerMTok)
	}
	if r.PriceTier() != "declared" {
		t.Errorf("tier = %q, want declared: a rate nobody can observe must say so", r.PriceTier())
	}

	// With no discount stated, nothing is applied and the tier is unchanged.
	plain := rules(ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true})
	if p, _ := plain.PriceAt("claude-opus-5", at("2026-09-05T00:00:00Z")); p.InputPerMTok != 10 {
		t.Errorf("input = %v with no discount declared, want 10 untouched", p.InputPerMTok)
	}
	if plain.PriceTier() == "declared" {
		t.Error("a document with no declared discount must not claim one")
	}
}

func TestW8_AnImplausibleDiscountIsRefused(t *testing.T) {
	// PASS: refused at load.
	// FAIL: believing it. A negative multiplier turns spend into savings, and
	// one above 1 is a surcharge nobody negotiated — both are far more likely
	// to be a typo than a deal.
	// Zero is absent, not implausible: an omitted JSON field unmarshals to
	// zero, so a document with no discount and one that explicitly writes 0
	// are the same bytes. Refusing it would refuse every ordinary document.
	for _, d := range []float64{-0.5, 1, 1.5, 100} {
		r := rules(ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true})
		r.AccountDiscount = d
		if err := r.validate(); err == nil {
			t.Errorf("accountDiscount %v must be refused", d)
		}
	}
	for _, d := range []float64{0, 0.85, 0.999} {
		ok := rules(ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true})
		ok.AccountDiscount = d
		if err := ok.validate(); err != nil {
			t.Errorf("accountDiscount %v must be accepted: %v", d, err)
		}
	}
}
