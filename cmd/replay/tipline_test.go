package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The tip line appears at the one moment it is relevant: just after the tool
// has told you what you already paid twice. It is not a nag, so it has rules.
func TestTipLine(t *testing.T) {
	cases := []struct {
		name      string
		avoidable float64
		want      bool
		why       string
	}{
		{"nothing found", 0, false, "asking for money after finding nothing is a nag"},
		{"trivial finding", 0.40, false, "a 40-cent finding does not earn an ask"},
		// Just over the floor: two percent of $6 is 12 cents, so the minimum
		// has to lift it. Without a case here the min-clamp was unexercised
		// and deleting it left the suite green.
		{"small but real finding", 6.00, true, "rounds up to one coffee"},
		{"real finding", 152.40, true, "this is the moment the ask makes sense"},
		{"large finding", 4000, true, ""},
	}
	for _, c := range cases {
		got := tipLineFor(c.avoidable, false)
		if (got != "") != c.want {
			t.Errorf("%s: line=%q, want shown=%v. %s", c.name, got, c.want, c.why)
		}
		if !c.want {
			continue
		}
		// It must name the figure it found, or it is a generic appeal and those
		// are noise.
		if !strings.Contains(got, "$") {
			t.Errorf("%s: the line does not name an amount: %q", c.name, got)
		}
		// It must not invent a pre-filled amount. Buy Me a Coffee echoes
		// ?amount= into its canonical URL and leaves the input's value
		// attribute empty, so a link claiming to pre-fill would be a lie the
		// user discovers on arrival. Verified against the live page 2026-09-05.
		if strings.Contains(got, "amount=") {
			t.Errorf("%s: the link carries an amount parameter, which BMC does not "+
				"honour. The field arrives empty: %q", c.name, got)
		}
		// The suggestion is capped. Someone who finds $4,000 of waste is not
		// being asked for a percentage of it without limit.
		//
		// Parsed rather than substring-matched: an earlier version of this
		// check looked for "4000" anywhere in the line and matched the FOUND
		// amount, failing a correctly capped suggestion. Assert the number you
		// mean, not a number that happens to be nearby.
		// The line asks in coffees, because that is the unit the page sells.
		// The phrase moved from "worth N coffees of what it found" to "N
		// coffees back" on 2026-09-06; the number is still what is asserted.
		m := regexp.MustCompile(`(\d+) coffees? back`).FindStringSubmatch(got)
		if m == nil {
			t.Fatalf("%s: cannot find the suggestion in %q", c.name, got)
		}
		units, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("%s: unparsable count %q", c.name, m[1])
		}
		sug := float64(units * tipUnitUSD)
		// Singular and plural must agree with the number, or the line reads
		// like a template that nobody finished.
		if (units == 1) != strings.Contains(got, "1 coffee back") {
			t.Errorf("%s: %d coffees is written with the wrong plural: %q", c.name, units, got)
		}
		// Buy Me a Coffee sells whole coffees at $5 and cannot pre-fill a custom
		// amount, so a suggestion of $3 names a figure the page will not offer.
		// Verified against the live page 2026-09-05: coffee_price 5.0000 USD.
		if int(sug)%tipUnitUSD != 0 {
			t.Errorf("%s: suggested $%.2f is not a whole multiple of the $%d coffee, "+
				"so the page cannot offer it", c.name, sug, tipUnitUSD)
		}
		if sug > tipMaxUSD {
			t.Errorf("%s: suggested $%.2f exceeds the $%.2f cap", c.name, sug, tipMaxUSD)
		}
		if sug < tipMinUSD {
			t.Errorf("%s: suggested $%.2f is below the $%.2f floor", c.name, sug, tipMinUSD)
		}
		if sug > c.avoidable {
			t.Errorf("%s: suggested $%.2f is more than the $%.2f it found", c.name, sug, c.avoidable)
		}
	}
}

// Nothing in this line may look like an invoice, a total, or an obligation.
func TestTipLineIsNotABill(t *testing.T) {
	line := tipLineFor(152.40, false)
	for _, banned := range []string{"owe", "due", "invoice", "please pay", "must"} {
		if strings.Contains(strings.ToLower(line), banned) {
			t.Errorf("the tip line reads as a demand (%q): %q", banned, line)
		}
	}
}

// A terminal hyperlink is an escape sequence, and an escape sequence in a file
// or a pipe is corruption. `replay cost > report.txt` and `| grep` have to keep
// working, so the link is only emitted when stdout is genuinely a terminal.
func TestTipLinkOnlyHyperlinksATerminal(t *testing.T) {
	const esc = "\x1b]8;;"

	plain := tipLineFor(147.73, false)
	if strings.Contains(plain, "\x1b") {
		t.Errorf("escape bytes reached a non-terminal writer: %q", plain)
	}
	if !strings.Contains(plain, shareCoffee) {
		t.Errorf("the plain form lost the URL: %q", plain)
	}

	linked := tipLineFor(147.73, true)
	if !strings.Contains(linked, esc) {
		t.Errorf("a terminal got no hyperlink: %q", linked)
	}
	// The visible text must still be the URL itself. A terminal that does not
	// understand OSC 8 shows the label, and a reader who wants to copy the
	// address should find an address rather than the words "click here".
	if !strings.Contains(linked, esc+"https://"+tipURL("A")) {
		t.Errorf("the hyperlink target is not the coffee URL: %q", linked)
	}
	// The label carries the experiment tag too, deliberately. Showing a clean
	// label over a tagged target would be a link whose text and destination
	// disagree, and a reader who copies the visible text would silently leave
	// the experiment. Transparent beats tidy.
	if !strings.Contains(linked, tipURL("A")+"\x1b]8;;\x1b\\") {
		t.Errorf("the visible label is not the URL, so it cannot be copied: %q", linked)
	}
}

// Stripped of escapes, the two forms must say exactly the same thing. A
// terminal reader and a log reader should not be given different asks.
func TestTipLinkSaysTheSameThingBothWays(t *testing.T) {
	strip := regexp.MustCompile(`\x1b]8;;[^\x1b]*\x1b\\`)
	linked := strip.ReplaceAllString(tipLineFor(147.73, true), "")
	if linked != tipLineFor(147.73, false) {
		t.Errorf("the hyperlinked form differs once escapes are removed:\n  %q\n  %q",
			linked, tipLineFor(147.73, false))
	}
}
