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

// TV-4: variant B names the maintainer and what the money funds.
//
// PASS: both present.
// FAIL: B identical in substance to A, which makes the experiment a no-op that
// still costs a month of asks to run.
func TestTV4_VariantBNamesThePersonAndTheCost(t *testing.T) {
	b := tipBody("B", 149.44, 2, "coffees", "LINK")
	if !strings.Contains(b, "Daniel") {
		t.Error("variant B must name the maintainer, not the project")
	}
	if !strings.Contains(strings.ToLower(b), "price table") && !strings.Contains(strings.ToLower(b), "api") {
		t.Error("variant B must say concretely what the money pays for")
	}
	a := tipBody("A", 149.44, 2, "coffees", "LINK")
	if a == b {
		t.Error("the two arms are identical, so the experiment measures nothing")
	}
}
