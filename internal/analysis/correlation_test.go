package analysis

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The offline classifier has the same join as the live one and one less thing
// to join with.
//
// Calibrate pairs request i with request i-1 in the lane slice, and that slice
// is in the order the records were written, which is the order the responses
// finished. Where the proxy recorded that two requests were open at once, the
// pairing is a guess with a coin's accuracy; where it did not, the timestamps
// still show a request that began before its supposed predecessor ended.

func corrRequest(id string, start time.Time, latency time.Duration, model, correlation string, u transcript.Usage) *transcript.Request {
	return &transcript.Request{
		ID:          id,
		IDMeasured:  id != "",
		Model:       model,
		Timestamp:   start,
		Correlation: correlation,
		Usage:       u,
		Output:      &transcript.Message{UUID: "out-" + id, Role: transcript.RoleAssistant, Timestamp: start.Add(latency)},
	}
}

var (
	acSeed   = transcript.Usage{Input: 10, CacheCreation: 20_000}
	acBroken = transcript.Usage{Input: 500, CacheRead: 800, CacheCreation: 1_000}
)

// AC1: an overlap the proxy measured is not classified.
//
// PASS: NOT MEASURED, with the reason on the break.
// FAIL: a named cause, which is the confident wrong answer the whole change is
// about — a developer follows it to the wrong sub-agent and spends the
// afternoon there.
func TestAC1_MeasuredOverlapIsNotClassified(t *testing.T) {
	at := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	// Disjoint on the clock and overlapped in fact. That is not a contrived
	// pair: the timestamps are the proxy's own request-start plus a rounded
	// latency, and a retry, a held sibling or a millisecond of rounding is
	// enough to close the gap the clock shows. The counter saw both requests
	// open; the arithmetic on the timestamps does not, and the counter wins.
	lane := &transcript.Lane{ID: "main", Requests: []*transcript.Request{
		corrRequest("req_1", at, 2*time.Second, "claude-opus-5", transcript.CorrelationLaneOverlap, acSeed),
		corrRequest("req_2", at.Add(10*time.Second), 2*time.Second, "claude-opus-5", transcript.CorrelationLaneOverlap, acBroken),
	}}
	breaks := FindBreaks(Calibrate(lane), TokenFit{})
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1", len(breaks))
	}
	if breaks[0].Cause != cachemodel.CauseNotMeasured {
		t.Fatalf("cause = %q, want %q", breaks[0].Cause, cachemodel.CauseNotMeasured)
	}
	if breaks[0].Detail == "" {
		t.Fatal("a refusal to classify still owes the reader the reason for it")
	}
}

// AC2: a transcript carries no correlation field, and the timestamps still
// show the overlap.
//
// The proxy is the only surface that can measure this directly. Declining to
// look at what a transcript does say would make the degrade proxy-only, and
// most sessions this tool reads never went through a proxy.
func TestAC2_OverlapVisibleInTimestampsIsNotClassified(t *testing.T) {
	at := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{ID: "main", Requests: []*transcript.Request{
		// Finishes at +3s; the next one starts at +1s, inside it.
		corrRequest("req_1", at, 3*time.Second, "claude-opus-5", transcript.CorrelationUnmeasured, acSeed),
		corrRequest("req_2", at.Add(time.Second), 2*time.Second, "claude-opus-5", transcript.CorrelationUnmeasured, acBroken),
	}}
	breaks := FindBreaks(Calibrate(lane), TokenFit{})
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1", len(breaks))
	}
	if breaks[0].Cause != cachemodel.CauseNotMeasured {
		t.Fatalf("cause = %q, want %q: the second request began before the first one answered",
			breaks[0].Cause, cachemodel.CauseNotMeasured)
	}
}

// AC3: a lane that ran one request at a time is still classified.
//
// Without this the degrade is unfalsifiable: NOT MEASURED on every break would
// pass AC1 and AC2 and measure nothing.
func TestAC3_DisjointRequestsAreStillClassified(t *testing.T) {
	at := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{ID: "main", Requests: []*transcript.Request{
		corrRequest("req_1", at, 3*time.Second, "claude-opus-5", transcript.CorrelationLaneSerial, acSeed),
		corrRequest("req_2", at.Add(10*time.Second), 2*time.Second, "claude-opus-5", transcript.CorrelationLaneSerial, acBroken),
	}}
	breaks := FindBreaks(Calibrate(lane), TokenFit{})
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1", len(breaks))
	}
	if breaks[0].Cause == cachemodel.CauseNotMeasured {
		t.Fatal("nothing overlapped, so the classifier has a predecessor and owes an answer")
	}
}

// AC4: the proxy's reading beats the inference, in the other direction too.
//
// Where the record says the lane was serial, the timestamps are not consulted.
// They are a weaker instrument — request-start at one end of the wire, a
// rounded latency at the other — and letting them overrule a direct count of
// what was open would put the worse reading in charge.
func TestAC4_ARecordedSerialLaneOverridesTheTimestamps(t *testing.T) {
	at := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{ID: "main", Requests: []*transcript.Request{
		// The clock says these overlap; the proxy counted one at a time.
		corrRequest("req_1", at, 5*time.Second, "claude-opus-5", transcript.CorrelationLaneSerial, acSeed),
		corrRequest("req_2", at.Add(time.Second), 2*time.Second, "claude-opus-5", transcript.CorrelationLaneSerial, acBroken),
	}}
	breaks := FindBreaks(Calibrate(lane), TokenFit{})
	if len(breaks) != 1 {
		t.Fatalf("breaks = %d, want 1", len(breaks))
	}
	if breaks[0].Cause == cachemodel.CauseNotMeasured {
		t.Fatal("the proxy counted one request at a time and the timestamps overruled it")
	}
}
