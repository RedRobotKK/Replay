package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Serving identity is dropped on the floor by the OpenAI-compatible parser.
//
// `system_fingerprint` and `finish_reason` are present in every live DeepSeek
// response in testdata and neither reaches the record. RawUsage does not rescue
// them: it holds the usage object only, and both of these sit outside it, one
// at the top level and one inside each choice.
//
// This matters for two different reasons and they are worth keeping apart. A
// finish reason distinguishes a response that ended because the model was done
// from one that ran out of room, and those are not the same evidence: a
// truncated answer decided nothing. A serving fingerprint identifies the
// configuration that served a request, which is what makes two otherwise
// identical measurements comparable or not.
func TestOpenAIResponseKeepsServingIdentity(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "deepseek", "01-chat-cold.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The fixture is a live capture. Read the expected values out of it rather
	// than hard-coding them, so a re-capture cannot make this test pass against
	// a value the provider no longer sends.
	var probe struct {
		SystemFingerprint string `json:"system_fingerprint"`
		Model             string `json:"model"`
		Choices           []struct {
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.SystemFingerprint == "" || len(probe.Choices) == 0 || probe.Choices[0].FinishReason == "" {
		t.Fatalf("fixture carries no serving identity to test against: %+v", probe)
	}

	got := ParseOpenAIResponse(body)
	if got.ServingFingerprint != probe.SystemFingerprint {
		t.Errorf("ServingFingerprint = %q, want %q from the fixture",
			got.ServingFingerprint, probe.SystemFingerprint)
	}
	if got.FinishReason != probe.Choices[0].FinishReason {
		t.Errorf("FinishReason = %q, want %q from the fixture",
			got.FinishReason, probe.Choices[0].FinishReason)
	}
	if got.ModelReturned != probe.Model {
		t.Errorf("ModelReturned = %q, want %q from the fixture",
			got.ModelReturned, probe.Model)
	}
}

// A response that ran out of room is not a response that finished.
func TestFinishReasonSeparatesTruncationFromCompletion(t *testing.T) {
	for _, tc := range []struct {
		reason    string
		truncated bool
	}{
		{"stop", false},
		{"tool_calls", false},
		{"length", true},
		{"max_tokens", true},
		{"", false}, // NOT_OBSERVED is not truncation
	} {
		r := Response{FinishReason: tc.reason}
		if got := r.Truncated(); got != tc.truncated {
			t.Errorf("finish_reason %q: Truncated() = %v, want %v",
				tc.reason, got, tc.truncated)
		}
	}
}

// Absent identity stays absent. A parser that invents a value would make two
// requests served by different configurations look comparable.
func TestAbsentServingIdentityIsNotInvented(t *testing.T) {
	got := ParseOpenAIResponse([]byte(`{"model":"m","choices":[{"message":{"content":"x"}}],` +
		`"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	if got.ServingFingerprint != "" {
		t.Errorf("ServingFingerprint = %q, want empty when the provider sent none",
			got.ServingFingerprint)
	}
	if got.FinishReason != "" {
		t.Errorf("FinishReason = %q, want empty when the provider sent none", got.FinishReason)
	}
	if got.Truncated() {
		t.Error("an absent finish reason must not read as truncation")
	}
}

// Absent usage and a zero cache read are different facts.
//
// A response that reported no usage was not measured. A response that reported
// a zero cache read was measured and read nothing. Collapsing them lets a run
// claim a cache regime it never observed, which is the defect that produced a
// wrong reading in the DeepSeek campaign before the harness separated them.
func TestCacheStateSeparatesAbsentFromZero(t *testing.T) {
	for _, tc := range []struct {
		name string
		resp Response
		want CacheState
	}{
		{"no usage at all", Response{}, CacheUnknown},
		{"measured, read nothing", Response{Usage: &Usage{Input: 10}}, CacheCold},
		{"measured, read something", Response{Usage: &Usage{CacheRead: 128}}, CacheWarm},
	} {
		if got := tc.resp.CacheState(); got != tc.want {
			t.Errorf("%s: CacheState() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The distinction has to be visible at the call site, not only internally.
func TestCacheUnknownIsNotCold(t *testing.T) {
	if CacheUnknown == CacheCold {
		t.Fatal("unknown and cold must be distinct values")
	}
	if (Response{}).CacheState() == CacheCold {
		t.Error("a response with no usage must not report a cold cache: nothing " +
			"was measured, and cold is a measurement")
	}
}
