package analysis

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A report on one lane must not call itself complete.
//
// Every offline command routes through MainLane, which picks the largest
// non-sidechain lane, so a session that spawned sub-agents is reported on the
// main loop alone. The other lanes are parsed off disk and discarded.
//
// That is defensible. Silence about it is not. On a real 4-lane session the
// report covered 6 of 12 requests and printed "Complete: nothing was cleared or
// compacted in this session, so every block counted here is still in the
// context." The word is doing active harm: it asserts completeness over a
// partial read, which is the same defect the proxy had this morning with one
// lane standing in for a session, in the half of the codebase that was not
// fixed.
//
// PASS: a multi-lane session says how many lanes it left out.
// FAIL: it claims completeness, or hides the omission.
func TestContextGap_AMultiLaneSessionIsNotCalledComplete(t *testing.T) {
	g := ContextGap{AttributedTokens: 100_000, LanesTotal: 4, LanesReported: 1, RequestsOmitted: 6}

	note := g.Note()
	if strings.HasPrefix(note, "Complete") {
		t.Errorf("a report covering 1 of 4 lanes called itself complete:\n%s", note)
	}
	for _, want := range []string{"3", "lane"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note does not mention %q, so a reader cannot tell what was left "+
				"out:\n%s", want, note)
		}
	}
}

// A single-lane session is still allowed to be complete.
//
// The disclosure must not fire on every session. Most sessions have one lane
// and nothing was omitted from them; a warning there would be noise, and noise
// is how a real warning stops being read.
//
// PASS: one lane, nothing cleared, still says Complete.
// FAIL: the disclosure fires unconditionally.
func TestContextGap_ASingleLaneSessionStaysComplete(t *testing.T) {
	g := ContextGap{AttributedTokens: 100_000, LanesTotal: 1, LanesReported: 1}
	if !strings.HasPrefix(g.Note(), "Complete") {
		t.Errorf("a single-lane session with nothing cleared must still read as complete:\n%s",
			g.Note())
	}
}

// Both a clearing and an omission must be reported, not one of them.
//
// These are independent facts about the same figures. Reporting only the first
// found would let an overstated multi-lane session hide its omission behind the
// overstatement.
//
// PASS: the note carries both.
// FAIL: one silenced the other.
func TestContextGap_OmissionAndOverstatementAreBothReported(t *testing.T) {
	g := ContextGap{
		AttributedTokens: 100_000, ClearedTokens: 20_000, ContextEdits: 1,
		LanesTotal: 4, LanesReported: 1, RequestsOmitted: 6,
	}
	note := g.Note()
	if !strings.Contains(note, "OVERSTATED") {
		t.Errorf("the clearing was not reported:\n%s", note)
	}
	if !strings.Contains(note, "lane") {
		t.Errorf("the omitted lanes were not reported:\n%s", note)
	}
}

// MeasureGap must count the lanes itself.
//
// The caller already hands it the session and the lane it reported on, so
// asking a caller to supply the counts would be asking it to remember
// something the function can see. It is also how the first version of this
// would have shipped wrong: one caller updated, three not.
//
// PASS: the counts come back from a real session without the caller computing
// them.
// FAIL: MeasureGap ignores the lanes it was given.
func TestMeasureGap_CountsTheLanesItWasGiven(t *testing.T) {
	// Real requests, not nil pointers: MeasureGap walks the reported lane, so
	// a fixture of nils panics rather than asserting anything.
	main := &transcript.Lane{Requests: reqs(6)}
	session := &transcript.Session{Lanes: []*transcript.Lane{
		main,
		{Requests: reqs(2)},
		{Requests: reqs(2)},
		{Requests: reqs(2)},
	}}

	g := MeasureGap(session, main, 100_000)
	if g.LanesTotal != 4 {
		t.Errorf("LanesTotal = %d, want 4", g.LanesTotal)
	}
	if g.LanesReported != 1 {
		t.Errorf("LanesReported = %d, want 1", g.LanesReported)
	}
	if g.RequestsOmitted != 6 {
		t.Errorf("RequestsOmitted = %d, want 6 (three lanes of two)", g.RequestsOmitted)
	}
}

// reqs builds n usable requests for a lane fixture.
func reqs(n int) []*transcript.Request {
	out := make([]*transcript.Request, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, &transcript.Request{})
	}
	return out
}
