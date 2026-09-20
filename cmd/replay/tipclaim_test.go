package main

import (
	"strings"
	"testing"
)

// The ask may not tell a reader what they paid.
//
// RebilledUSD is a price-table valuation of measured re-billed tokens. The
// tokens are arithmetic over provider-reported usage; the dollars are that
// token count multiplied by the input rate in the price table in force, or by
// the operator's own negotiated rate when their rules document states one
// (cost.go:718, cachemodel.PriceForAt). No provider ever told Replay what this
// reader was charged, and on a subscription seat the reader is not charged per
// token at all.
//
// The same render already says so. `replay cost` prints "at list prices dated
// ..." in its header, and eight lines above the ask it prints "you are not
// billed per token, so the dollars above are list price for someone who is".
// The ask then said "Replay just found $7.57 you had already paid for once",
// which contradicts both. Measured on the shipped binary against a four-copy
// fixture corpus: both sentences appeared in one output.
//
// So this is a wording defect, not a missing disclosure and not an accounting
// one. Nothing here changes RebilledUSD, the token arithmetic, the price
// tables or the cost schema, and nothing introduces a billing concept. What
// the ask may say is what was established: tokens were billed twice, and at
// list price they come to this much.
//
// TestTipLineIsNotABill is the neighbouring guard and does not cover this. It
// bans "owe", "due", "invoice", "please pay" and "must", which stop the line
// reading as a DEMAND. A false statement about what the reader already paid is
// a different failure and passed that guard untouched.

// tipClaimForms is every rendering a reader can get: both arms, and with and
// without the coffee comparison, which fires only above three times the ask.
func tipClaimForms() map[string]string {
	return map[string]string{
		"A, no comparison":   tipBody("A", 7.57, 1, "coffee", "LINK"),
		"B, no comparison":   tipBody("B", 7.57, 1, "coffee", "LINK"),
		"A, with comparison": tipBody("A", 4000.00, 5, "coffees", "LINK"),
		"B, with comparison": tipBody("B", 4000.00, 5, "coffees", "LINK"),
	}
}

// TC1: no form of the ask claims the reader personally paid the figure.
//
// Every phrase below asserts a charge against this reader. Replay observed a
// token count and applied a price table; it observed no payment.
func TestTC1_TheAskDoesNotClaimTheReaderPaid(t *testing.T) {
	for name, body := range tipClaimForms() {
		low := strings.ToLower(body)
		for _, banned := range []string{
			"you had already paid",
			"you already paid",
			"you paid",
			"you were billed",
			"you bought",
			"you spent",
			"cost you",
			"out of your pocket",
		} {
			if strings.Contains(low, banned) {
				t.Errorf("%s: the ask claims %q, which is a charge to this reader that "+
					"Replay never observed:\n%s", name, banned, body)
			}
		}
	}
}

// TC2: the figure is named as a list-price valuation.
//
// Banning the false claim is half of it. A bare dollar amount with no basis
// invites the reader to supply the missing one, and the one they supply is
// their own bill.
func TestTC2_TheAskNamesTheFigureAsListPrice(t *testing.T) {
	for name, body := range tipClaimForms() {
		if !strings.Contains(strings.ToLower(body), "list price") {
			t.Errorf("%s: the ask quotes a dollar figure without saying it is a list-price "+
				"valuation:\n%s", name, body)
		}
	}
}

// TC3: the ask still says what was actually established.
//
// The correction must not empty the sentence out. Tokens really were billed
// twice, that is the finding the ask is built on, and an ask that no longer
// names its finding is a generic appeal.
func TestTC3_TheAskStillNamesItsFinding(t *testing.T) {
	for name, body := range tipClaimForms() {
		if !strings.Contains(body, "$") {
			t.Errorf("%s: the ask no longer names an amount:\n%s", name, body)
		}
		if !strings.Contains(strings.ToLower(body), "twice") {
			t.Errorf("%s: the ask no longer says the tokens were billed twice, which is "+
				"the finding it exists to report:\n%s", name, body)
		}
	}
}

// TC4: the comparison line is held to the same standard as the lead.
//
// It said "N coffees you bought your provider", which is the same payment
// assertion one line down. Correcting only the lead would leave the claim in
// the output and move it.
func TestTC4_TheComparisonMakesNoPaymentClaim(t *testing.T) {
	for _, arm := range []string{"A", "B"} {
		body := tipBody(arm, 4000.00, 5, "coffees", "LINK")
		var line string
		for _, l := range strings.Split(body, "\n") {
			if strings.Contains(l, "coffees") && !strings.Contains(l, "LINK") {
				line = l
			}
		}
		if line == "" {
			t.Fatalf("arm %s: no comparison line found to check:\n%s", arm, body)
		}
		for _, banned := range []string{"you bought", "you paid", "you spent"} {
			if strings.Contains(strings.ToLower(line), banned) {
				t.Errorf("arm %s: the comparison claims %q: %q", arm, banned, line)
			}
		}
	}
}

// TC5: the arms still differ, and only on their own variable.
//
// The reword must not flatten the experiment. B's variable is a named person;
// A's is the same sentence without one. This is the property
// TestTipVariant_OnlyBNamesTheMaintainer pins, restated here so a change to
// the claim wording cannot quietly take the experiment with it.
func TestTC5_TheRewordKeptTheArmsApart(t *testing.T) {
	a := tipBody("A", 4000.00, 5, "coffees", "LINK")
	b := tipBody("B", 4000.00, 5, "coffees", "LINK")
	if a == b {
		t.Fatal("the arms are identical, so the experiment measures nothing")
	}
	if strings.Contains(a, "Daniel") {
		t.Errorf("arm A named the maintainer:\n%s", a)
	}
	if !strings.Contains(b, "Daniel") {
		t.Errorf("arm B must name the maintainer; that is its variable:\n%s", b)
	}
	if !strings.Contains(a, "the person who wrote this") {
		t.Errorf("arm A must still name who got the money instead:\n%s", a)
	}
}
