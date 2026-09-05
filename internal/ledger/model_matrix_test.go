package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Per-model conformance, captured live on 2026-09-05 across every model
// api.deepseek.com will answer to.
//
// The provider suite next door asks "does Replay read this shape correctly".
// This asks a different question: "does every model behave the same way, and
// where does one of them differ". A model that reports cache or reasoning
// differently from its siblings is a silently wrong number, not a crash, and
// nothing else in the suite would notice.
//
// PASS/FAIL conditions, applied to every model:
//
//	M1  a cold call reports hit == 0 and miss == prompt_tokens
//	M2  a warm call on the same prefix reports hit > 0
//	M3  every cache hit is a whole number of 64-token blocks
//	M4  reasoning tokens appear if and only if the model is a reasoning model
//	M5  tool calling is answered with a tool_calls reply
//
// M3 is the one worth reading twice. It is not documented anywhere: it was
// measured, it held on 8 of 8 captures spanning four orders of prompt size,
// and it is exactly the kind of claim `cachemodel.Claim` exists to carry as
// observed-but-unverified rather than believed.

// cacheBlockTokens is the largest value the evidence supports, not a published
// figure and not a guess.
//
// Every observed cache hit — 256, 2304, 2432, 9600 and 14080 across ten
// captures — is a multiple of 128, and 128 is their greatest common divisor.
// The true block size therefore DIVIDES 128; it could be 64 or 32 and this
// corpus could not tell. 128 is used because it is the tightest constraint the
// data actually carries: a smaller constant would pass on inputs that should
// fail, which is how the first version of this test was written and why it
// could not distinguish 64 from 128 at all.
//
// A future capture that is not a multiple of 128 is a finding about the
// provider. Record it and lower this to the new GCD. Do not widen it to make
// a red test go green.
const cacheBlockTokens = 128

type modelExpectation struct {
	name      string
	reasoning bool // M4: does this model report reasoning tokens at all
}

var deepSeekModels = []modelExpectation{
	{name: "deepseek-v4-pro", reasoning: true},
	{name: "deepseek-v4-flash", reasoning: true},
	{name: "deepseek-v4-flash-vision-exp", reasoning: true},
	{name: "deepseek-reasoner", reasoning: true},
	// The odd one out, and the reason this file exists. deepseek-chat reports
	// no reasoning tokens at all where its four siblings do.
	{name: "deepseek-chat", reasoning: false},
}

type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	Details          struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
	CacheHit  int `json:"prompt_cache_hit_tokens"`
	CacheMiss int `json:"prompt_cache_miss_tokens"`
}

func modelUsage(t *testing.T, model, pass string) (wireUsage, []byte) {
	t.Helper()
	path := filepath.Join("testdata", "deepseek", "models", model+"-"+pass+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture %s: %v", path, err)
	}
	var env struct {
		Usage   *wireUsage `json:"usage"`
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if env.Usage == nil {
		t.Fatalf("%s: no usage object", path)
	}
	return *env.Usage, b
}

func TestModelMatrix(t *testing.T) {
	for _, m := range deepSeekModels {
		t.Run(m.name, func(t *testing.T) {
			cold, _ := modelUsage(t, m.name, "cold")
			warm, _ := modelUsage(t, m.name, "warm")

			// M1. A cold call has nothing cached.
			if cold.CacheHit != 0 {
				t.Errorf("M1 cold call reports %d cached tokens; the prefix was "+
					"nonce-salted and cannot have been seen before", cold.CacheHit)
			}
			if cold.CacheMiss != cold.PromptTokens {
				t.Errorf("M1 cold miss %d != prompt %d", cold.CacheMiss, cold.PromptTokens)
			}

			// M2. The same prefix, seconds later, hits.
			if warm.CacheHit <= 0 {
				t.Errorf("M2 warm call reports no cache hit. Either this model " +
					"does not cache, or the pair was not really cold-then-warm")
			}
			if warm.PromptTokens != cold.PromptTokens {
				t.Errorf("M2 prompt size moved between passes, %d then %d; the "+
					"pair no longer isolates caching",
					cold.PromptTokens, warm.PromptTokens)
			}

			// M3. Cache hits come in whole blocks.
			if warm.CacheHit%cacheBlockTokens != 0 {
				t.Errorf("M3 cache hit %d is not a multiple of %d. Observed on "+
					"8 of 8 captures and published nowhere; a violation is a "+
					"finding about the provider, so record it rather than "+
					"widening the constant", warm.CacheHit, cacheBlockTokens)
			}

			// The engine's own split must agree with the provider's independent
			// hit/miss fields, on every model rather than only the ones the
			// provider suite happens to cover.
			for _, u := range []wireUsage{cold, warm} {
				got := ParseOpenAIResponse(mustBody(t, m.name, u)).Usage
				if got == nil {
					t.Fatal("nil usage")
				}
				if got.Input != u.CacheMiss || got.CacheRead != u.CacheHit {
					t.Errorf("split disagrees with the provider: fresh %d read %d, "+
						"provider miss %d hit %d",
						got.Input, got.CacheRead, u.CacheMiss, u.CacheHit)
				}
			}

			// M4. Reasoning tokens appear if and only if expected.
			has := warm.CompletionDetails != nil
			if has != m.reasoning {
				t.Errorf("M4 reasoning tokens present = %v, want %v. A model whose "+
					"reasoning accounting differs from its siblings prices "+
					"differently, and nothing else in the suite would notice",
					has, m.reasoning)
			}

			// M5. Tool calling is answered.
			tb, err := os.ReadFile(filepath.Join("testdata", "deepseek", "models", m.name+"-tools.json"))
			if err != nil {
				t.Fatalf("M5 fixture: %v", err)
			}
			var tenv struct {
				Choices []struct {
					Message struct {
						ToolCalls []json.RawMessage `json:"tool_calls"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(tb, &tenv); err != nil {
				t.Fatalf("M5 parse: %v", err)
			}
			if len(tenv.Choices) == 0 || len(tenv.Choices[0].Message.ToolCalls) == 0 {
				t.Error("M5 no tool_calls in the reply. Note the first attempt at " +
					"this probe capped max_tokens at 8 and truncated every model " +
					"before it could emit one, which looked like unanimous " +
					"non-support and was the probe's fault")
			}
		})
	}
}

// mustBody rebuilds a minimal response around a captured usage object, so the
// engine's conversion can be exercised per model without re-reading the file.
func mustBody(t *testing.T, model string, u wireUsage) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"model":   model,
		"choices": []any{},
		"usage": map[string]any{
			"prompt_tokens":            u.PromptTokens,
			"completion_tokens":        u.CompletionTokens,
			"prompt_tokens_details":    map[string]any{"cached_tokens": u.Details.CachedTokens},
			"prompt_cache_hit_tokens":  u.CacheHit,
			"prompt_cache_miss_tokens": u.CacheMiss,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}
