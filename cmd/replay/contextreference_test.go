package main

import (
	"strings"
	"testing"
)

// systemPromptLine's own doc comment states the rule this file exists to
// enforce, and the function three lines below it broke the rule:
//
//	a zero share of a system prompt is not the same claim as a session that
//	carried none — absence, zero and unknown are three values (ADR-0018).
//
// referenceLine opened with `if whole <= 0 || part <= 0 { return "" }`. Both
// arms produced the same output — nothing — so the reader could not tell a
// measured zero from a corpus that was never measured, and neither from a
// metric this build does not carry. Three values, one rendering.
//
// internal/reference was built with the third value in it. Reference.Compare
// takes a measured figure; Reference.CompareMissing takes none and renders
// "not measured here; X across <population>". CompareMissing had ZERO
// production callers — the arm existed, was tested in its own package, and
// nothing ever asked for it. The package read as ADR-0018-compliant to anyone
// who grepped for the function and stopped there.
//
// The distinction is not academic here. The comment says why: a Claude Code
// transcript always produces a system row, but the ledger and the Codex reader
// are not bound by that. A Codex session that genuinely carried no system
// prompt is a real 0%, and it is a different thing from a session where
// nothing counted the tokens.

// TestCR7 is the measured zero. It must print a figure, because 0% is an
// answer — and it is the arm that used to vanish.
func TestCR7_AMeasuredZeroSharePrintsAZero(t *testing.T) {
	got := systemPromptLine(0, 4000)
	if got == "" {
		t.Fatal("a system prompt measured at 0 of 4000 tokens printed nothing. " +
			"That is a measurement, and the reader cannot tell it from a corpus " +
			"nobody measured")
	}
	if !strings.Contains(got, "0") {
		t.Errorf("the line does not carry the local figure: %q", got)
	}
	if strings.Contains(got, "not measured") {
		t.Errorf("a measured zero is being reported as unmeasured: %q", got)
	}
}

// TestCR8 is the unknown. Nothing to divide by means no local figure exists,
// and the published half is still worth printing — that is what
// CompareMissing is for.
func TestCR8_NoDenominatorIsReportedAsUnmeasured(t *testing.T) {
	got := systemPromptLine(0, 0)
	if got == "" {
		t.Fatal("a corpus with no tokens at all printed nothing. The published " +
			"population is still worth showing, and CompareMissing exists to " +
			"render exactly this case")
	}
	if !strings.Contains(got, "not measured") {
		t.Errorf("the unknown case must say it was not measured, not imply a "+
			"figure: %q", got)
	}
}

// TestCR9 is the ordinary arm, kept so a fix for the two above cannot be
// "always print not measured".
func TestCR9_AMeasuredShareStillPrintsTheComparison(t *testing.T) {
	got := systemPromptLine(1000, 4000)
	if got == "" {
		t.Fatal("an ordinary measured share printed nothing")
	}
	if strings.Contains(got, "not measured") {
		t.Errorf("a measured share reported as unmeasured: %q", got)
	}
}

// TestCR10 pins what all three share: the population and the citation. That
// is the safety property internal/reference exists for — a bare local figure
// is a number a reader cannot weigh.
func TestCR10_EveryArmCarriesThePopulationAndCitation(t *testing.T) {
	for _, c := range []struct {
		name        string
		part, whole int
	}{
		{"measured zero", 0, 4000},
		{"unmeasured", 0, 0},
		{"ordinary", 1000, 4000},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := systemPromptLine(c.part, c.whole)
			if got == "" {
				t.Fatal("printed nothing")
			}
			if !strings.Contains(got, "arXiv") {
				t.Errorf("no citation: %q", got)
			}
			if !strings.Contains(got, "across") {
				t.Errorf("no population: %q", got)
			}
		})
	}
}
