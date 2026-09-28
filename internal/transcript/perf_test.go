package transcript

import "testing"

// Performance evidence has to survive the read, and say where it came from.
//
// Ollama's server log already carries three timings, and ollama.go already
// parses them into OllamaRequest. Nothing read them: `git grep PromptMS` at
// 2974d192 returns the parser and nothing else. Evidence that is extracted and
// then dropped is the same defect UNWIRED-LOG.md records nine times, and a
// figure Replay could have reported about a surface it already supports.
//
// These tests pin the contract: a timing is present with its provenance, or it
// is absent and says so. There is no third state where a number appears with no
// account of how it was obtained.

// A source that reports its own durations yields OBSERVED timings.
func TestOllamaTimingsAreObserved(t *testing.T) {
	r := OllamaRequest{PromptMS: 120.5, EvalMS: 880.25, TotalMS: 1000.75}
	p := r.Perf()

	if p.PromptProcessMS.Provenance != Observed {
		t.Errorf("prompt processing provenance = %q, want %q: Ollama prints this "+
			"figure itself, so it is measured rather than inferred", p.PromptProcessMS.Provenance, Observed)
	}
	if p.PromptProcessMS.Value != 120.5 {
		t.Errorf("prompt processing = %v, want 120.5", p.PromptProcessMS.Value)
	}
	if p.GenerateMS.Value != 880.25 || p.GenerateMS.Provenance != Observed {
		t.Errorf("generation = %v/%q, want 880.25/%q", p.GenerateMS.Value, p.GenerateMS.Provenance, Observed)
	}
	if p.TotalMS.Value != 1000.75 || p.TotalMS.Provenance != Observed {
		t.Errorf("total = %v/%q, want 1000.75/%q", p.TotalMS.Value, p.TotalMS.Provenance, Observed)
	}
}

// The boundary Ollama does NOT establish stays unmeasured.
//
// A server log that reports prompt-eval and eval time says nothing about when
// the first token reached the client: queueing, transport and streaming all sit
// outside those numbers. Deriving TTFT from them would be inventing the very
// boundary the source declines to draw.
func TestOllamaDoesNotClaimTimeToFirstToken(t *testing.T) {
	r := OllamaRequest{PromptMS: 120, EvalMS: 880, TotalMS: 1000}
	p := r.Perf()

	if p.TTFTMS.Provenance != NotMeasured {
		t.Errorf("TTFT provenance = %q, want %q: nothing in an Ollama server log "+
			"establishes when the first token was delivered", p.TTFTMS.Provenance, NotMeasured)
	}
	if p.TTFTMS.Value != 0 {
		t.Errorf("TTFT carries the value %v while claiming to be unmeasured; a "+
			"number and its absence must not occupy the same field", p.TTFTMS.Value)
	}
}

// Absent evidence is absent, not zero.
//
// A log line that never appeared must not read as "took 0 ms". This is the same
// distinction meanSeen() keeps in the advisor and Correlation keeps in Request.
func TestMissingTimingsAreNotMeasuredNotZero(t *testing.T) {
	p := OllamaRequest{}.Perf()

	for name, m := range map[string]Measure{
		"prompt processing": p.PromptProcessMS,
		"generation":        p.GenerateMS,
		"total":             p.TotalMS,
		"TTFT":              p.TTFTMS,
	} {
		if m.Provenance != NotMeasured {
			t.Errorf("%s = %q on an empty record, want %q: a timing nobody "+
				"reported is not a timing of zero", name, m.Provenance, NotMeasured)
		}
	}
}

// Throughput is DERIVED, and only when both of its inputs were observed.
func TestThroughputIsDerivedAndOnlyFromObservedInputs(t *testing.T) {
	r := OllamaRequest{Generated: 200, EvalMS: 1000}
	got := r.Perf().TokensPerSec
	if got.Provenance != Derived {
		t.Errorf("throughput provenance = %q, want %q: it is a quotient of two "+
			"measurements, not a reading", got.Provenance, Derived)
	}
	if got.Value != 200 {
		t.Errorf("throughput = %v tok/s, want 200 (200 tokens / 1.000 s)", got.Value)
	}

	if m := (OllamaRequest{Generated: 200}).Perf().TokensPerSec; m.Provenance != NotMeasured {
		t.Errorf("throughput with no eval time = %q, want %q: dividing by a "+
			"duration nobody measured produces a number about nothing", m.Provenance, NotMeasured)
	}
}

// Local inference gets no dollar figure.
func TestLocalInferenceCarriesNoCost(t *testing.T) {
	p := OllamaRequest{PromptMS: 1, EvalMS: 1, TotalMS: 2, Generated: 5}.Perf()
	if p.CostBasis != CostNotMeasured {
		t.Errorf("cost basis = %q, want %q: electricity and hardware are not "+
			"modelled, so a local run has no priced cost to report", p.CostBasis, CostNotMeasured)
	}
}
