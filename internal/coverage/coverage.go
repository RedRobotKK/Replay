// Package coverage is the evidence coverage vector and the claim gates that
// read it.
//
// Three layers, and they are not the same question (§2 of the contract):
//
//   - EVIDENCE CLASS answers how a claim was derived. MEASURED, RECONSTRUCTED
//     or COUNTERFACTUAL.
//   - COVERAGE VECTOR answers what the inputs were. Six dimensions, §3.
//   - CLAIM RESULT answers whether a claim exists at all. The claim, or
//     NOT_MEASURED.
//
// The vector is the middle layer and only the middle layer. It is not
// provenance, which is one of its six dimensions rather than the whole of it,
// and it is not confidence, which §5 removed outright: there is no HIGH, no
// MEDIUM, no LOW, no percentage, and nothing here combines two dimensions into
// a third number. A dimension that could be averaged would be a grade wearing
// a vector's clothes.
//
// WHY A VECTOR AND NOT A BIT. A single "is this trustworthy" flag has to
// answer for every claim at once, so it ends up answering for the weakest one
// and dragging the rest down with it. A missing price is fatal to a dollar
// figure and irrelevant to a token count; a lane whose requests overlapped is
// fatal to per-turn attribution and irrelevant to the session total. §4's
// matrix is where that claim-specificity lives, and this package is the part
// of it a program can execute.
//
// WHAT THIS PACKAGE DOES NOT DO. It emits no document, changes no output
// wording, and touches no contribution payload. §11 of the contract withholds
// contribution and schema authorization, and nothing here reaches for it: the
// vector is computed from evidence already in memory and handed to a caller.
// Whether a vector may travel with a contributed row, and how each dimension's
// evidence scope would be carried if it did, are open questions this package
// does not answer and must not be read as answering.
package coverage

import (
	"strings"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Observation is whether the evidence required to join a request to its
// predecessor was observed (§3.1).
type Observation string

const (
	// LaneSerial means no other request of the lane was open during this one, so
	// the predecessor is determined.
	LaneSerial Observation = "lane-serial"
	// LaneOverlap means another request was open concurrently, so which one wrote
	// the entry this one read is not determined by anything observable.
	LaneOverlap Observation = "lane-overlap"
	// ObservationUnmeasured means nothing recorded whether the lane was exclusive.
	//
	// Named for its dimension because "unmeasured" alone reads like the claim
	// result NOT_MEASURED, and §2 keeps those layers apart.
	ObservationUnmeasured Observation = "unmeasured"
)

// Observations is the closed vocabulary of §3.1, in contract order.
var Observations = []Observation{LaneSerial, LaneOverlap, ObservationUnmeasured}

// Completeness is whether the observed evidence carries every field the claim
// requires (§3.2).
type Completeness string

const (
	// Complete means all required fields are present.
	Complete Completeness = "complete"
	// PartialTokens means at least one contributing count came from the
	// byte-to-token fit rather than provider usage.
	PartialTokens Completeness = "partial-tokens"
	// PartialPricing means at least one contributing request used a model absent
	// from the rules.
	PartialPricing Completeness = "partial-pricing"
	// PartialParse means the reader could not interpret one or more lines of a
	// contributing session.
	PartialParse Completeness = "partial-parse"
)

// Completenesses is the closed vocabulary of §3.2, in contract order.
var Completenesses = []Completeness{Complete, PartialTokens, PartialPricing, PartialParse}

// Provenance is where the usage figures came from (§3.3).
type Provenance string

const (
	// ProxyRecorded means the figures were taken from the wire by `replay serve`.
	ProxyRecorded Provenance = "proxy-recorded"
	// TranscriptDerived means the figures were read from a client transcript on disk.
	//
	// It forces evidence class RECONSTRUCTED for any claim depending on those
	// counts. It blocks no claim by itself, and it is NOT ordered against
	// proxy-recorded: the difference is a consequence for the class, not a
	// ranking of the states.
	TranscriptDerived Provenance = "transcript-derived"
)

// Provenances is the closed vocabulary of §3.3, in contract order.
var Provenances = []Provenance{ProxyRecorded, TranscriptDerived}

// PricingBasis is what a dollar figure rests on (§3.4).
type PricingBasis string

const (
	// Documented means published provider rates.
	Documented PricingBasis = "documented"
	// Declared means an account discount stated by the operator, private and
	// unverifiable by anyone else. It blocks no claim and is carried on it.
	Declared PricingBasis = "declared"
	// PricingAbsent means no rules document resolved for a contributing model.
	//
	// Named for its dimension so the bare word "absent" cannot be mistaken for
	// a general absence of evidence.
	PricingAbsent PricingBasis = "absent"
)

// PricingBases is the closed vocabulary of §3.4, in contract order.
var PricingBases = []PricingBasis{Documented, Declared, PricingAbsent}

// Calibration is whether the replay engine reproduced the provider's
// behaviour on this corpus (§3.5).
type Calibration string

const (
	// CalibrationPassing means MatchRate() >= analysis.CalibrationThreshold.
	CalibrationPassing Calibration = "passing"
	// CalibrationFailing means evidence exists and the rate is below threshold.
	CalibrationFailing Calibration = "failing"
	// CalibrationDrifted means earlier lanes calibrated and enough recent lanes
	// stopped, per ST-1. A lane that calibrates on its own is still drifted,
	// because drift is a fact about the provider and the model, not about
	// this lane.
	CalibrationDrifted Calibration = "drifted"
	// CalibrationNoEvidence means no turn had a predecessor to compare against.
	CalibrationNoEvidence Calibration = "no-evidence"
)

// Calibrations is the closed vocabulary of §3.5, in contract order.
//
// The four are not ranked. Failing, drifted and no-evidence are three
// different things that happened, and the contract defines no order among
// them; §4 blocks on membership, never on which is "worse".
var Calibrations = []Calibration{
	CalibrationPassing, CalibrationFailing, CalibrationDrifted, CalibrationNoEvidence,
}

// Freshness is whether the accounting rules in effect have been verified
// against provider behaviour independently of this corpus (§3.6).
type Freshness string

// NotVerified is the only state. Exactly one exists because Replay has exactly
// one answer it can stand behind today: it has not. `verified` and `stale` are not
// defined and are not manufactured for symmetry.
const NotVerified Freshness = "not-verified"

// Freshnesses is the closed vocabulary of §3.6.
var Freshnesses = []Freshness{NotVerified}

// Vector is the six dimensions, and exactly the six.
//
// Field order is §3's own numbering, so a rendering that walks the struct
// produces the contract's order without a second list to keep in step. That is
// a serialization order and not a ranking: §3 numbers its subsections and
// defines no precedence between dimensions.
type Vector struct {
	Observation Observation
	// Completeness is a set, because more than one partial condition can hold
	// at once and §3.2 emits the array form when it does. Complete is the
	// whole set or no part of it, and never appears beside a partial state.
	Completeness []Completeness
	Provenance   Provenance
	PricingBasis PricingBasis
	Calibration  Calibration
	Freshness    Freshness
}

// Has reports whether the completeness set carries a state.
func (v Vector) Has(c Completeness) bool {
	for _, got := range v.Completeness {
		if got == c {
			return true
		}
	}
	return false
}

// CompletenessString is §3.2's serialization: the bare state when one holds,
// the array form when more than one does.
func (v Vector) CompletenessString() string {
	if len(v.Completeness) == 1 {
		return string(v.Completeness[0])
	}
	quoted := make([]string, 0, len(v.Completeness))
	for _, c := range v.Completeness {
		quoted = append(quoted, `"`+string(c)+`"`)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// Evidence is what §3 names as required, in the shapes this repository
// actually has.
//
// Two of §3's named symbols do not exist in this build: `cachemodel.Ledger`
// carries no Unpriced field and there is no `analysis.ModelStaleness` type.
// The facts they stand for do exist and are passed in here instead: Priced and
// Unpriced are the per-request price resolution a caller already computes with
// cachemodel.PriceForAt, and Drifted is the per-model answer from
// analysis.StaleModels over analysis.ModelCalibrations. Naming the substitution
// here keeps it visible rather than letting a reader assume the contract's
// symbols were found.
//
// Every field is optional. §4.7 makes the vector always emitable, so nothing
// here may fail on absence: a zero Evidence yields a valid vector describing an
// absence, which is the honest answer rather than an error.
type Evidence struct {
	// Lane supplies Request.Correlation for observation (§3.1).
	Lane *transcript.Lane
	// Session supplies Source.Tier() for provenance and Skipped for
	// partial-parse (§3.2, §3.3).
	Session *transcript.Session
	// Calibration supplies HasEvidence() and Passes() (§3.5).
	Calibration *analysis.Calibration
	// Fit supplies the estimated counts that make a lane partial-tokens
	// (§3.2). Estimated is the byte-to-token fit; measured counts are not.
	Fit analysis.TokenFit
	// Drifted is the ST-1 answer for this lane's model (§3.5).
	Drifted bool
	// Priced and Unpriced are contributing requests whose model a rules
	// document did and did not carry (§3.2, §3.4).
	Priced, Unpriced int
	// PriceTier is cachemodel.Rules.PriceTier(): "documented" or "declared".
	// Empty is read as documented, which is what a build with no account
	// discount has always meant.
	PriceTier string
}

// Of resolves every dimension to exactly one state (§4.7).
//
// It is total. There is no error return and no refusal, because a vector is
// not a claim: §4.7 says success is always, and a vector describing an absence
// is still a vector. The refusing happens one layer up, in Gate.
func Of(e Evidence) Vector {
	return Vector{
		Observation:  observationOf(e.Lane),
		Completeness: completenessOf(e),
		Provenance:   provenanceOf(e.Session),
		PricingBasis: pricingBasisOf(e),
		Calibration:  calibrationOf(e.Calibration, e.Drifted),
		Freshness:    NotVerified,
	}
}

// observationOf reads §3.1 off the correlation the reader recorded.
//
// The lane is serial only when every request says so. One overlapped request
// means another request of the lane was open, which is what lane-overlap
// states; one unrecorded request means nothing recorded whether the lane was
// exclusive, which is what unmeasured states. Neither is a weaker grade of
// serial, and serial is never assumed from silence: a lane with no requests at
// all recorded nothing and is unmeasured.
func observationOf(lane *transcript.Lane) Observation {
	if lane == nil || len(lane.Requests) == 0 {
		return ObservationUnmeasured
	}
	unrecorded := false
	for _, req := range lane.Requests {
		switch req.Correlation {
		case transcript.CorrelationLaneOverlap:
			return LaneOverlap
		case transcript.CorrelationLaneSerial:
		default:
			unrecorded = true
		}
	}
	if unrecorded {
		return ObservationUnmeasured
	}
	return LaneSerial
}

// completenessOf collects every partial condition that holds (§3.2).
//
// The three are independent questions and all three can be true at once, so
// they are collected rather than chosen between. Complete is what is left when
// none of them holds, and it is returned alone.
func completenessOf(e Evidence) []Completeness {
	var out []Completeness
	if e.Fit.UnseenPrefix.Estimated > 0 || e.Fit.Injected.Estimated > 0 {
		out = append(out, PartialTokens)
	}
	if e.Unpriced > 0 {
		out = append(out, PartialPricing)
	}
	if e.Session != nil && e.Session.Skipped > 0 {
		out = append(out, PartialParse)
	}
	if len(out) == 0 {
		return []Completeness{Complete}
	}
	return out
}

// provenanceOf reads §3.3 off the existing tier.
//
// The repository's own wording is preserved in output; this is the machine
// field beside it, and §2 is explicit that the two are not collapsed.
func provenanceOf(s *transcript.Session) Provenance {
	if s != nil && s.Source == transcript.SourceLedger {
		return ProxyRecorded
	}
	return TranscriptDerived
}

// pricingBasisOf reads §3.4.
//
// Absent when no contributing request resolved to a rules document, including
// the case where nothing was read at all: a basis nothing rests on is absent,
// not documented. A lane that is only partly priced still has a documented
// basis for the part that resolved, and the part that did not is
// partial-pricing on the completeness dimension. The two dimensions are
// independent and are read independently.
func pricingBasisOf(e Evidence) PricingBasis {
	if e.Priced == 0 {
		return PricingAbsent
	}
	if e.PriceTier == string(Declared) {
		return Declared
	}
	return Documented
}

// calibrationOf reads §3.5.
//
// Drift is checked before the rate, because a lane whose model drifted is
// drifted whatever this lane's own rate says: ST-1 measures the provider
// changing under the corpus, and a single lane that still reproduces does not
// answer that. This is precedence in a derivation, not an ordering between
// states.
func calibrationOf(c *analysis.Calibration, drifted bool) Calibration {
	if c == nil || !c.HasEvidence() {
		return CalibrationNoEvidence
	}
	if drifted {
		return CalibrationDrifted
	}
	if c.Passes() {
		return CalibrationPassing
	}
	return CalibrationFailing
}
