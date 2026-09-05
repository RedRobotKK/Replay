package ledger

import (
	"strings"
	"testing"
)

// Record.RawUsage is documented as "the provider's own usage object, verbatim
// and unparsed", and the multi-provider design doc gives the reason: "a field
// we did not know mattered is exactly what tomorrow's calibration needs."
//
// It was re-marshalled from the typed struct instead, so every field the struct
// does not declare was silently discarded. Caught on 2026-09-05 against live
// DeepSeek, which reports prompt_cache_hit_tokens and prompt_cache_miss_tokens
// alongside the OpenAI-shaped prompt_tokens_details.cached_tokens. Those are
// precisely fields we did not know mattered, and they were dropped.
func TestOpenAIRawUsageKeepsUnknownFields(t *testing.T) {
	body := []byte(`{"id":"x","object":"chat.completion","choices":[],
		"usage":{"prompt_tokens":14010,"completion_tokens":1,"total_tokens":14011,
		"prompt_tokens_details":{"cached_tokens":13952},
		"prompt_cache_hit_tokens":13952,"prompt_cache_miss_tokens":58}}`)

	resp := ParseOpenAIResponse(body)
	if resp.RawUsage == nil {
		t.Fatal("RawUsage is nil")
	}
	raw := string(resp.RawUsage)

	for _, field := range []string{"prompt_cache_hit_tokens", "prompt_cache_miss_tokens"} {
		if !strings.Contains(raw, field) {
			t.Errorf("RawUsage dropped %q; it is documented as verbatim and unparsed.\ngot: %s",
				field, raw)
		}
	}

	// The parsed side must still be right: inclusive prompt_tokens minus the
	// cached figure is the fresh remainder.
	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if got := resp.Usage.Input; got != 58 {
		t.Errorf("fresh tokens: got %d, want 58 (14010 inclusive - 13952 cached)", got)
	}
	if got := resp.Usage.CacheRead; got != 13952 {
		t.Errorf("cache read: got %d, want 13952", got)
	}
}
