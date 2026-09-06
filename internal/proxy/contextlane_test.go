package proxy

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
)

// Every lane's context breakdown must survive its siblings finishing.
//
// rescore analyses ONE lane, the one the just-completed record belongs to
// (state.go:410), and then stores that lane's result in a session-wide field.
// In a fan-out session the last lane to finish therefore decides what the
// status endpoint reports for the whole session, and a quiet sub-agent erases
// a busy one.
//
// This is the same defect as the prefix hash fixed in the previous commit, and
// as errorByLane before that. errorByLane is keyed by AgentID three lines below
// the assignment that is not, with a comment explaining exactly why. The lesson
// was written down once and applied to one field out of four.
//
// PASS: after a sub-agent lane rescores, the main loop's own breakdown is still
// there and is still the main loop's.
// FAIL: the sibling overwrote it.
func TestRescore_ALaneDoesNotEraseItsSiblingsContext(t *testing.T) {
	recs := loadFixtureRecords(t)
	s := newStats()

	// The main loop runs first and builds a real context breakdown.
	for _, rec := range recs {
		r := rec
		r.SessionID = "fanout"
		r.AgentID = ""
		s.observe(&r)
		s.rescore(&r)
	}
	mainBefore := s.session("fanout").contextFor("")
	if len(mainBefore) == 0 {
		t.Fatal("the main loop produced no context entries, so this test cannot fail. The " +
			"fixture no longer carries content that Blame can attribute")
	}

	// A sub-agent lane then finishes. It analyses its own lane only.
	for _, rec := range recs {
		r := rec
		r.SessionID = "fanout"
		r.AgentID = "lane-a"
		s.observe(&r)
		s.rescore(&r)
	}

	mainAfter := s.session("fanout").contextFor("")
	if len(mainAfter) != len(mainBefore) {
		t.Errorf("the main loop's context went from %d entries to %d when a sub-agent lane "+
			"finished. rescore analyses one lane and must not store that lane's answer as "+
			"the session's.", len(mainBefore), len(mainAfter))
	}

	laneAfter := s.session("fanout").contextFor("lane-a")
	if len(laneAfter) == 0 {
		t.Error("the sub-agent lane's own context breakdown was not recorded. Keying by lane " +
			"is only worth doing if each lane is actually readable")
	}

	// The accessor must discriminate. Returning the main loop's rows for any
	// key asked of it would satisfy both assertions above while storing
	// nothing per lane at all.
	if got := s.session("fanout").contextFor("no-such-lane"); len(got) != 0 {
		t.Errorf("contextFor(\"no-such-lane\") returned %d entries. A lane that never ran has "+
			"no breakdown, and an accessor that answers anyway is reporting another lane's "+
			"figures under this lane's name", len(got))
	}
	if got := s.session("fanout").reReadsFor("no-such-lane"); got != (analysis.ReReads{}) {
		t.Errorf("reReadsFor(\"no-such-lane\") returned %+v, want the zero value", got)
	}
}

// Re-reads and what-if are keyed by lane too, not only context.
//
// Mutation found this gap: collapsing reReads to a single key survived the
// first version of this file, because nothing asserted the per-lane shape for
// anything but context. All three fields carried the same defect and all three
// need the same guard.
//
// PASS: both lanes appear in each per-lane map on the status endpoint.
// FAIL: one of the three quietly went back to a single slot.
func TestStatus_ReReadsAndWhatIfAreAlsoPerLane(t *testing.T) {
	recs := loadFixtureRecords(t)
	s := newStats()
	for _, agent := range []string{"", "lane-a"} {
		for _, rec := range recs {
			r := rec
			r.SessionID = "fanout"
			r.AgentID = agent
			s.observe(&r)
			s.rescore(&r)
		}
	}

	st := s.session("fanout")
	for name, got := range map[string]int{
		"context": len(st.context),
		"reReads": len(st.reReads),
		"whatIf":  len(st.whatIf),
	} {
		if got < 2 {
			t.Errorf("%s holds %d lane(s), want the main loop and the sub-agent. A single "+
				"slot here is the erasure this change exists to remove", name, got)
		}
	}

	var summary *SessionSummary
	for i := range s.status().Sessions {
		if sess := s.status().Sessions[i]; len(sess.ContextByLane) > 0 {
			summary = &sess
			break
		}
	}
	if summary == nil {
		t.Fatal("no session summary carried a per-lane context map")
	}
	if len(summary.ReReadsByLane) < 2 {
		t.Errorf("re_reads_by_lane holds %d lane(s), want 2", len(summary.ReReadsByLane))
	}
	if len(summary.WhatIfByLane) < 2 {
		t.Errorf("what_if_by_lane holds %d lane(s), want 2", len(summary.WhatIfByLane))
	}
}

// The status endpoint must keep reporting an array for `context`.
//
// /replay/status is a documented, Verified surface in docs/SURFACES.md and it
// carries no schema version, so a consumer has nothing to branch on. Turning
// `context` from a JSON array into an object would break it silently.
//
// The fix is additive: `context` keeps its shape and gains a defined meaning,
// the main loop's own breakdown, rather than whichever lane happened to finish
// last. The per-lane map arrives beside it under a new key.
//
// PASS: context is the main loop's, and context_by_lane carries every lane.
// FAIL: the published shape changed, or context is still last-lane-wins.
func TestStatus_ContextStaysAnArrayAndGainsAPerLaneMap(t *testing.T) {
	recs := loadFixtureRecords(t)
	s := newStats()
	for _, agent := range []string{"", "lane-a"} {
		for _, rec := range recs {
			r := rec
			r.SessionID = "fanout"
			r.AgentID = agent
			s.observe(&r)
			s.rescore(&r)
		}
	}

	status := s.status()
	var summary *SessionSummary
	for i := range status.Sessions {
		if len(status.Sessions[i].ContextByLane) > 0 || len(status.Sessions[i].Context) > 0 {
			summary = &status.Sessions[i]
			break
		}
	}
	if summary == nil {
		t.Fatal("no session summary carried any context, so this test cannot fail")
	}

	blob, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("the summary must marshal: %v", err)
	}
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(blob, &generic); err != nil {
		t.Fatalf("the summary must round-trip: %v", err)
	}

	raw, ok := generic["context"]
	if !ok {
		t.Fatal("the `context` key vanished from /replay/status. It is a documented surface " +
			"with no schema version, so a consumer has nothing to branch on")
	}
	if len(raw) == 0 || raw[0] != '[' {
		t.Errorf("`context` is no longer a JSON array: %s", raw)
	}

	if _, ok := generic["context_by_lane"]; !ok {
		t.Error("`context_by_lane` was not published, so the per-lane breakdown this whole " +
			"change exists for is not reachable by anyone")
	}

	if len(summary.ContextByLane) < 2 {
		t.Errorf("context_by_lane holds %d lane(s), want the main loop and the sub-agent",
			len(summary.ContextByLane))
	}
}

// loadFixtureRecords reads the shipped ledger fixture as records.
func loadFixtureRecords(t *testing.T) []ledger.Record {
	t.Helper()
	path := filepath.Join("..", "ledger", "testdata", "late-binding-tools.jsonl")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("the shipped fixture must open: %v", err)
	}
	defer func() { _ = f.Close() }()

	var out []ledger.Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		var rec ledger.Record
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("the fixture must parse: %v", err)
		}
		out = append(out, rec)
	}
	if len(out) == 0 {
		t.Fatal("the fixture is empty")
	}
	return out
}

// The lane accessors must read the key they are given.
//
// Driven directly rather than through rescore, because the shipped fixture
// produces a zero-valued ReReads for every lane, so a pipeline test cannot
// tell "read the right key" apart from "read any key". Mutation found that:
// making reReadsFor ignore its argument survived the pipeline tests.
//
// PASS: each accessor returns the lane asked for, and nothing for a lane that
// never ran.
// FAIL: an accessor answers with some other lane's figures.
func TestSessionState_LaneAccessorsReadTheKeyTheyAreGiven(t *testing.T) {
	st := &sessionState{
		context: map[string][]analysis.ContextEntry{
			"":       {{Label: "main-tool", Tokens: 10}},
			"lane-a": {{Label: "lane-a-tool", Tokens: 20}, {Label: "second", Tokens: 5}},
		},
		reReads: map[string]analysis.ReReads{
			"":       {Reads: 1, Repeated: 1},
			"lane-a": {Reads: 7, Repeated: 3},
		},
		whatIf: map[string][]WhatIf{
			"":       {{Policy: "as-run"}},
			"lane-a": {{Policy: "as-run"}, {Policy: "trim"}},
		},
	}

	if got := st.contextFor("lane-a"); len(got) != 2 || got[0].Label != "lane-a-tool" {
		t.Errorf("contextFor(\"lane-a\") = %+v, want lane-a's own two entries", got)
	}
	if got := st.contextFor(""); len(got) != 1 || got[0].Label != "main-tool" {
		t.Errorf("contextFor(\"\") = %+v, want the main loop's single entry", got)
	}
	if got := st.reReadsFor("lane-a"); got.Reads != 7 || got.Repeated != 3 {
		t.Errorf("reReadsFor(\"lane-a\") = %+v, want lane-a's own figures. An accessor that "+
			"answers with the main loop's numbers under a sub-agent's name is the erasure "+
			"this change removed, moved one layer down", got)
	}
	if got := st.reReadsFor(""); got.Reads != 1 {
		t.Errorf("reReadsFor(\"\") = %+v, want the main loop's figures", got)
	}
	if got := st.whatIfFor("lane-a"); len(got) != 2 {
		t.Errorf("whatIfFor(\"lane-a\") returned %d row(s), want 2", len(got))
	}
	for _, lane := range []string{"no-such-lane", "lane-b"} {
		if got := st.contextFor(lane); got != nil {
			t.Errorf("contextFor(%q) = %+v, want nothing for a lane that never ran", lane, got)
		}
		if got := st.reReadsFor(lane); got != (analysis.ReReads{}) {
			t.Errorf("reReadsFor(%q) = %+v, want the zero value", lane, got)
		}
		if got := st.whatIfFor(lane); got != nil {
			t.Errorf("whatIfFor(%q) = %+v, want nothing", lane, got)
		}
	}
}
