package coverage

import "github.com/RedRobotKK/Replay/internal/cachemodel"

// Claim is one row of §4's claim-gate matrix.
//
// §4.2 carries two claims rather than one, because its own Scope row splits
// them: an aggregate cached share is claimable under any observation state,
// and a per-turn cache read attribution is claimable only under lane-serial.
// Collapsing the two would mean either refusing an aggregate a lane can
// support or attributing a read to a turn nothing joins to its predecessor.
type Claim string

const (
	// TokenUsage is §4.1, observed token usage.
	TokenUsage Claim = "observed token usage"
	// CacheShareAggregate is §4.2 at aggregate scope.
	CacheShareAggregate Claim = "observed cache behaviour, aggregate"
	// CacheReadPerTurn is §4.2 at per-turn scope.
	CacheReadPerTurn Claim = "observed cache behaviour, per turn"
	// BilledSpend is §4.3, observed billed spend.
	BilledSpend Claim = "observed billed spend"
	// BreakIdentification is §4.4, cache-break identification.
	BreakIdentification Claim = "cache-break identification"
	// CoveragePresentation is §4.7. It is listed because the matrix lists it,
	// and it is the one row that can never refuse: the vector describes
	// evidence and is not itself a claim.
	CoveragePresentation Claim = "coverage presentation"
)

// Claims is every gated row of §4 this package implements.
//
// §4.5 and §4.6, the counterfactual families, are deliberately absent. They
// are implemented at their call site in cmd/replay under ADR-0025 and carry a
// numerical-stability gate that is a property of a computed delta rather than
// of evidence. Restating them here would be a second spelling of a locked
// gate, which is how two answers to one question get shipped.
var Claims = []Claim{
	TokenUsage, CacheShareAggregate, CacheReadPerTurn, BilledSpend,
	BreakIdentification, CoveragePresentation,
}

// Class is the evidence class: how a claim was derived (§2).
type Class string

const (
	// Measured maps to evidence taken from SourceLedger with no contributing
	// count from the fit.
	Measured Class = "MEASURED"
	// Reconstructed maps to what the repository calls the estimated tier: the
	// byte-to-token fit, and the replay engine reproducing provider caching.
	Reconstructed Class = "RECONSTRUCTED"
)

// NotMeasured is the claim result, and only ever the claim result (§2, §10).
//
// It is not a coverage state and not an evidence class. A vector cannot be
// NOT_MEASURED; a claim can. The distinction is the whole reason the layers
// are separate, and it is checked by test rather than left to this comment.
const NotMeasured = "NOT_MEASURED"

// Result is a claim, or the refusal of one.
//
// There is no confidence field, no score and no grade (§5). A refused claim is
// not a weak claim: it is the absence of a claim, with the coverage states
// that produced the absence named beside it.
type Result struct {
	// Claimed is whether a claim exists. False is NOT_MEASURED.
	Claimed bool
	// Class is set only when Claimed. §4.7 leaves it empty, because a vector
	// is not a claim and has no evidence class.
	Class Class
	// Causes names the gate conditions that produced a refusal, each one a
	// state from §3's closed vocabularies (§8). Empty when the claim stands.
	//
	// EVERY prohibited state present is named, not one chosen from among
	// them. Choosing would need an order the contract does not define, and
	// naming only the first would hide the rest from a reader who then fixes
	// one thing and is refused again for another. The slice is in §3's
	// dimension order, which is a deterministic serialization and not a
	// ranking.
	Causes []string
}

// Gate evaluates one claim against one vector (§4).
//
// This is CV-2 in executable form: a conclusion never reads stronger than its
// weakest dimension permits. The mechanism is membership in the row's own
// Prohibited set, never a comparison between states. The contract defines no
// order among lane-overlap and unmeasured, among failing, drifted and
// no-evidence, among documented and declared, or between proxy-recorded and
// transcript-derived, so nothing here asks which of two states is worse.
func Gate(c Claim, v Vector) Result {
	if c == CoveragePresentation {
		// §4.7: success is always, and the evidence class is not applicable.
		return Result{Claimed: true}
	}
	causes := prohibited(c, v)
	if len(causes) > 0 {
		return Result{Causes: causes}
	}
	return Result{Claimed: true, Class: classOf(c, v)}
}

// prohibited returns the states of v that the row's Prohibited line names, in
// §3's dimension order.
func prohibited(c Claim, v Vector) []string {
	var out []string
	observationBlocks := func() {
		if v.Observation != LaneSerial {
			out = append(out, string(v.Observation))
		}
	}
	parseBlocks := func() {
		if v.Has(PartialParse) {
			out = append(out, string(PartialParse))
		}
	}
	switch c {
	case TokenUsage:
		// 4.1 Prohibited: partial-parse. Observation is any state, because
		// token counts do not depend on predecessor joining, and calibration
		// is not required, because the counts are read rather than reproduced.
		parseBlocks()
	case CacheShareAggregate:
		// 4.2 Prohibited: partial-parse. Cache read and write counts are
		// reported by the provider, so calibration is not required.
		parseBlocks()
	case CacheReadPerTurn:
		// 4.2 Scope: per-turn only under lane-serial. Which request wrote the
		// entry this one read is not determined by anything observable once
		// the lane overlapped, and is not recorded at all when unmeasured.
		observationBlocks()
		parseBlocks()
	case BilledSpend:
		// 4.3 Prohibited: pricing_basis absent, partial-pricing, partial-parse.
		if v.Has(PartialPricing) {
			out = append(out, string(PartialPricing))
		}
		parseBlocks()
		if v.PricingBasis == PricingAbsent {
			out = append(out, string(PricingAbsent))
		}
	case BreakIdentification:
		// 4.4 Prohibited: lane-overlap, unmeasured, and calibration failing,
		// drifted or no-evidence. Required completeness: not partial-parse.
		// The cause is inferred by the engine rather than reported by the
		// provider, which is why this row needs calibration where 4.2 does
		// not.
		observationBlocks()
		parseBlocks()
		if v.Calibration != CalibrationPassing {
			out = append(out, string(v.Calibration))
		}
	}
	return out
}

// classOf is §2's mapping, applied per row.
//
// 4.1, 4.2 and 4.3 are MEASURED when the figures came off the wire and no
// contributing count came from the fit, and RECONSTRUCTED otherwise. 4.4 is
// RECONSTRUCTED whatever the provenance, because the provider never reports a
// break cause: the engine infers it.
func classOf(c Claim, v Vector) Class {
	if c == BreakIdentification {
		return Reconstructed
	}
	if v.Provenance == ProxyRecorded && !v.Has(PartialTokens) {
		return Measured
	}
	return Reconstructed
}

// BreakCauseFor expresses a refused §4.4 in the existing cause field.
//
// §8 creates no new limitation enum, and §4.4 says a refusal here is carried
// as cachemodel.CauseNotMeasured, which already exists and already reads as a
// refusal rather than as a cause. The second return says whether the claim was
// refused at all, so a caller cannot mistake the zero value for a finding.
func BreakCauseFor(v Vector) (cachemodel.BreakCause, bool) {
	if Gate(BreakIdentification, v).Claimed {
		return "", false
	}
	return cachemodel.CauseNotMeasured, true
}
