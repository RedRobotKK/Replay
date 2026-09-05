package analysis

import (
	"fmt"
	"math"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// Routing arithmetic, and the line between the part of it that survives a
// tokenizer change and the part that does not.
//
// Every function above the Dilation section is built from multipliers, prices
// and rates. Not one of them takes a token count, so a model that tokenises
// the same text differently cannot move any of their answers. That is the
// whole reason this file can say anything at all about a model the ledger has
// never seen.
//
// Everything below that line is an absolute figure, and an absolute figure
// crossing model families needs sigma, the ratio between two tokenizers on the
// same content. Sigma is measured here or it does not exist. It is deliberately
// not a field on a rate card: a scalar per model asserts that one model dilates
// everything by one factor, which the tokens-per-byte fit already disproves at
// +/-159% across eleven sessions of one person's work. And the constant is not
// harmless. Routing to a 3.3x cheaper model with a worse read multiple at a 99%
// cached share breaks even at sigma = 1.0627, so a plausible-looking 1.15 does
// not absorb variance, it casts the deciding vote.

// BreakEvenTrim is the share of the prompt a trimming policy must remove
// before it beats simply letting the cache do its work.
//
//	gamma > (w - alpha) / (f + w - alpha),  f = h*alpha + (1 - h)
//
// f is the expected cost of a token that is offered to the cache: it reads at
// alpha when the prefix holds, and at full price when it does not. Trimming
// buys back a token at f and pays w for a token it has to write again, so the
// threshold rises with the write penalty and falls as reads get cheaper.
func BreakEvenTrim(alpha, w, hitRate float64) float64 {
	f := hitRate*alpha + (1 - hitRate)
	return (w - alpha) / (f + w - alpha)
}

// CrossRatio is the per-turn cost of running a turn on the destination model
// over the cost of running it on the source, at cached share c and token
// dilation sigma, in steady state with no cache write.
//
// Below one, the switch pays. The bracket is the effective share of a prompt
// that is actually billed once the cache read multiple is applied, so a
// destination with a worse multiple has to win the difference back on price.
func CrossRatio(alphaFrom, alphaTo, priceRatio, cachedShare, sigma float64) float64 {
	from := (1 - cachedShare) + alphaFrom*cachedShare
	to := (1 - cachedShare) + alphaTo*cachedShare
	return priceRatio * sigma * (to / from)
}

// InversionShare is the cached share at which a cheaper destination model
// stops being cheaper, because its worse cache read multiple has eaten the
// price advantage. Reported only when it lands inside 0..1: two models with
// the same read multiple never invert, and a boundary outside the interval is
// a pair that either always pays or never does.
//
// Solving priceRatio * to == from at sigma = 1 gives
//
//	c* = (r - 1) / (alphaFrom + r - 1 - r*alphaTo)
//
// This is the number that makes the difference between a switch that saves
// money on ordinary turns and one that quietly costs more on exactly the long,
// heavily cached sessions the switch was meant to help.
func InversionShare(alphaFrom, alphaTo, priceRatio float64) (float64, bool) {
	den := alphaFrom + priceRatio - 1 - priceRatio*alphaTo
	if den == 0 {
		return 0, false
	}
	c := (priceRatio - 1) / den
	if math.IsNaN(c) || c <= 0 || c >= 1 {
		return 0, false
	}
	return c, true
}

// Topology is one model's cost structure with every token count removed.
// It is derived from the rate card alone, so it renders with no ledger.
type Topology struct {
	Model string
	// ReadMult is alpha: what a cached token costs against a fresh one.
	ReadMult float64
	// InputPerMTok is carried so a caller can form the price ratio the
	// inversion boundary needs. At equal price c* collapses to zero for
	// every pair, so a report without a real ratio shows no boundary ever.
	InputPerMTok float64
	// WriteShort and WriteLong are w at the two TTLs the client may ask for.
	// They are properties of the request, not of the model, and are carried
	// here so the two thresholds below can be quoted side by side.
	WriteShort float64
	WriteLong  float64
	// BreakEvenShort and BreakEvenLong are gamma at those two TTLs.
	BreakEvenShort float64
	BreakEvenLong  float64
	// HitRate is the h the thresholds were computed at, measured from the
	// corpus rather than assumed, because gamma moves with it.
	HitRate float64
	// Known is false when the rules do not price this model. An unpriced
	// model gets no topology rather than inheriting a neighbour's numbers.
	Known bool
}

// TopologyOf builds the dimensionless report for one model at a measured hit
// rate. It asks the rules for a price first: a model the rules cannot price is
// a model whose read multiple is also unknown, and a zero alpha would quietly
// value every cached token at nothing.
func TopologyOf(model string, hitRate float64) Topology {
	price, ok := cachemodel.PriceFor(model)
	if !ok || price.ReadMult <= 0 {
		return Topology{Model: model}
	}
	ws := cachemodel.WriteMultiplier(5 * time.Minute)
	wl := cachemodel.WriteMultiplier(time.Hour)
	return Topology{
		Model:          model,
		ReadMult:       price.ReadMult,
		InputPerMTok:   price.InputPerMTok,
		WriteShort:     ws,
		WriteLong:      wl,
		BreakEvenShort: BreakEvenTrim(price.ReadMult, ws, hitRate),
		BreakEvenLong:  BreakEvenTrim(price.ReadMult, wl, hitRate),
		HitRate:        hitRate,
		Known:          true,
	}
}

// MinDilationTurns is how many fitted turns each side needs before their
// tokens-per-byte ratio is called a measurement. Ten is the same floor the
// before/after comparison uses, and it is a floor on the input to a ratio
// whose error is already the two inputs added in quadrature.
const MinDilationTurns = 10

// Dilation is sigma: how the destination model's tokenizer scales the same
// content against the source model's, measured on this corpus.
//
// Sigma is zero unless Measured. There is no default and no fallback, because
// a fallback of 1.0 is a claim that two tokenizers agree, which is the exact
// assertion the measurement exists to test.
type Dilation struct {
	From, To string
	// Sigma is ToFit / FromFit, zero when unmeasured.
	Sigma float64
	// RelativeError is the two fits' relative errors in quadrature. A ratio
	// of noisy quantities is noisier than either of them, and sigma quoted
	// without it is the fabricated constant wearing a measurement's clothes.
	RelativeError float64
	// FromTurns and ToTurns are the sample sizes behind each side.
	FromTurns, ToTurns int
	Measured           bool
	// Why names what is missing when Measured is false, by model, so the
	// operator knows which side to go and collect.
	Why string
}

// MeasureDilation computes sigma from wire-measured fits, or refuses.
//
// The fits come from provider-reported token counts against content bytes the
// ledger already records, so both sides are observations rather than estimates.
// A side with no entry, no turns, or a non-positive ratio is not an
// observation, and dividing by it would produce a confident number from
// nothing.
func MeasureDilation(from, to string, fits map[string]TokenFit) Dilation {
	d := Dilation{From: from, To: to}
	a, aok := fits[from]
	b, bok := fits[to]
	d.FromTurns, d.ToTurns = a.Turns, b.Turns

	var missing []string
	for _, side := range []struct {
		model string
		fit   TokenFit
		ok    bool
	}{{from, a, aok}, {to, b, bok}} {
		switch {
		case !side.ok:
			missing = append(missing, fmt.Sprintf("%s: no turns on the wire", side.model))
		case side.fit.Turns < MinDilationTurns:
			missing = append(missing, fmt.Sprintf("%s: %d of %d turns", side.model, side.fit.Turns, MinDilationTurns))
		case side.fit.TokensPerByte <= 0:
			missing = append(missing, fmt.Sprintf("%s: no fitted tokens-per-byte", side.model))
		}
	}
	if len(missing) > 0 {
		d.Why = "sigma unmeasured for this pair (" + join(missing, "; ") + ")"
		return d
	}

	d.Sigma = b.TokensPerByte / a.TokensPerByte
	d.RelativeError = math.Hypot(a.RelativeError, b.RelativeError)
	d.Measured = true
	return d
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
