package proxy

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A cache read must be classified against the lane's own previous request.
//
// ClassifyRead compares this request's CacheRead against ExpectedRead of the
// previous usage. The proxy held that previous usage in one session-wide
// field, st.last, so in a fan-out session lane A's usage decided what lane B
// was measured against. Two lanes carrying different prefix sizes then
// manufacture breaks and overruns out of nothing, on every interleaved
// request.
//
// This is the last of five fields with the same defect. It is the worst of
// them, because the other four decided how a break was described or reported
// and this one decides whether there was a break at all.
//
// The vocabulary already says so: cachemodel.ReadFirst is documented as "the
// first request in a lane; nothing to compare against", and the proxy gated it
// on a session-wide request count.
//
// PASS: two lanes, each internally consistent, produce no breaks.
// FAIL: interleaving them forges breaks.
func TestObserve_ConcurrentLanesDoNotForgeCacheBreaks(t *testing.T) {
	s := newStats()

	// Two lanes with different prefix sizes. Read lane by lane, every request
	// after the first reads exactly what the one before it established, so
	// each lane is a clean run of reproductions.
	steps := []struct {
		agent                           string
		input, cacheCreation, cacheRead int
	}{
		{"lane-a", 100, 1000, 0},  // lane-a opens: expected read becomes 1000
		{"lane-b", 100, 5000, 0},  // lane-b opens: expected read becomes 5000
		{"lane-a", 50, 200, 1000}, // reproduces lane-a exactly
		{"lane-b", 50, 200, 5000}, // reproduces lane-b exactly
		{"lane-a", 50, 200, 1200}, // reproduces lane-a exactly
		{"lane-b", 50, 200, 5200}, // reproduces lane-b exactly
	}
	for i, step := range steps {
		s.observe(usageRecord(step.agent, i, step.input, step.cacheCreation, step.cacheRead))
	}

	st := s.session("sess-fanout-usage")
	if st.breaks != 0 {
		t.Errorf("breaks = %d, want 0. Each lane reproduced its own prefix on every request "+
			"after its first. Every break counted here is one lane's usage being measured "+
			"against a different lane's, which is how a fan-out session manufactures cache "+
			"misses that never happened.", st.breaks)
	}
}

// A real break inside a lane is still caught.
//
// Keying by lane must not buy silence. A lane that genuinely fails to read the
// prefix it established is a break and has to stay one.
//
// PASS: exactly one break, in the lane that broke.
// FAIL: the classifier stopped being able to fire.
func TestObserve_AGenuineWithinLaneBreakIsStillCaught(t *testing.T) {
	s := newStats()
	steps := []struct {
		agent                           string
		input, cacheCreation, cacheRead int
	}{
		{"lane-a", 100, 1000, 0},
		{"lane-b", 100, 5000, 0},
		{"lane-a", 50, 200, 1000}, // reproduces
		{"lane-b", 50, 200, 0},    // lane-b really did lose its prefix
		{"lane-a", 50, 200, 1200}, // reproduces
	}
	for i, step := range steps {
		s.observe(usageRecord(step.agent, i, step.input, step.cacheCreation, step.cacheRead))
	}

	st := s.session("sess-fanout-usage")
	if st.breaks != 1 {
		t.Errorf("breaks = %d, want exactly 1. lane-b read nothing where it had established a "+
			"5000-token prefix, and that is a real break that must still register.", st.breaks)
	}
}

// The model is compared per lane too, or a mixed-model fan-out forges a cause.
//
// ClassifyBreak returns CauseModelChanged when the previous model differs from
// this one, and it read the same session-wide slot. A session running opus in
// the main loop and haiku in its sub-agents, which is the ordinary shape of a
// fan-out, reports every interleaved request as a model change.
//
// PASS: a lane that never changes model never reports one.
// FAIL: a sibling on another model supplied the comparison.
func TestObserve_ASiblingOnAnotherModelDoesNotForgeAModelChange(t *testing.T) {
	s := newStats()
	for i, step := range []struct {
		agent, model                    string
		input, cacheCreation, cacheRead int
	}{
		{"", "claude-opus-5", 100, 1000, 0},
		{"lane-a", "claude-haiku-4-5-20251001", 100, 5000, 0},
		{"", "claude-opus-5", 50, 200, 0}, // a real break in the main loop
		{"lane-a", "claude-haiku-4-5-20251001", 50, 200, 5000},
	} {
		rec := usageRecord(step.agent, i, step.input, step.cacheCreation, step.cacheRead)
		rec.Model = step.model
		out := s.observe(rec)
		// The main loop's third request is the real break, and it is the one
		// whose cause is under test.
		if i == 2 {
			if out == nil || out.Outcome != "broken" {
				t.Fatalf("the main loop's third request was expected to break, so this test "+
					"could assert its cause; got %+v", out)
			}
			if out.Cause == cachemodel.CauseModelChanged {
				t.Errorf("the main loop's break was blamed on a model change, but the main "+
					"loop ran claude-opus-5 on every one of its own requests. The only other "+
					"model in the session belongs to lane-a. cause = %q", out.Cause)
			}
		}
	}
}

// usageRecord builds one lane's request with a given usage shape.
func usageRecord(agentID string, i, input, cacheCreation, cacheRead int) *ledger.Record {
	rec := &ledger.Record{
		Timestamp: time.Date(2026, 9, 6, 12, 0, i, 0, time.UTC),
		SessionID: "sess-fanout-usage",
		AgentID:   agentID,
		Status:    200,
	}
	rec.Model = "claude-opus-5"
	rec.PrefixHash = "stable-" + agentID
	rec.Response.Usage = &transcript.Usage{
		Input:         input,
		CacheCreation: cacheCreation,
		CacheRead:     cacheRead,
		Output:        10,
	}
	return rec
}

// A lane's opening request is ReadFirst, not a reproduction.
//
// The gate on the first request used a session-wide request count, so the
// second lane to open in a session had its opening request classified rather
// than exempted. It does not produce a break: an unseen lane's usage is the
// zero value, ExpectedRead of that is 0, and an opening request reads 0, so
// the two match exactly and score as "reproduced".
//
// That is a fabricated reproduction. Every sub-agent lane in a fan-out session
// contributes one, and they inflate the cached share the status endpoint
// reports.
//
// Found by mutation: swapping the lane gate back to the session-wide count
// survived every other test in this file.
//
// PASS: an opening request in any lane is exempt from classification.
// FAIL: it was compared against a lane that had not run yet.
func TestObserve_AnOpeningRequestInAnyLaneIsNotAReproduction(t *testing.T) {
	s := newStats()

	// The main loop runs first, so the session already has requests when
	// lane-a opens.
	if out := s.observe(usageRecord("", 0, 100, 1000, 0)); out != nil {
		t.Fatalf("the session's very first request was classified: %+v", out)
	}
	if out := s.observe(usageRecord("", 1, 50, 200, 1000)); out == nil {
		t.Fatal("the main loop's second request was not classified, so this test cannot fail")
	}

	// lane-a now opens. It has never run, so there is nothing to compare it
	// against and it must be exempt.
	out := s.observe(usageRecord("lane-a", 2, 100, 5000, 0))
	if out != nil {
		t.Errorf("lane-a's opening request was classified as %q against a lane that had not "+
			"run. An unseen lane's usage is the zero value and ExpectedRead of it is 0, so an "+
			"opening request matches it exactly and scores as a reproduction that never "+
			"happened.", out.Outcome)
	}
}
