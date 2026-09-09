package advisor

import (
	"strings"
	"testing"
	"time"
)

// An absent session is not a zero.
//
// aggregation appended ob.targets[k].share for every observation, and a Go map
// returns the zero value for a key it does not hold. So a session that never
// exercised the target contributed a hard 0.0 to the mean, indistinguishable
// from a session where the target was measured at nothing.
//
// Two different facts were collapsed into one number, and the arithmetic
// rewarded the wrong one. Measured on the code before this change:
//
//	steady at 30%, simply absent from the 2 newest -> verified, realized 0.300
//	genuinely reduced from 30% to 15%              -> verified, realized 0.150
//
// Not using a tool for two sessions produced TWICE the credited saving of
// actually halving it. The strongest possible success this screen can report
// was a frontend day.
//
// There is a second path into the same hole. note() drops a target whose share
// is under MinShare (0.10), so a target genuinely sitting at 0.09 is absent
// from the map and scored as 0.000. The true drop from 0.30 is 0.210; the
// reported one was 0.300, overstated by 43%.
//
// The fix is to record whether a session saw the target at all, and to average
// only over the sessions that did. Absence stops being evidence of reduction,
// which it never was: a session that did not use a tool says nothing about
// what that tool costs when it is used.
//
// It also disposes of the below-threshold case for free. The aggregation
// cannot tell 0.09 from never-used — note() discarded that distinction before
// it arrived — so the honest reading of an absence is "no measurement", and an
// unknown excluded from a mean cannot corrupt it.

// MN1: a target that is merely absent from the newest sessions is not credited
// with a saving.
func TestMN1_AbsenceIsNotAReduction(t *testing.T) {
	steady := []sample{
		{0.30, true}, {0.30, true}, {0.30, true},
		{0.30, true}, {0.30, true}, {0.30, true},
		{0, false}, {0, false},
	}
	st, realized := track(KindUnusedTools, steady, 0.15, true)
	if realized > 0 {
		t.Errorf("a target whose share never moved when it was used is credited with a "+
			"realized saving of %.3f, because it went unused in the two newest sessions. "+
			"Status %s.", realized, st)
	}
	if st == Verified {
		t.Error("absence from the newest sessions verified a saving nobody made")
	}
}

// MN2: a real reduction is still measured, and is not weakened by the fix.
//
// The guard against overcorrecting. A change that made MN1 pass by refusing to
// verify anything would satisfy MN1 and destroy the feature.
func TestMN2_ARealReductionStillVerifies(t *testing.T) {
	reduced := []sample{
		{0.30, true}, {0.30, true}, {0.30, true},
		{0.30, true}, {0.30, true}, {0.30, true},
		{0.15, true}, {0.15, true},
	}
	st, realized := track(KindUnusedTools, reduced, 0.15, true)
	if st != Verified {
		t.Errorf("a genuine halving from 30%% to 15%% no longer verifies: %s, realized %.3f",
			st, realized)
	}
	if realized < 0.14 || realized > 0.16 {
		t.Errorf("the realized saving is %.3f; the share fell by 0.150", realized)
	}
}

// MN3: absence is never worth more than a real reduction.
//
// The property underneath MN1, stated so it holds for any shape rather than
// the one case. Whatever the arithmetic ends up being, going unused must not
// out-score actually cutting the target in half.
func TestMN3_AbsenceNeverOutscoresAReduction(t *testing.T) {
	absent := []sample{
		{0.30, true}, {0.30, true}, {0.30, true}, {0.30, true},
		{0, false}, {0, false},
	}
	halved := []sample{
		{0.30, true}, {0.30, true}, {0.30, true}, {0.30, true},
		{0.15, true}, {0.15, true},
	}
	_, ra := track(KindUnusedTools, absent, 0.15, true)
	_, rh := track(KindUnusedTools, halved, 0.15, true)
	if ra > rh {
		t.Errorf("going unused scores %.3f and halving the share scores %.3f; the screen "+
			"rewards not running the tool over acting on the advice", ra, rh)
	}
}

// MN4: with no recent observation, nothing is claimed.
//
// The status when every recent session is absent. There is no measurement of
// the recent share, so Pending is the only honest answer — the suggestion
// stands and nothing is asserted about it, which is exactly what Pending means
// everywhere else in this file.
func TestMN4_NoRecentObservationIsPending(t *testing.T) {
	none := []sample{
		{0.30, true}, {0.30, true}, {0.30, true}, {0.30, true},
		{0, false}, {0, false},
	}
	st, realized := track(KindUnusedTools, none, 0.15, true)
	if st != Pending {
		t.Errorf("with no recent measurement the status is %s, not pending", st)
	}
	if realized != 0 {
		t.Errorf("with no recent measurement a saving of %.3f is reported", realized)
	}
}

// seen turns measured shares into samples, all present.
//
// Every share in track_test.go is a real reading — 0.05, 0.22, 0.30 — and none
// of them was ever standing in for an absent session. Wrapping them keeps that
// intent exact after the type changed: those tests are about what track() does
// with numbers it has, and this file is about what it does with numbers it
// does not.
func seen(xs ...float64) []sample {
	out := make([]sample, len(xs))
	for i, x := range xs {
		out[i] = sample{share: x, seen: true}
	}
	return out
}

// MN5: the aggregation records absence, end to end.
//
// MN1-MN4 hand samples to track() directly, so they say nothing about the code
// that BUILDS those samples. Verified by mutation: putting the aggregation back
// to
//
//	a.shares = append(a.shares, sample{share: ob.targets[k].share, seen: true})
//
// left MN1-MN4 green, because none of them goes through Suggest. A fix in two
// halves needs a test that crosses the join, or half of it is unguarded — the
// same shape as every finding in the vacuous-test audit.
//
// So this drives the real path. Six sessions: four where five tools are defined
// and never called, then two where only the tool that IS called is defined, so
// the unused-tools target does not exist at all. The reader has marked the
// advice applied, which is what lets track() past its first gate.
//
// Absent from the two newest is not "fell to zero". Under the old aggregation
// it was, and the screen reported a verified saving of the target's entire
// share for two sessions that simply had nothing to report.
func TestMN5_TheAggregationDistinguishesAbsenceFromZero(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var obs []Observation
	add := func(i int, defined []string) {
		ob, ok := Observe(synthetic(base.AddDate(0, 0, i), defined,
			[]string{"Bash", "Bash", "Bash"}, nil, 200))
		if !ok {
			t.Fatal("fixture must calibrate")
		}
		obs = append(obs, ob)
	}
	for i := 0; i < 4; i++ {
		add(i, []string{"Bash", "Idle1", "Idle2", "Idle3", "Idle4", "Idle5"})
	}
	// Nothing defined but the tool that is used, so there are no unused tools
	// to report and the target is absent rather than measured at zero.
	add(4, []string{"Bash"})
	add(5, []string{"Bash"})

	applied := map[string]bool{id(KindUnusedTools, "built-in"): true}
	got := kinds(Suggest(obs, applied))[KindUnusedTools]
	if got.Status == Verified || got.RealizedShare > 0 {
		t.Errorf("two sessions with no unused tools at all were scored as the target "+
			"falling to zero: status %s, realized %.3f. Absence is not a measurement.",
			got.Status, got.RealizedShare)
	}
}

// MN6: the unused-tools evidence carries no borrowed error bar.
//
// Replaces a source grep with the behaviour it stood in for.
//
// DM4 in internal/analysis reads advisor.go for "fit.RelativeError" and for
// the string "EstimateOutsideFit". Both halves are defeatable and an audit
// defeated them: the banned names are SUBSTRINGS, so another receiver spelling
// walks past, and the required call is satisfied by the COMMENT naming it even
// when the call is gone. The defect #121 exists to fix was restored under a
// different variable name and the whole tree stayed green.
//
// No grep survives that. The next spelling always escapes, because the property
// is about what the figure CLAIMS, not how the line is written.
//
// It could not be asserted behaviourally before, and that is the finding under
// the finding: note() took an analysis.Figure and kept only its Value, so
// ErrorMeasured died at the observation boundary. evidence carries it now.
//
// Tool definitions are sized with the session's prose ratio because it is the
// only ratio anyone has. The figure must not also carry that ratio's spread — a
// standard deviation over prose turns is not evidence about schema density.
func TestMN6_TheUnusedToolsEvidenceCarriesNoBorrowedSpread(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	ob, ok := Observe(synthetic(base,
		[]string{"Bash", "Idle1", "Idle2", "Idle3", "Idle4", "Idle5"},
		[]string{"Bash", "Bash", "Bash"}, nil, 200))
	if !ok {
		t.Fatal("fixture must calibrate")
	}

	var found bool
	for k, ev := range ob.targets {
		if !strings.HasPrefix(k, string(KindUnusedTools)) {
			continue
		}
		found = true
		if ev.tokens <= 0 {
			t.Errorf("the unused-tools evidence is sized at %d tokens; with nothing to "+
				"rank it is not a suggestion", ev.tokens)
		}
		if ev.errorMeasured {
			t.Errorf("the unused-tools evidence for %q claims a measured error bar. It is "+
				"tool-definition JSON sized with a ratio Fit excludes schema turns from, so "+
				"any spread attached to it was measured on different content.", k)
		}
	}
	if !found {
		t.Fatal("the fixture produced no unused-tools target, so this guard is measuring nothing")
	}
}
