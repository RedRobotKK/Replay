package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Claude Code conformance, from ledger records written by a real `claude -p`
// session driven through `replay serve`.
//
// This is the primary path and the source of the 1363-session corpus, and until
// 2026-09-05 it had no captured wire fixtures at all while the secondary
// provider had ten. The first capture found a defect immediately.
//
// The invariants here are NOT the OpenAI ones, and the difference is the whole
// reason both files exist. Anthropic counts EXCLUSIVELY: input_tokens is the
// uncached remainder, so there is no inclusive total to check a partition
// against. What can be checked is the TTL split, which the provider computes
// separately and which must account for the write exactly.
//
// PASS/FAIL conditions:
//
//	A1  the 5m and 1h ephemeral figures sum to cache_creation_input_tokens
//	A2  a response that reports usage keeps the provider's object verbatim
//	A3  an aborted or usage-less response records no usage at all, not zeros
//	A4  no field is negative
//	A5  the ledger holds no message text
type ledgerRecord struct {
	Model    string `json:"model"`
	Status   int    `json:"status"`
	Response struct {
		Usage *struct {
			Input         int `json:"input_tokens"`
			CacheCreation int `json:"cache_creation_input_tokens"`
			CacheRead     int `json:"cache_read_input_tokens"`
			Output        int `json:"output_tokens"`
			Ephemeral5m   int `json:"ephemeral_5m_input_tokens"`
			Ephemeral1h   int `json:"ephemeral_1h_input_tokens"`
		} `json:"usage"`
		RawUsage json.RawMessage `json:"raw_usage"`
		Blocks   []struct {
			Kind  string `json:"kind"`
			Bytes int    `json:"bytes"`
			Text  string `json:"text"`
		} `json:"blocks"`
	} `json:"response"`
}

func loadRecord(t *testing.T, name string) ledgerRecord {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "anthropic", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	var r ledgerRecord
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return r
}

func TestClaudeCodeSurfaces(t *testing.T) {
	cases := []struct {
		file      string
		wantUsage bool
		check     func(t *testing.T, r ledgerRecord)
	}{
		{
			file: "cache-write-cold.json", wantUsage: true,
			check: func(t *testing.T, r ledgerRecord) {
				u := r.Response.Usage
				if u.CacheCreation <= 0 {
					t.Errorf("expected a cache write, got %d", u.CacheCreation)
				}
				if u.CacheRead != 0 {
					t.Errorf("a cold turn read %d cached tokens", u.CacheRead)
				}
				// Claude Code writes at the 1h TTL and never the 5m one. The
				// trim and TTL advice compares a choice the client is not
				// making; if this ever flips, that advice needs revisiting.
				if u.Ephemeral1h == 0 && u.Ephemeral5m > 0 {
					t.Errorf("a 5m write appeared where every observed capture " +
						"used 1h. Worth confirming before trusting TTL advice")
				}
			},
		},
		{
			file: "cache-read-and-write.json", wantUsage: true,
			check: func(t *testing.T, r ledgerRecord) {
				u := r.Response.Usage
				// One turn can read an existing prefix and extend it at the
				// same time. Treating these as mutually exclusive would
				// misprice the most common shape in a long session.
				if u.CacheRead <= 0 || u.CacheCreation <= 0 {
					t.Errorf("expected a simultaneous read and write, got read %d write %d",
						u.CacheRead, u.CacheCreation)
				}
			},
		},
		{
			file: "no-cache.json", wantUsage: true,
			check: func(t *testing.T, r ledgerRecord) {
				u := r.Response.Usage
				if u.CacheCreation != 0 || u.CacheRead != 0 {
					t.Errorf("expected no cache activity, got write %d read %d",
						u.CacheCreation, u.CacheRead)
				}
				if u.Input <= 0 {
					t.Errorf("an uncached turn must still report input tokens, got %d", u.Input)
				}
			},
		},
		{
			// A3. The client hung up mid-stream. There is no measurement, and
			// a zeroed record would enter every average as a free request.
			file: "aborted-no-usage.json", wantUsage: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			r := loadRecord(t, tc.file)

			if !tc.wantUsage {
				if r.Response.Usage != nil {
					t.Errorf("A3 an aborted response recorded usage %+v; absence "+
						"of a measurement must stay absent", r.Response.Usage)
				}
				return
			}
			u := r.Response.Usage
			if u == nil {
				t.Fatal("no usage recorded")
			}

			// A1. The TTL split accounts for the write exactly.
			if split := u.Ephemeral5m + u.Ephemeral1h; u.CacheCreation > 0 && split != u.CacheCreation {
				t.Errorf("A1 TTL split %d (5m %d + 1h %d) != cache_creation %d. "+
					"The split is reported separately, so a mismatch means one "+
					"of them is being read wrongly",
					split, u.Ephemeral5m, u.Ephemeral1h, u.CacheCreation)
			}

			// A2. The provider's own object survives.
			if len(r.Response.RawUsage) == 0 {
				t.Error("A2 raw_usage is empty. It was null on this path until " +
					"2026-09-05, and the first capture after the fix revealed " +
					"iterations[] and output_tokens_details, neither of which " +
					"this build declares")
			}

			// A4. Nothing negative.
			for name, v := range map[string]int{
				"input": u.Input, "write": u.CacheCreation, "read": u.CacheRead,
				"output": u.Output, "5m": u.Ephemeral5m, "1h": u.Ephemeral1h,
			} {
				if v < 0 {
					t.Errorf("A4 %s is negative (%d) and would read as a saving", name, v)
				}
			}

			// A5. Structure only.
			for _, b := range r.Response.Blocks {
				if b.Text != "" {
					t.Errorf("A5 a block carries text %q", b.Text)
				}
			}
			tc.check(t, r)
		})
	}
}

// The retained object carries fields this build does not declare. That is the
// entire argument for keeping it verbatim, so it is asserted rather than left
// as a claim in a document.
func TestRawUsageCarriesUndeclaredFields(t *testing.T) {
	r := loadRecord(t, "raw-usage-undeclared-fields.json")
	if len(r.Response.RawUsage) == 0 {
		t.Fatal("raw_usage is empty")
	}
	var raw map[string]any
	if err := json.Unmarshal(r.Response.RawUsage, &raw); err != nil {
		t.Fatalf("raw_usage is not valid JSON: %v", err)
	}
	for _, field := range []string{"iterations", "output_tokens_details"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("raw_usage lost %q. It was captured live on 2026-09-05 and "+
				"is not declared anywhere in this build; losing it is the "+
				"defect keeping the raw payload exists to prevent", field)
		}
	}
}

// The tests above read ledger records, which are already-parsed. That means a
// parser regression cannot fail them: mutating ParseResponse leaves the JSON on
// disk untouched. Discovered by mutation on 2026-09-05, and it is the same
// vacuity the OpenAI suite's I1 had — a check that cannot fail is not evidence.
//
// These parse RAW WIRE BODIES, recorded between the proxy and the provider, so
// the production parser is genuinely on the path.
func TestClaudeCodeWireStreams(t *testing.T) {
	cases := []struct {
		file                           string
		wantFresh, wantWrite, wantRead int
	}{
		{"stream-no-cache.sse", 1172, 0, 0},
		{"stream-cache-read-and-write.sse", 2, 11383, 170642},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", "anthropic", tc.file))
			if err != nil {
				t.Fatalf("fixture: %v", err)
			}
			var p StreamParser
			if _, err := p.Write(b); err != nil {
				t.Fatalf("stream write: %v", err)
			}
			r := p.Result()
			if r.Usage == nil {
				t.Fatal("no usage parsed from the stream. Anthropic splits it " +
					"across message_start and message_delta, so a parser that " +
					"reads only one frame loses half the accounting")
			}
			if r.Usage.Input != tc.wantFresh {
				t.Errorf("fresh: got %d, want %d", r.Usage.Input, tc.wantFresh)
			}
			if r.Usage.CacheCreation != tc.wantWrite {
				t.Errorf("cache write: got %d, want %d", r.Usage.CacheCreation, tc.wantWrite)
			}
			if r.Usage.CacheRead != tc.wantRead {
				t.Errorf("cache read: got %d, want %d", r.Usage.CacheRead, tc.wantRead)
			}
			// A1 on the live parser: the TTL split must account for the write.
			if tc.wantWrite > 0 {
				if split := r.Usage.Create5m + r.Usage.Create1h; split != r.Usage.CacheCreation {
					t.Errorf("A1 TTL split %d (5m %d + 1h %d) != write %d",
						split, r.Usage.Create5m, r.Usage.Create1h, r.Usage.CacheCreation)
				}
			}
			// A2 on the live parser.
			if len(r.RawUsage) == 0 {
				t.Error("A2 raw_usage empty on the streaming path")
			}
		})
	}
}

// Streaming tool use and streaming extended thinking, captured 2026-09-05 from
// a real Claude Code session. Both are wire bodies, so the production parser is
// on the path and a regression fails these.
func TestClaudeCodeToolUseStream(t *testing.T) {
	r := parseWire(t, "stream-tool-use.sse")

	// Exact sizes from the capture, not a > 0 check. A loose bound passed
	// while input_json_delta accumulation was mutated away entirely, because
	// content_block_start already contributes some bytes — the same weakness
	// that let a constant PrefixHash satisfy a non-empty assertion in the
	// OpenAI suite.
	wantBytes := map[string]int{"Read": 106, "Bash": 75}

	var tools []string
	for _, b := range r.Blocks {
		if b.Kind != "tool_use" {
			continue
		}
		tools = append(tools, b.ToolName)
		if want, ok := wantBytes[b.ToolName]; ok && b.Bytes != want {
			t.Errorf("tool call %q recorded %d bytes, want %d. The size comes "+
				"from input_json_delta frames, and a tool call whose arguments "+
				"are not accumulated is undersized in `replay blame` once the "+
				"call is replayed as input", b.ToolName, b.Bytes, want)
		}
		// CallKey is what the loop detector compares. Without it, an agent
		// repeating one call forever is undetectable.
		if b.CallKey == "" {
			t.Errorf("tool call %q has no CallKey; loop detection is blind to it",
				b.ToolName)
		}
		if b.ToolUseID == "" {
			t.Errorf("tool call %q has no tool_use_id to pair with its result",
				b.ToolName)
		}
	}
	if len(tools) < 2 {
		t.Errorf("expected two tool calls in this capture, got %v", tools)
	}
}

// Extended thinking. The important assertion here is the one that looks like a
// bug and is not, so that nobody "fixes" it later.
func TestClaudeCodeThinkingStream(t *testing.T) {
	r := parseWire(t, "stream-thinking.sse")

	var think *Block
	for i := range r.Blocks {
		if r.Blocks[i].Kind == "thinking" {
			think = &r.Blocks[i]
		}
	}
	if think == nil {
		t.Fatalf("no thinking block; blocks were %+v", r.Blocks)
	}

	// The provider withholds the thinking text: every thinking_delta in this
	// capture carries an EMPTY `thinking` string alongside an estimated_tokens
	// count, and a separate signature_delta carries 1208 bytes of signature.
	// So a byte size of zero is not a parsing failure, it is an honest record
	// of content Replay never saw. Do not "fix" this by counting the
	// signature: the signature is not tokens the model generated, and the fit
	// runs on user bytes anyway.
	if think.Bytes != 0 {
		t.Errorf("thinking block recorded %d bytes. Every observed thinking_delta "+
			"carried an empty string, so a non-zero size means something other "+
			"than model output is being counted — most likely the signature",
			think.Bytes)
	}

	// The size that does exist is the token count, and it must survive. The
	// provider replays these when the block is sent back, so this is the
	// block's measured input size on the next turn.
	if r.Usage == nil || r.Usage.ThinkingTokens <= 0 {
		t.Errorf("thinking tokens missing from usage; that figure is the only " +
			"size signal for a block whose text is withheld")
	}
}

func parseWire(t *testing.T, name string) Response {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "anthropic", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	var p StreamParser
	if _, err := p.Write(b); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return p.Result()
}
