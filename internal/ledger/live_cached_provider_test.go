package ledger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// READINESS failure mode 1, closed on the wire.
//
// `usage.FromInclusive` exists because OpenAI-compatible providers report
// prompt_tokens INCLUSIVE of the cache while this codebase counts exclusively.
// Copying the number across instead of subtracting double-counts every cached
// token, and the error is largest on exactly the sessions the tool exists for:
// it grows with the hit rate.
//
// The conversion has unit tests in internal/transcript. What it has never had
// is a run against a provider that actually reports a cache. Every
// OpenAI-path test to date used Ollama, which reports none — re-verified
// 2026-09-09, where two identical requests returned identical usage with no
// prompt_tokens_details at all. A synthetic body proves the arithmetic; it
// cannot prove the field names are the ones a live provider sends.
//
// This test is skipped without credentials, which makes it worthless unless
// somebody runs it. That is a deliberate trade and the reason the skip message
// names the exact variables rather than saying "not configured".
//
//	REPLAY_LIVE_BASE_URL   https://api.deepseek.com
//	REPLAY_LIVE_KEY        the key
//	REPLAY_LIVE_MODEL      deepseek-chat
//
// Any OpenAI-compatible provider that reports cached tokens will do. DeepSeek
// is the cheap one, and it is also the interesting one: it sends BOTH
// vocabularies — prompt_cache_hit_tokens/prompt_cache_miss_tokens alongside the
// OpenAI-shaped prompt_tokens_details — and openai.go:137 says both were
// discarded until 2026-09-05. Whether the two agree has never been checked.
func TestLive_CachedTokensSurviveTheInclusiveConversion(t *testing.T) {
	base := os.Getenv("REPLAY_LIVE_BASE_URL")
	key := os.Getenv("REPLAY_LIVE_KEY")
	model := os.Getenv("REPLAY_LIVE_MODEL")
	if base == "" || key == "" || model == "" {
		t.Skip("live cached-provider check not run. Set REPLAY_LIVE_BASE_URL, " +
			"REPLAY_LIVE_KEY and REPLAY_LIVE_MODEL (e.g. https://api.deepseek.com, " +
			"<key>, deepseek-chat). Costs a few cents. This is READINESS failure " +
			"mode 1 and it is the last unmet abort criterion before launch.")
	}

	// Long enough to clear a provider's minimum cacheable prefix. DeepSeek
	// caches on 64-token blocks; Anthropic's documented floor is 1024 for most
	// models. This is comfortably past both.
	prefix := "You are given reference material.\n\n" +
		strings.Repeat("Cache behaviour under repeated prefixes is the subject here. ", 400)

	call := func(what string) transcriptUsage {
		body, _ := json.Marshal(map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": prefix},
				{"role": "user", "content": "Reply with one word: ok"},
			},
			"max_tokens":  4,
			"temperature": 0,
		})
		req, err := http.NewRequest("POST", strings.TrimRight(base, "/")+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		req.Header.Set("content-type", "application/json")
		req.Header.Set("authorization", "Bearer "+key)

		resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		defer func() { _ = resp.Body.Close() }()
		raw := readAll(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("%s: HTTP %d: %s", what, resp.StatusCode, truncate(raw, 300))
		}

		// The point of the test: the provider's own bytes through the real
		// parse path, not a hand-built struct.
		parsed := ParseOpenAIResponse(raw)
		if parsed.Usage == nil {
			t.Fatalf("%s: no usage parsed from a live response: %s", what, truncate(raw, 300))
		}
		t.Logf("%s: prompt=%d fresh=%d cacheRead=%d output=%d",
			what, parsed.Usage.PromptTotal(), parsed.Usage.Input,
			parsed.Usage.CacheRead, parsed.Usage.Output)
		t.Logf("%s raw usage: %s", what, rawUsageOf(raw))
		return transcriptUsage{u: parsed.Usage, raw: raw}
	}

	cold := call("cold")
	warm := call("warm")

	// A test that passes because nothing was cached has verified nothing. This
	// is the vacuous-pass guard, and it is the whole reason the test is worth
	// having: without it, running against a provider that silently reports no
	// cache would print a green line and close a launch criterion on nothing.
	if warm.u.CacheRead == 0 {
		t.Fatalf("the warm call reported zero cached tokens, so the inclusive "+
			"conversion was never exercised and this test proves nothing. Either "+
			"the prefix (%d chars) is below this provider's minimum cacheable "+
			"size, the cache had expired, or this provider does not report a "+
			"cache at all — in which case it is the wrong provider for this "+
			"check.\nraw: %s", len(prefix), rawUsageOf(warm.raw))
	}

	// The invariant that catches a double count. Every downstream figure
	// divides by the prompt total, so a conversion that adds the cache back in
	// is not slightly wrong — it is wrong in the denominator of the cached
	// share, the break-even threshold and the cost.
	for _, c := range []struct {
		name string
		u    transcriptUsage
	}{{"cold", cold}, {"warm", warm}} {
		sum := c.u.u.Input + c.u.u.CacheRead + c.u.u.CacheCreation
		if sum != c.u.u.PromptTotal() {
			t.Errorf("%s: fresh %d + read %d + write %d = %d, but the provider "+
				"billed %d prompt tokens. An inclusive provider must be converted, "+
				"not copied.", c.name, c.u.u.Input, c.u.u.CacheRead,
				c.u.u.CacheCreation, sum, c.u.u.PromptTotal())
		}
		if c.u.u.Input < 0 {
			t.Errorf("%s: negative fresh tokens (%d)", c.name, c.u.u.Input)
		}
	}

	// DeepSeek's two vocabularies must agree. If prompt_cache_hit_tokens and
	// prompt_tokens_details.cached_tokens disagree, one of them is not what we
	// think it is, and the parser reads only one.
	if hit, ok := rawField(warm.raw, "prompt_cache_hit_tokens"); ok {
		if hit != warm.u.CacheRead {
			t.Errorf("the provider's two cache vocabularies disagree: "+
				"prompt_cache_hit_tokens=%d but the parsed CacheRead=%d. The "+
				"parser reads one of them; if they mean different things, every "+
				"cached share on this provider is wrong.", hit, warm.u.CacheRead)
		}
		if miss, ok := rawField(warm.raw, "prompt_cache_miss_tokens"); ok {
			if hit+miss != warm.u.PromptTotal() {
				t.Errorf("hit %d + miss %d = %d, but prompt_tokens is %d",
					hit, miss, hit+miss, warm.u.PromptTotal())
			}
		}
	}

	// The warm call should be cached more than the cold one. Not an equality:
	// providers are free to cache partially, and a shared system prompt may
	// already be warm from another caller.
	if warm.u.CacheRead <= cold.u.CacheRead {
		t.Errorf("the warm call cached %d tokens against the cold call's %d. The "+
			"second identical request should reuse more, so either caching is off "+
			"or the two calls did not share a prefix.",
			warm.u.CacheRead, cold.u.CacheRead)
	}

	share := float64(warm.u.CacheRead) / float64(warm.u.PromptTotal())
	t.Logf("warm cached share %.2f%% over %d prompt tokens — the inclusive "+
		"conversion is exercised at this hit rate, which is where a double "+
		"count would be largest", share*100, warm.u.PromptTotal())
}

type transcriptUsage struct {
	u   *Usage
	raw []byte
}

func readAll(t *testing.T, r *http.Response) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// rawField reads one integer straight out of the provider's bytes, bypassing
// every struct in this package. A field the parser does not declare is exactly
// the field worth checking it against.
func rawField(body []byte, name string) (int, bool) {
	var m struct {
		Usage map[string]json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(body, &m) != nil || m.Usage == nil {
		return 0, false
	}
	v, ok := m.Usage[name]
	if !ok {
		return 0, false
	}
	var n int
	if json.Unmarshal(v, &n) != nil {
		return 0, false
	}
	return n, true
}

func rawUsageOf(body []byte) string {
	var m struct {
		Usage json.RawMessage `json:"usage"`
	}
	if json.Unmarshal(body, &m) != nil {
		return "<unparseable>"
	}
	return fmt.Sprintf("%s", m.Usage)
}
