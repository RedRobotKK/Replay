package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// R-1 at the parsers, on the wire fixtures in testdata/openai-responses.
// The proxy tests prove the chain; these pin each parser's reading of the
// bytes, including the chunking a stream arrives in.

func responsesFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "openai-responses", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestResponses_ParsesTheNonStreamingReply(t *testing.T) {
	got := ParseResponsesResponse(responsesFixture(t, "nonstream-cached-reasoning.json"))
	if got.Unparsed {
		t.Fatal("a well-formed reply is marked unparsed")
	}
	if got.Usage == nil {
		t.Fatal("no usage read")
	}
	if want := (transcript.Usage{Input: 500, CacheRead: 1000, Output: 300, ThinkingTokens: 200}); *got.Usage != want {
		t.Errorf("usage = %+v, want %+v", *got.Usage, want)
	}
	if !strings.Contains(string(got.RawUsage), `"reasoning_tokens":200`) {
		t.Errorf("raw usage not verbatim: %s", got.RawUsage)
	}
	if len(got.Blocks) != 3 {
		t.Fatalf("blocks = %+v", got.Blocks)
	}
	if got.Blocks[0].Kind != transcript.KindThinking || got.Blocks[0].Bytes != len("Weighing the two options.") {
		t.Errorf("reasoning block: %+v", got.Blocks[0])
	}
	if got.Blocks[1].Kind != transcript.KindText || got.Blocks[1].Bytes != len("Running the listing now.") || got.Blocks[1].Text != "" {
		t.Errorf("text block: %+v", got.Blocks[1])
	}
	tool := got.Blocks[2]
	if tool.Kind != transcript.KindToolUse || tool.ToolName != "shell" || tool.ToolUseID != "call_1" || tool.CallKey == "" || tool.Text != "" {
		t.Errorf("tool block: %+v", tool)
	}
}

// cache_write_tokens, when the provider sends it, is a write and is part of
// the inclusive input. Nothing is inferred when it is absent.
func TestResponses_CacheWriteTokensAreAWriteOnlyWhenSent(t *testing.T) {
	got := ParseResponsesResponse(responsesFixture(t, "nonstream-cache-write.json"))
	if got.Usage == nil {
		t.Fatal("no usage")
	}
	if want := (transcript.Usage{Input: 300, CacheCreation: 1200, Output: 50}); *got.Usage != want {
		t.Errorf("usage = %+v, want %+v", *got.Usage, want)
	}
	plain := ParseResponsesResponse(responsesFixture(t, "nonstream-cached-reasoning.json"))
	if plain.Usage.CacheCreation != 0 {
		t.Errorf("a write was reconstructed from a reply that reported none: %+v", *plain.Usage)
	}
}

// Absence is not zero: no usage key and an empty usage object both yield nil
// and neither is marked unparsed.
func TestResponses_NoUsageAndEmptyUsageAreAbsence(t *testing.T) {
	for _, f := range []string{"nonstream-no-usage.json", "nonstream-empty-usage.json"} {
		got := ParseResponsesResponse(responsesFixture(t, f))
		if got.Unparsed {
			t.Errorf("%s: marked unparsed", f)
		}
		if got.Usage != nil {
			t.Errorf("%s: usage %+v recorded for a reply that reported none", f, *got.Usage)
		}
		if len(got.Blocks) == 0 {
			t.Errorf("%s: the message structure was dropped with the usage", f)
		}
	}
}

// QT-7c at this parser: a body that is not JSON, an error object behind a
// 200, and a reply cut short are marked unparsed and carry nothing.
func TestResponses_UnreadableBodiesAreMarkedUnparsed(t *testing.T) {
	cases := map[string][]byte{
		"not JSON":         []byte("<html>"),
		"error behind 200": responsesFixture(t, "nonstream-error-200.json"),
		"cut short":        responsesFixture(t, "nonstream-cached-reasoning.json")[:200],
		"another object":   []byte(`{"object":"chat.completion","usage":{"prompt_tokens":3}}`),
	}
	for name, body := range cases {
		got := ParseResponsesResponse(body)
		if !got.Unparsed {
			t.Errorf("%s: not marked unparsed: %+v", name, got)
		}
		if got.Usage != nil || len(got.Blocks) != 0 {
			t.Errorf("%s: an unparsed body carries usage or blocks: %+v", name, got)
		}
	}
}

// The stream. Usage comes from the terminal event, text and reasoning from
// deltas, the tool call from the finished item; and the result does not
// depend on how the bytes were chunked.
func TestResponses_StreamUsageComesFromTheTerminalEvent(t *testing.T) {
	want := transcript.Usage{Input: 500, CacheRead: 1000, Output: 300, ThinkingTokens: 200}
	for _, chunk := range []int{0, 1, 7, 1 << 20} {
		p := &ResponsesStreamParser{}
		feed(p, responsesFixture(t, "stream-completed.sse"), chunk)
		got := p.Result()
		if got.Usage == nil || *got.Usage != want {
			t.Fatalf("chunk %d: usage = %v, want %+v", chunk, got.Usage, want)
		}
		if !strings.Contains(string(got.RawUsage), `"total_tokens":1800`) {
			t.Errorf("chunk %d: raw usage not kept: %s", chunk, got.RawUsage)
		}
		var text, thinking int
		var tool string
		for _, b := range got.Blocks {
			switch b.Kind {
			case transcript.KindText:
				text += b.Bytes
			case transcript.KindThinking:
				thinking += b.Bytes
			case transcript.KindToolUse:
				tool = b.ToolName
			}
		}
		if text != len("Running the listing now.") || thinking != len("Weighing the two options.") || tool != "shell" {
			t.Errorf("chunk %d: text=%d thinking=%d tool=%q", chunk, text, thinking, tool)
		}
	}
	inc := &ResponsesStreamParser{}
	feed(inc, responsesFixture(t, "stream-incomplete.sse"), 0)
	if got := inc.Result(); got.Usage == nil || got.Usage.Output != 64 {
		t.Errorf("response.incomplete carries usage and it was not read: %+v", got)
	}
	cut := &ResponsesStreamParser{}
	feed(cut, responsesFixture(t, "stream-cut.sse"), 0)
	if got := cut.Result(); got.Usage != nil {
		t.Errorf("a stream with no terminal event recorded usage %+v", *got.Usage)
	} else if len(got.Blocks) == 0 {
		t.Error("the text that did arrive was dropped")
	}
}

func feed(p *ResponsesStreamParser, b []byte, chunk int) {
	if chunk <= 0 {
		_, _ = p.Write(b)
		return
	}
	for i := 0; i < len(b); i += chunk {
		end := i + chunk
		if end > len(b) {
			end = len(b)
		}
		_, _ = p.Write(b[i:end])
	}
}

// The summary the guards read: structure, never text; tool calls keyed; the
// instructions counted as the system prefix.
func TestResponses_SummarizeReadsStructureNotText(t *testing.T) {
	sum, err := SummarizeResponsesRequest(responsesFixture(t, "request-codex.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Model != "gpt-6-astra" || !sum.Stream {
		t.Errorf("model/stream: %+v", sum)
	}
	if sum.Prompt.SystemBytes != len("You are Codex, running in a sandbox.") || sum.Prompt.ToolCount != 1 || len(sum.Prompt.Tools) != 1 || sum.Prompt.Tools[0].Name != "shell" || sum.Prompt.ToolBytes == 0 {
		t.Errorf("prompt header: %+v", sum.Prompt)
	}
	if len(sum.Prompt.Messages) != 3 {
		t.Fatalf("messages: %+v", sum.Prompt.Messages)
	}
	if m := sum.Prompt.Messages[1]; m.Role != transcript.RoleAssistant || m.Blocks[0].Kind != transcript.KindToolUse || m.Blocks[0].CallKey == "" || m.Blocks[0].Text != "" {
		t.Errorf("the tool call is not an assistant tool_use with a key and no text: %+v", m)
	}
	if m := sum.Prompt.Messages[2]; m.Blocks[0].Kind != transcript.KindToolResult || m.Blocks[0].ToolUseID != "call_1" {
		t.Errorf("tool result: %+v", m)
	}
	for _, m := range sum.Prompt.Messages {
		for _, b := range m.Blocks {
			if b.Text != "" {
				t.Errorf("text kept on a summarised block: %+v", b)
			}
		}
	}
	if sum.PrefixHash == "" || sum.SessionHash == "" {
		t.Errorf("hashes missing: %+v", sum)
	}
	// A labeler keys the calls under the ledger secret, so the key differs
	// from the unkeyed digest while equal calls still share it.
	keyed, err := SummarizeResponsesRequest(responsesFixture(t, "request-codex.json"), NewLabeler([]byte("secret")))
	if err != nil {
		t.Fatal(err)
	}
	if keyed.Prompt.Messages[1].Blocks[0].CallKey == sum.Prompt.Messages[1].Blocks[0].CallKey {
		t.Error("a labeler did not re-key the tool call")
	}
}

// Session identity is the client's own prompt_cache_key, hashed, when sent;
// the structural fallback otherwise. Two requests on one key are one
// session whatever their content; the raw key never appears.
func TestResponses_SessionHashIsThePromptCacheKey(t *testing.T) {
	base := responsesFixture(t, "request-codex.json")
	const key = "019a2c6e-7f3b-7c1d-9d2e-4b5f6a7b8c9d"
	other := []byte(strings.Replace(string(base), `"text":"list the files"`, `"text":"delete them"`, 1))
	a, _ := SummarizeResponsesRequest(base, nil)
	b, _ := SummarizeResponsesRequest(other, nil)
	if a.SessionHash != b.SessionHash {
		t.Errorf("one prompt_cache_key, two sessions: %q %q", a.SessionHash, b.SessionHash)
	}
	if strings.Contains(a.SessionHash, key) {
		t.Errorf("the raw key is the session hash: %q", a.SessionHash)
	}
	c, _ := SummarizeResponsesRequest([]byte(strings.Replace(string(base), key, "0199ffff-0000-7000-8000-000000000000", 1)), nil)
	if c.SessionHash == a.SessionHash {
		t.Error("a different prompt_cache_key hashed to the same session")
	}
	noKey := []byte(strings.Replace(string(base), `,"prompt_cache_key":"`+key+`"`, "", 1))
	d, _ := SummarizeResponsesRequest(noKey, nil)
	e, _ := SummarizeResponsesRequest(noKey, nil)
	if d.SessionHash == "" || d.SessionHash != e.SessionHash {
		t.Errorf("structural fallback: %q %q", d.SessionHash, e.SessionHash)
	}
	if d.SessionHash == a.SessionHash {
		t.Error("the fallback collided with the keyed session")
	}
}

// The guard gate on PR #334 reported this summariser's error branch and the
// bound on its fallback session identity as never entered by any test.

// A body the parser refuses must come back as an error and an empty summary,
// so the proxy forwards it unsummarised rather than guarding and ledgering a
// request nobody read.
func TestResponses_SummarizeRefusesAnUnreadableBody(t *testing.T) {
	for name, body := range map[string][]byte{"not JSON": []byte("<html>"), "input is an object": []byte(`{"model":"m","input":{"x":1}}`)} {
		sum, err := SummarizeResponsesRequest(body, nil)
		if err == nil {
			t.Errorf("%s: no error", name)
		}
		if sum.Model != "" || sum.SessionHash != "" || sum.PrefixHash != "" || len(sum.Prompt.Messages) != 0 {
			t.Errorf("%s: a summary was returned beside the error: %+v", name, sum)
		}
	}
}

// Without a prompt_cache_key the session is the prefix and the first two
// items, by structure. Bounded at two on purpose: every later turn appends
// items, and an identity that read them all would make each turn its own
// session, so no cap could accumulate and no lane could be compared. Two
// bodies that agree on the prefix and the first two items are one session
// whatever follows; two that differ in the second item are not.
func TestResponses_FallbackSessionIdentityIsBoundedToTheFirstTwoItems(t *testing.T) {
	body := func(items ...string) []byte {
		return []byte(`{"model":"gpt-6-astra","instructions":"be brief","input":[` + strings.Join(items, ",") + `]}`)
	}
	user := `{"type":"message","role":"user","content":[{"type":"input_text","text":"list the files"}]}`
	call := `{"type":"function_call","call_id":"c1","name":"shell","arguments":"{\"command\":[\"ls\"]}"}`
	out := `{"type":"function_call_output","call_id":"c1","output":"total 0"}`
	later := `{"type":"message","role":"user","content":[{"type":"input_text","text":"now something else entirely"}]}`
	turn1, _ := SummarizeResponsesRequest(body(user, call), nil)
	turn2, _ := SummarizeResponsesRequest(body(user, call, out), nil)
	turn3, _ := SummarizeResponsesRequest(body(user, call, out, later), nil)
	if turn1.SessionHash == "" || turn1.SessionHash != turn2.SessionHash || turn2.SessionHash != turn3.SessionHash {
		t.Errorf("one conversation, three turns, three sessions: %q %q %q", turn1.SessionHash, turn2.SessionHash, turn3.SessionHash)
	}
	other, _ := SummarizeResponsesRequest(body(user, later), nil)
	if other.SessionHash == turn1.SessionHash {
		t.Error("a different second item hashed to the same session")
	}
	one, _ := SummarizeResponsesRequest(body(user), nil)
	if one.SessionHash == "" || one.SessionHash == turn1.SessionHash {
		t.Errorf("a single-item body: %q (must be an identity, and not the two-item one)", one.SessionHash)
	}
}

// The stream parser's pending-line cap, on the precedent of
// TestStreamParserStopsOnAnEndlessLine. A line that never ends is not a
// stream this parser understands; it stops keeping it rather than growing
// without bound, and once dropped it stays dropped: a well-formed stream
// arriving afterwards on the same connection is not read as if nothing
// happened, because the record for this turn is already unreliable.
func TestResponsesStreamParserStopsOnAnEndlessLineAndStaysStopped(t *testing.T) {
	p := &ResponsesStreamParser{}
	chunk := []byte(strings.Repeat("x", 300_000))
	for i := 0; i < 10; i++ {
		if _, err := p.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if !p.dropped || p.pending.Len() != 0 {
		t.Fatalf("parser must stop buffering past the cap: dropped=%v pending=%d", p.dropped, p.pending.Len())
	}
	// A complete, valid stream after the drop.
	if _, err := p.Write(responsesFixture(t, "stream-completed.sse")); err != nil {
		t.Fatal(err)
	}
	if p.pending.Len() != 0 {
		t.Errorf("a dropped parser resumed buffering: pending=%d", p.pending.Len())
	}
	if got := p.Result(); got.Usage != nil || len(got.Blocks) != 0 {
		t.Errorf("a dropped stream reported usage or structure from bytes that followed the drop: %+v", got)
	}
}

// Framing the wire is allowed to send and a terminal event that carries no
// usage object. An empty data line is a keepalive and is ignored; a
// response.completed with usage null, or with no response object at all,
// yields no usage and no panic. These are the observable contracts behind
// two branches that were removed as redundant with the JSON decoder.
func TestResponsesStreamParserTolerantOfKeepalivesAndTerminalEventsWithoutUsage(t *testing.T) {
	p := &ResponsesStreamParser{}
	feed(p, []byte("data:\n\ndata: \n\n"+string(responsesFixture(t, "stream-completed.sse"))), 1)
	if got := p.Result(); got.Usage == nil || got.Usage.Output != 300 {
		t.Errorf("keepalive lines broke the read: %+v", got)
	}
	for name, terminal := range map[string]string{
		"usage null":         `{"type":"response.completed","response":{"id":"r","usage":null}}`,
		"usage absent":       `{"type":"response.completed","response":{"id":"r"}}`,
		"no response object": `{"type":"response.completed"}`,
		"usage empty object": `{"type":"response.incomplete","response":{"id":"r","usage":{}}}`,
	} {
		q := &ResponsesStreamParser{}
		feed(q, []byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\ndata: "+terminal+"\n\n"), 0)
		got := q.Result()
		if got.Usage != nil {
			t.Errorf("%s: usage %+v recorded for a terminal event that carried none", name, *got.Usage)
		}
		if len(got.Blocks) != 1 || got.Blocks[0].Bytes != 2 {
			t.Errorf("%s: the text that arrived was lost: %+v", name, got.Blocks)
		}
	}
}
