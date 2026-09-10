package analysis

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// synthLaneIn is synthLane with the session the lane belongs to attached.
//
// A Claude Code session writes one transcript per lane — the main one plus one
// for every subagent it spawns — and they all carry the same session id.
// synthLane leaves Session nil, so every test written with it happened to have
// one lane per session and could not express the difference.
func synthLaneIn(sessionID, model string, start time.Time, turns int, broken bool) *LaneReport {
	rep := synthLane(model, start, turns, broken)
	rep.Session = &transcript.Session{ID: sessionID}
	return rep
}

// ModelCalibration.Sessions must count sessions.
//
// It was `len(reps)`, a count of lanes. `replay corpus` prints that value in a
// column headed "Sessions", so a session that spawned four subagents was
// published as five independent draws — the same file-count-as-session-count
// conflation the 2026-09-06 calibration corpus retracts in its own header,
// surviving in the generator that produces the table below it.
func TestModelCalibrationSessionsCountsSessionsNotLanes(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var reports []*LaneReport
	for i := 0; i < 5; i++ {
		reports = append(reports, synthLaneIn("session-a", "claude-opus-5", t0.Add(time.Duration(i)*time.Minute), 12, false))
	}
	reports = append(reports, synthLaneIn("session-b", "claude-opus-5", t0.Add(time.Hour), 12, false))

	cals := ModelCalibrations(reports)
	if len(cals) != 1 {
		t.Fatalf("expected one model, got %d", len(cals))
	}
	if got := cals[0].Sessions; got != 2 {
		t.Errorf("Sessions = %d, want 2: six lanes were written by two sessions, and a "+
			"column headed Sessions that reports 6 overstates the independent sample "+
			"threefold", got)
	}
}

// A report with no session attached counts as its own session.
//
// Callers that build a LaneReport straight from a lane leave Session nil.
// Treating those as one shared anonymous session would collapse unrelated
// lanes into a single draw — the opposite error, and a worse one.
func TestLanesWithNoSessionCountSeparately(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	reports := []*LaneReport{
		synthLane("claude-opus-5", t0, 12, false),
		synthLane("claude-opus-5", t0.Add(time.Hour), 12, false),
		synthLane("claude-opus-5", t0.Add(2*time.Hour), 12, false),
	}
	cals := ModelCalibrations(reports)
	if got := cals[0].Sessions; got != 3 {
		t.Errorf("Sessions = %d, want 3: three lanes with no session id are three draws, "+
			"not one", got)
	}
}

// The staleness window is deliberately still lane-based, and this pins it.
//
// Making the window session-based is the right end state and is NOT what this
// change does. It moves verdicts: on the maintainer's own corpus of 2026-09-10
// it cleared claude-opus-4-8, flagged claude-haiku-4-5, and flagged
// claude-opus-5 stale at a 98.9% recent match rate because three of its newest
// sessions were small enough to fail on a single missed turn. Counting a
// session as failing needs a minimum-evidence bar per session, and ADR-0009
// says no guard threshold here may be a number somebody liked. Until that bar
// is derived, the window keeps slicing lanes and only the published count is
// corrected.
//
// This test exists so that limitation cannot be mistaken for an oversight, and
// so whoever derives the bar has to come here and delete it on purpose.
func TestRecentWindowIsStillLaneBased(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var reports []*LaneReport
	for i := 0; i < 6; i++ {
		reports = append(reports, synthLaneIn(
			"old-"+string(rune('a'+i)), "claude-opus-5", t0.Add(time.Duration(i)*time.Hour), 12, false))
	}
	// One later session spawning five lanes. A session-based window would
	// reach back past it into four of the older sessions; a lane-based one
	// stops inside it.
	for i := 0; i < 5; i++ {
		reports = append(reports, synthLaneIn(
			"fanout", "claude-opus-5", t0.Add(24*time.Hour+time.Duration(i)*time.Minute), 12, false))
	}

	cals := ModelCalibrations(reports)
	perLane := cals[0].Compared / 11
	if got, want := cals[0].RecentCompared, StalenessRecentSessions*perLane; got != want {
		t.Errorf("RecentCompared = %d, want %d: the window slices the newest %d LANES. "+
			"If this now fails because the window counts sessions, that is the intended "+
			"end state — delete this test and derive the per-session evidence bar",
			got, want, StalenessRecentSessions)
	}
	if got := cals[0].Sessions; got != 7 {
		t.Errorf("Sessions = %d, want 7: the count is corrected even though the window is not", got)
	}
}
