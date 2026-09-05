package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A gate with nothing behind it is not a gate. The report suppresses dollars
// until sigma is measured, so the measured path has to actually produce a
// figure, or the suppression is theatre and the feature is a comment.
func TestMeasuredSigmaProducesADollarFigure(t *testing.T) {
	c := modelCorpus{
		fits: map[string]analysis.TokenFit{
			"claude-opus-5":    {TokensPerByte: 0.60, RelativeError: 0.2, Turns: 240},
			"claude-fable-5-1": {TokensPerByte: 0.66, RelativeError: 0.2, Turns: 90},
		},
		turns: map[string]int{"claude-opus-5": 240, "claude-fable-5-1": 90},
		usage: map[string]transcript.Usage{
			"claude-opus-5": {Input: 200_000, CacheRead: 4_000_000, CacheCreation: 300_000, Output: 120_000},
		},
		hits: 970, total: 1000,
	}
	r := buildRoute("claude-opus-5", "claude-fable-5-1", c)
	if !r.Dilation.Measured {
		t.Fatalf("both sides have enough turns: %s", r.Dilation.Why)
	}
	if r.Dollars == nil {
		t.Fatal("sigma is measured and no dollar figure was produced; the gate guards nothing")
	}
	if r.Observed == nil {
		t.Fatal("a projected figure is meaningless without the observed one it is compared against")
	}
	if *r.Observed <= 0 {
		t.Fatalf("the source model's observed cost must be positive, got %.6f", *r.Observed)
	}
	// The projection must move with sigma. If it does not, sigma is decorative.
	c2 := c
	c2.fits = map[string]analysis.TokenFit{
		"claude-opus-5":    {TokensPerByte: 0.60, RelativeError: 0.2, Turns: 240},
		"claude-fable-5-1": {TokensPerByte: 0.90, RelativeError: 0.2, Turns: 90},
	}
	r2 := buildRoute("claude-opus-5", "claude-fable-5-1", c2)
	if r2.Dollars == nil {
		t.Fatal("second projection missing")
	}
	if *r2.Dollars <= *r.Dollars {
		t.Fatalf("a larger sigma must project a larger destination cost: %.6f then %.6f", *r.Dollars, *r2.Dollars)
	}
}

// And the suppressed path must stay suppressed: no fallback of 1.0, no
// estimate, nothing with a currency symbol in front of it.
func TestUnmeasuredSigmaSuppressesTheDollarFigure(t *testing.T) {
	c := modelCorpus{
		fits:  map[string]analysis.TokenFit{"claude-opus-5": {TokensPerByte: 0.60, RelativeError: 0.2, Turns: 240}},
		turns: map[string]int{"claude-opus-5": 240},
		usage: map[string]transcript.Usage{"claude-opus-5": {Input: 200_000, CacheRead: 4_000_000}},
		hits:  970, total: 1000,
	}
	r := buildRoute("claude-opus-5", "claude-fable-5-1", c)
	if r.Dollars != nil {
		t.Fatalf("a dollar figure appeared without a measured sigma: %.6f", *r.Dollars)
	}
	var sb strings.Builder
	if err := r.write(&sb); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sb.String(), "$") {
		t.Fatalf("the suppressed report printed a currency figure:\n%s", sb.String())
	}
}
