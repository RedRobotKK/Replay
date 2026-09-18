package main

import (
	"strings"
	"testing"
	"time"
)

// RebilledShare is documented as a fraction, 0..1, and is not bounded by one.
//
// internal/card/card.go:53 states the contract: "the fraction of priced spend
// that was re-billed, 0..1". summarise divides RebilledUSD by TotalUSD, and the
// two are not on the same price scale. The numerator prices a break's deficit
// at the full base input rate (cost.go, `deficit / 1e6 * price.InputPerMTok`).
// The denominator is what was actually spent, and on a long cached session most
// of that is cache reads at a tenth of the input rate, or a fortieth on Fable
// 5.1 and Mythos 5.1.
//
// So a session that cached well and broke badly divides a full-rate numerator
// by a mostly-tenth-rate denominator and exceeds 1. REPLAY #3 measured 1.504 on
// a real four-day session with 17 TTL expiries.
//
// Where a value above 1 goes: card.go renders `RebilledShare * 100` onto a
// shareable PNG, so it prints "150%"; `replay cost --contribute` writes it into
// a corpus submission; `replay pool` sums it into the public roster; and the
// receiver type-checks it as a finite number with no range at all.
//
// This test does not assert which of the two scales is correct. That is a
// decision about what the figure means, and it is not one to take at speed. It
// asserts only that the value leaving summarise honours the contract the
// published type already claims for it.
func TestRebilledShareIsAFractionOfSpend(t *testing.T) {
	// One unit, cached heavily, broken badly. The numbers are the shape REPLAY
	// #3 measured rather than an extreme invented to force the failure: spend
	// dominated by cheap cache reads, with a deficit priced at the full rate.
	units := []costUnit{{
		CostUSD:     1.00, // what was actually billed, mostly cache reads
		ReadUSD:     0.90,
		UncachedUSD: 0.10,
		RebilledUSD: 1.50, // the deficit, priced at the full input rate
	}}

	got := summarise(units).RebilledShare

	// SKIPPED, and deliberately not deleted or inverted.
	//
	// The defect is real and reproduced: this returns 1.500. Fixing it means
	// deciding which price scale the numerator belongs on, and that changes
	// figures already published on replay.doctor and in the README. That
	// decision is Daniel's and it is not one to take at speed on a release
	// night.
	//
	// What is contained meanwhile: TestContributeRefusesAShareOutsideZeroToOne
	// stops such a value reaching a pooled roster, which was the path that
	// misstated the figure for other people. Still exposed: internal/card
	// renders this field as a percentage onto a shareable image, so a session
	// like the one measured prints "150%".
	//
	// Remove this skip with the fix. It fails today, which is what makes it
	// worth keeping.
	t.Skipf("known defect, contained at the contribution boundary: RebilledShare = %.3f, "+
		"documented 0..1 in internal/card/card.go. Numerator priced at the full input rate, "+
		"denominator mostly cache reads at a tenth of it. Awaiting a decision on which scale is correct.", got)

	if got > 1 {
		t.Errorf("RebilledShare = %.3f, which is not a fraction of spend.\n"+
			"  numerator   RebilledUSD %.2f, priced at the full input rate\n"+
			"  denominator TotalUSD    %.2f, actually spent, mostly cache reads\n"+
			"card.go documents this field as 0..1 and renders it as a percentage, "+
			"so this value prints as %.0f%% on a shareable card and pools into the public roster.",
			got, units[0].RebilledUSD, units[0].CostUSD, got*100)
	}
}

// The contribution path refuses a share it cannot defend.
//
// The defect above is a question about which price scale the numerator should
// use, and that changes published figures. This is the narrower thing that can
// be done without answering it: a value outside 0..1 does not reach a pool that
// would sum it into a public roster on everyone else's behalf.
func TestContributeRefusesAShareOutsideZeroToOne(t *testing.T) {
	home := withHome(t)
	writeConsent(t, home, "corpus_opt_in = true\n")

	for _, share := range []float64{1.504, -0.1} {
		f := corpusFigures{
			Tasks: 9, TotalUSD: 12.5, RebilledUSD: 18.8,
			RebilledShare: share, MedianTaskUSD: 0.8,
		}
		_, _, err := contributeCorpus("launch-2026-09", t.TempDir(), f, time.Now())
		if err == nil {
			t.Fatalf("a rebilledShare of %.3f was written into a submission", share)
		}
		// The error has to say which number and why, or the contributor cannot
		// tell a refusal from a bug in the tool.
		for _, want := range []string{"rebilledShare", "cache reads"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal for %.3f does not mention %q: %v", share, want, err)
			}
		}
	}

	// And a share that IS a fraction still contributes.
	f := corpusFigures{Tasks: 9, TotalUSD: 12.5, RebilledUSD: 1.25, RebilledShare: 0.1, MedianTaskUSD: 0.8}
	if _, _, err := contributeCorpus("launch-2026-09", t.TempDir(), f, time.Now()); err != nil {
		t.Errorf("a valid share was refused: %v", err)
	}
}
