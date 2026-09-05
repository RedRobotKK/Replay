package main

import (
	"strings"
	"testing"
)

// The share card exists to be pasted in public. Its whole design constraint is
// what it must NOT contain: a spend total tells a reader the poster's burn rate
// and lets them infer team size, which is the one thing a startup would regret
// posting. A rate is comparable across everyone and reveals nothing.
func TestShareCardOmitsTheTotal(t *testing.T) {
	s := costSummary{
		Tasks: 1384, TotalUSD: 2906.39, MedianUSD: 0.65,
		P90USD: 2.29, AvoidableUSD: 151.54, AvoidableShare: 0.052,
	}
	card := shareCard(s, 67)

	for _, leaked := range []string{"2906", "2,906", "151.54"} {
		if strings.Contains(card, leaked) {
			t.Errorf("the card leaks an absolute spend figure (%q):\n%s", leaked, card)
		}
	}
	if !strings.Contains(card, "5%") {
		t.Errorf("the card must lead with the avoidable rate:\n%s", card)
	}
	for _, want := range []string{"0.65", "2.29", "1384"} {
		if !strings.Contains(card, want) {
			t.Errorf("the card should carry %q (comparable, not sensitive):\n%s", want, card)
		}
	}
	if !strings.Contains(card, "github.com/RedRobotKK/Replay") {
		t.Errorf("the card must carry the repo, or it cannot travel:\n%s", card)
	}
}

// A card built from an empty corpus must refuse rather than post zeros as if
// they were a measurement. The tool declines to state figures it cannot stand
// behind everywhere else.
func TestShareCardRefusesEmpty(t *testing.T) {
	if card := shareCard(costSummary{}, 0); card != "" {
		t.Errorf("an empty corpus should produce no card, got:\n%s", card)
	}
}

// Nothing in the card may identify a project, a path, or a machine.
func TestShareCardCarriesNoIdentifiers(t *testing.T) {
	s := costSummary{Tasks: 12, MedianUSD: 1.10, P90USD: 4.00, AvoidableShare: 0.11, TotalUSD: 90}
	card := shareCard(s, 3)
	for _, bad := range []string{"/Users/", "/home/", ".claude", "projects/"} {
		if strings.Contains(card, bad) {
			t.Errorf("the card carries an identifier (%q):\n%s", bad, card)
		}
	}
}
