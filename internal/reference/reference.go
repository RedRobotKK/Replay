// Package reference carries published population figures, so that one machine
// can be read against many.
//
// internal/analysis/outlier.go states the constraint this package exists for:
// a bare figure is not actionable, and neither the reader nor this tool can
// supply a reference from a population, because the pooled corpus has one
// member and publishing a population figure derived from one machine is the
// shape of claim this project has already retracted twice.
//
// The way out is not to publish our own. It is to carry somebody else's, with
// their citation and their population attached, and to be careful about what a
// difference from it licenses anybody to say. See
// docs/design/reference-distribution.md.
package reference

import (
	"errors"
	"fmt"
	"strings"
)

// Unit says what a Value means, and therefore what domain it must sit in.
type Unit string

const (
	// UnitShare is a fraction of a whole, in 0..1.
	UnitShare Unit = "share"
	// UnitRatio is a multiple, non-negative and unbounded above.
	UnitRatio Unit = "ratio"
)

// Verdict is how a local figure sits against a published one.
//
// The vocabulary is deliberately narrow and deliberately free of judgement. A
// provider's published minimum is a claim about the world, and one
// counterexample refutes it — that asymmetry is what internal/cachemodel/claim.go
// encodes. A population's published median is a claim about a POPULATION, and
// one machine differing from it refutes nothing at all: it is one draw. So
// there is no "high", no "above average" and no "worse" here, and a test
// asserts there never will be.
type Verdict string

const (
	// Unmeasured: this machine did not compute the figure. Absence, zero and
	// unknown are three values (ADR-0018), and this is the first.
	Unmeasured Verdict = "unmeasured"
	// NoReference: nothing published covers this metric.
	NoReference Verdict = "no-reference"
	// Within: the local figure falls inside a published spread.
	Within Verdict = "within"
	// Outside: it falls outside a published spread.
	Outside Verdict = "outside"
	// Differs: no spread was published, so only direction and magnitude can be
	// stated. This is the common case, because most papers report a median and
	// not a distribution.
	Differs Verdict = "differs"
)

// Reference is one published figure and the population it describes.
//
// Population is mandatory, for the reason rules.go requires a version: a report
// has to be able to name what produced its numbers. It is also the guard
// against this package's central risk — that a reader takes 14% as THE
// system-prompt share rather than as Copilot users on Copilot's harness, and
// concludes their own 28.8% is wrong.
type Reference struct {
	Metric     string
	Value      float64
	Unit       Unit
	Population string
	Citation   string
	// MeasuredAt is when the DATA was collected, not when it was published or
	// fetched. A paper published in August about June traces ages from June: a
	// published population is never wrong, and what decays is how well it still
	// describes the world.
	MeasuredAt string
	// P10 and P90 are the published spread, where one was reported. Absent is
	// absent: a figure with no reported dispersion must not be rendered as a
	// point estimate with an implied tight band around it.
	P10 *float64
	P90 *float64
	// DeclaredVerdict exists only to be refused, exactly as
	// cachemodel.Claim.DeclaredStatus does. A hand-written "typical" is another
	// claim wearing a verdict's clothes, and the point is to have one field
	// here that nobody can simply assert.
	DeclaredVerdict string
}

func (r Reference) validate() error {
	switch {
	case strings.TrimSpace(r.Metric) == "":
		return errors.New("a reference with no metric names nothing")
	case strings.TrimSpace(r.Population) == "":
		return fmt.Errorf("%s: no population. A figure without one is a number with "+
			"no referent, and a reader cannot tell whose traffic it describes", r.Metric)
	case strings.TrimSpace(r.Citation) == "":
		return fmt.Errorf("%s: no citation, so the figure cannot be checked", r.Metric)
	case strings.TrimSpace(r.MeasuredAt) == "":
		return fmt.Errorf("%s: no collection date, so nothing can age it", r.Metric)
	case r.DeclaredVerdict != "":
		return fmt.Errorf("%s: declares the verdict %q. The verdict is computed from "+
			"the published figure and the local one; a written one is another claim "+
			"wearing a verdict's clothes", r.Metric, r.DeclaredVerdict)
	case r.Unit == UnitShare && (r.Value < 0 || r.Value > 1):
		return fmt.Errorf("%s: share %v is outside 0..1", r.Metric, r.Value)
	case r.Unit == UnitRatio && r.Value < 0:
		return fmt.Errorf("%s: negative ratio %v", r.Metric, r.Value)
	case r.P10 != nil && r.P90 != nil && *r.P10 > *r.P90:
		return fmt.Errorf("%s: p10 %v is above p90 %v", r.Metric, *r.P10, *r.P90)
	}
	return nil
}

// Comparison is where a local figure sits against a published one.
type Comparison struct {
	Reference Reference
	Local     float64
	HasLocal  bool
	Verdict   Verdict
}

// Compare places a measured local figure against this reference.
func (r Reference) Compare(local float64) Comparison {
	c := Comparison{Reference: r, Local: local, HasLocal: true, Verdict: Differs}
	if r.P10 != nil && r.P90 != nil {
		if local >= *r.P10 && local <= *r.P90 {
			c.Verdict = Within
		} else {
			c.Verdict = Outside
		}
	}
	return c
}

// CompareMissing is the comparison for a metric this machine did not compute.
//
// Separate from Compare rather than a zero value passed to it, because a local
// figure of zero is a measurement and the absence of one is not.
func (r Reference) CompareMissing() Comparison {
	return Comparison{Reference: r, Verdict: Unmeasured}
}

// Line is the one row a reader sees, and it carries the population every time.
//
// That is the whole safety property of this package. "28.8% here, 14% across
// 13.5M sessions (arXiv:2608.00101)" is a sentence a reader can weigh.
// "28.8%, roughly double the average" is not, and is what this exists to make
// unwriteable.
func (c Comparison) Line() string {
	r := c.Reference
	if !c.HasLocal {
		return fmt.Sprintf("not measured here; %s across %s (%s)",
			format(r.Value, r.Unit), r.Population, r.Citation)
	}
	return fmt.Sprintf("%s here, %s across %s (%s)",
		format(c.Local, r.Unit), format(r.Value, r.Unit), r.Population, r.Citation)
}

func format(v float64, u Unit) string {
	if u == UnitShare {
		return fmt.Sprintf("%.1f%%", v*100)
	}
	return fmt.Sprintf("%.1fx", v)
}

// Compiled is the set this build ships with.
//
// Two entries, not the four the publications supply, and the omission is the
// point. A reference is only usable when the local figure it is compared
// against measures the same thing, and that mapping is a judgement rather than
// a lookup. systemPromptShare maps cleanly: `replay context`'s `system` row IS
// the system prompt, and Copilot reports the same partition. historyShare and
// toolShare do not: Copilot's "conversation history, 48%" against this tool's
// separate assistant and user rows is not obviously the same cut of the same
// prompt, and shipping a comparison across an undefended mapping would put a
// number on screen that nobody can check.
//
// A floor rather than a stale document, in the sense the rules table is: it is
// what a machine that has installed nothing is compared against. Two
// publications, both from 2026, both measuring coding-agent traffic at a scale
// this project will not reach.
func Compiled() []Reference {
	return []Reference{
		{
			Metric: "systemPromptShare", Value: 0.14, Unit: UnitShare,
			Population: "13.5M sessions, 760.5M LLM calls",
			Citation:   "arXiv:2608.00101", MeasuredAt: "2026-06",
		},
		{
			Metric: "cachedShare", Value: 0.957, Unit: UnitShare,
			Population: "4,265 sessions, 43 developers, Claude Code and Codex",
			Citation:   "arXiv:2606.30560", MeasuredAt: "2026-05",
		},
	}
}

// For returns the compiled reference for a metric, and false when nothing
// published covers it.
func For(metric string) (Reference, bool) {
	for _, r := range Compiled() {
		if r.Metric == metric {
			return r, true
		}
	}
	return Reference{}, false
}
