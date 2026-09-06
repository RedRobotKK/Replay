package analysis

import "testing"

// The pre-flight policy must refuse only what the user consented to refuse.
//
// Five cases, and each one kills a specific mutation. That is the point of the
// table: a suite where one case carries the whole assertion passes on code
// where the other terms have been deleted, which is how the first version of
// this guard scored green with the ceiling hardcoded.
//
// PASS: every row's decision matches.
// FAIL: a term was dropped, an operator flipped, or the boundary moved.
func TestPredictDeficitBreaker(t *testing.T) {
	tests := []struct {
		name          string
		isDiverged    bool
		payloadTokens int64
		ceiling       int64
		optInActive   bool
		expectBlock   bool
		kills         string
	}{
		{
			name:          "context matches the established prefix, passes at any size",
			isDiverged:    false,
			payloadTokens: 140000,
			ceiling:       50000,
			optInActive:   true,
			expectBlock:   false,
			kills:         "dropping the isDiverged term",
		},
		{
			name:          "diverged under the ceiling, passes",
			isDiverged:    true,
			payloadTokens: 35000,
			ceiling:       50000,
			optInActive:   true,
			expectBlock:   false,
			kills:         "flipping > to <, or ignoring the ceiling",
		},
		{
			name:          "diverged over the ceiling with no opt-in, warns only",
			isDiverged:    true,
			payloadTokens: 145000,
			ceiling:       50000,
			optInActive:   false,
			expectBlock:   false,
			kills:         "deleting the opt-in gate",
		},
		{
			name:          "diverged over the ceiling with opt-in, refuses",
			isDiverged:    true,
			payloadTokens: 145000,
			ceiling:       50000,
			optInActive:   true,
			expectBlock:   true,
			kills:         "returning false unconditionally",
		},
		{
			name:          "exactly on the ceiling passes, the boundary is strictly greater",
			isDiverged:    true,
			payloadTokens: 50000,
			ceiling:       50000,
			optInActive:   true,
			expectBlock:   false,
			kills:         "widening > to >=",
		},
		// Every row above sets the ceiling to 50,000, so all of them pass on
		// code with that number written into the comparison. These two move
		// it in both directions. Found by mutation, not by review: M-E
		// survived the first version of this table.
		{
			name:          "a ceiling above the default refuses nothing at 60,000",
			isDiverged:    true,
			payloadTokens: 60000,
			ceiling:       100000,
			optInActive:   true,
			expectBlock:   false,
			kills:         "hardcoding the ceiling at 50000",
		},
		{
			name:          "a ceiling below the default refuses at 20,000",
			isDiverged:    true,
			payloadTokens: 20000,
			ceiling:       10000,
			optInActive:   true,
			expectBlock:   true,
			kills:         "hardcoding the ceiling at 50000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluatePreFlightPolicy(tt.isDiverged, tt.payloadTokens, tt.ceiling, tt.optInActive)
			if got != tt.expectBlock {
				t.Errorf("refuse = %v, want %v (diverged=%v tokens=%d ceiling=%d optIn=%v). "+
					"This case exists to catch %s.",
					got, tt.expectBlock, tt.isDiverged, tt.payloadTokens, tt.ceiling,
					tt.optInActive, tt.kills)
			}
		})
	}
}

// The zero PolicyState must never refuse.
//
// A user who has not set a ceiling has not consented to anything. If the zero
// value blocked, every session would start refusing diverged requests above
// zero tokens, which is every diverged request.
//
// PASS: nothing is refused under the zero value.
// FAIL: the default became a policy.
func TestPreFlightPolicy_ZeroValueNeverRefuses(t *testing.T) {
	var p PolicyState
	for _, tokens := range []int64{0, 1, 50000, 145000, 1 << 40} {
		d := NewPreFlightDeficit(tokens, true, true)
		if d.WouldRefuse(p) {
			t.Errorf("the zero policy refused a %d-token diverged request. A ceiling "+
				"nobody set must not refuse anybody's request", tokens)
		}
	}
}

// A refusal decided inside the estimate's own error band is decided on noise.
//
// The pre-flight figure is bytes through a fitted ratio, not a measurement.
// When the ceiling sits inside that band the tool cannot tell which side of it
// the request falls on, and refusing anyway reports precision it does not have.
//
// PASS: a ceiling inside the band straddles; one clear of it does not.
// FAIL: the band is ignored, which is the "1% tolerance five tokens wide"
// defect in a new place.
func TestPreFlightDeficit_StraddleIsDetected(t *testing.T) {
	d := NewPreFlightDeficit(100000, true, true) // band 85,000 to 115,000

	if d.Low != 85000 || d.High != 115000 {
		t.Fatalf("band = [%d,%d], want [85000,115000]; the fixture no longer has the "+
			"shape this test reasons about", d.Low, d.High)
	}

	straddling := PolicyState{CeilingTokens: 90000, OptInActive: true}
	if !d.Straddles(straddling) {
		t.Error("a ceiling of 90,000 sits inside the 85,000-115,000 band and must be " +
			"reported as undecidable, not as a clean refusal")
	}
	if !d.WouldRefuse(straddling) {
		t.Error("the point estimate is above 90,000, so the policy does refuse; " +
			"Straddles reports that the refusal is not supported, it does not veto it")
	}

	belowBand := PolicyState{CeilingTokens: 50000, OptInActive: true}
	if d.Straddles(belowBand) {
		t.Error("a ceiling of 50,000 is well below the band and the refusal is clean")
	}

	// Without opt-in there is no refusal to qualify.
	if d.Straddles(PolicyState{CeilingTokens: 90000}) {
		t.Error("straddling is meaningless when nothing would be refused")
	}
}

// A matching prefix must never straddle, however large it is.
//
// Straddling qualifies a refusal. A request that matches the established
// prefix is not refused at any size, so there is nothing to qualify, and
// reporting one would put an undecidable warning in front of a user whose
// request was always going to pass.
func TestPreFlightDeficit_MatchingPrefixNeverStraddles(t *testing.T) {
	d := NewPreFlightDeficit(100000, false, true)
	if d.Straddles(PolicyState{CeilingTokens: 90000, OptInActive: true}) {
		t.Error("a request that matches the established prefix is never refused, so it " +
			"cannot straddle a ceiling")
	}
}
