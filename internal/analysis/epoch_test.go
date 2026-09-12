package analysis

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

func TestAsRunSession_MixedEpochsAreNotOneAsRun(t *testing.T) {
	now := time.Now()
	s := &transcript.Session{ID: "s", Lanes: []*transcript.Lane{{
		ID: "main",
		Requests: []*transcript.Request{
			{ID: "a", IDMeasured: true, Model: "claude-opus-5", Timestamp: now, Epoch: "e1",
				Usage: transcript.Usage{Input: 100, CacheRead: 1000, Output: 1}},
			{ID: "b", IDMeasured: true, Model: "claude-opus-5", Timestamp: now.Add(time.Second), Epoch: "e2",
				Usage: transcript.Usage{Input: 100, CacheRead: 1000, Output: 1}},
		},
	}}}
	got := AsRunSession(s)
	if !got.MixedEpochs {
		t.Fatal("two epochs summed as one as-run; the kernel judge cannot be a silent total")
	}
	if got.CostUSD == 0 {
		t.Fatal("the dollars are still counted; MixedEpochs means they are not one as-run, not that they vanish")
	}
}

func TestAsRunSession_OneEpochIsAsRun(t *testing.T) {
	now := time.Now()
	s := &transcript.Session{ID: "s", Lanes: []*transcript.Lane{{
		ID: "main",
		Requests: []*transcript.Request{
			{ID: "a", IDMeasured: true, Model: "claude-opus-5", Timestamp: now, Epoch: "e1",
				Usage: transcript.Usage{Input: 100, CacheRead: 1000, Output: 1}},
			{ID: "b", IDMeasured: true, Model: "claude-opus-5", Timestamp: now.Add(time.Second), Epoch: "e1",
				Usage: transcript.Usage{Input: 100, CacheRead: 1000, Output: 1}},
		},
	}}}
	got := AsRunSession(s)
	if got.MixedEpochs {
		t.Fatal("a single epoch is as-run")
	}
}

// An unlabelled request is absence, not a third epoch. toolsWireHash returns
// "" when the body has no tools key, which is routine for a first request
// or a tool-free sub-agent lane. Counting that empty string would report
// MixedEpochs on a session that has one labelled epoch and one unlabelled
// request (ADR-0018).
func TestAsRunSession_UnlabelledIsNotASecondEpoch(t *testing.T) {
	now := time.Now()
	s := &transcript.Session{ID: "s", Lanes: []*transcript.Lane{{
		ID: "main",
		Requests: []*transcript.Request{
			{ID: "a", IDMeasured: true, Model: "claude-opus-5", Timestamp: now, Epoch: "e1",
				Usage: transcript.Usage{Input: 100, CacheRead: 1000, Output: 1}},
			{ID: "b", IDMeasured: true, Model: "claude-opus-5", Timestamp: now.Add(time.Second),
				Usage: transcript.Usage{Input: 100, CacheRead: 1000, Output: 1}},
		},
	}}}
	got := AsRunSession(s)
	if got.MixedEpochs {
		t.Fatal("an unlabelled request is absence, not a second epoch")
	}
}
