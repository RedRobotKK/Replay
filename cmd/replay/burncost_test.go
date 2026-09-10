package main

import (
	"strings"
	"testing"
)

// `replay burn` counts four surfaces and priced none of them.
//
// Its own closing paragraph says why the token columns cannot be added:
// Anthropic partitions the cached share out of the prompt, Codex nests it
// inside, Ollama excludes it entirely. All true, and all about TOKENS. Dollars
// are a common unit, and pricing is the thing that would make the surfaces
// comparable at all — which is the whole point of a report headed "what your
// agents are consuming".
//
// The column now exists. What it must never do is print a number it cannot
// stand behind, and there are two distinct reasons a cell can be empty:
// nothing bills for the work, and nothing has told Replay the price.

// BC1: a surface with a price shows one.
func TestBC1_APricedSurfaceShowsDollars(t *testing.T) {
	s := surfaceBurn{name: "claude-code", requests: 100, costUSD: 12.34, pricedReqs: 100}
	if got := costCell(s); !strings.Contains(got, "12.34") {
		t.Errorf("a priced surface renders %q", got)
	}
}

// BC2: no bill and no price are different cells.
//
// Ollama runs locally and nobody invoices for it; Codex has a bill that Replay
// cannot read. Rendering both as "$0.00", or both as "-", tells the reader the
// same thing about two opposite situations — and the one that matters is the
// one with real money behind it.
func TestBC2_NoBillAndNoPriceReadDifferently(t *testing.T) {
	local := costCell(surfaceBurn{name: "ollama", requests: 50, localOnly: true})
	unpriced := costCell(surfaceBurn{name: "codex", requests: 6751, unpricedReqs: 6751})
	if local == unpriced {
		t.Fatalf("a surface nobody bills for and a surface with no price installed "+
			"both render %q", local)
	}
	if !strings.Contains(local, "no bill") {
		t.Errorf("the local surface does not say why it is free: %q", local)
	}
	if strings.Contains(unpriced, "0") {
		t.Errorf("an unpriced surface renders a number: %q", unpriced)
	}
}

// BC3: a surface read but not priced says so, and a surface not read at all
// says something else again.
func TestBC3_NotReadIsNotUnpriced(t *testing.T) {
	notRead := costCell(surfaceBurn{name: "codex"})
	unpriced := costCell(surfaceBurn{name: "codex", requests: 10, unpricedReqs: 10})
	if notRead == unpriced {
		t.Errorf("a surface with no transcripts and a surface with no price both "+
			"render %q", notRead)
	}
}

// BC4: a partly priced surface does not present its total as complete.
//
// Half a bill under a column header that says "cost" is the defect this whole
// phase is about, one column to the left.
func TestBC4_APartlyPricedSurfaceSaysSo(t *testing.T) {
	got := costCell(surfaceBurn{name: "claude-code", requests: 100, costUSD: 9.99,
		pricedReqs: 60, unpricedReqs: 40})
	if !strings.Contains(got, "9.99") {
		t.Errorf("the priced part is not shown: %q", got)
	}
	if !strings.Contains(got, "%") {
		t.Errorf("a partial total does not say how much of the surface it covers: %q", got)
	}
	if !strings.Contains(got, "60%") {
		t.Errorf("the coverage share is wrong; 60 of 100 requests priced should read 60%%: %q", got)
	}
	// And a fully priced surface carries no share at all, because a
	// qualification on every figure is one readers learn to skip.
	if full := costCell(surfaceBurn{name: "x", requests: 10, costUSD: 1, pricedReqs: 10}); strings.Contains(full, "%") {
		t.Errorf("a complete total is qualified anyway: %q", full)
	}
}

// BC5: a coverage share is floored, never rounded up to complete.
//
// 60,370 of 60,402 requests is 99.95%, which %.0f prints as "100%" beside a
// total that is missing 32 requests. It is the same defect the outlier note
// carries a comment about, one report over: a partial figure claiming full
// coverage is worse than no figure, because the reader stops looking for the
// part that is missing.
func TestBC5_CoverageIsFlooredNotRounded(t *testing.T) {
	got := costCell(surfaceBurn{name: "claude-code", requests: 60402, costUSD: 10498.50,
		pricedReqs: 60370, unpricedReqs: 32})
	if strings.Contains(got, "100%") {
		t.Errorf("32 unpriced requests are reported as full coverage: %q", got)
	}
	if !strings.Contains(got, "99%") {
		t.Errorf("the floored coverage is not shown: %q", got)
	}
}

// BC6: what the cost column is not covering, per shape of corpus.
//
// This used to be two conditionals and a pair of counters inside the loop that
// prints the table, so the sentence a reader gets about missing spend was
// decided by code no test entered — on a column added specifically to stop
// unpriced spend being invisible.
func TestBC6_PricingNotesSayWhatIsMissing(t *testing.T) {
	priced := surfaceBurn{name: "claude-code", requests: 100, pricedReqs: 100, costUSD: 9}
	unpriced := surfaceBurn{name: "codex", requests: 50, unpricedReqs: 50}
	local := surfaceBurn{name: "ollama", requests: 20, localOnly: true}
	unread := surfaceBurn{name: "codex"}

	// Nothing missing: no note at all, because a paragraph printed every run is
	// one the reader learns to skip.
	if got := pricingNotes([]surfaceBurn{priced, local, unread}); got != nil {
		t.Errorf("a fully priced machine was told something is missing: %v", got)
	}

	// One unpriced surface beside a priced one: both sentences.
	both := strings.Join(pricingNotes([]surfaceBurn{priced, unpriced, local}), "\n")
	if !strings.Contains(both, "1 surface(s) were read and could not be priced") {
		t.Errorf("the count of unpriced surfaces is wrong or missing:\n%s", both)
	}
	if !strings.Contains(both, "replay rules --update") {
		t.Errorf("the note does not name the command that fixes it:\n%s", both)
	}
	if !strings.Contains(both, "against nothing") {
		t.Errorf("with one priced and one unpriced surface, the ranking caveat is "+
			"missing:\n%s", both)
	}

	// Unpriced with nothing priced: there is no ranking to caveat.
	alone := strings.Join(pricingNotes([]surfaceBurn{unpriced, local}), "\n")
	if !strings.Contains(alone, "could not be priced") {
		t.Errorf("an entirely unpriced machine is told nothing:\n%s", alone)
	}
	if strings.Contains(alone, "against nothing") {
		t.Errorf("a caveat about comparing surfaces, with only one surface:\n%s", alone)
	}
}

// BC7: a surface nobody bills for is not a surface with a missing price.
//
// Counting Ollama as unpriced would send a reader after a rules document that
// would change nothing, which is the same confusion costCell has four states
// to avoid.
func TestBC7_LocalSurfacesAreNotCountedAsUnpriced(t *testing.T) {
	local := surfaceBurn{name: "ollama", requests: 20, localOnly: true}
	if got := pricingNotes([]surfaceBurn{local}); got != nil {
		t.Errorf("a machine running only a local model is told a price is missing: %v", got)
	}
}
