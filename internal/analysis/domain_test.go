package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fit describes prose, and something applies it to schemas.
//
// Fit refuses to sample a turn that re-laid the shared prefix, and says why at
// fit.go:190-194: "its write covers tool definitions, which are denser than
// prose and would drag the fit". internal/proxy/preflight.go:41-48 reaches the
// same conclusion independently — "System prompts and tool definitions are JSON
// schemas, which are denser than prose, so the prose default understates them".
//
// Then internal/advisor/advisor.go:303 calls fit.EstimateTokens on exactly
// tool-definition bytes to size every KindUnusedTools suggestion, and builds a
// Figure by hand with the prose fit's RelativeError attached:
//
//	tokens := fit.EstimateTokens(b.bytes) * len(lane.Requests)
//	ob.note(..., analysis.Figure{Value: tokens, Error: int(float64(tokens) * fit.RelativeError)}, true)
//
// The code states in two places that this ratio does not describe schemas, and
// then uses it on schemas — carrying an error bar that was measured on the
// prose it does describe.
//
// What this change does NOT claim: that the estimate is wrong by some amount.
// Nobody has measured tokens-per-byte on schema JSON for this provider, and
// doing so needs a tokenizer this tree does not have. The direction is stated
// by the code itself (denser), the magnitude is unknown, and an error bar
// borrowed from a different population is not evidence about this one.
//
// So the estimate stays — it is the only number available and it is better than
// nothing — and it stops claiming a measured uncertainty it does not have.
// That is the same rule fiterror_test.go applies to a spread that was never
// taken.

// DM1: an estimate outside the fitted domain carries no measured error.
func TestDM1_OutsideTheFitThereIsNoMeasuredError(t *testing.T) {
	f := TokenFit{TokensPerByte: 0.467, RelativeError: 0.12, Turns: 35}
	got := f.EstimateOutsideFit(40_000)
	if got.ErrorMeasured {
		t.Error("bytes the fit was not fitted on produce a figure claiming a measured " +
			"error bar; the spread was taken over prose turns and says nothing about " +
			"schema density")
	}
	if got.Error != 0 {
		t.Errorf("the figure carries an error of %d borrowed from the prose fit", got.Error)
	}
}

// DM2: the estimate itself is still produced, and still tracks the ratio.
//
// The guard against fixing this by refusing to estimate. The unused-tools
// suggestion needs a size to be ranked at all, and this ratio is the only
// number anyone has. Deleting the estimate would remove a useful finding to
// avoid an honest caveat.
func TestDM2_TheEstimateIsStillMade(t *testing.T) {
	f := TokenFit{TokensPerByte: 0.5, RelativeError: 0.12, Turns: 35}
	got := f.EstimateOutsideFit(40_000)
	if want := 20_000; got.Value != want {
		t.Errorf("EstimateOutsideFit(40000) = %d, want %d at 0.5 tokens/byte", got.Value, want)
	}
	// And it moves with the fit, so it is the session's own ratio and not a
	// constant wearing one.
	f.TokensPerByte = 0.25
	if got := f.EstimateOutsideFit(40_000); got.Value != 10_000 {
		t.Errorf("the estimate did not follow the ratio: %d", got.Value)
	}
}

// DM3: EstimateTokens and EstimateOutsideFit agree on the number.
//
// They must differ only in what they claim about uncertainty. If they ever
// disagreed on the value, the advisor and the report would size the same bytes
// differently and one of them would be wrong.
func TestDM3_OnlyTheClaimDiffersNotTheArithmetic(t *testing.T) {
	f := TokenFit{TokensPerByte: 0.467, RelativeError: 0.12, Turns: 35}
	for _, b := range []int{0, 1, 999, 40_000, 1 << 20} {
		if in, out := f.EstimateTokens(b), f.EstimateOutsideFit(b).Value; in != out {
			t.Errorf("%d bytes: EstimateTokens gives %d, EstimateOutsideFit gives %d", b, in, out)
		}
	}
}

// DM4: nothing hand-rolls a Figure with the fit's error on out-of-domain bytes.
//
// DM1-DM3 add a method. A method nobody calls is the built-but-unwired shape
// this repository keeps finding, and the advisor's line is the one that made
// this a defect rather than a doc comment:
//
//	analysis.Figure{Value: tokens, Error: int(float64(tokens) * fit.RelativeError)}
//
// It bypasses Figure() entirely and attaches the prose spread by hand. This
// reads the advisor's source and fails if that construction comes back, because
// the alternative — asserting on a number the screen prints — would pass just
// as well if somebody rebuilt the same Figure a different way.
func TestDM4_TheAdvisorDoesNotBorrowTheProseSpread(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "advisor", "advisor.go"))
	if err != nil {
		t.Fatalf("reading the caller: %v", err)
	}
	src := string(b)
	for _, banned := range []string{
		"fit.RelativeError",
		"f.RelativeError",
	} {
		if strings.Contains(src, banned) {
			t.Errorf("internal/advisor/advisor.go still reaches for %s. That spread was "+
				"measured over prose turns, and the bytes it is being applied to are the "+
				"tool definitions Fit excludes for being denser than prose.", banned)
		}
	}
	if !strings.Contains(src, "EstimateOutsideFit") {
		t.Error("the advisor does not call EstimateOutsideFit, so the method added here " +
			"is unreachable and the caller still sizes schemas with the prose fit")
	}
}
