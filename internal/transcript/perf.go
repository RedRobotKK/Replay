package transcript

// Performance measurements, carried with an account of how each was obtained.
//
// Cost already works this way. A price is versioned, a derived figure says it
// is derived, and an absent one is NOT_MEASURED rather than zero. Timings
// arrived later and had no such discipline, which is how Ollama's three
// durations came to be parsed by ollama.go and read by nothing: at
// 2974d192, `git grep PromptMS` returns the parser and no consumer.
//
// The rule this file encodes is the one the rest of the codebase already
// follows: a number appears with its provenance, or it does not appear. There
// is no third state where a figure is shown and its basis is unrecorded.

// Provenance says how a measurement was obtained.
type Provenance string

// Provenance values, weakest last.
const (
	// Observed: the source reported this figure itself.
	Observed Provenance = "observed"
	// Derived: computed from observed values by a stated rule.
	Derived Provenance = "derived"
	// Estimated: modelled, not measured. Reserved; nothing emits it yet.
	Estimated Provenance = "estimated"
	// NotMeasured: the source establishes no such boundary. The zero value,
	// so a Measure nobody filled in reads as unmeasured rather than as zero.
	NotMeasured Provenance = ""
)

// CostBasis says what a cost figure rests on.
type CostBasis string

// Cost bases.
const (
	// CostProviderReported: the provider billed a figure and stated it.
	CostProviderReported CostBasis = "provider-reported"
	// CostDerived: observed usage priced by a versioned table.
	CostDerived CostBasis = "derived"
	// CostNotMeasured: no priced cost. Local inference lands here, and must:
	// electricity, amortised hardware and opportunity cost are not modelled,
	// so any dollar figure for a local run would be invented. The zero value.
	CostNotMeasured CostBasis = ""
)

// Measure is one number and the account of where it came from.
//
// Value is meaningless unless Provenance says otherwise, which is why the zero
// value is (0, NotMeasured) rather than a bare float that reads as zero.
type Measure struct {
	Value      float64    `json:"value"`
	Provenance Provenance `json:"provenance"`
}

// observed builds a measurement the source reported.
func observed(v float64) Measure { return Measure{Value: v, Provenance: Observed} }

// derived builds a computed measurement.
func derived(v float64) Measure { return Measure{Value: v, Provenance: Derived} }

// Perf is the canonical performance record for one request.
//
// Every field is a Measure, so a surface that establishes three of these and
// not the other three says so in the record rather than in a comment. Fields
// are added only when some real source establishes the boundary they name;
// TTFTMS is present because the proxy could establish it, and stays
// NotMeasured for sources that cannot.
type Perf struct {
	// TTFTMS is time to first token: request sent to first byte of output.
	// Only a client that watched the stream can establish this.
	TTFTMS Measure `json:"ttft_ms"`
	// PromptProcessMS is time spent evaluating the prompt.
	PromptProcessMS Measure `json:"prompt_process_ms"`
	// GenerateMS is time spent producing output tokens.
	GenerateMS Measure `json:"generate_ms"`
	// TotalMS is the source's own end-to-end figure. Not the sum of the two
	// above: a source that reports all three may account for time in neither.
	TotalMS Measure `json:"total_ms"`
	// TokensPerSec is output throughput, derived from generated tokens and
	// generation time when both were observed.
	TokensPerSec Measure `json:"tokens_per_sec"`
	// CostBasis says what, if anything, a cost figure for this request rests
	// on. Separate from the money package: this records whether a priced
	// figure is even admissible, not what it is.
	CostBasis CostBasis `json:"cost_basis"`
}

// Perf reports what an Ollama server log establishes, and only that.
//
// Ollama prints prompt eval time, eval time and total time, so those three are
// observed. It says nothing about when the first token reached the client:
// queueing, transport and streaming all sit outside those numbers, so TTFT
// stays unmeasured rather than being reconstructed from figures that do not
// bound it. Cost is NotMeasured because local inference has no modelled price.
func (r OllamaRequest) Perf() Perf {
	p := Perf{CostBasis: CostNotMeasured}
	if r.PromptMS > 0 {
		p.PromptProcessMS = observed(r.PromptMS)
	}
	if r.EvalMS > 0 {
		p.GenerateMS = observed(r.EvalMS)
	}
	if r.TotalMS > 0 {
		p.TotalMS = observed(r.TotalMS)
	}
	// Throughput needs both inputs measured. Generated tokens over a
	// generation time nobody reported is a quotient about nothing.
	if r.Generated > 0 && r.EvalMS > 0 {
		p.TokensPerSec = derived(float64(r.Generated) / (r.EvalMS / 1000))
	}
	return p
}
