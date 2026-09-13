package main

import (
	"errors"
	"fmt"
	"testing"
)

// EC. A CI runner with no transcripts and a real breach exited the same way.
//
// `exitCode` mapped payment-required to 2 and everything else to 1. So
// `replay cost --max-avoidable-usd` returned 1 both when avoidable spend was
// over the ceiling and when it refused because the corpus priced nothing, and
// a shell could not tell "your agents wasted money" from "this runner has no
// data and I declined to bless it".
//
// Those are opposite situations. One is the finding the flag exists to
// produce. The other is the flag protecting the operator from a green tick
// nobody earned, and it usually means a path is wrong rather than that
// anything is expensive. Blocking a merge on the second is a false positive,
// and a false positive in somebody else's CI is the most expensive kind of
// defect this project can ship.
//
// It is shipped today, in a free capability, which is why this is fixed before
// the paid gate rather than with it. And it gets worse with entitlement: the
// natural code for "cannot evaluate" is 2, and 2 is taken.
//
// THE CONTRACT IS FROZEN HERE BECAUSE IT IS ABOUT TO BECOME ONE. The moment a
// gate runs in a stranger's pipeline, the exit code is a compatibility surface
// and cannot be changed without breaking builds. docs/ROADMAP.md names those
// surfaces for the version number to cover; this is the first one written down
// before it had users rather than after.

// EC1: over the ceiling and nothing measured are different codes.
//
// PASS: the breach code and the cannot-evaluate code differ.
// FAIL: a runner with no transcripts fails a merge exactly as a real breach
// does, and nobody can tell which happened without reading prose.
func TestEC1_ABreachAndAnEmptyCorpusAreDifferentCodes(t *testing.T) {
	breach := exitCode(fmt.Errorf("%w: $9.00 over $1.00", errGate))
	empty := exitCode(fmt.Errorf("refusing to pass a gate over 0 priced sessions: %w", errNotMeasured))

	if breach == empty {
		t.Errorf("both exited %d.\nA CI runner with no transcripts is indistinguishable from "+
			"agents that wasted money. One of those should block a merge and the other "+
			"should not.", breach)
	}
	if breach != exitGateBreached {
		t.Errorf("a ceiling breach exited %d, want %d", breach, exitGateBreached)
	}
	if empty != exitCannotEvaluate {
		t.Errorf("an unmeasurable corpus exited %d, want %d", empty, exitCannotEvaluate)
	}
}

// EC2: the codes are distinct from each other and from the shell's own.
//
// 0 is success and 2 was already spoken for by payment-required, which an
// agent uses to tell a priced resource from a broken URL. 126 and 127 belong
// to the shell, for "found but not executable" and "not found", and a tool
// that returns either is lying about what happened.
func TestEC2_TheCodesAreDistinctAndDoNotCollideWithTheShell(t *testing.T) {
	named := map[string]int{
		"usage":           exitUsage,
		"payment":         exitPaymentRequired,
		"gate breached":   exitGateBreached,
		"cannot evaluate": exitCannotEvaluate,
	}

	seen := map[int]string{}
	for name, code := range named {
		if code == 0 {
			t.Errorf("%s exits 0, which is success", name)
		}
		if code == 126 || code == 127 {
			t.Errorf("%s exits %d, which the shell uses for its own failures", name, code)
		}
		if prev, dup := seen[code]; dup {
			t.Errorf("%s and %s both exit %d, so a caller cannot tell them apart", name, prev, code)
		}
		seen[code] = name
	}
}

// EC3: only a breach may block a merge.
//
// This is the property the whole contract exists for, and it is stated as code
// rather than as a sentence in a guide. Everything that is not a measured
// finding is a reason to warn, and a tool that blocks on its own inability to
// measure has substituted its opinion for a measurement.
func TestEC3_OnlyAMeasuredBreachIsBlocking(t *testing.T) {
	for _, tc := range []struct {
		name     string
		code     int
		blocking bool
	}{
		{"gate breached", exitGateBreached, true},
		{"cannot evaluate", exitCannotEvaluate, false},
		{"payment required", exitPaymentRequired, false},
		{"usage error", exitUsage, false},
	} {
		if got := blocksAMerge(tc.code); got != tc.blocking {
			t.Errorf("%s: blocksAMerge=%v, want %v", tc.name, got, tc.blocking)
		}
	}
}

// EC4: an unrecognised error still fails, and fails as a usage error.
//
// A tool that exits 0 on an error it did not anticipate is worse than one that
// crashes, because it reports success. The default must be non-zero.
func TestEC4_AnUnknownFailureIsStillAFailure(t *testing.T) {
	if got := exitCode(errors.New("something nobody classified")); got == 0 {
		t.Error("an unclassified error exited 0, reporting success for a failure")
	}
}
