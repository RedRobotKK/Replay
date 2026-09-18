package cachemodel

import (
	"strings"
	"testing"
)

// A verdict may not assert a cause the function cannot observe.
//
// ClassifyCounters takes two int64s. A field the client never sent and a field
// it sent as zero both arrive here as 0, and nothing downstream of that
// conversion can separate them. CountersWriteMissing nevertheless read "the
// write field is absent rather than zero", which is a claim about the wire
// made by a function that never saw it.
//
// It was true when written. On 2026-09-15 exactly one of 158 Codex rollouts
// carried `cache_write_input_tokens` at all. Measured again on 2026-09-17
// across the same corpus grown to 176 files: the field is present in 6,883 of
// 6,883 token_count records and is zero in every one. The wire changed and the
// sentence did not, so `replay burn` told every Codex user that a field they
// were in fact sending was missing.
//
// The impossibility itself stands. Reads above zero beside writes of exactly
// zero cannot describe a real session: something wrote the prefix being read.
// That is what this verdict is for, and it is all it may say.
func TestAVerdictDoesNotAssertWhatTheCountersCannotShow(t *testing.T) {
	for _, c := range []Counters{
		CountersInstrumented, CountersWriteMissing, CountersColdOnly,
		CountersSilent, CountersUnreadable,
	} {
		if strings.Contains(string(c), "absent") {
			t.Errorf("verdict %q claims a field was absent; ClassifyCounters sees "+
				"two integers and cannot tell an unsent field from a zero one", c)
		}
	}
}

// The impossibility must survive the rewording. This is the half that is real.
func TestReadsWithoutWritesRemainImpossible(t *testing.T) {
	if got := ClassifyCounters(1, 0); !got.Impossible() {
		t.Errorf("ClassifyCounters(1, 0) = %q, which must stay impossible: "+
			"something wrote the prefix being read", got)
	}
	if got := ClassifyCounters(363472768, 0); !got.Impossible() {
		t.Errorf("a corpus-scale read count with no writes stopped being impossible: %q", got)
	}
	for _, tc := range []struct{ read, write int64 }{{0, 0}, {0, 5}, {5, 5}} {
		if ClassifyCounters(tc.read, tc.write).Impossible() {
			t.Errorf("ClassifyCounters(%d, %d) must not be impossible", tc.read, tc.write)
		}
	}
}
