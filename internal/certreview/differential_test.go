// Package certreview is an INDEPENDENT certification review surface. It does
// NOT modify any candidate implementation; it only exercises public APIs to
// compare the specialized production is_error scanner (Bundle D) against the
// rigorous occurrence verifier (Bundle A) for their shared supported domain.
package certreview

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/evidenceverify"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// buildTranscript wraps one tool_result content block (a bare object, no
// surrounding array) in a minimal Claude Code JSONL transcript.
func buildTranscript(t *testing.T, toolResult string) *transcript.Session {
	t.Helper()
	src := `{"type":"user","uuid":"u0","timestamp":"2026-01-01T00:00:00.000Z","message":{"role":"user","content":[` +
		toolResult + `]}}` + "\n" +
		`{"type":"assistant","uuid":"a0","parentUuid":"u0","timestamp":"2026-01-01T00:00:01.000Z","requestId":"req0","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"x"}],"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":10}}}` + "\n"
	s, err := transcript.ParseClaudeCode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return s
}

// toolResultEvidence extracts the Evidence flag of the first tool_result block.
func toolResultEvidence(t *testing.T, s *transcript.Session) *transcript.ErrorFlag {
	t.Helper()
	for _, l := range s.Lanes {
		for _, r := range l.Requests {
			for _, m := range r.Context {
				for _, b := range m.Blocks {
					if b.Kind == transcript.KindToolResult {
						return b.Evidence
					}
				}
			}
		}
	}
	t.Fatalf("no tool_result block found")
	return nil
}

// referenceVerdict computes the rigorous verifier's answer for "does this
// tool_result block record is_error == true", pinning the first is_error
// occurrence (whatever its value). content is a bare object; it is wrapped.
func referenceVerdict(t *testing.T, content string) evidenceverify.Verdict {
	t.Helper()
	raw := []byte("[" + content + "]")
	idx := bytes.Index(raw, []byte(`"is_error"`))
	if idx < 0 {
		t.Fatalf("fixture has no is_error key: %q", content)
	}
	colon := bytes.IndexByte(raw[idx:], ':')
	valOff := idx + colon + 1
	for valOff < len(raw) && (raw[valOff] == ' ' || raw[valOff] == '\t' || raw[valOff] == '\n' || raw[valOff] == '\r') {
		valOff++
	}
	loc, ok := evidenceverify.LocateValue(raw, int64(valOff))
	if !ok {
		t.Fatalf("LocateValue failed at %d for %q", valOff, content)
	}
	r := evidenceverify.Verify(raw, loc, evidenceverify.Predicate{Key: "is_error", Expect: evidenceverify.BoolLiteral(true)})
	return r.Verdict
}

func flagName(f *transcript.ErrorFlag) string {
	if f == nil {
		return "<nil>"
	}
	switch *f {
	case transcript.ErrorFlagNone:
		return "None"
	case transcript.ErrorFlagAbsent:
		return "Absent"
	case transcript.ErrorFlagFalse:
		return "False"
	case transcript.ErrorFlagTrue:
		return "True"
	case transcript.ErrorFlagConflict:
		return "Conflict"
	case transcript.ErrorFlagNonBoolean:
		return "NonBoolean"
	default:
		return "??"
	}
}

// TestDifferentialEquivalence compares specialized scanner output against the
// rigorous verifier for the shared supported domain.
func TestDifferentialEquivalence(t *testing.T) {
	cases := []struct {
		name     string
		content  string // bare tool_result object (no surrounding array)
		wantFlag string // expected specialized scanner flag
		wantVerd evidenceverify.Verdict
	}{
		{"unique_true", `{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":"x"}`, "True", evidenceverify.Confirmed},
		{"unique_false", `{"type":"tool_result","tool_use_id":"t1","is_error":false,"content":"x"}`, "False", evidenceverify.Refuted},
		{"absent", `{"type":"tool_result","tool_use_id":"t1","content":"x"}`, "Absent", 0}, // verifier has no basis
		{"true_true", `{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":true,"content":"x"}`, "True", evidenceverify.Confirmed},
		{"false_false", `{"type":"tool_result","tool_use_id":"t1","is_error":false,"is_error":false,"content":"x"}`, "False", evidenceverify.Refuted},
		{"true_false", `{"type":"tool_result","tool_use_id":"t1","is_error":true,"is_error":false,"content":"x"}`, "Conflict", evidenceverify.Ambiguous},
		{"false_true", `{"type":"tool_result","tool_use_id":"t1","is_error":false,"is_error":true,"content":"x"}`, "Conflict", evidenceverify.Ambiguous},
		{"escaped_key", `{"type":"tool_result","tool_use_id":"t1","\u0069s_error":true,"content":"x"}`, "True", evidenceverify.Confirmed},
		// Non-boolean values — the divergence candidates.
		{"null_value", `{"type":"tool_result","tool_use_id":"t1","is_error":null,"content":"x"}`, "NonBoolean", evidenceverify.Refuted},
		{"null_then_true", `{"type":"tool_result","tool_use_id":"t1","is_error":null,"is_error":true,"content":"x"}`, "Conflict", evidenceverify.Ambiguous},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := buildTranscript(t, tc.content)
			got := toolResultEvidence(t, s)
			gotName := flagName(got)
			if gotName != tc.wantFlag {
				t.Errorf("specialized scanner flag = %s, want %s", gotName, tc.wantFlag)
			}
			if bytes.Contains([]byte(tc.content), []byte(`"is_error"`)) {
				v := referenceVerdict(t, tc.content)
				if v != tc.wantVerd {
					t.Errorf("reference verifier = %s, want %s", v, tc.wantVerd)
				}
			}
		})
	}
}

// TestNullMixedConflict is the focused RED reproducer for the divergence.
func TestNullMixedConflict(t *testing.T) {
	s := buildTranscript(t, `{"type":"tool_result","tool_use_id":"t1","is_error":null,"is_error":true,"content":"x"}`)
	got := toolResultEvidence(t, s)
	if got != nil && *got == transcript.ErrorFlagTrue {
		t.Errorf("DIVERGENCE: specialized scanner reports True for null/true conflict; rigorous verifier says AMBIGUOUS")
	}
	if got != nil && *got == transcript.ErrorFlagConflict {
		t.Logf("specialized scanner correctly reports Conflict (no divergence)")
	}
}
