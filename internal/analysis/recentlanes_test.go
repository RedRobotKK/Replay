package analysis

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A count named for sessions must never be fed a lane count.
//
// ModelCalibration.Sessions was corrected to count sessions on 2026-09-10. The
// recent window was left slicing lanes on purpose (TestRecentWindowIsStillLaneBased
// says why), and that left the two halves of one struct denominated differently
// while both were named for sessions. The staleness reason then subtracted one
// from the other:
//
//	m.Sessions - m.RecentSessions
//
// sessions minus lanes, published verbatim in every corpus document as "after
// N% on the M before them". On a fan-out corpus M is not a count of anything,
// and where a model's lanes outnumber its sessions by more than the window it
// goes negative.
//
// The fix is not to make the window session-based — that moves verdicts and
// needs an evidence bar nobody has derived. It is to stop calling lanes
// sessions: the window is lanes, the field is RecentLanes, the total it is
// subtracted from is Lanes, and the sentence says lanes.
func TestStalenessReasonDoesNotSubtractLanesFromSessions(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var reports []*LaneReport
	// Six healthy lanes, all written by ONE session — the fan-out shape that
	// makes sessions and lanes disagree.
	for i := 0; i < 6; i++ {
		reports = append(reports, synthLaneIn("healthy", "claude-opus-5", t0.Add(time.Duration(i)*time.Minute), 12, false))
	}
	// Three newer lanes from one more session, on a changed provider.
	for i := 0; i < 3; i++ {
		reports = append(reports, synthLaneIn("changed", "claude-opus-5", t0.Add(time.Hour+time.Duration(i)*time.Minute), 12, true))
	}

	cals := ModelCalibrations(reports)
	m := cals[0]
	if !m.Stale {
		t.Fatalf("fixture did not go stale, so the reason string was never built: %+v", m)
	}
	if m.Sessions != 2 {
		t.Fatalf("Sessions = %d, want 2: nine lanes, two sessions", m.Sessions)
	}
	if m.Lanes != 9 {
		t.Errorf("Lanes = %d, want 9: the recent window slices lanes, so the total it is "+
			"taken out of has to be lanes too", m.Lanes)
	}
	if m.RecentLanes != StalenessRecentLanes {
		t.Errorf("RecentLanes = %d, want %d", m.RecentLanes, StalenessRecentLanes)
	}

	// The published sentence. Under the old arithmetic this read "the -3
	// before them": two sessions minus five lanes. Nine lanes minus a
	// five-lane window is four.
	earlier := m.Lanes - m.RecentLanes
	if earlier != 4 {
		t.Fatalf("fixture arithmetic changed: %d earlier lanes", earlier)
	}
	if !strings.Contains(m.Reason, fmt.Sprintf("the %d lanes before them", earlier)) {
		t.Errorf("reason does not name %d earlier lanes: %s", earlier, m.Reason)
	}
	if strings.Contains(m.Reason, "sessions") {
		t.Errorf("the reason says \"sessions\" about a window that slices lanes: %s", m.Reason)
	}
	if !strings.Contains(m.Reason, "lanes") {
		t.Errorf("the reason never says what it counted: %s", m.Reason)
	}
}
