package ledger

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The ledger is where an order-based join is introduced rather than merely
// inherited.
//
// A record without a provider request id is given `ledger-<n>`, where n is the
// record's position in the file. That string is a real id's shape and a real
// id's type, and every consumer downstream — the cost command deduplicates on
// it — treats the two alike. ADR-0018 rule 1: provenance is a field, not a
// naming convention, because a naming convention is a comment that happens to
// be executable.

func usageRecord(id string, at time.Time, correlation string) Record {
	u := transcript.Usage{Input: 10, CacheCreation: 100}
	return Record{
		Timestamp:      at,
		SessionID:      "s1",
		RequestID:      id,
		Correlation:    correlation,
		LatencyMS:      250,
		RequestSummary: RequestSummary{Model: "claude-opus-5", Prompt: Prompt{Messages: []Message{{Role: "user", Blocks: []Block{{Kind: "text", Bytes: 4}}}}}},
		Response:       Response{Usage: &u},
	}
}

// LJ1: a synthesised id is not a provider id, and the type says so.
//
// PASS: the request carries whether its id was measured.
// FAIL: `ledger-0` is indistinguishable from `req_011CenqQ...`, so a consumer
// joining on it joins two different sessions' first requests together.
func TestLJ1_SynthesisedIDIsMarkedUnmeasured(t *testing.T) {
	at := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)

	b := NewSessionBuilder("s1", "s1.jsonl")
	b.Add(usageRecord("req_provider", at, CorrelationLaneSerial))
	b.Add(usageRecord("", at.Add(time.Second), CorrelationLaneSerial))
	reqs := b.Session().Lanes[0].Requests
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}
	if !reqs[0].IDMeasured {
		t.Errorf("a provider request id is a measurement and must be marked as one")
	}
	if reqs[1].IDMeasured {
		t.Errorf("id %q was synthesised from the record's position in the file and is reported as measured", reqs[1].ID)
	}
	if reqs[1].ID == "" {
		t.Errorf("a record still needs an identifier within its own file")
	}
}

// LJ2: the correlation the proxy measured survives the round trip.
//
// The proxy is the only thing that can know two requests were open at once;
// the offline reader cannot recover it from a file that does not carry it.
// This is the join ADR-0018's consequences section names: a field nothing
// reads is the built-but-unwired shape.
func TestLJ2_CorrelationReachesTheAnalysisShape(t *testing.T) {
	at := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	b := NewSessionBuilder("s1", "s1.jsonl")
	b.Add(usageRecord("req_1", at, CorrelationLaneOverlap))
	reqs := b.Session().Lanes[0].Requests
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	if reqs[0].Correlation != transcript.CorrelationLaneOverlap {
		t.Fatalf("correlation = %q, want %q: the proxy measured the overlap and the reader dropped it",
			reqs[0].Correlation, transcript.CorrelationLaneOverlap)
	}
}
