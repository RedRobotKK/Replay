package learn

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// syntheticChain builds a two-request Claude Code transcript so calibration
// passes (one compared turn) and the error-classification evidence is real.
// The first user message carries the given tool_result content block.
func syntheticChain(t *testing.T, toolResult string) *transcript.Session {
	t.Helper()
	src := `{"type":"user","uuid":"u0","timestamp":"2026-01-01T00:00:00.000Z","message":{"role":"user","content":[` + toolResult + `]}}` + "\n" +
		`{"type":"assistant","uuid":"a0","parentUuid":"u0","timestamp":"2026-01-01T00:00:01.000Z","requestId":"req0","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"x"}],"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":10}}}` + "\n" +
		`{"type":"user","uuid":"u1","parentUuid":"a0","timestamp":"2026-01-01T00:00:02.000Z","message":{"role":"user","content":"second turn"}}` + "\n" +
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","timestamp":"2026-01-01T00:00:03.000Z","requestId":"req1","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"y"}],"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":10,"cache_creation_input_tokens":10}}}` + "\n"
	s, err := transcript.ParseClaudeCode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Lanes) != 1 || len(s.Lanes[0].Requests) < 2 {
		t.Fatalf("synthetic chain produced %d lanes/%d requests, want >=1/2", len(s.Lanes), len(s.Lanes[0].Requests))
	}
	return s
}

// RED-2: the learning outcome must carry the error-classification uncertainty
// so a graduation cannot read a precise ErrorShare over an incomplete basis.
func TestScoreCarriesErrorEvidence(t *testing.T) {
	t.Run("conflict_flows", func(t *testing.T) {
		s := syntheticChain(t, `{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"boom"}`)
		sc, ok := Score(s, Catalog())
		if !ok {
			t.Fatalf("synthetic chain must calibrate")
		}
		if sc.ErrorUnresolved != 1 {
			t.Fatalf("conflicting duplicate: ErrorUnresolved=%d, want 1", sc.ErrorUnresolved)
		}
		if sc.ErrorLegacy != 0 {
			t.Fatalf("conflicting duplicate: ErrorLegacy=%d, want 0", sc.ErrorLegacy)
		}
	})

	t.Run("clean_fixture_has_no_false_positive", func(t *testing.T) {
		s, err := transcript.ParseClaudeCodeFile("../transcript/testdata/session-redacted.jsonl")
		if err != nil {
			t.Fatal(err)
		}
		sc, ok := Score(s, Catalog())
		if !ok {
			t.Fatal("fixture must calibrate")
		}
		if sc.ErrorUnresolved != 0 {
			t.Fatalf("clean fixture: ErrorUnresolved=%d, want 0", sc.ErrorUnresolved)
		}
		if sc.ErrorLegacy != 0 {
			t.Fatalf("clean fixture: ErrorLegacy=%d, want 0 (Claude Code supplies evidence)", sc.ErrorLegacy)
		}
	})
}
