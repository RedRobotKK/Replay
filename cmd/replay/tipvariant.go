package main

import (
	"crypto/sha256"
	"fmt"
)

// Two framings of the ask, assigned per machine.
//
// A is the reciprocity framing already shipped: the tool gave something first,
// and asks for a small share of it back. B adds the two things the giving
// literature supports most strongly and A leaves on the table - a named person
// rather than a project, and a concrete statement of what the money pays for.
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
	if arm == "B" {
		return fmt.Sprintf(
			"\nReplay found $%.2f you had already paid for once. It is free, and it is\n"+
				"maintained by one person: Daniel. The price table behind that number is\n"+
				"checked against live API calls, which cost real money to make. If it was\n"+
				"worth %d %s of what it found, that is what keeps it current: %s\n",
			avoidableUSD, coffees, unit, link)
	}
	return fmt.Sprintf(
		"\nReplay found $%.2f you had already paid for once. It is free, and the\n"+
			"measurements behind it are not. If it was worth %d %s of that back, that\n"+
			"is what keeps it maintained: %s\n",
		avoidableUSD, coffees, unit, link)
}
