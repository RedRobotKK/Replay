package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Provider conformance, against payloads a live provider actually sent.
//
// Every fixture under testdata/deepseek was captured from api.deepseek.com on
// 2026-09-05, not written from the API documentation. That distinction is the
// point of the file: the two defects found that day were both invisible to a
// stub, because a stub only ever sends the fields the parser already knows
// about. A fixture captured off the wire carries the fields nobody thought to
// declare — prompt_cache_hit_tokens among them — and those are precisely the
// ones that broke.
//
// The live script (scripts/verify-provider.sh) needs an API key and a few
// cents. This file needs neither and runs in CI on every commit, which is what
// makes the guarantee durable after the key that produced it is rotated.
//
// PASS means every universal invariant below holds, plus the surface's own
// condition. FAIL on any one of them is a release blocker: each maps to a
// number a user would read off `replay cost` and act on.

// ---------------------------------------------------------------- invariants

// checkUniversal asserts what must be true of EVERY successfully parsed
// response, on every provider and every surface. A new surface that cannot
// satisfy these is not a surface Replay supports yet.
func checkUniversal(t *testing.T, name string, body []byte, resp Response) {
	t.Helper()
	if resp.Usage == nil {
		t.Fatalf("%s: usage is nil; a 200 with a usage object must parse", name)
	}
	u := resp.Usage

	// I1. The counting invariant. OpenAI-compatible providers report
	// prompt_tokens INCLUSIVE of cached tokens; Anthropic reports input_tokens
	// EXCLUSIVE of them. Copying one into the other double-counts the cache,
	// and the error grows with hit rate, so it is largest on exactly the
	// sessions this tool exists for.
	var raw struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			Details          struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("%s: fixture is not valid JSON: %v", name, err)
	}
	parts := u.Input + u.CacheRead + u.CacheCreation
	if parts != raw.Usage.PromptTokens {
		t.Errorf("%s: I1 counting invariant violated.\n"+
			"  provider prompt_tokens = %d (inclusive of %d cached)\n"+
			"  replay fresh+read+write = %d + %d + %d = %d\n"+
			"  a mismatch here misprices every task in the session",
			name, raw.Usage.PromptTokens, raw.Usage.Details.CachedTokens,
			u.Input, u.CacheRead, u.CacheCreation, parts)
	}

	// I2. Output is copied, not derived. A provider's completion count is
	// authoritative and there is nothing to compute.
	if u.Output != raw.Usage.CompletionTokens {
		t.Errorf("%s: I2 output_tokens %d != provider completion_tokens %d",
			name, u.Output, raw.Usage.CompletionTokens)
	}

	// I3. The cached figure reaches the ledger as a cache read. A provider that
	// reports a hit and a ledger that records none is the silent-zero failure:
	// the tool would report a session as having no cache activity at all.
	if raw.Usage.Details.CachedTokens > 0 && u.CacheRead == 0 {
		t.Errorf("%s: I3 provider reported %d cached tokens, replay recorded a "+
			"cache read of 0. The adapter is not reading this provider's shape",
			name, raw.Usage.Details.CachedTokens)
	}

	// I4. raw_usage is verbatim. Documented as "the provider's own usage
	// object, verbatim and unparsed", and the design note's reason is that a
	// field we did not know mattered is what tomorrow's calibration needs. It
	// was re-marshalled from our struct until 2026-09-05, which silently
	// dropped every undeclared field.
	if resp.RawUsage == nil {
		t.Fatalf("%s: I4 raw_usage is nil", name)
	}
	var sent, kept map[string]any
	_ = json.Unmarshal(body, &sent)
	if err := json.Unmarshal(resp.RawUsage, &kept); err != nil {
		t.Fatalf("%s: I4 raw_usage is not valid JSON: %v", name, err)
	}
	sentUsage, _ := sent["usage"].(map[string]any)
	for field := range sentUsage {
		if _, ok := kept[field]; !ok {
			t.Errorf("%s: I4 raw_usage dropped %q. Every provider field must "+
				"survive, including the ones no type declares", name, field)
		}
	}

	// I5. No message text, ever. The ledger's standing promise, and the reason
	// a regulated estate can install this at all.
	blob := string(resp.RawUsage)
	for _, b := range resp.Blocks {
		if b.Text != "" {
			t.Errorf("%s: I5 a block carries text %q; the ledger holds structure only",
				name, b.Text)
		}
	}
	for _, leak := range []string{"Tokyo", "two plus two", "18C"} {
		if strings.Contains(blob, leak) {
			t.Errorf("%s: I5 message content %q reached raw_usage", name, leak)
		}
	}
}

func load(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "deepseek", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return b
}

// ------------------------------------------------ surfaces 1, 2, 4, 5: JSON

func TestDeepSeekNonStreamingSurfaces(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		// check is the surface's own condition, on top of the universal set.
		check func(t *testing.T, resp Response)
	}{
		{
			name:    "1. chat, cold: no cache, everything fresh",
			fixture: "01-chat-cold.json",
			check: func(t *testing.T, r Response) {
				if r.Usage.Input != 9630 {
					t.Errorf("fresh: got %d, want 9630 (nothing cached)", r.Usage.Input)
				}
				if r.Usage.CacheRead != 0 {
					t.Errorf("cache read: got %d, want 0", r.Usage.CacheRead)
				}
			},
		},
		{
			name:    "2. chat, warm: the cached share is subtracted, not double-counted",
			fixture: "02-chat-warm.json",
			check: func(t *testing.T, r Response) {
				// This is the single most consequential assertion in the file.
				// 14010 inclusive, 13952 cached, so 58 fresh. Copying the
				// provider's 14010 into fresh would bill the cache twice.
				if r.Usage.Input != 30 {
					t.Errorf("fresh: got %d, want 30 (9630 inclusive - 9600 cached). "+
						"Getting 9630 here means the cache is counted twice", r.Usage.Input)
				}
				if r.Usage.CacheRead != 9600 {
					t.Errorf("cache read: got %d, want 9600", r.Usage.CacheRead)
				}
			},
		},
		{
			name:    "4. reasoner: reasoning tokens map to thinking tokens",
			fixture: "04-reasoner.json",
			check: func(t *testing.T, r Response) {
				// The real capture is cache-warm: 14087 inclusive of 14080
				// cached leaves 7 fresh. Reasoning models cache like any other.
				if r.Usage.Input != 7 || r.Usage.CacheRead != 14080 {
					t.Errorf("reasoner fresh %d read %d, want 7 and 14080",
						r.Usage.Input, r.Usage.CacheRead)
				}
				if r.Usage.ThinkingTokens != 8 {
					t.Errorf("thinking: got %d, want 8 from completion_tokens_details."+
						"reasoning_tokens. The provider replays these when the block is "+
						"sent back, so they are a measured input size later",
						r.Usage.ThinkingTokens)
				}
			},
		},
		{
			name:    "5. tool calling: a tool_calls reply still prices correctly",
			fixture: "05-tool-calls.json",
			check: func(t *testing.T, r Response) {
				if r.Usage.Input != 278 || r.Usage.Output != 44 {
					t.Errorf("tool-call reply: fresh %d out %d, want 278 and 44",
						r.Usage.Input, r.Usage.Output)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := load(t, tc.fixture)
			resp := ParseOpenAIResponse(body)
			checkUniversal(t, tc.name, body, resp)
			tc.check(t, resp)
		})
	}
}

// ------------------------------------------------------ surface 6: an error

// A non-2xx body has no usage. The proxy must record that as nothing rather
// than as zero: a zero is a measurement and would drag every average it enters.
func TestDeepSeekErrorProducesNoUsage(t *testing.T) {
	resp := ParseOpenAIResponse(load(t, "06-error-400.json"))
	if resp.Usage != nil {
		t.Errorf("6. a 400 body produced usage %+v; an error must yield nil, "+
			"never a zeroed record that looks like a free request", resp.Usage)
	}
	if resp.RawUsage != nil {
		t.Errorf("6. a 400 body produced raw_usage %s", resp.RawUsage)
	}
}

// --------------------------------------------------- surfaces 3, 7: streaming

func TestDeepSeekStreamingSurfaces(t *testing.T) {
	cases := []struct {
		name       string
		fixture    string
		wantFresh  int
		wantRead   int
		wantOut    int
		wantThinks int
	}{
		{
			name:      "3. chat streaming: usage arrives on the final chunk",
			fixture:   "03-chat-stream.sse",
			wantFresh: 56, wantRead: 13952, wantOut: 8,
		},
		{
			name:      "7. reasoner streaming: reasoning deltas and usage together",
			fixture:   "07-reasoner-stream.sse",
			wantFresh: 7, wantRead: 14080, wantOut: 8, wantThinks: 8,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p OpenAIStreamParser
			if _, err := p.Write(load(t, tc.fixture)); err != nil {
				t.Fatalf("stream write: %v", err)
			}
			r := p.Result()
			if r.Usage == nil {
				t.Fatal("no usage parsed from the stream; the final chunk carries it " +
					"and a parser that stops at [DONE] too early loses the whole session")
			}
			if r.Usage.Input != tc.wantFresh {
				t.Errorf("fresh: got %d, want %d", r.Usage.Input, tc.wantFresh)
			}
			if r.Usage.CacheRead != tc.wantRead {
				t.Errorf("cache read: got %d, want %d", r.Usage.CacheRead, tc.wantRead)
			}
			if r.Usage.Output != tc.wantOut {
				t.Errorf("output: got %d, want %d", r.Usage.Output, tc.wantOut)
			}
			if tc.wantThinks > 0 && r.Usage.ThinkingTokens != tc.wantThinks {
				t.Errorf("thinking: got %d, want %d", r.Usage.ThinkingTokens, tc.wantThinks)
			}
			// Streaming must not be a second, weaker path: raw_usage is kept
			// here too, and the streaming parser was already correct on this
			// when the non-streaming one was not.
			if r.RawUsage == nil {
				t.Error("raw_usage is nil on the streaming path")
			} else if !strings.Contains(string(r.RawUsage), "prompt_cache_hit_tokens") {
				t.Errorf("raw_usage dropped the provider's own cache fields: %s", r.RawUsage)
			}
		})
	}
}

// --------------------------------------- surface 9: the agent loop request

// The shape an agent actually sends: a tool call and its result in the history.
// Replay's loop detector and `replay blame` both key off these block kinds, so
// a request whose tool turns are flattened to text is one the guards cannot see.
func TestDeepSeekToolLoopRequest(t *testing.T) {
	sum, err := SummarizeOpenAIRequest(load(t, "09-request-tool-loop.json"), nil)
	if err != nil {
		t.Fatalf("9. summarize: %v", err)
	}

	kinds := map[string]int{}
	for _, m := range sum.Prompt.Messages {
		for _, b := range m.Blocks {
			kinds[b.Kind]++
			if b.Text != "" {
				t.Errorf("9. block carries text %q; the ledger holds structure only", b.Text)
			}
		}
	}
	for _, want := range []string{"text", "tool_use", "tool_result"} {
		if kinds[want] == 0 {
			t.Errorf("9. no %q block recognised. Kinds seen: %v. The loop detector "+
				"and blame both key off these", want, kinds)
		}
	}

	// Session and prefix identity must be set, or the record is dropped and the
	// sibling gate collapses onto one key. This is the 2026-09-05 defect.
	if sum.SessionHash == "" {
		t.Error("9. SessionHash empty: the proxy falls back to it when a client " +
			"sends no session header, and every OpenAI-compatible client does")
	}
	if sum.PrefixHash == "" {
		t.Error("9. PrefixHash empty: --hold-siblings keys on it")
	}
}
