package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

// What an absent reasoning figure actually becomes, pinned rather than assumed.
//
// docs/PRODUCTION-WIRING.md classifies every field per surface, and a
// classification nobody checks is a sentence. This is the check for the one
// field on the DeepSeek surface whose class is ASSUMED rather than OBSERVED.
//
// DeepSeek omits completion_tokens_details entirely on a non-reasoner chat
// call and reports it on the reasoner. The parser maps the absent case to
// zero. For this field that is very probably the true value, because a model
// that did not reason produced no reasoning tokens, and it costs nothing
// either way: reasoning is a SHARE of completion_tokens, which is observed and
// billed on its own. So the zero changes no figure a user acts on.
//
// It is still an assumption and not a measurement, and the difference is the
// one this project keeps having to relearn. The same shape on cache writes is
// exactly the Codex defect: a counter absent because there was nothing to
// count is observationally identical to a counter the client dropped. Here the
// consequence is confined to a breakdown; there it was money.
//
// PASS: the absent case reads as zero and the present case reads the value.
// FAIL: either the absent case starts carrying a fabricated figure, or the
// present case stops being read, and the matrix row is then wrong.
func TestDS_AbsentReasoningReadsAsZeroAndThatIsAnAssumption(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		want    int
		why     string
	}{
		{"01-chat-cold.json", 0, "no completion_tokens_details on a non-reasoner call"},
		{"02-chat-warm.json", 0, "no completion_tokens_details on a non-reasoner call"},
		{"04-reasoner.json", 8, "completion_tokens_details.reasoning_tokens is present"},
	} {
		body, err := os.ReadFile(filepath.Join("testdata", "deepseek", tc.fixture))
		if err != nil {
			t.Fatalf("%s: %v", tc.fixture, err)
		}
		resp := ParseOpenAIResponse(body)
		if resp.Usage == nil {
			t.Fatalf("%s: usage did not parse", tc.fixture)
		}
		if got := resp.Usage.ThinkingTokens; got != tc.want {
			t.Errorf("%s: reasoning = %d, want %d (%s)", tc.fixture, got, tc.want, tc.why)
		}
	}
}
