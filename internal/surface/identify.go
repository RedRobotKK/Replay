// Package surface decides whether E005's estimand can be measured at a given
// measurement boundary, from what a probe of that boundary actually found.
//
// The conditions are stated in docs/THEORY-IDENTIFICATION-2026-09-26.md and
// the corpus readings behind them in
// docs/evidence/cross-surface-observability-2026-09-26.md. This package is the
// executable form of that classification, so a claim about a surface fails a
// test rather than only contradicting a paragraph.
//
// # The invariant
//
// A corpus probe must never infer a provider's economic class from transcript
// observations alone. Codex is why. A corpus carrying populated cache reads
// and a cache-write counter that is zero on every record is observationally
// identical under two incompatible worlds:
//
//   - a provider that performs no billable cache writes, and
//   - a provider that does, whose client drops the counter before it reaches
//     the artifact.
//
// Nothing in the bytes separates them. The provider's published pricing does.
// So Classify takes two independently sourced inputs, corpus evidence and a
// contract fact carrying its own provenance, and when the contract is absent
// it returns unknown rather than reading the zeros as an answer.
package surface

import "fmt"

// WriteContract is what the PROVIDER's published pricing says about cache
// writes. It is condition N1a and it is NOT observable from any corpus.
type WriteContract int

const (
	// ContractUnknown is the honest default and the one that keeps the
	// package safe: every path that would otherwise read a zero as a finding
	// stops here instead.
	ContractUnknown WriteContract = iota
	// ContractPricedDistinctly means a write is billed at a rate that differs
	// from ordinary input, so "write" names an economic category. Anthropic
	// and OpenAI GPT-5.6 and later, both at 1.25x, are the observed instances.
	ContractPricedDistinctly
	// ContractNotPricedDistinctly means a miss bills as ordinary input. The
	// cost of a break is real and visible, but it is not a write premium, so
	// it is not E005's dependent variable.
	ContractNotPricedDistinctly
)

// ContractFact is a provider contract claim together with where it came from.
//
// The provenance is not decoration. This is the input that cannot be checked
// against the corpus, so the only thing standing behind it is the document it
// was read from, and a reviewer has to be able to go and disagree with that
// document. A fact with no source is refused by Validate.
type ContractFact struct {
	Write  WriteContract
	Source string
}

// Validate refuses a contract claim that asserts something without saying why.
func (c ContractFact) Validate() error {
	if c.Write != ContractUnknown && c.Source == "" {
		return fmt.Errorf("contract asserts a write pricing class with no source; an unsourced contract fact is the one input a corpus cannot check")
	}
	return nil
}

// Class is the identification status of a (surface, boundary) pair.
type Class string

// The identification classes, in the order the conditions rule them out.
const (
	Class0Identified       Class = "0-identified"
	ClassIMarginalOnly     Class = "I-marginal-only"
	ClassIIArtifactLoss    Class = "II-artifact-loss"
	ClassIIINoObservable   Class = "III-no-observable"
	ClassIVIncommensurable Class = "IV-incommensurable"
	// ClassUndetermined is a verdict, not a failure to reach one. It is what
	// an all-zero write counter means when the provider's pricing is unknown,
	// and what an empty corpus means always.
	ClassUndetermined Class = "undetermined"
)

// Basis says which of the two independent inputs a condition was decided from.
type Basis string

// The two independent sources a condition can be decided from.
const (
	FromCorpus   Basis = "corpus"
	FromContract Basis = "contract"
)

// Condition is one identification condition and how it was decided.
type Condition struct {
	ID     string
	Name   string
	Pass   bool
	Basis  Basis
	Detail string
}

// Verdict is the auditable result: what was observed, what was independently
// known, which conditions passed, and why the class follows from those.
type Verdict struct {
	Boundary   string
	Class      Class
	Why        string
	Conditions []Condition
	Observed   Observables
	Contract   ContractFact
}

// Passed reports whether the named condition passed. Absent reads as false.
func (v Verdict) Passed(id string) bool {
	for _, c := range v.Conditions {
		if c.ID == id {
			return c.Pass
		}
	}
	return false
}

// Classify returns the identification status and the reasoning behind it.
//
// Order follows the conditions. N1a, whether a write is a priced category at
// all, is prior to N1b, whether the counter survived to this boundary: a
// surface whose provider does not price writes distinctly is measuring a
// different quantity however well its client logs.
//
// Every branch that could return a STRONGER class than the evidence supports
// is guarded, and the guards are mutation-tested. The failure this package
// exists to prevent is a confident class derived from zeros.
func Classify(o Observables, c ContractFact) Verdict {
	v := Verdict{Boundary: o.Boundary, Observed: o, Contract: c}

	add := func(id, name string, pass bool, basis Basis, detail string) {
		v.Conditions = append(v.Conditions, Condition{ID: id, Name: name, Pass: pass, Basis: basis, Detail: detail})
	}

	if err := c.Validate(); err != nil {
		v.Class, v.Why = ClassUndetermined, err.Error()
		add("N1a", "write is a distinctly priced category", false, FromContract, err.Error())
		return v
	}

	// An empty corpus is ambiguous about everything. Reporting a class from
	// no records would be the purest form of the defect this package guards.
	if o.Records == 0 {
		v.Class, v.Why = ClassUndetermined, "no records at this boundary, so nothing was observed to classify"
		add("corpus", "records present", false, FromCorpus, "0 records")
		return v
	}
	add("corpus", "records present", true, FromCorpus, fmt.Sprintf("%d records", o.Records))

	// N1a. Contract only. The corpus cannot speak to this.
	switch c.Write {
	case ContractNotPricedDistinctly:
		add("N1a", "write is a distinctly priced category", false, FromContract, c.Source)
		v.Class = ClassIVIncommensurable
		v.Why = "the provider does not price cache writes distinctly, so a break bills as ordinary input; the quantity is real but it is not E005's dependent variable"
		return v
	case ContractPricedDistinctly:
		add("N1a", "write is a distinctly priced category", true, FromContract, c.Source)
	case ContractUnknown:
		add("N1a", "write is a distinctly priced category", false, FromContract, "not established")
	}

	// N1b. Corpus only. Did the counter survive to this boundary.
	switch {
	case !o.WriteFieldPresent:
		add("N1b", "write counter survives to this boundary", false, FromCorpus, "field absent from every record")
		v.Class = ClassIIINoObservable
		v.Why = "no cache-write field at this boundary, so no write can be differenced at any price"
		return v
	case o.WriteNonZero == 0:
		add("N1b", "write counter survives to this boundary", false, FromCorpus,
			fmt.Sprintf("field present on %d records, non-zero on 0", o.WriteObservations))
		// THE INVARIANT. Zeros plus an unknown contract is unknown, never a
		// finding about the provider.
		if c.Write != ContractPricedDistinctly {
			v.Class = ClassUndetermined
			v.Why = fmt.Sprintf(
				"the write field is present on %d records (%d observations) and non-zero on none; a provider that never writes and a client that drops the counter produce this identically, and the provider's pricing is not established, so the zeros are uninterpreted",
				o.RecordsWithWrite, o.WriteObservations)
			return v
		}
		v.Class = ClassIIArtifactLoss
		v.Why = fmt.Sprintf(
			"the provider prices writes distinctly (%s) but the counter is non-zero on 0 of %d observations across %d records, so it did not survive to this boundary",
			c.Source, o.WriteObservations, o.RecordsWithWrite)
		return v
	default:
		add("N1b", "write counter survives to this boundary", true, FromCorpus,
			fmt.Sprintf("non-zero on %d of %d records", o.WriteNonZero, o.WriteObservations))
	}

	// A populated write counter is not enough on its own: without N1a the
	// quantity may not be a priced one, and the class stays open.
	if c.Write != ContractPricedDistinctly {
		v.Class = ClassUndetermined
		v.Why = "writes are observed at this boundary, but whether they are a distinctly priced quantity is not established, so the economic class does not follow"
		return v
	}

	// N2. Corpus only.
	if !o.PerRequestSequencing {
		add("N2", "per-request sequencing", false, FromCorpus, "no per-request usage block")
		v.Class = ClassIIINoObservable
		v.Why = "writes are reported but not per request, so no boundary difference can be taken"
		return v
	}
	add("N2", "per-request sequencing", true, FromCorpus, "per-request usage block present")

	// N3. Corpus only, and it must be the PROVIDER's partition. Replay's own
	// detector is not admissible here: it branches on the same usage block
	// that supplies the dependent variable, so it would be a conditioning
	// variable computed from the DV. Probe never reads it, which is how that
	// rule is enforced rather than asserted.
	if o.OracleClasses == 0 {
		add("N3", "provider cause partition", false, FromCorpus, "no cause field in any record")
		v.Class = ClassIMarginalOnly
		v.Why = "writes are observable and differenceable, but no provider cause partition exists, so the marginal distribution of the write change is reachable and the conditional family m(c) is not"
		return v
	}
	add("N3", "provider cause partition", true, FromCorpus, fmt.Sprintf("%d distinct classes", o.OracleClasses))

	v.Class = Class0Identified
	v.Why = fmt.Sprintf("a distinctly priced write counter (%s), non-zero on %d of %d observations across %d records, per-request sequencing, and a provider cause partition of %d classes",
		c.Source, o.WriteNonZero, o.WriteObservations, o.RecordsWithWrite, o.OracleClasses)
	return v
}

// NullsInterpretable reports whether a median of zero at this boundary can be
// read as an observed zero rather than an absent observation.
//
// Condition (d), deliberately NOT part of Classify. It does not decide whether
// the contrast is identified; it decides whether a null result means anything.
// E005 needs it because three of its four classes report a median write change
// of zero, and a null is a finding only when the instrument was demonstrably
// recording at the time.
//
// It narrows the ambiguity without closing it: a populated read shows the
// usage block was being written, which is consistent both with a genuine zero
// and with a defaulted one. What settled Codex was the pricing argument, not
// this.
func NullsInterpretable(o Observables) bool { return o.ReadNonZero > 0 }
