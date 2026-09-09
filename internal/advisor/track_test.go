package advisor

import "testing"

// track() infers that advice was applied from the number it exists to measure.
//
// The circularity is the defect. `track` never learns that anybody did
// anything. It splits a target's per-session shares into earlier and the last
// two, and if the recent mean fell by appliedDrop it treats that fall AS the
// application — then measures the same fall to decide whether the prediction
// held.
//
// So "not verified" does not mean the advice failed. It means a share drifted
// down by more than 20% and less than half the prediction, for reasons nobody
// recorded. On this machine that is 20 of 21 suggestions with a measured
// outcome, and the one that verified is indistinguishable from the rest except
// that its drift was larger.
//
// A corpus that grows while the work changes produces exactly this signal with
// nobody having applied anything, which is the confound. The status is reported
// to a reader as though somebody had.
//
// These tests pin the behaviour that separates the two. Until a real
// application signal exists, a drop that nobody caused must not be reported as
// applied-and-failed.

// TK1: drift with nobody applying anything must not be reported as verified.
//
// The false positive, and the worse of the two. A target sitting at 30% that
// drifts to 22% over two sessions of different work returns **verified** with a
// realized saving of 8 points. Nobody touched anything. The tool tells the
// reader their change was made and confirmed.
func TestTK1_DriftIsNotAVerifiedSaving(t *testing.T) {
	shares := []float64{0.30, 0.30, 0.30, 0.30, 0.22, 0.22}
	status, realized := track(KindLargeResults, shares, 0.15, false)
	if status == Verified {
		t.Errorf("a drift from 30%% to 22%% with no recorded application is reported "+
			"as %q with a realized saving of %.1f points. Nobody applied anything; "+
			"the number moved and the tool called it a confirmed win.",
			status, realized*100)
	}
}

// TK1b: noise with nobody applying anything must not be reported as a failure.
//
// The mirror, and the one that produced the headline. Earlier sessions of
// 20/40/30/30 average 30; two recent at 23 read as a 7-point fall, short of
// half the prediction, so the tool reports **not verified** — the reader is
// told a change they never made did not work.
//
// Together these explain the corpus: 20 not-verified against 1 verified is not
// evidence that the advice fails. It is evidence that the verifier is measuring
// corpus drift and cannot tell which direction it is being fooled in.
func TestTK1b_NoiseIsNotAFailedPrediction(t *testing.T) {
	shares := []float64{0.20, 0.40, 0.30, 0.30, 0.23, 0.23}
	status, _ := track(KindLargeResults, shares, 0.15, false)
	if status == NotVerified {
		t.Errorf("noise across sessions is reported as %q, telling the reader a change "+
			"they never made did not work", status)
	}
}

// TK1c: with an application actually recorded, a real drop still verifies.
//
// The fix must not make the tool silent. `replay tui` lets a reader mark a
// finding applied, and that keystroke is a recorded fact rather than an
// inference. When it exists, the comparison it was always trying to make
// becomes legitimate.
func TestTK1c_ARecordedApplicationStillVerifies(t *testing.T) {
	shares := []float64{0.30, 0.30, 0.30, 0.30, 0.05, 0.05}
	status, realized := track(KindLargeResults, shares, 0.15, true)
	if status != Verified {
		t.Errorf("a recorded application followed by a 25-point drop is %q, want "+
			"verified; the fix has made the tool unable to confirm anything", status)
	}
	if realized <= 0 {
		t.Errorf("verified with no realized figure (%.3f)", realized)
	}
}

// TK2: an unchanged share is Pending, not a verdict.
//
// The control. If nothing moved, there is nothing to say, and the existing
// code gets this right — the test exists so the fix cannot break it.
func TestTK2_NoMovementIsPending(t *testing.T) {
	shares := []float64{0.30, 0.30, 0.30, 0.30, 0.30, 0.30}
	if status, _ := track(KindLargeResults, shares, 0.15, false); status != Pending {
		t.Errorf("an unmoved share is %q, want pending", status)
	}
}

// TK3: too few sessions cannot support a verdict.
//
// Also already correct, and also worth pinning: two sessions is not a
// before-and-after, it is two numbers.
func TestTK3_TooFewSessionsIsPending(t *testing.T) {
	if status, _ := track(KindLargeResults, []float64{0.3, 0.1}, 0.15, false); status != Pending {
		t.Errorf("two sessions produced %q rather than pending", status)
	}
}

// TK4: a kind whose application cannot be detected says so, and always has.
//
// 117 of 140 suggestions are AdviceOnly. That is not a defect — it is the
// honest answer for a target that comes and goes with the work rather than
// with a change the reader made. It is pinned here because the fix below must
// not quietly widen it to cover the cases it cannot measure either.
func TestTK4_UndetectableKindsStayAdviceOnly(t *testing.T) {
	for _, k := range []Kind{KindHotFile, KindCacheBreaks} {
		status, realized := track(k, []float64{0.3, 0.3, 0.3, 0.1, 0.1}, 0.15, false)
		if status != AdviceOnly || realized != 0 {
			t.Errorf("%s: got %q/%.2f, want advice only and no realized figure",
				k, status, realized)
		}
	}
}
