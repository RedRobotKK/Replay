package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleCalibration() Calibration {
	return Calibration{
		Schema:         CalibrationSchema,
		TakenAt:        "2026-09-10T05:00:00Z",
		RulesVersion:   "anthropic-2026-09-01",
		ClientVersions: []string{"2.1.257", "2.1.260"},
		Models: []ModelCalibrationRow{{
			Model: "claude-opus-5", Sessions: 74, Compared: 28400, Matched: 27800,
			RuleMinPrefix: 512, LargestUncached: 0, SmallestCached: 13745,
		}, {
			Model: "claude-opus-4-8", Sessions: 3, Compared: 1200, Matched: 1150,
			RuleMinPrefix: 1024, LargestUncached: 0, SmallestCached: 20457, Stale: true,
		}},
		BreakCauses: map[string]int{"cache expired (gap longer than the TTL)": 49},
		SourceTag:   "a3f19c02b7e4d581",
		TagBasis:    "local",
	}.Digested()
}

// The calibration report is what ADR-0007 specified as the unit of
// contribution, and it is not what shipped.
//
// "replay corpus emits session id prefixes, client version, request counts,
// match rates, fit parameters, prefix bounds and break causes... A contribution
// is that report and nothing else." What shipped instead was five money figures
// — ADR-0008's credibility claim. The moat data is computed on every run and
// discarded.
//
// One deliberate departure from ADR-0007: no session id prefixes. A session id
// is a linkable identifier that appears in other artifacts, and the aggregate
// answers every question the prefixes were there for. Rows are per model.
func TestCalibrationCarriesTheBoundsAndNothingIdentifying(t *testing.T) {
	c := sampleCalibration()
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{"claude-opus-5", "smallestCached", "ruleMinPrefix", "breakCauses"} {
		if !strings.Contains(s, want) {
			t.Errorf("the report must carry %q — it is the interval that one machine cannot "+
				"close and a hundred can:\n%s", want, s)
		}
	}
	// The whole basis for doing this at all.
	for _, forbidden := range []string{"sessionId", "session_id", "path", "project", "prompt", "content"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(forbidden)) {
			t.Errorf("the report carries %q, which is about the contributor rather than "+
				"about the provider:\n%s", forbidden, s)
		}
	}
}

// A report with no compared turns is not a calibration.
//
// Every refusal here is a case where pooling the submission would move a
// denominator without moving its numerator — the same failure Corpus.Validate
// refuses for tasks.
func TestCalibrationRefusesWhatCannotBePooled(t *testing.T) {
	base := sampleCalibration()

	empty := base
	empty.Models = nil
	err := empty.Digested().Validate()
	if err == nil {
		t.Fatal("a report with no model rows was accepted; there is nothing in it to pool")
	}
	// The message is asserted, not just the refusal. Without it the branch is
	// unfalsifiable: the compared==0 check below refuses an empty report too, so
	// deleting this case changes nothing observable — which ADR-0014 rules out.
	// Two states that a reader must tell apart deserve two sentences.
	if !strings.Contains(err.Error(), "no model rows") {
		t.Errorf("an empty report and a report that compared nothing must say different\n"+
			"things, or one of the two refusals is decoration. got: %v", err)
	}

	nothingCompared := base
	nothingCompared.Models = []ModelCalibrationRow{{Model: "m", Sessions: 3, Compared: 0}}
	if err := nothingCompared.Digested().Validate(); err == nil {
		t.Error("a report whose rows compared no turns was accepted: NOT MEASURED is not zero")
	}

	noRules := base
	noRules.RulesVersion = ""
	if err := noRules.Digested().Validate(); err == nil {
		t.Error("a report with no rules version was accepted; bounds measured against an " +
			"unnamed table cannot be pooled with bounds measured against another")
	}

	matchedOverCompared := base
	matchedOverCompared.Models = []ModelCalibrationRow{{Model: "m", Sessions: 1, Compared: 10, Matched: 11}}
	if err := matchedOverCompared.Digested().Validate(); err == nil {
		t.Error("matched exceeding compared describes a state that cannot occur and was accepted")
	}

	if err := base.Validate(); err != nil {
		t.Errorf("a well-formed report was refused: %v", err)
	}
}
