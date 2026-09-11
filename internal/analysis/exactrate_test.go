package analysis

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// MatchRate folds ReadExceeded into the numerator, and every surface that
// prints it inherits the fold silently.
//
// An exceeded read is the provider serving MORE cached prefix than the model
// predicted — almost always because a concurrent sibling lane extended the
// prefix. calibrate.go's justification is "the provider served at least the
// prefix the model predicted", which is true and is not the same claim as
// "the model predicted the read". An unexpectedly larger read is evidence the
// prediction was wrong in the other direction.
//
// So MatchRate keeps its published meaning — a statistic that has been
// quoted is not redefined underneath its readers — and an exact-reproduction
// rate is reported beside it everywhere the headline appears. On the corpus
// this was written against (2026-09-11, 37925 compared turns) the headline
// 97.87% is 94.10% exact and 3.78% exceeded: 3.86% of every "matched" turn is
// a turn the model got wrong.

// exceededLane builds a lane whose second turn reproduces the predicted read
// and whose third reads more than predicted.
//
// ExpectedRead is the previous request's cache creation plus its cache read,
// so r1 reading exactly 100 is reproduced and r2 reading more than r1 wrote is
// exceeded.
func exceededLane() *transcript.Lane {
	return &transcript.Lane{Requests: []*transcript.Request{
		{ID: "r0", Usage: transcript.Usage{Input: 10, CacheCreation: 100}},
		{ID: "r1", Usage: transcript.Usage{Input: 2, CacheRead: 100, CacheCreation: 20}},
		{ID: "r2", Usage: transcript.Usage{Input: 2, CacheRead: 500}},
	}}
}

// MR1: the fixture actually produces one of each outcome.
//
// Without this the tests below could pass against a lane that never exceeded,
// asserting that two identical numbers are identical.
func TestMR1_TheFixtureProducesOneOfEachOutcome(t *testing.T) {
	cal := Calibrate(exceededLane())
	if cal.Reproduced != 1 || cal.Exceeded != 1 || cal.Broken != 0 {
		t.Fatalf("fixture is reproduced=%d exceeded=%d broken=%d; it must be 1/1/0 or the "+
			"split tests assert nothing", cal.Reproduced, cal.Exceeded, cal.Broken)
	}
	if cal.Turns[2].Outcome != cachemodel.ReadExceeded {
		t.Fatalf("turn 2 is %s, not exceeded", cal.Turns[2].Outcome)
	}
}

// MR2: an exact-reproduction rate exists and excludes exceeded turns.
//
// PASS: ExactRate counts only Reproduced over Compared, so the fixture is 0.5
// while MatchRate is 1.
// FAIL: the two agree, which means the exact rate is the headline again and a
// reader still cannot see how much of it is exact.
func TestMR2_ExactRateExcludesExceededReads(t *testing.T) {
	cal := Calibrate(exceededLane())
	if got := cal.MatchRate(); got != 1 {
		t.Fatalf("MatchRate() = %.3f, want 1: the published statistic must keep its meaning", got)
	}
	got := cal.ExactRate()
	if got != 0.5 {
		t.Errorf("ExactRate() = %.3f, want 0.500. One of the two compared turns read MORE than "+
			"predicted, which is evidence the prediction was wrong; folding it in reports a "+
			"perfect reproduction of reads the model did not reproduce.", got)
	}
	if got >= cal.MatchRate() {
		t.Errorf("ExactRate() = %.3f is not below MatchRate() = %.3f on a lane with an exceeded "+
			"turn, so the exact rate is not excluding it", got, cal.MatchRate())
	}
}

// MR3: no evidence is not a perfect exact rate either.
//
// The defect E2 pins for MatchRate is available to any new rate written beside
// it, and a rate that returns 1 over an empty comparison is the thing ADR-0014
// calls a check that cannot fail.
//
// PASS: below CalibrationThreshold with nothing compared.
// FAIL: 1, or anything a threshold would admit.
func TestMR3_ExactRateOverNothingIsNotPerfect(t *testing.T) {
	cal := Calibrate(&transcript.Lane{Requests: []*transcript.Request{
		{ID: "r0", Usage: transcript.Usage{Input: 10, CacheCreation: 100}},
	}})
	if cal.HasEvidence() {
		t.Fatalf("fixture compares %d turns; it must compare none", cal.Compared())
	}
	got := cal.ExactRate()
	if got == 1 {
		t.Error("ExactRate() = 1 with nothing compared: an absent measurement reported as a perfect one")
	}
	if got >= CalibrationThreshold {
		t.Errorf("ExactRate() = %.3f with nothing compared, at or above the %.2f threshold",
			got, CalibrationThreshold)
	}
}

// MR4: the lane report prints the split, not just the total.
//
// This is the surface `replay replay`, `replay diff` and `replay blame` all
// share. It printed "reproduced provider cache reads on N/M turns" where N was
// Reproduced+Exceeded, and named the exceeded count only in a parenthetical
// with no rate beside it. A reader could not tell what share of the headline
// was exact without arithmetic the line did not give them the inputs for.
//
// PASS: the header carries the exact count and both percentages.
// FAIL: one number, which is the conflation this test exists to stop.
func TestMR4_LaneReportHeaderPrintsBothRates(t *testing.T) {
	lane := exceededLane()
	session := &transcript.Session{ID: "s", ClientVersion: "test", Lanes: []*transcript.Lane{lane}}
	rep := AnalyzeLane(session, lane)
	var buf bytes.Buffer
	p := NewPrinter(&buf)
	rep.header(p)
	got := buf.String()
	line := calibrationLineOf(got)
	if !strings.Contains(line, "50.0% exact") {
		t.Errorf("the calibration line does not report the exact reproduction rate. One of two "+
			"compared turns read more than predicted, so 100%% match is 50%% exact:\n%s", line)
	}
	if !strings.Contains(line, "100.0% match") {
		t.Errorf("the calibration line dropped the published match rate. MatchRate keeps its "+
			"meaning; the exact rate is printed BESIDE it, not instead of it:\n%s", line)
	}
	if !strings.Contains(line, "1 of 2") {
		t.Errorf("the calibration line does not give the exact count over the compared count, so "+
			"a reader cannot check either percentage:\n%s", line)
	}
}

// MR5: a model's calibration carries the exact count too.
//
// ModelCalibration is what the per-model table and the poolable contribution
// record are both built from. If the split stops at the lane, every aggregate
// above it is back to one number.
//
// PASS: Exact is the reproduced-only count and ExactRate is below MatchRate.
// FAIL: Exact tracks Matched, which is the fold one level up.
func TestMR5_ModelCalibrationCarriesTheExactCount(t *testing.T) {
	lane := exceededLane()
	session := &transcript.Session{ID: "s", ClientVersion: "test", Lanes: []*transcript.Lane{lane}}
	cals := ModelCalibrations([]*LaneReport{AnalyzeLane(session, lane)})
	if len(cals) != 1 {
		t.Fatalf("got %d model rows, want 1", len(cals))
	}
	m := cals[0]
	if m.Compared != 2 || m.Matched != 2 {
		t.Fatalf("fixture is compared=%d matched=%d; it must be 2/2", m.Compared, m.Matched)
	}
	if m.Exact != 1 {
		t.Errorf("ModelCalibration.Exact = %d, want 1: two turns matched but only one reproduced "+
			"the predicted read exactly", m.Exact)
	}
	if got := m.ExactRate(); got != 0.5 {
		t.Errorf("ExactRate() = %.3f, want 0.500", got)
	}
	if m.RecentExact != 1 {
		t.Errorf("ModelCalibration.RecentExact = %d, want 1: the recent window needs the same "+
			"split or a staleness verdict is read off the folded number", m.RecentExact)
	}
	if got := m.RecentExactRate(); got != 0.5 {
		t.Errorf("RecentExactRate() = %.3f, want 0.500", got)
	}
}

// calibrationLineOf returns the header's calibration line, for readable
// failures.
func calibrationLineOf(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "Calibration:") {
			return line
		}
	}
	return s
}

// ExactRate returns zero without evidence, for the reason MatchRate does.
//
// guard-reachability reported the HasEvidence branch unreached: every test
// called ExactRate on a lane that had compared something. So nothing
// established that a lane with NO compared turns reports 0 rather than the
// 0/0 the division would produce, or worse, a 1 that reads as perfect.
//
// MatchRate returned 1 for an empty lane until 2026-09-06, and 18 of 1,450
// lanes were admitted to alternative scoring for having tested nothing. This
// is the same trap one function over, and it was added without a test.
func TestER1_ExactRateIsZeroWithoutEvidence(t *testing.T) {
	var empty Calibration
	if empty.HasEvidence() {
		t.Fatal("the fixture compared something; this test asserts nothing")
	}
	if got := empty.ExactRate(); got != 0 {
		t.Errorf("ExactRate = %v on a lane that compared nothing, want 0. An absent "+
			"measurement must not read as a good one", got)
	}
	// And it must not pass a threshold by accident.
	if empty.Passes() {
		t.Error("a lane with no evidence passes the calibration gate")
	}
}
