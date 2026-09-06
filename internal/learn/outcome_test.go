package learn

import (
	"strings"
	"testing"
)

// A trial that measures only cost is half a result.
//
// Graduate consumed exactly one number per session, CostPerNewToken, and
// nothing else. So a policy that cut spend by a fifth while doubling the
// agent's failed edits graduated, and the report said so approvingly. Every
// cost finding in this project has been judged that way.
//
// The signals were already computed and joined to nothing: analysis/errors.go
// classifies failed edits, anchor-not-found and repeated identical calls;
// rereads.go counts ReadsAfterClear and RepeatedAfterClear - an agent
// re-reading content it already had is direct evidence it did not use what it
// was given.
//
// Cheaper is not better if the work got worse, and until now this could not
// tell the difference.

func arm(a string, cost, errShare float64, rereads int) ArmCost {
	return ArmCost{SessionID: "s", Arm: a, CostPerNewToken: cost,
		ErrorShare: errShare, ReadsAfterClear: rereads}
}

func trial(treatedErr, controlErr float64) []ArmCost {
	var cs []ArmCost
	for i := 0; i < 6; i++ {
		cs = append(cs, arm("treated", 0.80, treatedErr, 1))
		cs = append(cs, arm("control", 1.00, controlErr, 1))
	}
	return cs
}

// OV-1: a policy that saves money and degrades the outcome does not graduate.
//
// PASS: refused, and the reason names the outcome rather than the cost.
// FAIL: graduated on a 20% saving while the error share doubled - which is
// what shipped, and what makes a cost tool dangerous rather than merely
// incomplete.
func TestOV1_CheaperButWorseDoesNotGraduate(t *testing.T) {
	r := Graduate("p", trial(0.20, 0.10), 0.20, 5)
	if r == nil {
		t.Fatal("a populated trial must produce a report")
	}
	if r.Graduated {
		t.Errorf("graduated a policy that doubled the error share: %+v", r)
	}
	if !strings.Contains(strings.ToLower(r.Reason), "error") {
		t.Errorf("the refusal must name the outcome, not just the cost: %q", r.Reason)
	}
}

// OV-2: a policy that saves money without harming the outcome still graduates.
//
// The guard must not simply block everything. If it cannot pass a clean win it
// is not a guard, it is an off switch.
func TestOV2_CheaperAndNoWorseStillGraduates(t *testing.T) {
	r := Graduate("p", trial(0.10, 0.10), 0.20, 5)
	if r == nil || !r.Graduated {
		t.Errorf("a 20%% saving with an unchanged error share must graduate: %+v", r)
	}
}

// OV-3: no outcome data is reported as unknown, never as fine.
//
// A trial whose sessions carry no error signal must not be judged as though
// the outcome were measured and good. UNKNOWN is a label here, not a default -
// the same rule the classifiers hold.
//
// PASS: it graduates on cost but the report says the outcome was not observed.
// FAIL: silence, which reads as "the outcome was fine".
func TestOV3_NoOutcomeDataIsSaidOutLoud(t *testing.T) {
	var cs []ArmCost
	for i := 0; i < 6; i++ {
		cs = append(cs, ArmCost{SessionID: "s", Arm: "treated", CostPerNewToken: 0.80})
		cs = append(cs, ArmCost{SessionID: "s", Arm: "control", CostPerNewToken: 1.00})
	}
	r := Graduate("p", cs, 0.20, 5)
	if r == nil {
		t.Fatal("expected a report")
	}
	if r.OutcomeObserved {
		t.Error("no session carried an outcome signal, so none was observed")
	}
	if !strings.Contains(strings.ToLower(r.Reason), "outcome") {
		t.Errorf("the report must say the outcome was not observed: %q", r.Reason)
	}
}

// OV-4: re-reads count as a degradation too, not only the error share.
//
// An agent re-reading content it already held is reasoning-action mismatch in
// the field's vocabulary, and it is the second signal already computed and
// never used.
func TestOV4_MoreRereadsIsADegradation(t *testing.T) {
	var cs []ArmCost
	for i := 0; i < 6; i++ {
		cs = append(cs, arm("treated", 0.80, 0.10, 8))
		cs = append(cs, arm("control", 1.00, 0.10, 1))
	}
	r := Graduate("p", cs, 0.20, 5)
	if r.Graduated {
		t.Errorf("graduated a policy that multiplied re-reads eightfold: %+v", r)
	}
}

// OV-5: the wiring. A guard that is never fed is decoration.
//
// The construction site built ArmCost from CostPerNewToken alone, so the
// outcome fields would have stayed zero on every real run and the guard could
// never fire. This project has shipped that exact shape before - a payment
// gate no test imported, where deleting it left 25 of 25 green.
//
// PASS: a session score carrying an outcome reaches ArmCost with it intact.
// FAIL: zeroes, which is the guard existing and never running.
func TestOV5_ScoresCarryTheOutcomeIntoArmCost(t *testing.T) {
	got := armCostsOf([]SessionScore{
		{SessionID: "a", Arm: "treated", CostPerNewToken: 0.8, ErrorShare: 0.25, ReadsAfterClear: 4},
	})
	if len(got) != 1 {
		t.Fatalf("expected one ArmCost, got %d", len(got))
	}
	if got[0].ErrorShare != 0.25 {
		t.Errorf("ErrorShare did not reach ArmCost: %+v", got[0])
	}
	if got[0].ReadsAfterClear != 4 {
		t.Errorf("ReadsAfterClear did not reach ArmCost: %+v", got[0])
	}
}
