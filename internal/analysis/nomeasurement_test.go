package analysis

import "testing"

// An absent measurement must not read as a good one.
//
// calibrate.go states this and enforces it: Calibration.MatchRate returned 1
// until 2026-09-06, which let 18 of 1450 lanes score a perfect 100% for having
// no turn to check, and every one of them was admitted to alternative scoring.
//
// The fix was applied to Calibration and not to ModelCalibration, forty lines
// away in staleness.go, which reaches the same zero through the shared rate()
// helper. `replay corpus` published the result: a `<synthetic>` row, six
// sessions, 100.0% match rate, verdict "calibrated". `<synthetic>` is Claude
// Code's label for assistant messages generated locally that never reached the
// API; every one carries input, output, cache-creation and cache-read tokens of
// zero, so nothing was ever compared. Two lines below the table, the same
// command said "no evidence" about the same model.
//
// These tests pin both implementations, because fixing one and leaving the
// other is the failure this class keeps reproducing.

// NM-1: a model with nothing compared does not report a passing match rate.
func TestNM1_NothingComparedIsNotAPerfectModel(t *testing.T) {
	m := ModelCalibration{Model: "<synthetic>", Sessions: 6}
	if got := m.MatchRate(); got >= CalibrationThreshold {
		t.Fatalf("a model with %d compared turns reported a match rate of %.3f, at or above the %.2f threshold; an absent measurement must not read as a good one", m.Compared, got, CalibrationThreshold)
	}
}

// NM-2: the recent window has the same duty. A model can carry earlier
// evidence and have nothing in the recent window, which is exactly when a
// forgiving default is read as "still calibrated".
func TestNM2_NothingComparedRecentlyIsNotAPerfectWindow(t *testing.T) {
	m := ModelCalibration{Model: "m", Sessions: 9, Compared: 400, Matched: 396}
	if got := m.RecentMatchRate(); got >= CalibrationThreshold {
		t.Fatalf("a model with %d recent compared turns reported a recent match rate of %.3f, at or above the %.2f threshold", m.RecentCompared, got, CalibrationThreshold)
	}
}

// NM-3: the two implementations agree at zero.
//
// This is the guard the class actually needs. Both types expose a method named
// MatchRate over the same idea, and the defect was that they disagreed about
// what "nothing was compared" means. Asserting each one separately would still
// pass if a later change fixed one and regressed the other.
func TestNM3_BothMatchRatesAgreeThatNothingIsNotEverything(t *testing.T) {
	lane := (&Calibration{}).MatchRate()
	model := ModelCalibration{}.MatchRate()
	if lane != model {
		t.Fatalf("Calibration.MatchRate reports %.3f with no evidence and ModelCalibration.MatchRate reports %.3f; the two must agree about an empty comparison", lane, model)
	}
	if model >= CalibrationThreshold {
		t.Fatalf("both report %.3f for an empty comparison, at or above the %.2f threshold", model, CalibrationThreshold)
	}
}
