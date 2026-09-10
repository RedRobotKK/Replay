package analysis

import "testing"

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

// DM4 is deleted, not weakened.
//
// It read internal/advisor/advisor.go for the strings "fit.RelativeError" and
// "EstimateOutsideFit" and treated what it found as proof. An audit defeated
// both halves at once: the banned names are SUBSTRINGS, so any other receiver
// spelling walks past, and the required call is satisfied by the COMMENT that
// names it even when the call itself is gone. #121's defect was restored under
// a different variable name with the whole tree green.
//
// No grep survives that. The property is about what the figure CLAIMS, not how
// the line is written, and the next spelling always escapes.
//
// It could not be asserted behaviourally at the time, which is the finding
// under the finding: note() took an analysis.Figure and kept only its Value, so
// ErrorMeasured died at the observation boundary and there was nothing left to
// observe. evidence carries it now, and
// internal/advisor's MN6 asserts it through Observe — the same mutation that
// walked past DM4 fails MN6 by name.
//
// A grep replaced by a behavioural test in another package is worth a note
// here, because the obvious repair is to strengthen the grep and that repair
// does not work.
