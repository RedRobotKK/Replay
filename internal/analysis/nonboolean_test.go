package analysis

import (
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Permanent RED-1 regression at the production boundary: a present non-boolean
// is_error value must never become a verified error or verified clean. The
// only non-boolean value encoding/json will accept into RawBlock.IsError is
// null, so that is the case that reaches the production path; the others are
// rejected earlier but must not regress either.
func TestNonBooleanIsErrorBecomesUnresolved(t *testing.T) {
	// null,true — the original divergence reproducer.
	s := syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":null,"is_error":true,"content":"boom"}`)
	costs, ev := errorCostsFor(t, s)
	if ev.Unresolved != 1 {
		t.Fatalf("null,true: unresolved=%d, want 1 (costs=%+v)", ev.Unresolved, costs)
	}
	for _, c := range costs {
		if c.Count > 0 && (c.Class == ErrorToolFailed || c.Class == ErrorEditAnchor) {
			t.Fatalf("null,true counted as verified error: %+v", c)
		}
	}

	// null alone — must be unresolved, not clean, not error.
	s = syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":null,"content":"boom"}`)
	costs, ev = errorCostsFor(t, s)
	if ev.Unresolved != 1 {
		t.Fatalf("null alone: unresolved=%d, want 1", ev.Unresolved)
	}
	for _, c := range costs {
		if c.Count > 0 {
			t.Fatalf("null alone counted in an error class: %+v", c)
		}
	}
}

// Legacy false must never become verified false: a block with no evidence
// carrier reads its decoded bool but is labeled Legacy.
func TestLegacyFalseIsNotVerifiedFalse(t *testing.T) {
	b := transcript.Block{Kind: transcript.KindToolResult, Bytes: 1, Text: "x", IsError: false, Evidence: nil}
	_, known := evidenceErrorState(b)
	if known {
		t.Fatalf("nil evidence must report unknown")
	}
}
