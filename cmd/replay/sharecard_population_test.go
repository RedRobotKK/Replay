package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// PROOF-1.0 blocker 2, the half that was not fixed.
//
// The share card's headline is a share of PRICED spend: costSummary.RebilledShare
// is RebilledUSD over TotalUSD, and TotalUSD excludes every session the price
// table cannot price. The PNG was corrected to say "of priced spend, paid twice"
// on 2026-10-01. The text card, which is the one a reader copies out of the
// terminal, kept saying "of my agent spend". On a corpus where one request in
// three was priced, it said "37% of my agent spend was paid twice" about a
// figure that covered a third of the work.
//
// The assertion is on the rendered card, through dispatch, not on a string
// somewhere in the source: the card is the artifact that leaves the machine.
func TestShareCardNamesThePricedPopulation(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	e2eLedger(t, dir)

	out, errb, err := e2e(t, "cost", "--share", dir)
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	// POSITIVE CONTROL: a headline was rendered at all.
	if !strings.Contains(out, "was paid twice.") {
		t.Fatalf("no headline rendered:\n%s", out)
	}
	headline := ""
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "was paid twice.") {
			headline = strings.TrimSpace(l)
		}
	}
	// The figure itself, from the fixture: one priced session of two requests
	// at 5,000 creation tokens each, the second a break, so the re-billed
	// share of priced spend is 30%. A denominator that drifted to all spend,
	// or to twice the priced spend, moves this number.
	if !strings.HasPrefix(headline, "30% ") {
		t.Errorf("the headline figure is not the fixture's 30%% of priced spend: %q", headline)
	}
	if !strings.Contains(headline, "priced") {
		t.Errorf("the headline names the wrong population: %q\n"+
			"RebilledShare is a share of priced spend, and this corpus priced one session of two", headline)
	}
	if strings.Contains(headline, "of my agent spend") {
		t.Errorf("the headline still says %q, which reads as all of it", "of my agent spend")
	}
}

// The ceiling the PNG has and the text card did not: a measured 99.6% must not
// round to "100%", because on a card whose only words are about this figure,
// 100% says every dollar was re-billed.
func TestShareCardDoesNotRoundToAHundredPercent(t *testing.T) {
	s := costSummary{Tasks: 3, MedianUSD: 1, P90USD: 2, RebilledShare: 0.996}
	card := shareCard(s, 3)
	if strings.Contains(card, "100%") {
		t.Errorf("a measured 99.6%% rendered as 100%%:\n%s", card)
	}
	if !strings.Contains(card, ">99%") {
		t.Errorf("a measured 99.6%% did not render as >99%%:\n%s", card)
	}
}
