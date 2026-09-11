package analysis

import (
	"os"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Turn.Gap is start-to-start, and these pin what "start" is on each source.
//
// The gap decides cachemodel.CauseTTLExpired, which is published as a certain
// cause. It is only as good as the instants it subtracts, and the two readers
// hand it instants with different meanings:
//
//   - Ledger: Request.Timestamp is the proxy's own clock at the moment the
//     request arrived (internal/proxy/server.go:490 -> ledger/store.go:276),
//     and Output.Timestamp is that plus the measured latency
//     (ledger/store.go:310). A request duration is recoverable.
//   - Transcript: Request.Timestamp is the assistant line's timestamp, which
//     approximates response COMPLETION — docs/design-review-2026-09-02.md:33
//     (risk R6) says so and accepts the error. Output.Timestamp is the same
//     line's timestamp. No duration is recoverable at all.
//
// Nothing in the code said which of the two Gap was being fed, so these tests
// exist to make the difference impossible to erase by accident. They do not
// correct the gap: re-anchoring it would move every published break-cause
// figure, and no measurement here says the other anchor is better.

// The ledger carries a request duration, so its Timestamp is a start.
func TestLedgerRequestTimestampIsAStartAndTheOutputIsTheEnd(t *testing.T) {
	start := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	b := ledger.NewSessionBuilder("clock-domain", "clock-domain.jsonl")
	b.Add(ledger.Record{
		Timestamp: start,
		LatencyMS: 4200,
		SessionID: "s1",
		RequestSummary: ledger.RequestSummary{
			Model:  "claude-opus-5",
			Prompt: ledger.Prompt{Messages: []ledger.Message{{Role: transcript.RoleUser}}},
		},
		Response: ledger.Response{Usage: &transcript.Usage{Input: 10, CacheCreation: 100}},
	})
	sess := b.Session()
	req := sess.Lanes[0].Requests[0]

	if !req.Timestamp.Equal(start) {
		t.Errorf("Request.Timestamp = %s, want the record's arrival instant %s", req.Timestamp, start)
	}
	if req.Output == nil {
		t.Fatal("no output message; the ledger records one")
	}
	if got, want := req.Output.Timestamp.Sub(req.Timestamp), 4200*time.Millisecond; got != want {
		t.Errorf("output minus request = %s, want the record's latency %s. If these are equal, "+
			"the ledger has stopped carrying a request duration and is now as blind as a "+
			"transcript — which is the thing the next test says a transcript is", got, want)
	}
}

// A transcript carries no request duration, so its Timestamp is not a start.
//
// Both instants come from the same assistant line: claudecode.go:228 sorts the
// group by API block index, buildRequest takes group[0].Timestamp, and
// decodeAssistantRun re-sorts the same slice and takes run[0].Timestamp. They
// are the same line, so they are the same instant, always.
//
// Two things follow, and neither is written down anywhere else. Turn.Gap on
// this source is completion-to-completion, so it carries the difference of two
// response durations — it is NOT the start-to-start interval its own field
// comment used to claim. And correlationOf's transcript fallback, which asks
// whether a request began before its predecessor had answered, reduces here to
// asking whether the lane's timestamps run backwards: it cannot see an overlap,
// only a file written out of order.
func TestTranscriptRequestsCarryNoDuration(t *testing.T) {
	f, err := os.Open("../transcript/testdata/session-redacted.jsonl")
	if err != nil {
		t.Fatalf("open the recorded session: %v", err)
	}
	defer f.Close() //nolint:errcheck // read-only fixture
	sess, err := transcript.ParseClaudeCode(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	checked := 0
	for _, lane := range sess.Lanes {
		for i, req := range lane.Requests {
			if req.Output == nil {
				continue
			}
			checked++
			if !req.Output.Timestamp.Equal(req.Timestamp) {
				t.Fatalf("lane %s request %d: output %s is a different instant from the request %s. "+
					"A transcript stamps one line; if these now differ, a duration has become "+
					"recoverable and Turn.Gap can be anchored on the previous response instead "+
					"of guessing", lane.ID, i, req.Output.Timestamp, req.Timestamp)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no request with an output was examined, so this test proved nothing")
	}
}

// Gap subtracts the two Request.Timestamps and nothing else.
//
// Whichever instant a source puts there, Gap is that instant's difference. This
// pins the arithmetic so that re-anchoring it on the previous response — the
// change this audit deliberately did not make — cannot happen silently.
func TestGapIsTheDifferenceOfRequestTimestamps(t *testing.T) {
	t0 := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{ID: "l"}
	for _, at := range []time.Time{t0, t0.Add(90 * time.Second)} {
		req := &transcript.Request{
			Timestamp: at,
			Model:     "claude-opus-5",
			Usage:     transcript.Usage{Input: 10, CacheCreation: 1000},
		}
		// A response that took a full minute. Anchoring the gap on it would
		// give 30s instead of 90s, which is the whole point of pinning this.
		req.Output = &transcript.Message{Role: transcript.RoleAssistant, Timestamp: at.Add(60 * time.Second)}
		lane.Requests = append(lane.Requests, req)
	}

	cal := Calibrate(lane)
	if got, want := cal.Turns[1].Gap, 90*time.Second; got != want {
		t.Errorf("Gap = %s, want %s: Gap is start-of-record to start-of-record. %s would mean "+
			"it had been re-anchored on the previous response, which moves every TTL-expiry "+
			"verdict in every published break-cause table", got, want, 30*time.Second)
	}
}
