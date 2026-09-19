package coverage

import (
	"reflect"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The six dimensions are closed (§3). These lists are written from the
// contract text rather than from the package, so a state added to the package
// without amending the contract fails here rather than passing quietly.
var (
	contractObservations  = []string{"lane-serial", "lane-overlap", "unmeasured"}
	contractCompleteness  = []string{"complete", "partial-tokens", "partial-pricing", "partial-parse"}
	contractProvenance    = []string{"proxy-recorded", "transcript-derived"}
	contractPricingBasis  = []string{"documented", "declared", "absent"}
	contractCalibration   = []string{"passing", "failing", "drifted", "no-evidence"}
	contractFreshness     = []string{"not-verified"}
	contractDimensionName = []string{
		"Observation", "Completeness", "Provenance", "PricingBasis", "Calibration", "Freshness",
	}
)

// CV-1: six named dimensions, and exactly six.
//
// The v6 draft row names them "population, provenance, freshness, surface,
// pricing basis, reproduction fidelity". §3 is LOCKED and CLOSED and names
// them observation, completeness, provenance, pricing_basis, calibration,
// freshness. §3 governs; the draft row's spelling is not implemented here and
// is not changed by this package.
func TestTheVectorIsExactlyTheSixLockedDimensions(t *testing.T) {
	typ := reflect.TypeOf(Vector{})
	if typ.NumField() != 6 {
		t.Fatalf("the vector carries %d fields; §3 closes it at six", typ.NumField())
	}
	for i, want := range contractDimensionName {
		if got := typ.Field(i).Name; got != want {
			t.Errorf("dimension %d is %q, want %q in §3 order", i, got, want)
		}
	}
}

// CV-3: dimensions are never combined into a score, a grade or a percentage.
//
// Checked structurally rather than by reading the code: no field of the vector
// is numeric, so there is nothing to average, and no exported function returns
// a number derived from more than one dimension.
func TestNoDimensionIsANumberAndNothingCombinesThem(t *testing.T) {
	typ := reflect.TypeOf(Vector{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		switch f.Type.Kind() {
		case reflect.Int, reflect.Int64, reflect.Float64, reflect.Float32, reflect.Uint:
			t.Errorf("dimension %s is numeric (%s); a numeric dimension is a score waiting "+
				"to happen, and §5 removed scalar confidence", f.Name, f.Type)
		case reflect.String, reflect.Slice:
			// A string state, or the §3.2 array form.
		default:
			t.Errorf("dimension %s has kind %s, which is neither a state nor the §3.2 array form",
				f.Name, f.Type.Kind())
		}
	}
}

// §3: every vocabulary is closed. No dimension may gain a state without
// amending the contract.
func TestTheVocabulariesAreExactlyTheLockedSets(t *testing.T) {
	got := map[string][]string{
		"observation":   asStrings(Observations),
		"completeness":  asStrings(Completenesses),
		"provenance":    asStrings(Provenances),
		"pricing_basis": asStrings(PricingBases),
		"calibration":   asStrings(Calibrations),
		"freshness":     asStrings(Freshnesses),
	}
	want := map[string][]string{
		"observation":   contractObservations,
		"completeness":  contractCompleteness,
		"provenance":    contractProvenance,
		"pricing_basis": contractPricingBasis,
		"calibration":   contractCalibration,
		"freshness":     contractFreshness,
	}
	for dim, w := range want {
		if !reflect.DeepEqual(got[dim], w) {
			t.Errorf("%s vocabulary is %v, want %v", dim, got[dim], w)
		}
	}
}

// §3.6: freshness has exactly one state. No symmetric state is manufactured.
func TestFreshnessHasOneStateAndNoSymmetricPartner(t *testing.T) {
	if len(Freshnesses) != 1 || Freshnesses[0] != NotVerified {
		t.Fatalf("freshness is %v; §3.6 defines exactly not-verified", Freshnesses)
	}
	for _, f := range Freshnesses {
		if f == "verified" || f == "stale" {
			t.Errorf("freshness carries %q; §3.6 says verified and stale are not defined", f)
		}
	}
}

// §4.7: the vector describes evidence and is always emitable. Every dimension
// resolves to exactly one state, on any input, including nothing at all.
func TestEveryDimensionResolvesOnAnyInputIncludingNothing(t *testing.T) {
	cases := []struct {
		name string
		ev   Evidence
	}{
		{"zero evidence", Evidence{}},
		{"empty session", Evidence{Session: &transcript.Session{}}},
		{"empty lane", Evidence{Lane: &transcript.Lane{}}},
		{"unparsed session", Evidence{Session: &transcript.Session{Skipped: 3}}},
		{"calibration with no evidence", Evidence{Calibration: &analysis.Calibration{}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := Of(c.ev)
			if !in(asStrings(Observations), string(v.Observation)) {
				t.Errorf("observation %q is not a locked state", v.Observation)
			}
			if len(v.Completeness) == 0 {
				t.Error("completeness resolved to no state; §4.7 says every dimension resolves")
			}
			for _, c := range v.Completeness {
				if !in(contractCompleteness, string(c)) {
					t.Errorf("completeness %q is not a locked state", c)
				}
			}
			if !in(contractProvenance, string(v.Provenance)) {
				t.Errorf("provenance %q is not a locked state", v.Provenance)
			}
			if !in(contractPricingBasis, string(v.PricingBasis)) {
				t.Errorf("pricing_basis %q is not a locked state", v.PricingBasis)
			}
			if !in(contractCalibration, string(v.Calibration)) {
				t.Errorf("calibration %q is not a locked state", v.Calibration)
			}
			if v.Freshness != NotVerified {
				t.Errorf("freshness %q; §3.6 has one state", v.Freshness)
			}
		})
	}
}

// §3.2: `complete` never appears in an array, and the array form is emitted
// only where more than one partial condition holds.
func TestCompleteNeverAppearsBesideAPartialState(t *testing.T) {
	v := Of(Evidence{
		Session:  &transcript.Session{Skipped: 1},
		Fit:      analysis.TokenFit{Injected: analysis.Estimated(10)},
		Unpriced: 1,
	})
	for _, c := range v.Completeness {
		if c == Complete {
			t.Fatalf("complete appears beside partial states: %v", v.Completeness)
		}
	}
	if len(v.Completeness) != 3 {
		t.Errorf("three partial conditions hold; completeness is %v", v.Completeness)
	}
	if got, want := v.CompletenessString(),
		`["partial-tokens","partial-pricing","partial-parse"]`; got != want {
		t.Errorf("array form is %s, want %s", got, want)
	}
	clean := Of(Evidence{Session: &transcript.Session{}, Priced: 1})
	if got := clean.CompletenessString(); got != "complete" {
		t.Errorf("a complete vector serialises as %s, want the bare string complete", got)
	}
}

// §3.1: lane-serial means no other request of the lane was open during this
// one. A lane carrying one overlapped request is not serial, and a lane whose
// requests recorded nothing is unmeasured rather than serial.
func TestObservationIsReadFromCorrelationAndNeverAssumedSerial(t *testing.T) {
	req := func(c string) *transcript.Request { return &transcript.Request{Correlation: c} }
	cases := []struct {
		name string
		reqs []*transcript.Request
		want Observation
	}{
		{"all serial", []*transcript.Request{req("lane-serial"), req("lane-serial")}, LaneSerial},
		{"one overlapped", []*transcript.Request{req("lane-serial"), req("lane-overlap")}, LaneOverlap},
		{"nothing recorded", []*transcript.Request{req(""), req("")}, ObservationUnmeasured},
		{"serial and unrecorded", []*transcript.Request{req("lane-serial"), req("")}, ObservationUnmeasured},
		{"no requests", nil, ObservationUnmeasured},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Of(Evidence{Lane: &transcript.Lane{Requests: c.reqs}}).Observation
			if got != c.want {
				t.Errorf("observation %q, want %q", got, c.want)
			}
		})
	}
}

// §3.3 and §2: provenance is read off Source.Tier(), and transcript-derived is
// what the repository calls the estimated tier. The existing wording is
// preserved; the vector carries the machine field beside it.
func TestProvenanceIsTheExistingTierAndNotANewLabel(t *testing.T) {
	ledger := Of(Evidence{Session: &transcript.Session{Source: transcript.SourceLedger}})
	if ledger.Provenance != ProxyRecorded {
		t.Errorf("a ledger session is %q, want proxy-recorded", ledger.Provenance)
	}
	if tier := transcript.SourceLedger.Tier(); tier != "measured (proxy-recorded)" {
		t.Errorf("the existing tier wording moved to %q; the vector must not change output", tier)
	}
	file := Of(Evidence{Session: &transcript.Session{}})
	if file.Provenance != TranscriptDerived {
		t.Errorf("a transcript session is %q, want transcript-derived", file.Provenance)
	}
}

// §3.4: absent when no rules document resolved for a contributing model;
// declared when an account discount was stated; documented otherwise.
func TestPricingBasisSeparatesAbsentFromDeclaredFromDocumented(t *testing.T) {
	cases := []struct {
		name             string
		priced, unpriced int
		tier             string
		want             PricingBasis
	}{
		{"nothing priced", 0, 2, "documented", PricingAbsent},
		{"nothing read at all", 0, 0, "documented", PricingAbsent},
		{"all priced, published rates", 3, 0, "documented", Documented},
		{"all priced, account discount", 3, 0, "declared", Declared},
		{"mixed, published rates", 2, 1, "documented", Documented},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Of(Evidence{Priced: c.priced, Unpriced: c.unpriced, PriceTier: c.tier}).PricingBasis
			if got != c.want {
				t.Errorf("pricing_basis %q, want %q", got, c.want)
			}
		})
	}
}

// §3.2: a model absent from the rules on any contributing request is
// partial-pricing, which is a completeness fact and not a pricing_basis fact.
// The two dimensions are independent and a mixed lane shows it.
func TestAMixedLaneIsPartialPricingWithADocumentedBasis(t *testing.T) {
	v := Of(Evidence{Priced: 2, Unpriced: 1, PriceTier: "documented"})
	if !v.Has(PartialPricing) {
		t.Errorf("a lane with one unpriced model is not partial-pricing: %v", v.Completeness)
	}
	if v.PricingBasis != Documented {
		t.Errorf("pricing_basis is %q; the rules that did resolve are still documented", v.PricingBasis)
	}
}

// §3.5: passing, failing, drifted and no-evidence are read from the existing
// calibration and the existing drift pass. Drift is a per-model fact about the
// provider, and a lane that calibrates on its own is still drifted.
func TestCalibrationCarriesTheFourStatesIncludingDriftOverAPassingLane(t *testing.T) {
	passing := &analysis.Calibration{Reproduced: 20}
	failing := &analysis.Calibration{Reproduced: 1, Broken: 19}
	cases := []struct {
		name    string
		cal     *analysis.Calibration
		drifted bool
		want    Calibration
	}{
		{"no calibration at all", nil, false, CalibrationNoEvidence},
		{"nothing compared", &analysis.Calibration{}, false, CalibrationNoEvidence},
		{"below threshold", failing, false, CalibrationFailing},
		{"above threshold", passing, false, CalibrationPassing},
		{"passing but the model drifted", passing, true, CalibrationDrifted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Of(Evidence{Calibration: c.cal, Drifted: c.drifted}).Calibration
			if got != c.want {
				t.Errorf("calibration %q, want %q", got, c.want)
			}
		})
	}
}

// §3.5 and §3.6: ModelStaleness drift belongs to calibration, and is
// deliberately not mapped to freshness.
func TestDriftNeverMovesFreshness(t *testing.T) {
	v := Of(Evidence{
		Calibration: &analysis.Calibration{Reproduced: 20},
		Drifted:     true,
	})
	if v.Freshness != NotVerified {
		t.Errorf("drift moved freshness to %q; §3.6 says ModelStaleness.Stale is not mapped here", v.Freshness)
	}
	if v.Calibration != CalibrationDrifted {
		t.Errorf("drift did not land on calibration: %q", v.Calibration)
	}
}

// §3.2: partial-tokens is at least one contributing count from the
// byte-to-token fit rather than provider usage.
func TestPartialTokensIsReadFromTheFitAndNotFromTheTier(t *testing.T) {
	fitted := Of(Evidence{Fit: analysis.TokenFit{UnseenPrefix: analysis.Estimated(400)}})
	if !fitted.Has(PartialTokens) {
		t.Errorf("an estimated unseen prefix is not partial-tokens: %v", fitted.Completeness)
	}
	measuredPrefix := Of(Evidence{
		Session: &transcript.Session{Source: transcript.SourceLedger},
		Fit:     analysis.TokenFit{UnseenPrefix: analysis.Measured(400)},
		Priced:  1,
	})
	if measuredPrefix.Has(PartialTokens) {
		t.Errorf("a measured prefix was called partial-tokens: %v", measuredPrefix.Completeness)
	}
}

// Determinism is excluded from the vector (§3, "Excluded from the vector").
// It is a property of the build, so the same evidence yields the same vector.
func TestTheSameEvidenceYieldsTheSameVector(t *testing.T) {
	ev := Evidence{
		Lane: &transcript.Lane{Requests: []*transcript.Request{
			{Correlation: "lane-serial", Timestamp: time.Unix(0, 0)},
		}},
		Session:     &transcript.Session{Source: transcript.SourceLedger},
		Calibration: &analysis.Calibration{Reproduced: 10},
		Priced:      1,
		PriceTier:   "documented",
	}
	first := Of(ev)
	for i := 0; i < 50; i++ {
		if got := Of(ev); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d produced %+v, first run produced %+v", i, got, first)
		}
	}
}

func asStrings[T ~string](in []T) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, string(v))
	}
	return out
}

func in(set []string, s string) bool {
	for _, v := range set {
		if v == s {
			return true
		}
	}
	return false
}
