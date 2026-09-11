package analysis

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// An error bar that was never measured must not print as one.
//
// Fit sets RelativeError = 1 on two paths that are not measurements:
//
//	fit.go:197-199  sumBytes == 0      no fittable turn at all, so TokensPerByte
//	                                   is DefaultTokensPerByte, an English prose
//	                                   average borrowed from nowhere in particular
//	weightedSpread  len(samples) < 2   one fitted turn, so there is no spread to
//	                                   take; the 1 is a stand-in
//
// A third path reaches the same value honestly: a genuine byte-weighted spread
// that happens to work out at 1.0. All three print `±100%`, and the report
// renders them identically.
//
// Measured across 1734 lanes by the benefit-gap audit: 315 lanes report exactly
// ±100%, of which at least 178 have fewer than two fitted turns. On one session
// that produces a headline row reading `1.50M in prompts (±1.50M)` — an
// uncertainty equal to the value, printed as the number one token source, with
// nothing on the line to say the bar is a placeholder.
//
// This is the defect this repository names most often: a value standing in for
// a different value it isn't. The information to tell them apart already exists
// on the struct — Turns — and nothing consulted it.
//
// What this does NOT fix, and must not be read as fixing: whether
// TokensPerByte is accurate. That needs a Claude tokenizer, which is not
// public, or the count_tokens endpoint, which needs a key and a network. The
// audit's 49% median relative error stands unaddressed. This change is about
// not dressing an unmeasured bar as a measured one.

// FE1: a fit with no fittable turns reports its error as unmeasured.
func TestFE1_NoFittedTurnsIsNotAMeasuredError(t *testing.T) {
	f := TokenFit{TokensPerByte: DefaultTokensPerByte, RelativeError: 1, Turns: 0}
	if f.ErrorMeasured() {
		t.Error("a fit with zero fitted turns claims a measured error bar; its " +
			"RelativeError is the placeholder Fit assigns when sumBytes == 0")
	}
}

// FE2: one fitted turn has no spread, so no measured error.
//
// weightedSpread returns 1 for len(samples) < 2 — with a single sample there is
// nothing to take a standard deviation of.
func TestFE2_OneFittedTurnIsNotAMeasuredError(t *testing.T) {
	f := TokenFit{TokensPerByte: 0.4, RelativeError: 1, Turns: 1}
	if f.ErrorMeasured() {
		t.Error("a fit from a single turn claims a measured spread; a standard deviation " +
			"of one sample is not a measurement")
	}
}

// FE3: two or more fitted turns is a real spread, including when it is 1.0.
//
// The guard against overcorrecting into "any ±100% is fake". A genuine
// byte-weighted spread can land on 1.0, and calling that unmeasured would throw
// away a real measurement to tidy up a presentational problem.
func TestFE3_ARealSpreadIsMeasuredEvenAtOne(t *testing.T) {
	for _, f := range []TokenFit{
		{TokensPerByte: 0.4, RelativeError: 1, Turns: 2},
		{TokensPerByte: 0.4, RelativeError: 0.12, Turns: 9},
	} {
		if !f.ErrorMeasured() {
			t.Errorf("a fit from %d turns is reported as unmeasured: %+v", f.Turns, f)
		}
	}
}

// FE4: Figure carries the provenance to whatever renders it.
//
// The renderers are where `(±1.50M)` is printed, and they had no way to ask.
// Putting it on Figure means a caller cannot print the bar without having been
// handed the fact that it is a placeholder.
func TestFE4_FigureCarriesWhetherTheErrorWasMeasured(t *testing.T) {
	unfitted := TokenFit{TokensPerByte: DefaultTokensPerByte, RelativeError: 1, Turns: 0}
	if got := unfitted.Figure(Estimated(1_500_000)); got.ErrorMeasured {
		t.Errorf("a figure from an unfitted session says its error was measured: %+v", got)
	}
	fitted := TokenFit{TokensPerByte: 0.4, RelativeError: 0.2, Turns: 6}
	if got := fitted.Figure(Estimated(1_000_000)); !got.ErrorMeasured {
		t.Errorf("a figure from a fitted session says its error was not measured: %+v", got)
	}
}

// FE5: a measured figure has nothing to qualify.
//
// Where the provider reported the tokens, there is no estimate and no error
// bar, so the question does not arise and must not produce a spurious caveat.
func TestFE5_AMeasuredFigureIsNotQualified(t *testing.T) {
	unfitted := TokenFit{TokensPerByte: DefaultTokensPerByte, RelativeError: 1, Turns: 0}
	got := unfitted.Figure(Tokens{Measured: 4000})
	if got.Error != 0 {
		t.Errorf("a wholly measured figure carries an error bar of %d", got.Error)
	}
	if !got.ErrorMeasured {
		t.Error("a figure with no estimated part is qualified as unmeasured; there is no " +
			"estimate for the caveat to be about")
	}
}

// FE6: the report does not print an unmeasured bar as a number.
//
// FE1-FE5 put the provenance on the struct. On its own that is a field nothing
// reads — the built-but-unwired shape this repository has spent a week
// clearing, and it would have been introduced by the commit that fixes an
// instance of it.
//
// The report is where `1.50M in prompts (±1.50M)` is printed. A reader looking
// at that line has no way to know the bar is a placeholder, and the fix is only
// real when that line changes.
func TestFE6_TheReportQualifiesAnUnmeasuredBar(t *testing.T) {
	unfitted := TokenFit{TokensPerByte: DefaultTokensPerByte, RelativeError: 1, Turns: 0}
	got := errorBar(unfitted.Figure(Estimated(1_500_000)))
	if got == "1.50M" || got == "1500000" {
		t.Fatalf("an unmeasured error bar renders as the bare figure %q, which is what "+
			"produced `1.50M in prompts (±1.50M)` on a session with no fittable turn", got)
	}
	if !strings.Contains(strings.ToLower(got), "not measured") {
		t.Errorf("the unmeasured bar renders as %q and does not say it was not measured", got)
	}

	fitted := TokenFit{TokensPerByte: 0.4, RelativeError: 0.2, Turns: 6}
	if got := errorBar(fitted.Figure(Estimated(1_000_000))); !strings.Contains(got, "200") &&
		!strings.Contains(got, "0.20M") {
		t.Errorf("a measured bar of 200,000 renders as %q; a real measurement must still "+
			"print as a number", got)
	}
}

// FE7: Fit never returns a non-positive ratio.
//
// A postcondition nothing stated, and a refusal in cmd/replay depends on it.
// budget.go declines when fit.TokensPerByte <= 0, and that branch is
// unreachable: Fit assigns DefaultTokensPerByte when sumBytes is zero, and
// otherwise divides sumTokens by sumBytes where both are positive by
// construction — a sample is only recorded when newTokens > 0 and userBytes
// clears minFitBytes.
//
// Mutation confirmed the consequence: disabling that refusal leaves the whole
// suite green, because nothing can reach it.
//
// The guard is left in place as defence in depth and this test is what makes
// that honest. If Fit's contract ever changes, the refusal becomes live code
// and somebody finds out here rather than in a budget priced from a zero ratio.
func TestFE7_FitAlwaysReturnsAPositiveRatio(t *testing.T) {
	if DefaultTokensPerByte <= 0 {
		t.Fatalf("the default ratio is %v; the empty-corpus branch cannot be positive",
			DefaultTokensPerByte)
	}
	// The empty case, which is the branch a session with no fittable turn takes.
	empty := Fit(&Calibration{Lane: &transcript.Lane{}}, false)
	if empty.TokensPerByte <= 0 {
		t.Errorf("a fit over no samples returned %v tokens/byte", empty.TokensPerByte)
	}
}
