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
		{"small but real finding", 6.00, true, "the suggestion floor applies here"},
		{"real finding", 152.40, true, "this is the moment the ask makes sense"},
		{"large finding", 4000, true, ""},
	}
	for _, c := range cases {
		got := tipLine(c.avoidable)
		if (got != "") != c.want {
			t.Errorf("%s: line=%q, want shown=%v. %s", c.name, got, c.want, c.why)
		}
		if !c.want {
			continue
		}
		// It must name the figure it is asking about, or it is a generic
		// appeal and those are noise.
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
		m := regexp.MustCompile(`worth about \$(\d+(?:\.\d+)?)`).FindStringSubmatch(got)
		if m == nil {
			t.Fatalf("%s: cannot find the suggested amount in %q", c.name, got)
		}
		sug, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatalf("%s: unparsable suggestion %q", c.name, m[1])
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
	line := tipLine(152.40)
	for _, banned := range []string{"owe", "due", "invoice", "please pay", "must"} {
		if strings.Contains(strings.ToLower(line), banned) {
			t.Errorf("the tip line reads as a demand (%q): %q", banned, line)
		}
	}
}
