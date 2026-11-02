package analysis

// Guard-reachability contract test for Tally.WriteAssumed (replay.go:94):
// a priced cache-write leg resting on the assumed short TTL default must be
// counted, and a measured split must not be.

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

func TestTallyWriteAssumedReflectsTTLBasis(t *testing.T) {
	r := &cachemodel.Rules{
		Schema:  cachemodel.RulesSchema,
		Version: "test",
		Models: []cachemodel.ModelRule{
			{Match: "opus-5", MinPrefix: 512, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true},
		},
	}
	defer cachemodel.Override(r)()

	// No provider split supplied: the write leg is priced on the assumed short
	// default, so WriteAssumed must increment.
	var assumed Tally
	assumed.AddAt(transcript.Usage{Input: 1_000_000, CacheCreation: 1_000}, "claude-opus-5", time.Time{})
	if assumed.WriteAssumed != 1 {
		t.Fatalf("assumed-basis write: WriteAssumed = %d, want 1", assumed.WriteAssumed)
	}

	// A provider split supplied: the write leg is measured, so WriteAssumed
	// must not increment.
	var measured Tally
	measured.AddAt(transcript.Usage{Input: 1_000_000, CacheCreation: 1_000, CreateSplitPresent: true, Create5m: 1_000}, "claude-opus-5", time.Time{})
	if measured.WriteAssumed != 0 {
		t.Fatalf("measured-basis write: WriteAssumed = %d, want 0", measured.WriteAssumed)
	}

	// A present-but-inconsistent 0/0 split with positive aggregate writes must
	// also read as assumed (fail closed), so WriteAssumed increments.
	var inconsistent Tally
	inconsistent.AddAt(transcript.Usage{Input: 1_000_000, CacheCreation: 1_000, CreateSplitPresent: true}, "claude-opus-5", time.Time{})
	if inconsistent.WriteAssumed != 1 {
		t.Fatalf("inconsistent 0/0 split: WriteAssumed = %d, want 1", inconsistent.WriteAssumed)
	}
}
