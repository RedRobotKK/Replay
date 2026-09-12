package main

import (
	"strings"
	"testing"
)

// Two asks, assigned per machine, tagged so the outcome can be told apart.
//
// A is the reciprocity framing already shipped. B adds the two things the
// giving literature supports most strongly and A leaves on the table: a named
// person rather than a project, and a concrete statement of what the money
// pays for. Both are true here - one maintainer, and a price table verified by
// real API calls - so neither is a manipulation, which matters because a tool
// whose entire value is honest measurement cannot ask for money dishonestly.

// TV-1: assignment is stable per machine.
//
// PASS: the same seed always lands in the same arm.
// FAIL: a coin flip per run, which shows one reader both messages and measures
// nothing.
func TestTV1_AssignmentIsStablePerMachine(t *testing.T) {
	for _, seed := range []string{"machine-a", "machine-b", "machine-c"} {
		first := tipVariant(seed)
		for i := 0; i < 20; i++ {
			if got := tipVariant(seed); got != first {
				t.Fatalf("seed %q moved between arms: %q then %q", seed, first, got)
			}
		}
	}
}

// TV-2: both arms are reachable.
//
// PASS: some seeds get A, some get B.
// FAIL: an assignment that always returns one arm - a test that cannot vary,
// which is this project's recurring defect.
func TestTV2_BothArmsAreReachable(t *testing.T) {
	seen := map[string]int{}
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		seen[tipVariant(s)]++
	}
	if len(seen) < 2 {
		t.Fatalf("only one arm is reachable across ten seeds: %v", seen)
	}
}

// TV-3: the arm is on the link, or the experiment has no readout.
//
// PASS: each arm's URL carries its own tag.
// FAIL: identical URLs, which is two messages and no measurement.
func TestTV3_TheArmIsOnTheLink(t *testing.T) {
	a := tipURL("A")
	b := tipURL("B")
	if a == b {
		t.Fatal("both arms link to the same URL, so nothing can be attributed")
	}
	for _, u := range []string{a, b} {
		if !strings.Contains(u, shareCoffee) {
			t.Errorf("the tagged URL lost the destination: %q", u)
		}
	}
}

// TV-4: the arms differ, and they differ on the variable they claim to.
//
// This test used to assert that B named the maintainer AND said what the money
// pays for, with A implicitly saying neither. The countdown then put "what the
// money pays for" into both arms, and this test stayed green - B still named
// Daniel, B still mentioned the price table, and the two strings were still
// unequal. Every assertion passed while the thing being asserted had stopped
// being true, which is the failure mode this repository keeps finding in other
// forms: a check that survives the change it exists to notice.
//
// So it now asserts the difference itself. The sole variable is the named
// person: strip the name out of B and the two arms must read the same.
//
// PASS: B names him, A does not, and nothing else separates them.
// FAIL: the arms stopped differing, or started differing on something the
// experiment is not attributing.
func TestTV4_TheArmsDifferOnlyOnTheNamedPerson(t *testing.T) {
	const clock = "COUNTDOWN"
	a := tipBody("A", 149.44, 2, "coffees", "LINK", clock)
	b := tipBody("B", 149.44, 2, "coffees", "LINK", clock)
	if !strings.Contains(b, "Daniel") {
		t.Error("variant B must name the maintainer, not the project")
	}
	if a == b {
		t.Error("the two arms are identical, so the experiment measures nothing")
	}
	// Both arms say what the money pays for. That is no longer B's variable and
	// no longer withheld from A, because it is true in both framings.
	for arm, body := range map[string]string{"A": a, "B": b} {
		if !strings.Contains(body, clock) {
			t.Errorf("arm %s dropped the countdown, so half the machines are not told\n"+
				"what the money buys: %q", arm, body)
		}
	}
	// Normalise B's two name clauses onto A's and the remainder must be
	// identical. Anything left over is a second variable nobody is attributing.
	//
	// The closing sentence used to differ too - "would go a long way" against
	// "would make a real difference" - and normalising that away here would
	// have been a test quietly accommodating the thing it forbids. The wording
	// was unified in tipvariant.go instead.
	norm := strings.Replace(b, "maintains it: Daniel.", "maintains it.", 1)
	norm = strings.Replace(norm, "instead of Daniel.", "instead of the person who wrote this.", 1)
	if norm != a {
		t.Errorf("the arms differ by more than the named person:\n  A: %q\n  B': %q", a, norm)
	}
}

// The coffee comparison only appears when there is a contrast to draw.
//
// It exists to make the ask look small next to the waste. At the $5 floor the
// waste IS the ask, so "you wasted one coffee, spare one coffee" reads as a
// reproach over a rounding error, which is the opposite of the intent.
//
// PASS: silent at the floor, present when the waste is at least three times
// what is being asked for, and the count is the waste rather than the ask.
// FAIL: it fires on small findings, or quotes the wrong number.
func TestTipVariant_TheCoffeeComparisonNeedsAContrast(t *testing.T) {
	if got := wastedCoffees(6.00, 1); got != 0 {
		t.Errorf("a $6 finding asking 1 coffee reported %d wasted coffees; at the floor the "+
			"waste and the ask are the same and saying so is a reproach, not a joke", got)
	}
	if got := wastedCoffees(14.99, 1); got != 0 {
		t.Errorf("just under 3x the ask reported %d; the gate is three times, so that the "+
			"contrast is real", got)
	}
	if got := wastedCoffees(15.00, 1); got != 3 {
		t.Errorf("at exactly 3x the ask the comparison should fire with 3, got %d", got)
	}
	if got := wastedCoffees(4000.00, 5); got != 800 {
		t.Errorf("$4000 is 800 coffees at $5 each, got %d. The comparison must quote the "+
			"WASTE; quoting the ask would collapse the contrast it exists for", got)
	}
}

// The arms must not both name Daniel.
//
// B's experimental variable is a named person rather than a project. Putting
// the name in a lead shared by both arms would leave the experiment measuring
// nothing, which is the failure this whole repository keeps finding in other
// forms: a comparison where both sides are the same.
//
// PASS: B names him, A does not.
// FAIL: the arms stopped differing.
func TestTipVariant_OnlyBNamesTheMaintainer(t *testing.T) {
	a := tipLineArm("A", 4000.00, false)
	b := tipLineArm("B", 4000.00, false)
	if strings.Contains(a, "Daniel") {
		t.Errorf("arm A named the maintainer, so the arms no longer differ on it:\n%s", a)
	}
	if !strings.Contains(b, "Daniel") {
		t.Errorf("arm B must name the maintainer; that is its variable:\n%s", b)
	}
	if !strings.Contains(a, "instead of the person who wrote this") {
		t.Errorf("arm A must still say who the coffees went to instead:\n%s", a)
	}
}
