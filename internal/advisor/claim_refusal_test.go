package advisor

import "testing"

// RPL-C015. The claim under attack is that Replay can establish that acting
// on its advice changed anything.
//
// It cannot, for the two kinds that matter most, and the refusal is in the
// product rather than in a disclaimer. This test pins the refusal so that a
// later change which quietly starts scoring those kinds has to argue with a
// test instead of slipping through.
//
// The history matters: inferring "applied" from the same share drop being
// measured produced 20 NOT_VERIFIED against 1 VERIFIED on real data, an
// artifact of the measurement rather than evidence the advice failed. The
// refusal is the fix for that circularity.
func TestC015_TheTwoLargestKindsAreRefusedScoring(t *testing.T) {
	// A share series that falls hard. For any kind that IS scored, this is
	// the shape that would read as a successful application.
	fell := []sample{
		{share: 0.40, seen: true},
		{share: 0.38, seen: true},
		{share: 0.05, seen: true},
		{share: 0.02, seen: true},
	}

	refused := []Kind{KindCacheBreaks, KindHotFile}
	for _, k := range refused {
		t.Run(string(k), func(t *testing.T) {
			status, realized := track(k, fell, 12.50, true)
			if status != AdviceOnly {
				t.Errorf("kind %s returned status %s on a falling share. It must be "+
					"%s: a target that comes and goes with the work cannot be "+
					"attributed to a change the reader made.", k, status, AdviceOnly)
			}
			if realized != 0 {
				t.Errorf("kind %s reported a realized figure of %v while refusing to "+
					"score. A refusal that still emits a number is not a refusal.", k, realized)
			}
		})
	}

	// NEGATIVE CONTROL. If track refused everything the refusal would carry
	// no information. At least one kind must be scoreable on the same input,
	// or this test is asserting that the advisor does nothing.
	var scored int
	for _, k := range []Kind{KindToolInputs, KindLargeResults, KindFirstTurn, KindUnusedTools} {
		if status, _ := track(k, fell, 12.50, true); status != AdviceOnly {
			scored++
		}
	}
	if scored == 0 {
		t.Fatal("no kind at all is scoreable on a falling share. The AdviceOnly " +
			"refusal above is then vacuous, because the verifier scores nothing.")
	}
	t.Logf("%d of 4 remaining kinds are scoreable on the same input, so the refusal "+
		"for the two largest is a decision rather than an absence of capability", scored)
}

// The reader's own decision must stay distinguishable from the verifier's
// inference. Collapsing them is how a tool starts reporting that a user did
// something the user never did.
func TestC015_VerifierInferenceIsNotAReaderDecision(t *testing.T) {
	// Status and Decision are separate types for this reason. If a future
	// change makes an inferred status imply a decision, this fails.
	if string(AdviceOnly) == string(DecisionApplied) {
		t.Fatal("the verifier's status vocabulary and the reader's decision " +
			"vocabulary share a value; an inference can now be read as a decision")
	}
}
