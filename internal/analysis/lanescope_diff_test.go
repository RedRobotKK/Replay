package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// `replay diff` reports one lane and does not say so.
//
// Every offline command analyses the lane MainLane picks and discards the
// rest. On the corpus this was measured against, the whole set holds 817
// breaks and 55,294,260 re-billed tokens; diff prints 758 and 33.8M. So
// 21.5 million tokens, 38.9% of the total, sit in lanes of the same files
// that the report never mentions, and every cause share it prints is a share
// of 61% of the tokens presented as if it were all of them.
//
// ContextGap already carries LanesTotal, LanesReported and RequestsOmitted
// for exactly this reason, and its comment says why: "That is a defensible
// choice and saying nothing about it is not: on a real four-lane session the
// report covered six of twelve requests and called itself Complete." The
// choice is still defensible. The silence in diff is the same defect that
// comment describes, in a different command.

func laneWithRequests(id string, sidechain bool, n int) *transcript.Lane {
	l := &transcript.Lane{ID: id, Sidechain: sidechain}
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		l.Requests = append(l.Requests, &transcript.Request{
			Model:     "claude-opus-5",
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Usage:     transcript.Usage{Input: 100, Output: 10},
		})
	}
	return l
}

// LS-1: a multi-lane session says how much of itself the report covers.
func TestLSD1_DiffSaysWhichLanesItRead(t *testing.T) {
	s := &transcript.Session{
		ID:            "sess-multi",
		ClientVersion: "2.1.257",
		Source:        transcript.SourceTranscript,
		Lanes: []*transcript.Lane{
			laneWithRequests("main", false, 6),
			laneWithRequests("sub-a", true, 4),
			laneWithRequests("sub-b", true, 2),
		},
	}
	rep := AnalyzeLane(s, MainLane(s))

	var b strings.Builder
	if err := rep.WriteDiff(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()

	if !strings.Contains(out, "1 of 3 lanes") {
		t.Errorf("diff does not say how many lanes it read; a reader cannot tell that 6 of 12 requests were dropped:\n%s", out)
	}
	if !strings.Contains(out, "6 request") {
		t.Errorf("diff does not say how many requests were omitted:\n%s", out)
	}
}

// LS-2: a single-lane session says nothing, because there is nothing to say.
//
// The disclosure has to be silent in the ordinary case or it becomes noise
// that readers learn to skip, and then it is not a disclosure. This also
// stops LS-1 being satisfied by printing the line unconditionally.
func TestLSD2_SingleLaneSaysNothing(t *testing.T) {
	s := &transcript.Session{
		ID:            "sess-single",
		ClientVersion: "2.1.257",
		Source:        transcript.SourceTranscript,
		Lanes:         []*transcript.Lane{laneWithRequests("main", false, 5)},
	}
	rep := AnalyzeLane(s, MainLane(s))

	var b strings.Builder
	if err := rep.WriteDiff(&b); err != nil {
		t.Fatal(err)
	}
	if out := b.String(); strings.Contains(out, "lanes") {
		t.Errorf("a one-lane session has nothing to disclose and should not mention lanes:\n%s", out)
	}
}
