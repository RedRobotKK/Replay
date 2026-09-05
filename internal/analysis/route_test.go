package analysis

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

func near(t *testing.T, got, want, tol float64, what string) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s: got %.6f, want %.6f (±%.6f)", what, got, want, tol)
	}
}

// The red team's central objection, encoded as a test: a different tokenizer
// scales every token count on one side. The structural verdict must not move,
// and the dollar figure must move by exactly that factor. If those two ever
// behave the same way, one of them is lying.
func TestTokenizerDilationMovesTheDollarsAndNotTheStructure(t *testing.T) {
	base := transcript.Usage{Input: 2_000, CacheRead: 198_000, Output: 900}
	for _, k := range []float64{0.7, 1.0, 1.15, 1.45} {
		scaled := scaleUsage(base, k)

		gotShare := cachemodel.CachedShare(scaled.CacheRead, scaled.Input+scaled.CacheRead)
		wantShare := cachemodel.CachedShare(base.CacheRead, base.Input+base.CacheRead)
		near(t, gotShare, wantShare, 1e-9, "cached share under dilation k="+ftoa(k))

		p := cachemodel.Price{InputPerMTok: 3, OutputPerMTok: 15, ReadMult: 0.1}
		got := cachemodel.CostUSD(scaled, p)
		want := cachemodel.CostUSD(base, p) * k
		near(t, got, want, 1e-9, "cost under dilation k="+ftoa(k))
	}
}

// The break-even trimming threshold is built from multipliers and rates only.
// No token count reaches it, so no tokenizer can perturb it.
func TestBreakEvenTrimIsBuiltOnlyFromRatesAndMultipliers(t *testing.T) {
	// h = 0.9732 is this corpus's measured hit rate: 731 breaks / 27,302 turns.
	// f = h*alpha + (1-h); gamma > (w-alpha)/(f+w-alpha).
	for _, tc := range []struct {
		name        string
		alpha, w, h float64
		want        float64
	}{
		{"5m TTL, alpha 0.10", 0.10, 1.25, 0.9732, (1.25 - 0.10) / (0.9732*0.10 + (1 - 0.9732) + 1.25 - 0.10)},
		{"1h TTL, alpha 0.10", 0.10, 2.00, 0.9732, (2.00 - 0.10) / (0.9732*0.10 + (1 - 0.9732) + 2.00 - 0.10)},
		{"1h TTL, alpha 0.025", 0.025, 2.00, 0.9732, (2.00 - 0.025) / (0.9732*0.025 + (1 - 0.9732) + 2.00 - 0.025)},
	} {
		got := BreakEvenTrim(tc.alpha, tc.w, tc.h)
		near(t, got, tc.want, 1e-9, tc.name)
		if got <= 0 || got >= 1 {
			t.Fatalf("%s: gamma %.4f is not a share", tc.name, got)
		}
	}
}

// The cache-read inversion: routing to a cheaper model with a worse read
// multiplier stops paying above some cached share. Closed form, checked
// against the worked example that started this argument.
func TestInversionBoundaryMatchesTheClosedForm(t *testing.T) {
	// from alpha 0.025 -> to alpha 0.100, destination 3.33x cheaper on input.
	share, ok := InversionShare(0.025, 0.100, 0.3)
	if !ok {
		t.Fatal("a boundary exists for these rates and was not reported")
	}
	near(t, share, 0.992908, 1e-5, "inversion share")

	// Below the boundary routing down wins; above it, it loses. Same algebra,
	// evaluated directly, so a sign error in the closed form cannot pass.
	for _, c := range []float64{0.90, 0.98, 0.99} {
		if r := CrossRatio(0.025, 0.100, 0.3, c, 1.0); r >= 1 {
			t.Fatalf("at cached share %.2f routing down should win, ratio %.4f", c, r)
		}
	}
	for _, c := range []float64{0.995, 0.999} {
		if r := CrossRatio(0.025, 0.100, 0.3, c, 1.0); r <= 1 {
			t.Fatalf("at cached share %.3f routing down should lose, ratio %.4f", c, r)
		}
	}
}

// When both models have the same read multiplier there is no inversion at all,
// and the function must say so rather than return a share outside 0..1.
func TestNoInversionWhenTheReadMultiplesMatch(t *testing.T) {
	if share, ok := InversionShare(0.1, 0.1, 0.3); ok {
		t.Fatalf("a cheaper model with identical cache economics never inverts; got %.4f", share)
	}
}

// G6. Without wire-measured token counts for both sides there is no sigma, and
// without sigma there is no dollar figure. Not a default of 1.0, not a
// rate-card constant: nothing.
func TestNoDollarFigureWithoutAMeasuredSigma(t *testing.T) {
	fits := map[string]TokenFit{
		"claude-opus-5": {TokensPerByte: 0.61, RelativeError: 0.21, Turns: 240},
	}
	d := MeasureDilation("claude-opus-5", "claude-fable-5-1", fits)
	if d.Measured {
		t.Fatal("sigma was reported as measured with only one side on the wire")
	}
	if d.Sigma != 0 {
		t.Fatalf("an unmeasured sigma must be zero, not a fabricated %.3f", d.Sigma)
	}
	if !strings.Contains(d.Why, "claude-fable-5-1") {
		t.Fatalf("the refusal must name the model it lacks samples for: %q", d.Why)
	}
}

func TestSigmaIsTheRatioOfTwoWireMeasuredFits(t *testing.T) {
	fits := map[string]TokenFit{
		"claude-opus-5":    {TokensPerByte: 0.60, RelativeError: 0.20, Turns: 240},
		"claude-fable-5-1": {TokensPerByte: 0.69, RelativeError: 0.15, Turns: 88},
	}
	d := MeasureDilation("claude-opus-5", "claude-fable-5-1", fits)
	if !d.Measured {
		t.Fatalf("both sides are on the wire with enough turns: %s", d.Why)
	}
	near(t, d.Sigma, 0.69/0.60, 1e-9, "sigma")
	// The uncertainty travels with it. A ratio of two noisy fits is noisier
	// than either, and a sigma quoted without that is the fabricated constant
	// wearing a measurement's clothes.
	near(t, d.RelativeError, math.Hypot(0.20, 0.15), 1e-9, "sigma relative error")
}

func TestSigmaNeedsEnoughTurnsOnBothSides(t *testing.T) {
	fits := map[string]TokenFit{
		"claude-opus-5":    {TokensPerByte: 0.60, RelativeError: 0.2, Turns: MinDilationTurns},
		"claude-fable-5-1": {TokensPerByte: 0.69, RelativeError: 0.2, Turns: MinDilationTurns - 1},
	}
	d := MeasureDilation("claude-opus-5", "claude-fable-5-1", fits)
	if d.Measured {
		t.Fatal("a fit from fewer turns than the gate must not be called measured")
	}
	if !strings.Contains(d.Why, "claude-fable-5-1") || !strings.Contains(d.Why, "turns") {
		t.Fatalf("the refusal must name the model and the shortfall: %q", d.Why)
	}
}

// A fit that fell back to the coarse prose default is not a measurement, and
// dividing two of them would produce a very confident 1.0.
func TestSigmaRefusesAFitThatFellBackToTheDefault(t *testing.T) {
	fits := map[string]TokenFit{
		"claude-opus-5":    {TokensPerByte: 0.60, RelativeError: 0.2, Turns: 240},
		"claude-fable-5-1": {TokensPerByte: 0.69, RelativeError: 0, Turns: 0},
	}
	if d := MeasureDilation("claude-opus-5", "claude-fable-5-1", fits); d.Measured {
		t.Fatal("a zero-turn fit is the default constant, not an observation")
	}
}

// The structural half owes nothing to the ledger and must render with none.
func TestTopologyRendersWithNoLedgerAtAll(t *testing.T) {
	top := TopologyOf("claude-opus-5", 0.9732)
	if !top.Known {
		t.Skip("the compiled rules do not price this model; nothing to assert")
	}
	if top.BreakEvenLong <= top.BreakEvenShort {
		t.Fatalf("the 1h write penalty is 2.0x against 1.25x, so its threshold must be higher: %.4f vs %.4f", top.BreakEvenLong, top.BreakEvenShort)
	}
	near(t, top.WriteShort, cachemodel.WriteMultiplier(5*time.Minute), 1e-9, "5m write multiplier")
	near(t, top.WriteLong, cachemodel.WriteMultiplier(time.Hour), 1e-9, "1h write multiplier")
}

// An unpriced model must not acquire a topology by inheriting someone else's
// numbers. This is the same rule the cost model already holds.
func TestUnknownModelGetsNoFabricatedTopology(t *testing.T) {
	if top := TopologyOf("some-model-nobody-has-priced", 0.97); top.Known {
		t.Fatalf("an unpriced model reported a topology: %+v", top)
	}
}

// scaleUsage stands in for a different tokenizer: the same content, counted
// differently on every field the provider reports.
func scaleUsage(u transcript.Usage, k float64) transcript.Usage {
	r := func(n int) int { return int(math.Round(float64(n) * k)) }
	u.Input, u.CacheCreation, u.CacheRead, u.Output = r(u.Input), r(u.CacheCreation), r(u.CacheRead), r(u.Output)
	return u
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 2, 64) }

// The inversion boundary is meaningless without a price ratio: at equal price
// c* collapses to zero for every pair, so a report that hardcodes 1 silently
// never shows a boundary. Topology must carry the input price so the ratio is
// available to compute one.
func TestTopologyCarriesTheInputPriceSoARatioCanBeFormed(t *testing.T) {
	top := TopologyOf("claude-opus-5", 0.9732)
	if !top.Known {
		t.Skip("the compiled rules do not price this model")
	}
	if top.InputPerMTok <= 0 {
		t.Fatalf("a priced model must expose its input price, got %.4f", top.InputPerMTok)
	}
	if share, ok := InversionShare(top.ReadMult, top.ReadMult, 1); ok {
		t.Fatalf("equal rates and equal price cannot invert, got %.4f", share)
	}
}

// A boundary is useless without a direction, and the direction is not
// obvious: a destination that is dearer per token but caches better loses on
// short turns and wins on long ones, which is the opposite of the reading
// "the price advantage is spent". Both orientations are checked so a report
// cannot describe a real crossover backwards.
func TestCrossRatioNamesWhichSideOfTheBoundaryWins(t *testing.T) {
	// Dearer input, cheaper reads: the compiled rate card's own shape.
	alphaFrom, alphaTo, r := 0.100, 0.025, 2.0
	share, ok := InversionShare(alphaFrom, alphaTo, r)
	if !ok {
		t.Fatal("these rates cross and no boundary was reported")
	}
	near(t, share, 0.952381, 1e-5, "inversion share")
	if got := CrossRatio(alphaFrom, alphaTo, r, share-0.05, 1); got <= 1 {
		t.Fatalf("below the boundary the dearer model must lose, ratio %.4f", got)
	}
	if got := CrossRatio(alphaFrom, alphaTo, r, share+0.03, 1); got >= 1 {
		t.Fatalf("above the boundary the better cache read must win, ratio %.4f", got)
	}
	near(t, CrossRatio(alphaFrom, alphaTo, r, share, 1), 1.0, 1e-9, "ratio at the boundary")

	// Cheaper input, dearer reads: the orientation that started this argument.
	// Same function, opposite verdicts on each side.
	share2, ok := InversionShare(0.025, 0.100, 0.3)
	if !ok {
		t.Fatal("no boundary for the cheaper-but-worse-reads pair")
	}
	if got := CrossRatio(0.025, 0.100, 0.3, share2-0.05, 1); got >= 1 {
		t.Fatalf("below the boundary the cheaper model must win, ratio %.4f", got)
	}
	if got := CrossRatio(0.025, 0.100, 0.3, share2+0.005, 1); got <= 1 {
		t.Fatalf("above the boundary it must lose, ratio %.4f", got)
	}
}
