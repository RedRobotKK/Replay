package proxy

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A prefix change must be judged against the lane's own previous request.
//
// The proxy holds one prefixHash per session. A fan-out session runs several
// sub-agent lanes at once and each carries its own tool set, so lane A's
// request overwrites the field and lane B's next request is then compared
// against lane A's hash. Neither lane changed anything and both are reported
// as having changed their prefix.
//
// This is the defect documented at the errorByLane field and fixed there
// alone: "a single field meant the last lane to rescore decided the numerator
// and a quiet sub-agent could erase a busy one". The same single-field shape
// sits under the break attribution, which is upstream of the cause string, the
// ledger's re-billed totals and the pre-flight guard.
//
// It is not academic. The 2026-09-06 trial attributed 3,734,134 re-billed
// tokens to a changed prefix through this path; re-read lane by lane, every
// one of its thirty sub-agent lanes is internally stable and never changes
// prefix at all.
//
// PASS: two lanes with stable, different prefixes report no change.
// FAIL: interleaving them manufactures a prefix change.
func TestObserve_ConcurrentLanesDoNotForgeAPrefixChange(t *testing.T) {
	s := newStats()

	// Two sub-agent lanes, each with its own tool set, interleaved the way a
	// fan-out session actually arrives. Neither lane ever changes its own
	// prefix.
	seq := []struct{ agent, prefix string }{
		{"lane-a", "hash-a"},
		{"lane-b", "hash-b"},
		{"lane-a", "hash-a"},
		{"lane-b", "hash-b"},
		{"lane-a", "hash-a"},
	}
	for i, step := range seq {
		rec := laneRecord(step.agent, step.prefix, i)
		s.observe(rec)
	}

	st := s.session("sess-fanout")
	if st.prefixChanges != 0 {
		t.Errorf("prefixChanges = %d, want 0. Two lanes each held a stable prefix and neither "+
			"changed one; every count here is a lane being compared against a different "+
			"lane's request. This is what attributed 3,734,134 tokens to a prefix change in "+
			"the trial.", st.prefixChanges)
	}
}

// A lane that really does change its prefix is still caught.
//
// The fix must not buy silence: keying by lane is only correct if a genuine
// within-lane change still registers. The real shape is an MCP connector
// finishing its handshake mid-session, which drops WaitForMcpServers and
// appends that connector's whole tool set to the same lane.
//
// PASS: one change, in the lane that changed.
// FAIL: the guard stopped being able to fire.
func TestObserve_AGenuineWithinLaneChangeIsStillCaught(t *testing.T) {
	s := newStats()
	for i, step := range []struct{ agent, prefix string }{
		{"lane-a", "hash-a"},
		{"lane-b", "hash-b"},
		{"lane-a", "hash-a-plus-connector"}, // the real change
		{"lane-b", "hash-b"},
	} {
		s.observe(laneRecord(step.agent, step.prefix, i))
	}

	st := s.session("sess-fanout")
	if st.prefixChanges != 1 {
		t.Errorf("prefixChanges = %d, want exactly 1. A lane appended a connector's tool set "+
			"to its own prefix and that must still be seen.", st.prefixChanges)
	}
}

// The first request in a lane establishes its prefix and cannot have changed.
//
// The old code guarded this with a session-wide request count, so the second
// lane to open in a session had its opening request judged against the first
// lane's prefix.
//
// PASS: no change counted when each lane opens.
// FAIL: opening a lane counts as changing it.
func TestObserve_OpeningALaneIsNotAChange(t *testing.T) {
	s := newStats()
	for i, step := range []struct{ agent, prefix string }{
		{"", "hash-main"},
		{"lane-a", "hash-a"},
		{"lane-b", "hash-b"},
		{"lane-c", "hash-c"},
	} {
		s.observe(laneRecord(step.agent, step.prefix, i))
	}

	st := s.session("sess-fanout")
	if st.prefixChanges != 0 {
		t.Errorf("prefixChanges = %d, want 0. Four lanes opened and none of them had a "+
			"previous request of its own to differ from.", st.prefixChanges)
	}
}

// laneRecord builds a usable record for one lane's request.
func laneRecord(agentID, prefixHash string, i int) *ledger.Record {
	rec := &ledger.Record{
		Timestamp: time.Date(2026, 9, 6, 9, 35, i, 0, time.UTC),
		SessionID: "sess-fanout",
		AgentID:   agentID,
		Status:    200,
	}
	rec.PrefixHash = prefixHash
	rec.Model = "claude-haiku-4-5-20251001"
	// A warm read, so the record reaches the prefix comparison rather than
	// being dropped for want of usage.
	rec.Response.Usage = &transcript.Usage{
		Input:     10,
		CacheRead: 1000,
		Output:    10,
	}
	return rec
}
