package learn

import "testing"

// A single-sample arm is not "separated", it is unmeasured.
//
// Found by mutation, not by reading: scripts' per-file sweep reported
// graduate.go:172 removable with this package green. Graduate's own callers
// never reach it, because the minSessions floor above rejects a one-session arm
// first — so the guard is live only if that floor is ever lowered or a second
// caller appears, which is exactly the case where a silent wrong answer is
// most likely.
//
// This test does NOT make graduate.go:172 distinguishable, and saying so is the
// point. meanInterval returns (-Inf, +Inf) for a single sample; separated
// divides that width to get a standard error, so without the guard seT is +Inf,
// the threshold is +Inf, and `diff > +Inf` is false. The guard and the
// arithmetic below it produce the same answer for every input. It is an
// EQUIVALENT MUTANT — defence in depth, not a gap — and no test can kill it
// without the code changing. Recorded here so the next sweep does not spend
// another hour rediscovering it.
//
// What this test does pin is the contract both of them rest on, which nothing
// else did. Measured: change meanInterval to return {0, 0} instead of
// (-Inf, +Inf) for n < 2, leaving graduate.go:172 exactly as it is, and the
// guard no longer sees an infinity, seT and seC are 0, the threshold is 0,
// and two arms one sample each are declared separated by a factor of a
// thousand. This test goes red on that. Before it, nothing did.
func TestSeparatedRefusesASingleSampleArm(t *testing.T) {
	// Treated far below control: if anything is ever going to look separated
	// on one sample each, it is this.
	if separated([]float64{1}, []float64{1000}) {
		t.Error("one sample per arm was reported as a separated result; a difference of " +
			"means with no estimate of its noise is not evidence")
	}
	// The same shape with enough samples to have an interval does separate, so
	// the test above is refusing for the right reason rather than because
	// separated refuses everything.
	if !separated([]float64{1, 1.1, 0.9, 1.05}, []float64{1000, 1010, 990, 1005}) {
		t.Error("four samples per arm, three orders of magnitude apart, were not " +
			"separated; the guard above is refusing everything")
	}
}
