package reference

import (
	"strings"
	"testing"
)

// A published population is the only reference this tool can offer while its
// own pooled corpus has one member.
//
// internal/analysis/outlier.go states the constraint: "a bare figure is not
// actionable... neither they nor this tool can answer from a population — the
// pooled corpus has one member, and publishing a population figure derived from
// one machine is the shape of claim this project has already retracted twice."
//
// The way out is not to publish our own. It is to carry somebody else's, with
// their citation and their population attached, and to be extremely careful
// about what a difference from it licenses anybody to say.

// RF1: every reference names its population and its source.
//
// A figure without one is a number with no referent, which is the shape of
// claim this repository has retracted twice. validate refuses it for the same
// reason rules.go refuses a document with no version: a report has to be able
// to name what produced it.
func TestRF1_EveryReferenceCarriesItsProvenance(t *testing.T) {
	for _, r := range Compiled() {
		if strings.TrimSpace(r.Population) == "" {
			t.Errorf("%s carries no population", r.Metric)
		}
		if strings.TrimSpace(r.Citation) == "" {
			t.Errorf("%s carries no citation", r.Metric)
		}
		if strings.TrimSpace(r.MeasuredAt) == "" {
			t.Errorf("%s does not say when its data was collected", r.Metric)
		}
		if err := r.validate(); err != nil {
			t.Errorf("%s: %v", r.Metric, err)
		}
	}
	if len(Compiled()) == 0 {
		t.Fatal("no compiled references, so every test here is vacuous")
	}
}

// RF2: a reference that tries to declare a verdict is refused.
//
// Taken from internal/cachemodel/claim.go, which solved the harder version of
// this for provider figures: "a hand-written 'consistent' is another claim
// wearing a verdict's clothes, and the whole point is to have one field in this
// system that nobody can simply assert."
func TestRF2_ADeclaredVerdictIsRefused(t *testing.T) {
	r := Compiled()[0]
	r.DeclaredVerdict = "typical"
	err := r.validate()
	if err == nil {
		t.Fatal("a reference declaring its own verdict was accepted")
	}
	if !strings.Contains(err.Error(), "verdict") {
		t.Errorf("the error does not name the field: %v", err)
	}
}

// RF3: a share outside 0..1 is a typo, not a measurement.
func TestRF3_AShareOutsideItsDomainIsRefused(t *testing.T) {
	for _, bad := range []float64{-0.1, 1.5} {
		r := Reference{Metric: "m", Value: bad, Unit: UnitShare,
			Population: "p", Citation: "c", MeasuredAt: "2026-06"}
		if err := r.validate(); err == nil {
			t.Errorf("a share of %v was accepted", bad)
		}
	}
}

// RF4: the comparison is derived, and its vocabulary describes a difference
// rather than judging one.
//
// A provider's published minimum is a claim about the world and one
// counterexample refutes it. A population's published median is a claim about a
// population, and one machine differing from it refutes nothing at all — it is
// one draw. So there is no "high", no "above average", and no "worse".
func TestRF4_TheVerdictDescribesRatherThanJudges(t *testing.T) {
	r := Reference{Metric: "m", Value: 0.14, Unit: UnitShare,
		Population: "13.5M sessions", Citation: "arXiv:2608.00101", MeasuredAt: "2026-06"}

	if got := r.Compare(0.288).Verdict; got != Differs {
		t.Errorf("28.8%% against a published 14%% with no spread is %q, want %q", got, Differs)
	}
	// With a published spread, inside and outside are distinguishable.
	lo, hi := 0.10, 0.20
	r.P10, r.P90 = &lo, &hi
	if got := r.Compare(0.15).Verdict; got != Within {
		t.Errorf("a figure inside the published spread is %q", got)
	}
	if got := r.Compare(0.288).Verdict; got != Outside {
		t.Errorf("a figure outside the published spread is %q", got)
	}
	// And no verdict in the vocabulary passes judgement.
	for _, v := range []Verdict{Within, Outside, Differs, Unmeasured, NoReference} {
		for _, banned := range []string{"high", "low", "worse", "better", "average", "typical"} {
			if strings.Contains(string(v), banned) {
				t.Errorf("verdict %q contains the judgement %q", v, banned)
			}
		}
	}
}

// RF5: an unmeasured local figure is not a comparison.
//
// Absence, zero and unknown are three values (ADR-0018). A machine that did not
// compute the metric must not be reported as sitting at zero against the
// population.
func TestRF5_AnUnmeasuredLocalFigureIsNotZero(t *testing.T) {
	r := Compiled()[0]
	c := r.CompareMissing()
	if c.Verdict != Unmeasured {
		t.Errorf("a missing local figure compared as %q", c.Verdict)
	}
	// Precisely: no LOCAL figure is rendered. The first assertion here was
	// Contains(line, "0%"), which matches any percentage ending in zero — the
	// published 14.0% among them — and would have failed on a correct line.
	if !strings.HasPrefix(c.Line(), "not measured here") {
		t.Errorf("an unmeasured metric does not say so first: %q", c.Line())
	}
	if strings.Contains(c.Line(), " here,") {
		t.Errorf("an unmeasured metric rendered a local figure: %q", c.Line())
	}
}

// RF6: the rendered line carries the population every time it is printed.
//
// This is the whole safety property. "28.8% here, 14% across 13.5M Copilot
// sessions" is a sentence a reader can weigh. "28.8%, roughly double the
// average" is not, and is what this package exists to make unwriteable.
func TestRF6_EveryLineCarriesThePopulation(t *testing.T) {
	r := Reference{Metric: "systemPromptShare", Value: 0.14, Unit: UnitShare,
		Population: "13.5M sessions, 760.5M LLM calls", Citation: "arXiv:2608.00101",
		MeasuredAt: "2026-06"}
	// BOTH lines, not the measured one. The first version of this test was
	// named for a universal and checked a single case, so a mutation that
	// stripped the population from the not-measured line survived it — the
	// exact shape of defect this repository spent a day finding elsewhere.
	for name, line := range map[string]string{
		"measured":     r.Compare(0.288).Line(),
		"not measured": r.CompareMissing().Line(),
	} {
		for _, want := range []string{"14", "13.5M sessions", "arXiv:2608.00101"} {
			if !strings.Contains(line, want) {
				t.Errorf("the %s line omits %q: %q", name, want, line)
			}
		}
	}
	line := r.Compare(0.288).Line()
	if !strings.Contains(line, "28.8") {
		t.Errorf("the measured line omits the local figure: %q", line)
	}
	for _, banned := range []string{"average", "typical", "should", "too high", "worse"} {
		if strings.Contains(strings.ToLower(line), banned) {
			t.Errorf("the line judges rather than describes (%q): %q", banned, line)
		}
	}
}

// RF7: it is ASCII, because it reaches a terminal that is audited for it.
func TestRF7_LinesAreASCII(t *testing.T) {
	for _, ref := range Compiled() {
		for _, s := range []string{ref.Compare(0.5).Line(), ref.CompareMissing().Line()} {
			for i, r := range s {
				if r > 127 {
					t.Errorf("%s: non-ASCII %q at %d in %q", ref.Metric, r, i, s)
				}
			}
		}
	}
}
