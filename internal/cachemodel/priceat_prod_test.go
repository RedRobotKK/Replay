package cachemodel

import (
	"testing"
	"time"
)

func TestPriceForAt_HonoursADatedWindowThatPriceForIgnores(t *testing.T) {
	r := rules(
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true},
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 2.5, OutputPerMTok: 12.5, ReadMult: 0.1, Priced: true,
			EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30"},
	)
	defer Override(r)()

	inside, ok := PriceForAt("claude-opus-5", at("2026-09-15T00:00:00Z"))
	if !ok || inside.InputPerMTok != 2.5 {
		t.Fatalf("inside the window PriceForAt = %v ok=%v, want promotional 2.5", inside.InputPerMTok, ok)
	}
	outside, ok := PriceForAt("claude-opus-5", at("2026-08-01T00:00:00Z"))
	if !ok || outside.InputPerMTok != 5 {
		t.Fatalf("outside the window PriceForAt = %v ok=%v, want base 5", outside.InputPerMTok, ok)
	}
	if inside.InputPerMTok == outside.InputPerMTok {
		t.Fatal("PriceForAt did not change across the window boundary; dated rows are still theatre")
	}
}

func TestPriceForAt_ZeroTimeFallsBackToPriceFor(t *testing.T) {
	// Dated row first so PriceFor (no clock) and PriceAt(year 1) disagree.
	// If the IsZero short-circuit is deleted, this test fails.
	r := rules(
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 2.5, OutputPerMTok: 12.5, ReadMult: 0.1, Priced: true,
			EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30"},
		ModelRule{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true},
	)
	defer Override(r)()
	want, ok := PriceFor("claude-opus-5")
	if !ok {
		t.Fatal("opus-5 must be priced")
	}
	got, ok := PriceForAt("claude-opus-5", time.Time{})
	if !ok || got.InputPerMTok != want.InputPerMTok {
		t.Fatalf("zero time: PriceForAt = %+v ok=%v, want PriceFor %+v", got, ok, want)
	}
	if want.InputPerMTok == 5 {
		t.Fatal("PriceFor picked the undated row; the short-circuit would be invisible")
	}
}
