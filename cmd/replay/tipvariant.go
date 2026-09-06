package main

import (
	"crypto/sha256"
	"fmt"
)

// Two framings of the ask, assigned per machine.
//
// A is the reciprocity framing: the tool gave something first, and asks for a
// small share of it back. B adds the two things the giving literature supports
// most strongly and A leaves on the table - a named person rather than a
// project, and a concrete statement of what the money pays for.
//
// Both ask directly, reworded 2026-09-06. They used to say "if it was worth N
// coffees of what it found, that is what keeps it current", which is asking
// sideways: it puts a condition in front of the request and leaves the reader
// to work out that a request was made. "Did that help? N coffees back would go
// a long way" is the same ask with the hedge removed.
//
// What neither says is "right now". The need behind this is real and it is
// also dated, and a binary installed today still prints its text next
// November. A tool whose whole value is honest measurement cannot ship a
// sentence that expires.
//
// Both are true, and that is not a footnote. One person maintains this and the
// price table really is verified against live API calls that cost money. A tool
// whose entire value is honest measurement cannot ask for money with a framing
// it would not defend, so the arms differ in emphasis and never in truth.
// Nothing here manufactures urgency, invents a deadline, or implies the project
// dies without you.

// tipVariant assigns a machine to an arm, stably.
//
// Stable per seed because a reader who sees a different message every run is
// not in an experiment, they are being shuffled, and the result measures
// nothing.
func tipVariant(seed string) string {
	sum := sha256.Sum256([]byte("replay-tip-arm:" + seed))
	if sum[0]%2 == 0 {
		return "A"
	}
	return "B"
}

// tipURL tags the destination with the arm, so a conversion can be attributed.
//
// The tag survives into the Buy Me a Coffee page, checked live on 2026-09-06.
// Whether their dashboard REPORTS it is a separate question and is not
// established: until it is, this experiment has an assignment and no readout,
// and a result read off it would be a guess with an arm label attached.
func tipURL(arm string) string {
	return shareCoffee + "?via=replay-" + arm
}

// tipBody renders one arm's text.
func tipBody(arm string, avoidableUSD float64, coffees int, unit, link string) string {
	// Who the coffees went to instead is the arm's own variable, so the lead
	// names Daniel in B and not in A. Naming him in a shared lead would have
	// put the named person in both arms and left the experiment measuring
	// nothing.
	rival := "the person who wrote this"
	if arm == "B" {
		rival = "Daniel"
	}
	lead := fmt.Sprintf("Replay just found $%.2f you had already paid for once.", avoidableUSD)
	if c := wastedCoffees(avoidableUSD, coffees); c > 0 {
		// The same number in a unit a person can picture, and cheeky on
		// purpose. It is not a rhetorical trick: at $5 a coffee this is
		// arithmetic, and the contrast is what makes the ask below look as
		// small as it actually is.
		lead = fmt.Sprintf("Replay just found $%.2f you had already paid for once.\n"+
			"That is %d coffees you bought your provider instead of %s.", avoidableUSD, c, rival)
	}
	if arm == "B" {
		return fmt.Sprintf(
			"\n%s\nDid that help?\n"+
				"It is free and one person maintains it: Daniel. The price table behind\n"+
				"that number is checked against live API calls that cost real money.\n"+
				"%d %s back would go a long way: %s\n",
			lead, coffees, unit, link)
	}
	return fmt.Sprintf(
		"\n%s\nDid that help?\n"+
			"It is free and one person maintains it. %d %s back would make a real\n"+
			"difference: %s\n",
		lead, coffees, unit, link)
}

// wastedCoffees is what the re-billed amount would have bought, in the unit
// the tip page sells, or 0 when saying so would not be worth the words.
//
// Gated on the waste being at least three times the ask. The line exists for
// the contrast, and at the $5 floor there is none: "you wasted one coffee,
// spare one coffee" is a worse sentence than not mentioning it, and it makes a
// small honest finding sound like a reproach.
func wastedCoffees(avoidableUSD float64, asking int) int {
	c := int(avoidableUSD / float64(tipUnitUSD))
	if c < asking*3 {
		return 0
	}
	return c
}
