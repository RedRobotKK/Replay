package main

import (
	"bytes"
	"strings"
	"testing"
)

// Help ranks by value, not by alphabet or by the order commands were written.
//
// The binary carries twenty commands. A person who has just installed it opens
// --help and meets twenty doors with nothing to say which one has the money
// behind it: `redact` and `cost` were listed as equals, and `cost` sat
// thirteenth. The capability was never the problem here - the ordering was.

// UX-1: the four commands that are the product come before the rest.
//
// PASS: cost/bare, diff, advise and serve all appear before the first of the
// specialist commands.
// FAIL: a flat list in any other order, which is what shipped.
func TestUX1_ValueCommandsComeFirst(t *testing.T) {
	var b bytes.Buffer
	if err := printUsage(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()

	// The last of the front-door commands must still precede the first
	// specialist one, whatever order they are in among themselves.
	front := []string{"replay diff", "replay advise", "replay serve"}
	later := []string{"replay redact", "replay probe", "replay corpus", "replay learn"}
	lastFront := -1
	for _, c := range front {
		i := strings.Index(out, c)
		if i < 0 {
			t.Fatalf("front-door command %q is missing from help", c)
		}
		if i > lastFront {
			lastFront = i
		}
	}
	for _, c := range later {
		i := strings.Index(out, c)
		if i < 0 {
			t.Fatalf("command %q vanished from help", c)
		}
		if i < lastFront {
			t.Errorf("%q is listed above a front-door command; help is not ranked by value", c)
		}
	}
}

// UX-2: the list is grouped, and the groups say what they are for.
//
// PASS: headings a reader can skim.
// FAIL: one undifferentiated block, which is the defect however it is sorted.
func TestUX2_HelpIsGrouped(t *testing.T) {
	var b bytes.Buffer
	if err := printUsage(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, h := range []string{"Start here:", "Look closer:", "Corpus", "Setup"} {
		if !strings.Contains(out, h) {
			t.Errorf("help has no %q section: the list is still one flat block", h)
		}
	}
}

// UX-3: nothing was dropped in the reordering.
//
// Ranking help is a presentation change. A command that quietly left the list
// is undiscoverable, which is worse than being listed thirteenth.
func TestUX3_NoCommandWasLost(t *testing.T) {
	var b bytes.Buffer
	if err := printUsage(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, c := range []string{
		"replay replay", "replay blame", "replay diff", "replay corpus", "replay advise",
		"replay learn", "replay doctor", "replay probe", "replay rules", "replay statusline",
		"replay cost", "replay context", "replay route", "replay trim", "replay redact",
		"replay serve", "replay version",
	} {
		if !strings.Contains(out, c) {
			t.Errorf("%q is no longer listed in help", c)
		}
	}
}
