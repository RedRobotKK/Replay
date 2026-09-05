package transcript

import (
	"encoding/json"
	"testing"
)

// The counting convention is the whole risk in this adapter. OpenAI reports
// prompt_tokens INCLUSIVE of the cache; this package's Usage is exclusive, with
// Input meaning the uncached remainder. Copying the number across rather than
// subtracting double-counts the cache, and the error is largest on exactly the
// sessions that cache best.
func TestOpenAIUsageIsConvertedFromInclusiveCounting(t *testing.T) {
	var u OpenAIUsage
	mustJSON(t, `{"prompt_tokens":150,"completion_tokens":20,"total_tokens":170,
	              "prompt_tokens_details":{"cached_tokens":50},
	              "completion_tokens_details":{"reasoning_tokens":5}}`, &u)

	got := u.Usage()
	if got.Input != 100 {
		t.Fatalf("Input must be the uncached remainder, 150-50: got %d", got.Input)
	}
	if got.CacheRead != 50 {
		t.Fatalf("CacheRead %d", got.CacheRead)
	}
	if got.CacheCreation != 0 {
		t.Fatalf("this provider reports no separate cache write: %d", got.CacheCreation)
	}
	if got.Output != 20 || got.ThinkingTokens != 5 {
		t.Fatalf("output %d thinking %d", got.Output, got.ThinkingTokens)
	}
	// The invariant that catches a double count: the prompt total this package
	// computes must equal what the provider billed as prompt_tokens.
	if got.PromptTotal() != 150 {
		t.Fatalf("PromptTotal %d, but the provider billed 150 prompt tokens", got.PromptTotal())
	}
}

// No details object means nothing was cached, not that nothing is known.
func TestOpenAIUsageWithoutCacheDetailsIsAllFresh(t *testing.T) {
	var u OpenAIUsage
	mustJSON(t, `{"prompt_tokens":80,"completion_tokens":9}`, &u)
	got := u.Usage()
	if got.Input != 80 || got.CacheRead != 0 || got.PromptTotal() != 80 {
		t.Fatalf("%+v", got)
	}
}

// A provider reporting more cached than prompt is a provider bug or a shape
// this build does not understand. Either way a negative Input would flow into
// every cost figure downstream, so it is clamped and the total is preserved.
func TestMoreCachedThanPromptDoesNotGoNegative(t *testing.T) {
	var u OpenAIUsage
	mustJSON(t, `{"prompt_tokens":100,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":250}}`, &u)
	got := u.Usage()
	if got.Input < 0 {
		t.Fatalf("negative fresh tokens: %d", got.Input)
	}
	if got.PromptTotal() < got.CacheRead {
		t.Fatalf("prompt total %d is below the cache read %d", got.PromptTotal(), got.CacheRead)
	}
}

// A nil receiver is the no-usage case and must be zero, not a panic.
func TestNilOpenAIUsageIsZero(t *testing.T) {
	var u *OpenAIUsage
	if got := u.Usage(); got != (Usage{}) {
		t.Fatalf("%+v", got)
	}
}

// The request side has to be readable enough for the guards to work: a size
// the spend cap can act on and a model the price table can look up.
func TestOpenAIRequestSummarisesWithoutReadingContent(t *testing.T) {
	body := []byte(`{"model":"gpt-x","stream":true,"messages":[
	  {"role":"system","content":"you are helpful"},
	  {"role":"user","content":"hello there"},
	  {"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"Bash","arguments":"{\"cmd\":\"ls\"}"}}]},
	  {"role":"tool","tool_call_id":"c1","content":"file listing"}
	]}`)
	req, err := ParseOpenAIRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "gpt-x" {
		t.Fatalf("model %q", req.Model)
	}
	if !req.Stream {
		t.Fatal("stream flag lost")
	}
	if len(req.Blocks) != 4 {
		t.Fatalf("expected a block per message, got %d", len(req.Blocks))
	}
	// Tool names are needed for loop detection; message text is not kept.
	var sawTool bool
	for _, b := range req.Blocks {
		if b.ToolName == "Bash" {
			sawTool = true
		}
		if b.Text != "" {
			t.Fatalf("message text was retained on a block: %q", b.Text)
		}
	}
	if !sawTool {
		t.Fatal("the tool call's name was not extracted, so loop detection cannot work")
	}
	if req.Bytes <= 0 {
		t.Fatal("no size was measured, so the spend cap has nothing to act on")
	}
}

func mustJSON(t *testing.T, s string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(s), v); err != nil {
		t.Fatal(err)
	}
}
