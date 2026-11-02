package analysis

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// syntheticTranscript builds a minimal Claude Code JSONL transcript whose user
// message carries one tool_result block, followed by an assistant line that
// turns it into a request. toolResult is the raw JSON for one content block.
func syntheticTranscript(t *testing.T, toolResult string) *transcript.Session {
	t.Helper()
	src := `{"type":"user","uuid":"u0","timestamp":"2026-01-01T00:00:00.000Z","message":{"role":"user","content":[` +
		toolResult + `]}}` + "\n" +
		`{"type":"assistant","uuid":"a0","parentUuid":"u0","timestamp":"2026-01-01T00:00:01.000Z","requestId":"req0","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"x"}],"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":10}}}` + "\n"

	s, err := transcript.ParseClaudeCode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("synthetic transcript did not parse: %v", err)
	}
	if len(s.Lanes) != 1 || len(s.Lanes[0].Requests) != 1 {
		t.Fatalf("synthetic transcript parsed to %d lanes / %d requests, want 1/1", len(s.Lanes), len(s.Lanes[0].Requests))
	}
	return s
}

func errorCostsFor(t *testing.T, s *transcript.Session) ([]ErrorCost, ErrorEvidence) {
	t.Helper()
	lane := MainLane(s)
	cal := Calibrate(lane)
	fit := Fit(cal, s.Source.PrefixVisible())
	return ErrorCosts(cal, fit)
}

// TestErrorCostsDuplicateKeyTrap is the RED-then-GREEN production test.
//
// A tool_result block recording {"is_error":true,"is_error":false} is a
// conflicting duplicate. encoding/json collapses it last-wins to IsError=false,
// and the old classifyResult read that as "clean", silently dropping the
// error-flag conflict. The evidence-aware path must surface it as unresolved,
// never as clean and never as a verified error.
func TestErrorCostsDuplicateKeyTrap(t *testing.T) {
	s := syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"boom"}`)
	costs, ev := errorCostsFor(t, s)

	if ev.Unresolved != 1 {
		t.Fatalf("duplicate-key conflict not surfaced: unresolved=%d, want 1 (costs=%+v)", ev.Unresolved, costs)
	}
	for _, c := range costs {
		if c.Class == ErrorToolFailed || c.Class == ErrorEditAnchor {
			t.Fatalf("conflicting duplicate was counted as a verified error: %+v", c)
		}
	}
}

// TestErrorCostsFourEvidenceStates exercises the evidence-aware path end to end
// through the production ErrorCosts function for the four evidence states.
func TestErrorCostsFourEvidenceStates(t *testing.T) {
	t.Run("P1_unique_true", func(t *testing.T) {
		s := syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"boom"}`)
		costs, ev := errorCostsFor(t, s)
		if ev.Unresolved != 0 || ev.Legacy != 0 {
			t.Fatalf("verified error mislabeled: %+v", ev)
		}
		if !hasClass(costs, ErrorToolFailed) {
			t.Fatalf("unique true not classified as a tool error: %+v", costs)
		}
	})
	t.Run("P2_unique_false", func(t *testing.T) {
		s := syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":false,"content":"ok"}`)
		costs, ev := errorCostsFor(t, s)
		if hasClass(costs, ErrorToolFailed) {
			t.Fatalf("unique false classified as error: %+v", costs)
		}
		if ev.Unresolved != 0 {
			t.Fatalf("verified false counted as unresolved: %+v", ev)
		}
	})
	t.Run("P3_conflicting_duplicate", func(t *testing.T) {
		s := syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"boom"}`)
		costs, ev := errorCostsFor(t, s)
		if ev.Unresolved != 1 {
			t.Fatalf("conflict not unresolved: %+v", ev)
		}
		if hasClass(costs, ErrorToolFailed) {
			t.Fatalf("conflict counted as error: %+v", costs)
		}
	})
	t.Run("P4_absent_is_error", func(t *testing.T) {
		s := syntheticTranscript(t, `{"type":"tool_result","tool_use_id":"t1","content":"ok"}`)
		costs, ev := errorCostsFor(t, s)
		if hasClass(costs, ErrorToolFailed) {
			t.Fatalf("absent is_error classified as error: %+v", costs)
		}
		if ev.Unresolved != 0 {
			t.Fatalf("absent is_error counted as unresolved: %+v", ev)
		}
	})
}

func hasClass(costs []ErrorCost, class ErrorClass) bool {
	for _, c := range costs {
		if c.Class == class {
			return true
		}
	}
	return false
}

// TestLegacyDecodedPathStillClassifies proves ordinary decoded behavior is
// preserved: a Block with no evidence still classifies by IsError, but is
// labeled legacy rather than forensically verified.
func TestLegacyDecodedPathStillClassifies(t *testing.T) {
	lane := &transcript.Lane{
		Requests: []*transcript.Request{{
			Context: []*transcript.Message{{
				UUID: "m0",
				Role: transcript.RoleUser,
				Blocks: []transcript.Block{{
					Kind: transcript.KindToolResult, Label: "tool result: x",
					Bytes: 4, Text: "boom", IsError: true,
				}},
			}},
		}},
	}
	cal := Calibrate(lane)
	fit := Fit(cal, false)
	costs, ev := ErrorCosts(cal, fit)
	if !hasClass(costs, ErrorToolFailed) {
		t.Fatalf("legacy decoded IsError=true not classified: %+v", costs)
	}
	if ev.Legacy != 1 {
		t.Fatalf("legacy classification not labeled legacy: %+v", ev)
	}
	if ev.Unresolved != 0 {
		t.Fatalf("legacy path wrongly counted unresolved: %+v", ev)
	}
}
