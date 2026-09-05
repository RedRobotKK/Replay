package ledger

import "testing"

// The measured tier depends on this: usage off the wire, converted rather than
// copied, with the raw payload kept beside it.
func TestParseOpenAIResponseKeepsUsageAndRaw(t *testing.T) {
	body := []byte(`{"model":"gpt-x","choices":[{"message":{"content":"ok",
	  "tool_calls":[{"function":{"name":"Bash"}}]}}],
	  "usage":{"prompt_tokens":150,"completion_tokens":20,
	           "prompt_tokens_details":{"cached_tokens":50},
	           "future_field":"keep me"}}`)
	got := ParseOpenAIResponse(body)
	if got.Usage == nil {
		t.Fatal("no usage parsed, so nothing about this request is measured")
	}
	if got.Usage.Input != 100 || got.Usage.CacheRead != 50 {
		t.Fatalf("inclusive counting was copied rather than converted: %+v", got.Usage)
	}
	if got.Usage.PromptTotal() != 150 {
		t.Fatalf("prompt total %d must match what the provider billed", got.Usage.PromptTotal())
	}
	if len(got.RawUsage) == 0 {
		t.Fatal("the raw usage payload was dropped")
	}
	if len(got.Blocks) != 2 {
		t.Fatalf("expected a text block and a tool call, got %d", len(got.Blocks))
	}
}

// A body this build cannot read must produce an empty response rather than a
// half-filled one that reads as measured.
func TestParseOpenAIResponseOnGarbageIsEmpty(t *testing.T) {
	if got := ParseOpenAIResponse([]byte("not json")); got.Usage != nil || len(got.Blocks) > 0 {
		t.Fatalf("%+v", got)
	}
}

// The summary has to give the guards something to act on.
func TestSummarizeOpenAIRequestFeedsTheGuards(t *testing.T) {
	body := []byte(`{"model":"gpt-x","messages":[
	  {"role":"system","content":"sys"},
	  {"role":"assistant","tool_calls":[{"id":"c1","function":{"name":"Bash","arguments":"{\"cmd\":\"ls\"}"}}]}]}`)
	sum, err := SummarizeOpenAIRequest(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Model != "gpt-x" {
		t.Fatalf("model %q", sum.Model)
	}
	if len(sum.Prompt.Messages) != 2 {
		t.Fatalf("messages %d", len(sum.Prompt.Messages))
	}
	var key string
	for _, m := range sum.Prompt.Messages {
		for _, b := range m.Blocks {
			if b.CallKey != "" {
				key = b.CallKey
			}
			if b.Text != "" {
				t.Fatalf("message text reached the ledger summary: %q", b.Text)
			}
		}
	}
	if key == "" {
		t.Fatal("no CallKey, so the loop detector cannot see a repeated call")
	}
}
