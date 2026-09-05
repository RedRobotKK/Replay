package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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
			// Computed by the provider separately from cached_tokens, which is
			// what lets I1 be falsifiable rather than an identity.
			CacheHit  int `json:"prompt_cache_hit_tokens"`
			CacheMiss int `json:"prompt_cache_miss_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("%s: fixture is not valid JSON: %v", name, err)
	}

	// I1. The split is checked against fields the provider computes
	// INDEPENDENTLY, not against itself.
	//
	// The obvious form of this check — fresh + read + write == prompt_tokens —
	// is a tautology and was one until an audit on 2026-09-05 proved it.
	// Usage() sets Input = prompt - cached, CacheRead = cached, and never
	// assigns CacheCreation, so the sum is identically prompt_tokens for every
	// input including negative ones. 200,000 random pairs produced zero
	// violations. It cannot fail, so it was never evidence.
	//
	// What makes it real is that DeepSeek publishes prompt_cache_hit_tokens and
	// prompt_cache_miss_tokens, which it derives separately from
	// prompt_tokens_details.cached_tokens. Any defect that mis-partitions the
	// prompt — the entire realistic defect class — disagrees with them.
	if raw.Usage.CacheMiss > 0 || raw.Usage.CacheHit > 0 {
		if u.Input != raw.Usage.CacheMiss {
			t.Errorf("%s: I1 fresh tokens %d != the provider's own "+
				"prompt_cache_miss_tokens %d. The prompt was split wrongly",
				name, u.Input, raw.Usage.CacheMiss)
		}
		if u.CacheRead != raw.Usage.CacheHit {
			t.Errorf("%s: I1 cache read %d != the provider's own "+
				"prompt_cache_hit_tokens %d", name, u.CacheRead, raw.Usage.CacheHit)
		}
	} else {
		t.Logf("%s: I1 has no independent fields to check against on this "+
			"provider; the partition sum alone would be vacuous, so this "+
			"surface is guarded only by its per-surface constants", name)
	}

	// I1b. The partition still has to close. This is structural, not evidence
	// about the split, and it is labelled that way so nobody mistakes it for
	// the check above.
	parts := u.Input + u.CacheRead + u.CacheCreation
	if parts != raw.Usage.PromptTokens {
		t.Errorf("%s: I1b partition does not close: %d + %d + %d = %d, prompt_tokens %d",
			name, u.Input, u.CacheRead, u.CacheCreation, parts, raw.Usage.PromptTokens)
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
				// The most consequential assertion in the file, and the
				// numbers are this fixture's own: 9630 inclusive, 9600
				// cached, so 30 fresh. Copying the provider's 9630 into
				// fresh would bill the cache twice.
				//
				// An earlier version of this comment said 14010 and 58,
				// which matched no fixture here. It was residue of the
				// truncated transcription the testdata README describes
				// escaping, and it survived directly above the line calling
				// itself most consequential.
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
			// Streaming is not a second, weaker path. The universal set runs
			// here too: an audit on 2026-09-05 found checkUniversal had one
			// call site covering four of nine surfaces, leaving streaming —
			// the path this family's clients use by default — with no
			// invariant at all beyond a substring probe.
			//
			// The final frame carries the usage object, so it is the body the
			// universal checks read.
			checkUniversal(t, tc.name, finalUsageFrame(t, load(t, tc.fixture)), r)

			// The streamed response's size must survive. Zeroing it left the
			// suite green before, because these surfaces asserted usage only.
			if r.Blocks == nil || len(r.Blocks) == 0 {
				t.Error("no blocks from the stream: the assistant text was " +
					"accumulated to nothing and every byte-to-token fit " +
					"downstream reads zero")
			} else if r.Blocks[0].Bytes <= 0 {
				t.Errorf("streamed block size is %d; deltas were not accumulated",
					r.Blocks[0].Bytes)
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

// finalUsageFrame returns the last SSE data frame carrying a usage object, so
// the universal checks can read a streaming surface with the same code that
// reads a JSON one.
func finalUsageFrame(t *testing.T, sse []byte) []byte {
	t.Helper()
	var last []byte
	for _, line := range strings.Split(string(sse), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" || !strings.Contains(payload, "\"usage\"") {
			continue
		}
		last = []byte(payload)
	}
	if last == nil {
		t.Fatal("no SSE frame carried a usage object")
	}
	return last
}

// The five defects below each left the suite fully green when planted as
// mutants during the 2026-09-05 audit. Every one is production behaviour a
// surface claimed to cover and did not.

// Surface 5 is titled "a tool_calls reply still prices correctly" and asserted
// only the token counts, so deleting the whole tool_calls loop changed nothing.
// A reply whose tool call is not recorded is invisible to the loop detector.
func TestToolCallsReplyProducesAToolBlock(t *testing.T) {
	r := ParseOpenAIResponse(load(t, "05-tool-calls.json"))
	var found bool
	for _, b := range r.Blocks {
		if strings.Contains(strings.ToLower(b.Kind), "tool") {
			found = true
		}
	}
	if !found {
		t.Errorf("no tool block from a tool_calls reply; blocks: %+v", r.Blocks)
	}
}

// PrefixHash was asserted non-empty, so a constant satisfied it — and a
// constant is exactly the failure --hold-siblings suffers from, collapsing
// every unrelated request onto one gate key.
func TestPrefixAndSessionHashesDiscriminate(t *testing.T) {
	loop, err := SummarizeOpenAIRequest(load(t, "09-request-tool-loop.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := SummarizeOpenAIRequest([]byte(
		`{"model":"deepseek-chat","messages":[{"role":"system","content":"a totally different and much longer system prompt for this session"},{"role":"user","content":"unrelated"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if loop.PrefixHash == other.PrefixHash {
		t.Errorf("two different prefixes share a hash %q: --hold-siblings would "+
			"serialise unrelated requests behind each other", loop.PrefixHash)
	}
	if loop.SessionHash == other.SessionHash {
		t.Errorf("two different sessions share a hash %q: their turns would "+
			"interleave in one ledger file", loop.SessionHash)
	}
}

// The clamp is what made the old I1 unfalsifiable, and no fixture reaches it.
// Its job is to stop a negative fresh count flowing downstream, where it would
// read as a saving.
func TestClampOnImpossibleProviderNumbers(t *testing.T) {
	cases := []struct {
		prompt, cached, wantFresh, wantRead int
		// wantNil marks a response so contradictory that refusing to measure
		// it beats recording a clamped zero. prompt=0 with cached>0 cannot
		// happen: a request with no prompt tokens has nothing to cache.
		wantNil bool
	}{
		{prompt: 100, cached: 500, wantFresh: 0, wantRead: 100},
		{prompt: 100, cached: -50, wantFresh: 100, wantRead: 0},
		{prompt: 0, cached: 9600, wantNil: true},
	}
	for _, c := range cases {
		body := []byte(`{"choices":[],"usage":{"prompt_tokens":` + itoa(c.prompt) +
			`,"completion_tokens":0,"prompt_tokens_details":{"cached_tokens":` +
			itoa(c.cached) + `}}}`)
		u := ParseOpenAIResponse(body).Usage
		if c.wantNil {
			if u != nil {
				t.Errorf("prompt=%d cached=%d: got usage %+v, want nil. A "+
					"contradictory response must not become a measurement",
					c.prompt, c.cached, u)
			}
			continue
		}
		if u == nil {
			t.Fatalf("prompt=%d cached=%d: nil usage", c.prompt, c.cached)
		}
		if u.Input != c.wantFresh || u.CacheRead != c.wantRead {
			t.Errorf("prompt=%d cached=%d: got fresh %d read %d, want %d and %d",
				c.prompt, c.cached, u.Input, u.CacheRead, c.wantFresh, c.wantRead)
		}
		if u.Input < 0 {
			t.Errorf("prompt=%d cached=%d: negative fresh count %d would read "+
				"as a saving in every cost figure", c.prompt, c.cached, u.Input)
		}
	}
}

// Surface 6 proved one error body yields nil usage. It did not prove the
// property: an empty usage object produces the zeroed record the surface's own
// comment forbids, and gateways emit those on refusal and content-filter paths.
func TestEmptyUsageObjectIsNotAZeroMeasurement(t *testing.T) {
	for _, body := range []string{
		`{"choices":[],"usage":{}}`,
		`{"choices":[],"usage":{"prompt_tokens":0,"completion_tokens":0}}`,
	} {
		u := ParseOpenAIResponse([]byte(body)).Usage
		if u != nil && u.Input == 0 && u.CacheRead == 0 && u.Output == 0 {
			t.Errorf("body %s produced a zeroed usage record. A zero is a "+
				"measurement and drags every average it enters; an absent one "+
				"must stay absent", body)
		}
	}
}

// The 401 body, which is the shape a rotated or wrong key produces. Like the
// 400 it must yield no usage at all.
func TestAuthErrorProducesNoUsage(t *testing.T) {
	r := ParseOpenAIResponse(load(t, "10-error-401.json"))
	if r.Usage != nil {
		t.Errorf("a 401 produced usage %+v", r.Usage)
	}
}

// The tool-loop response fixture was captured and then referenced by nothing.
func TestToolLoopResponse(t *testing.T) {
	body := load(t, "09-response-tool-loop.json")
	r := ParseOpenAIResponse(body)
	checkUniversal(t, "9. tool-loop response", body, r)
	if r.Usage.Input != 69 || r.Usage.CacheRead != 256 {
		t.Errorf("fresh %d read %d, want 69 and 256 (325 inclusive of 256 cached)",
			r.Usage.Input, r.Usage.CacheRead)
	}
}

func itoa(i int) string { return strconv.Itoa(i) }

// I2 says output is copied from the provider, never derived. No captured
// fixture can prove that: every real response satisfies
// total == prompt + completion, so `total - prompt` is numerically identical to
// `completion` on the entire corpus, and the mutation survived the whole suite.
//
// The only way to falsify it is a body where the identity does not hold, which
// a provider will never send. So this one is synthetic on purpose, and says so:
// it is a statement about which field the code reads, not about real traffic.
func TestOutputIsCopiedNotDerived(t *testing.T) {
	// total is deliberately inconsistent with prompt + completion.
	body := []byte(`{"choices":[],"usage":{"prompt_tokens":100,` +
		`"completion_tokens":7,"total_tokens":999}}`)
	u := ParseOpenAIResponse(body).Usage
	if u == nil {
		t.Fatal("nil usage")
	}
	if u.Output != 7 {
		t.Errorf("output %d: the code is deriving from total_tokens (999-100=899) "+
			"rather than reading completion_tokens (7). A provider that reports "+
			"total inconsistently would then misprice every response", u.Output)
	}
}

// SystemBytes feeds the byte-to-token fit and the trim advice. Nothing asserted
// it, so zeroing it left the suite green.
func TestSystemBytesIsRecorded(t *testing.T) {
	sum, err := SummarizeOpenAIRequest(load(t, "09-request-tool-loop.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Prompt.SystemBytes <= 0 {
		t.Errorf("SystemBytes is %d; the byte-to-token fit and every trim "+
			"suggestion read zero for this request", sum.Prompt.SystemBytes)
	}
}
