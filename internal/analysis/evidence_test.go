package analysis

import (
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A lane with nothing to compare has not passed calibration. It has not been
// calibrated.
//
// MatchRate returned 1 when Compared() was zero, and Passes() gates on
// MatchRate alone — so a lane with a single request, which offers no turn to
// check, scored a perfect 100% and was admitted to alternative scoring. On the
// real corpus that is 18 of 1450 lanes, every one of them passing on no
// evidence whatsoever.
//
// This is the shape ADR-0014 exists for: a check whose passing case includes
// "the check never ran". `learn` and `advise` then score policies partly on
// sessions where the engine's fidelity was never tested, and the corpus counts
// them as calibrated.

// oneRequestLane builds a lane offering no turn to compare: the first request
// is always ReadFirst, so a single-request lane has Compared() == 0.
func oneRequestLane() *transcript.Lane {
	return &transcript.Lane{Requests: []*transcript.Request{
		{ID: "r0", Usage: transcript.Usage{Input: 10, CacheCreation: 100}},
	}}
}

// E1: no evidence is not a pass.
//
// PASS: Passes() is false, and HasEvidence() says why.
// FAIL: admitted, which is how a policy gets scored against a session that
// never tested anything.
func TestE1_ALaneWithNothingComparedDoesNotPass(t *testing.T) {
	cal := Calibrate(oneRequestLane())
	if cal.Compared() != 0 {
		t.Fatalf("fixture compares %d turns; it must compare none or this test asserts nothing", cal.Compared())
	}
	if cal.HasEvidence() {
		t.Error("HasEvidence() is true with nothing compared")
	}
	if cal.Passes() {
		t.Error("a lane with no comparable turn passed calibration. Nothing was checked, so " +
			"there is no result to pass; alternatives scored on it rest on no evidence.")
	}
}

// E2: and it reports a rate the gate will refuse.
//
// This is the load-bearing one. Passes() has no separate evidence check —
// that conjunct was removed as dead code, because with the threshold a
// constant it never changed an answer. So the gate rests entirely on
// MatchRate being BELOW the threshold when nothing was compared. Asserting
// only "not 1" would leave 0.99 passing.
//
// PASS: the empty rate is below CalibrationThreshold.
// FAIL: anything at or above it, which readmits every uncalibrated lane to
// alternative scoring — 18 of 1450 on the real corpus.
func TestE2_NoEvidenceIsNotAPerfectScore(t *testing.T) {
	got := Calibrate(oneRequestLane()).MatchRate()
	if got == 1 {
		t.Error("MatchRate() = 1 with nothing compared. An absent measurement is reported as a " +
			"perfect one, so any threshold test on it passes for free.")
	}
	if got >= CalibrationThreshold {
		t.Errorf("MatchRate() = %.3f with nothing compared, at or above the %.2f threshold. "+
			"Passes() relies on this being below it, so the gate is open again.", got, CalibrationThreshold)
	}
}

// E3: the gate still admits a lane that earned it.
//
// Without this, returning false unconditionally would satisfy E1 and E2 and
// break the product silently.
//
// PASS: a lane whose turns reproduce passes.
// FAIL: the fix refuses everything.
func TestE3_ALaneWithEvidenceStillPasses(t *testing.T) {
	// Two requests: the second reads exactly the prefix the first wrote, which
	// is the reproduced case.
	lane := &transcript.Lane{Requests: []*transcript.Request{
		{ID: "r0", Usage: transcript.Usage{Input: 10, CacheCreation: 100}},
		{ID: "r1", Usage: transcript.Usage{Input: 2, CacheRead: 110, CacheCreation: 20}},
	}}
	cal := Calibrate(lane)
	if !cal.HasEvidence() {
		t.Fatalf("fixture compares %d turns; it must compare at least one", cal.Compared())
	}
	if !cal.Passes() {
		t.Errorf("a lane that reproduced its read was refused: %d/%d compared, rate %.3f",
			cal.Reproduced+cal.Exceeded, cal.Compared(), cal.MatchRate())
	}
}
