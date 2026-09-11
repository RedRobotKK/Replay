package main

import (
	"strings"
	"testing"
)

// The MCP overhead figure must say which way it is wrong.
//
// 0.25 tokens per byte is asserted, not derived: it is the English-prose
// four-bytes-per-token rule of thumb, and nothing in this tree measured it.
// What HAS been measured is the fitted ratio on real sessions, and every
// published value sits above it — ADR-0007 records 0.445 to 0.795 across eleven
// sessions, and docs/evidence/calibration-corpus-2026-09-10.md ranges 0.439 to
// 1.261. Two other places in the tree already say the prose ratio understates
// JSON schemas (internal/analysis/fit.go and internal/proxy/preflight.go).
//
// `replay mcp` applies it to nothing but JSON schemas. It said ESTIMATED, which
// is true and is not the point: an estimate a reader cannot sign is a different
// object from an estimate whose direction is known and stated. Naming it a
// floor is what lets a reader act on it — a tool that is not worth its cost at
// the floor is not worth it at all.
//
// What this does not fix: the magnitude. Nobody has run a tokenizer over schema
// JSON for this provider, so "at least" is the whole claim.
func TestMCPOverheadNamesTheFigureAsAFloor(t *testing.T) {
	got, err := mcpOverheadText("claude-opus-5", 40000, 500)
	if err != nil {
		t.Fatalf("mcpOverheadText: %v", err)
	}

	// Every line carrying a number has to carry the direction with it. A reader
	// quotes one line, not the paragraph, so a caveat that lives only in the
	// prose underneath is a caveat that does not travel with the figure.
	for _, want := range []string{
		"at least 10,000 tokens", // the token line
		"$25.00 across 500, at least",
		"$2.50 across 500, at least",
		"denser", // why it is a floor
		"understates them",
		"not measured", // and that the size of the gap is unknown
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not say %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "about 10,000 tokens") {
		t.Errorf("the token line still says \"about\", which reads as two-sided:\n%s", got)
	}
}

// There is deliberately no test that the three fallback ratios agree.
//
// internal/proxy/preflight.go said of its own 0.25 that it "is deliberately the
// same constant the analysis uses rather than a second number that could
// drift". It was not: analysis kept an unexported one and the proxy and the MCP
// surface each retyped the digits, so all three could drift and nothing would
// notice. They are now one declaration, aliased — which makes disagreement a
// compile error and makes any test of it a check that cannot fail. ADR-0014
// says not to write that test, so it is not written, and this note is here so
// nobody adds it back thinking it was forgotten.
