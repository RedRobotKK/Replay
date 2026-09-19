package coverage

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// everyVector enumerates the whole locked state space: three observations,
// every non-empty completeness set plus `complete`, two provenances, three
// pricing bases, four calibrations, one freshness.
//
// The matrix below is checked on all of them rather than on chosen examples,
// because a claim gate that is right on the cases its author thought of is the
// shape of a gate that is wrong on the case a user hits.
func everyVector() []Vector {
	partials := [][]Completeness{
		{Complete},
		{PartialTokens},
		{PartialPricing},
		{PartialParse},
		{PartialTokens, PartialPricing},
		{PartialTokens, PartialParse},
		{PartialPricing, PartialParse},
		{PartialTokens, PartialPricing, PartialParse},
	}
	var out []Vector
	for _, o := range Observations {
		for _, c := range partials {
			for _, p := range Provenances {
				for _, pb := range PricingBases {
					for _, cal := range Calibrations {
						out = append(out, Vector{
							Observation:  o,
							Completeness: c,
							Provenance:   p,
							PricingBasis: pb,
							Calibration:  cal,
							Freshness:    NotVerified,
						})
					}
				}
			}
		}
	}
	return out
}

func has(v Vector, c Completeness) bool {
	for _, got := range v.Completeness {
		if got == c {
			return true
		}
	}
	return false
}

// wantProhibited is written from §4's Prohibited rows, independently of the
// package, so an implementation that reads its expectation from itself cannot
// pass.
func wantProhibited(c Claim, v Vector) bool {
	switch c {
	case TokenUsage:
		// 4.1 Prohibited: partial-parse.
		return has(v, PartialParse)
	case CacheShareAggregate:
		// 4.2 Prohibited: partial-parse. Any observation state for aggregate.
		return has(v, PartialParse)
	case CacheReadPerTurn:
		// 4.2 Scope: per-turn only under lane-serial.
		return has(v, PartialParse) || v.Observation != LaneSerial
	case BilledSpend:
		// 4.3 Prohibited: pricing_basis=absent, partial-pricing, partial-parse.
		return v.PricingBasis == PricingAbsent || has(v, PartialPricing) || has(v, PartialParse)
	case BreakIdentification:
		// 4.4 Prohibited: lane-overlap, unmeasured, calibration failing,
		// drifted or no-evidence. Required completeness: not partial-parse.
		return v.Observation != LaneSerial ||
			has(v, PartialParse) ||
			v.Calibration != CalibrationPassing
	case CoveragePresentation:
		// 4.7 Required states: none. NOT_MEASURED: never.
		return false
	}
	panic("unlisted claim: " + string(c))
}

// CV-2, expressed exactly as §4 defines it: a conclusion never reads stronger
// than its weakest dimension permits, and the mechanism is the Prohibited row
// of the claim's own gate. No ordering between states is used, because the
// contract defines none: this is set membership.
func TestTheClaimGateMatrixHoldsOverTheWholeStateSpace(t *testing.T) {
	vectors := everyVector()
	if len(vectors) != 576 {
		t.Fatalf("the enumeration covers %d vectors; the locked space is 3*8*2*3*4 = 576", len(vectors))
	}
	for _, c := range Claims {
		for _, v := range vectors {
			got := Gate(c, v)
			if want := wantProhibited(c, v); got.Claimed == want {
				t.Fatalf("%s on %s: claimed=%v, §4 prohibits=%v",
					c, v.CompletenessString()+"/"+string(v.Observation)+"/"+
						string(v.PricingBasis)+"/"+string(v.Calibration), got.Claimed, want)
			}
		}
	}
}

// §4.3's own note: "a missing price blocks 4.3 and does not touch 4.1". This
// is the claim-specificity the vector exists for, so it is pinned on its own
// rather than left to the sweep.
func TestAMissingPriceBlocksSpendAndLeavesTokensStanding(t *testing.T) {
	v := Vector{
		Observation:  LaneSerial,
		Completeness: []Completeness{PartialPricing},
		Provenance:   ProxyRecorded,
		PricingBasis: PricingAbsent,
		Calibration:  CalibrationPassing,
		Freshness:    NotVerified,
	}
	if Gate(BilledSpend, v).Claimed {
		t.Error("billed spend stands with no price behind it")
	}
	if !Gate(TokenUsage, v).Claimed {
		t.Error("a missing price took the token claim down with it")
	}
	if !Gate(CacheShareAggregate, v).Claimed {
		t.Error("a missing price took the cache claim down with it")
	}
}

// §3.5: calibration blocks claims that depend on the engine reproducing
// provider behaviour, and does not block claims read directly off observed
// usage.
func TestCalibrationBlocksOnlyWhatTheMatrixSaysItBlocks(t *testing.T) {
	for _, cal := range []Calibration{CalibrationFailing, CalibrationDrifted, CalibrationNoEvidence} {
		v := Vector{
			Observation:  LaneSerial,
			Completeness: []Completeness{Complete},
			Provenance:   ProxyRecorded,
			PricingBasis: Documented,
			Calibration:  cal,
			Freshness:    NotVerified,
		}
		if Gate(BreakIdentification, v).Claimed {
			t.Errorf("calibration %s still identified a break cause", cal)
		}
		for _, c := range []Claim{TokenUsage, CacheShareAggregate, CacheReadPerTurn, BilledSpend} {
			if !Gate(c, v).Claimed {
				t.Errorf("calibration %s blocked %s, which is read off observed usage", cal, c)
			}
		}
	}
}

// §3.1: lane-overlap and unmeasured block every per-event claim and neither
// blocks any session-aggregate claim.
func TestObservationBlocksPerEventAndNeverTheAggregate(t *testing.T) {
	for _, o := range []Observation{LaneOverlap, ObservationUnmeasured} {
		v := Vector{
			Observation:  o,
			Completeness: []Completeness{Complete},
			Provenance:   ProxyRecorded,
			PricingBasis: Documented,
			Calibration:  CalibrationPassing,
			Freshness:    NotVerified,
		}
		if Gate(CacheReadPerTurn, v).Claimed {
			t.Errorf("observation %s attributed a cache read to a turn", o)
		}
		if Gate(BreakIdentification, v).Claimed {
			t.Errorf("observation %s identified a break cause", o)
		}
		for _, c := range []Claim{TokenUsage, CacheShareAggregate, BilledSpend} {
			if !Gate(c, v).Claimed {
				t.Errorf("observation %s blocked the aggregate claim %s", o, c)
			}
		}
	}
}

// §4.1 and §4.2: MEASURED when provenance is proxy-recorded and completeness
// excludes partial-tokens. Otherwise RECONSTRUCTED. §4.3 takes the same rule,
// and §4.4 is RECONSTRUCTED always because the cause is inferred.
func TestTheEvidenceClassIsTheLockedMapping(t *testing.T) {
	for _, v := range everyVector() {
		wantClass := Reconstructed
		if v.Provenance == ProxyRecorded && !has(v, PartialTokens) {
			wantClass = Measured
		}
		for _, c := range []Claim{TokenUsage, CacheShareAggregate, CacheReadPerTurn, BilledSpend} {
			r := Gate(c, v)
			if !r.Claimed {
				continue
			}
			if r.Class != wantClass {
				t.Fatalf("%s on %s/%s is %s, want %s",
					c, v.Provenance, v.CompletenessString(), r.Class, wantClass)
			}
		}
		if r := Gate(BreakIdentification, v); r.Claimed && r.Class != Reconstructed {
			t.Fatalf("break identification is %s; §4.4 fixes it at RECONSTRUCTED", r.Class)
		}
	}
}

// §5: no scalar confidence is defined, emitted or reserved, and a refusal is
// not a weak claim.
func TestNoResultCarriesAConfidenceOrAGrade(t *testing.T) {
	for _, v := range everyVector() {
		for _, c := range Claims {
			r := Gate(c, v)
			for _, banned := range []string{"HIGH", "MEDIUM", "LOW", "INSUFFICIENT"} {
				if strings.Contains(strings.Join(r.Causes, " ")+string(r.Class), banned) {
					t.Fatalf("%s produced %q, which is the scalar confidence §5 removed", c, banned)
				}
			}
		}
	}
}

// CV-4 and §8: a NOT_MEASURED result names the gate condition that produced
// it, drawn from the closed set of coverage states in §3. No new limitation
// enum exists, and nothing is refused with prose alone.
func TestEveryRefusalNamesAClosedCoverageState(t *testing.T) {
	closed := map[string]bool{}
	for _, s := range append(append(asStrings(Observations), asStrings(Completenesses)...),
		append(asStrings(PricingBases), asStrings(Calibrations)...)...) {
		closed[s] = true
	}
	for _, v := range everyVector() {
		for _, c := range Claims {
			r := Gate(c, v)
			if r.Claimed {
				if len(r.Causes) != 0 {
					t.Fatalf("%s stood and still named causes %v", c, r.Causes)
				}
				continue
			}
			if len(r.Causes) == 0 {
				t.Fatalf("%s refused on %+v and named nothing; §8 forbids silence", c, v)
			}
			for _, cause := range r.Causes {
				if !closed[cause] {
					t.Fatalf("%s refused citing %q, which is not a §3 state", c, cause)
				}
			}
		}
	}
}

// §8 and §4.4: a refused break identification is expressed in the existing
// cause field, not in a new one.
func TestARefusedBreakIdentificationUsesTheExistingCauseField(t *testing.T) {
	blocked := Vector{
		Observation:  LaneOverlap,
		Completeness: []Completeness{Complete},
		Provenance:   ProxyRecorded,
		PricingBasis: Documented,
		Calibration:  CalibrationPassing,
		Freshness:    NotVerified,
	}
	cause, refused := BreakCauseFor(blocked)
	if !refused {
		t.Fatal("an overlapped lane identified a break cause")
	}
	if cause != cachemodel.CauseNotMeasured {
		t.Errorf("the refusal is expressed as %q; §4.4 names the existing CauseNotMeasured", cause)
	}
	ok := blocked
	ok.Observation = LaneSerial
	if _, refused := BreakCauseFor(ok); refused {
		t.Error("a serial, calibrating lane was refused a break cause")
	}
}

// §4.7: the vector is not a claim and cannot be refused. Success is always.
func TestCoveragePresentationIsNeverRefused(t *testing.T) {
	for _, v := range everyVector() {
		r := Gate(CoveragePresentation, v)
		if !r.Claimed {
			t.Fatalf("coverage presentation was refused on %+v; §4.7 says it never can be", v)
		}
		if r.Class != "" {
			t.Fatalf("coverage presentation carries evidence class %q; §4.7 says not applicable", r.Class)
		}
	}
}

// CV-5 asks for a third conclusion state beside the existing outcomes, one
// that is neither a finding nor a clean result. That state exists and is
// NOT_MEASURED: a gate returns a claim or it returns this, and this is not a
// weak claim.
//
// CV-5's own spelling, `insufficient_coverage`, is NOT implemented, and this
// test pins its absence rather than its presence. §10 fixes the permitted
// refusal at NOT_MEASURED, so a second refusal token would be a new claim
// result the contract does not carry. The requirement's substance is met; the
// draft row's identifier is not, and reconciling the two is a decision above
// this package.
func TestTheThirdConclusionStateIsNotMeasuredAndNothingElse(t *testing.T) {
	outcomes := map[string]bool{}
	for _, v := range everyVector() {
		for _, c := range Claims {
			if r := Gate(c, v); r.Claimed {
				outcomes["claim"] = true
			} else {
				outcomes[NotMeasured] = true
			}
		}
	}
	if len(outcomes) != 2 {
		t.Fatalf("gates produced %d outcome kinds: %v; §10 permits a claim or NOT_MEASURED",
			len(outcomes), outcomes)
	}
	for _, banned := range []string{"insufficient_coverage", "INSUFFICIENT_COVERAGE"} {
		for _, v := range everyVector() {
			for _, c := range Claims {
				r := Gate(c, v)
				for _, cause := range r.Causes {
					if cause == banned {
						t.Fatalf("%s emitted %q; §10 fixes the refusal at NOT_MEASURED", c, banned)
					}
				}
			}
		}
	}
}

// §2: the three layers stay separate. A claim result is not a coverage state
// and a coverage state is not an evidence class, so no value appears in two.
func TestTheThreeLayersShareNoValue(t *testing.T) {
	states := map[string]bool{}
	for _, s := range append(append(asStrings(Observations), asStrings(Completenesses)...),
		append(append(asStrings(Provenances), asStrings(PricingBases)...),
			append(asStrings(Calibrations), asStrings(Freshnesses)...)...)...) {
		states[s] = true
	}
	for _, class := range []Class{Measured, Reconstructed} {
		if states[string(class)] {
			t.Errorf("evidence class %q is also a coverage state; §2 keeps the layers apart", class)
		}
	}
	if states[NotMeasured] {
		t.Errorf("the claim result %q is also a coverage state", NotMeasured)
	}
}
