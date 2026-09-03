package ledger

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

const sampleRequest = `{"model":"claude-opus-5","max_tokens":100,"stream":true,
 "system":[{"type":"text","text":"You are terse.","cache_control":{"type":"ephemeral"}}],
 "tools":[{"name":"Read","description":"read a file","input_schema":{"type":"object"}}],
 "output_config":{"effort":"high"},
 "messages":[
   {"role":"user","content":"hello there"},
   {"role":"assistant","content":[{"type":"thinking","thinking":"","signature":"sig"},{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/tmp/x.go"}}]},
   {"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"package x","is_error":false,"cache_control":{"type":"ephemeral"}}]}
 ]}`

func TestSummarizeRequest(t *testing.T) {
	labeler := NewLabeler([]byte("test-key"))
	p, model, stream, effort, err := SummarizeRequest([]byte(sampleRequest), labeler)
	if err != nil {
		t.Fatal(err)
	}
	if model != "claude-opus-5" || !stream || effort != "high" {
		t.Fatalf("model/stream/effort = %q/%v/%q", model, stream, effort)
	}
	if p.SystemBytes != len("You are terse.") || p.ToolCount != 1 || p.ToolBytes == 0 {
		t.Fatalf("prefix sizes wrong: %+v", p)
	}
	if p.CacheControlCount != 2 {
		t.Fatalf("cache markers = %d, want 2", p.CacheControlCount)
	}
	if len(p.Messages) != 3 {
		t.Fatalf("messages = %d", len(p.Messages))
	}
	result := p.Messages[2].Blocks[0]
	if result.Kind != transcript.KindToolResult || !strings.HasPrefix(result.Label, "tool result: Read r/") || !strings.HasSuffix(result.Label, ".go") || result.Bytes != len("package x") {
		t.Fatalf("tool result not summarized as a content-free label: %+v", result)
	}
	if strings.Contains(result.Label, "/tmp") || strings.Contains(result.Label, "x.go") {
		t.Fatalf("label leaks the path: %q", result.Label)
	}
	for _, m := range p.Messages {
		for _, b := range m.Blocks {
			if strings.Contains(b.Label, "hello") || strings.Contains(b.Label, "package") {
				t.Fatalf("label leaks content: %q", b.Label)
			}
		}
	}
}

// LG-2: the ledger never holds message text. A secret inside a command
// argument, a URL, or a query must not survive summarization.
func TestLedgerHoldsNoArgumentContent(t *testing.T) {
	body := `{"model":"m","messages":[
	 {"role":"assistant","content":[
	   {"type":"tool_use","id":"a","name":"Bash","input":{"command":"curl -H 'Authorization: Bearer sk-ant-LEAKED' https://x"}},
	   {"type":"tool_use","id":"b","name":"WebFetch","input":{"url":"https://example.com/?token=LEAKED2"}},
	   {"type":"tool_use","id":"c","name":"Grep","input":{"pattern":"LEAKED3"}},
	   {"type":"tool_use","id":"d","name":"Read","input":{"file_path":"/Users/me/secret-plans.md"}}]},
	 {"role":"user","content":[
	   {"type":"tool_result","tool_use_id":"a","content":"ok"},{"type":"tool_result","tool_use_id":"b","content":"ok"},
	   {"type":"tool_result","tool_use_id":"c","content":"ok"},{"type":"tool_result","tool_use_id":"d","content":"ok"}]}]}`
	p, _, _, _, err := SummarizeRequest([]byte(body), NewLabeler([]byte("k")))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"LEAKED", "curl", "example.com", "secret-plans", "/Users"} {
		if strings.Contains(string(encoded), leak) {
			t.Fatalf("ledger prompt contains %q:\n%s", leak, encoded)
		}
	}
	labels := map[string]bool{}
	for _, b := range p.Messages[1].Blocks {
		labels[b.Label] = true
	}
	if !labels["tool result: Bash"] || !labels["tool result: WebFetch"] || !labels["tool result: Grep"] {
		t.Fatalf("non-path tools must be labeled by name only: %v", labels)
	}
	if !strings.HasSuffix(p.Messages[0].Blocks[3].Label, "Read") {
		t.Fatalf("tool call label must be the tool name: %q", p.Messages[0].Blocks[3].Label)
	}
}

func TestParseResponseAndStream(t *testing.T) {
	body := `{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hi"},{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"ls"}}],"usage":{"input_tokens":5,"cache_creation_input_tokens":10,"cache_read_input_tokens":100,"output_tokens":7,"output_tokens_details":{"thinking_tokens":2},"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":10}}}`
	resp := ParseResponse([]byte(body))
	if resp.Usage == nil || resp.Usage.CacheRead != 100 || resp.Usage.Create1h != 10 || resp.Usage.ThinkingTokens != 2 {
		t.Fatalf("usage not parsed: %+v", resp.Usage)
	}
	if len(resp.Blocks) != 2 || resp.Blocks[0].Bytes != 2 {
		t.Fatalf("blocks not parsed: %+v", resp.Blocks)
	}

	stream := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_2","usage":{"input_tokens":3,"cache_creation_input_tokens":20,"cache_read_input_tokens":200,"output_tokens":1}}}` + "\n\n" +
		"event: content_block_start\n" + `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}` + "\n\n" +
		"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hel"}}` + "\n\n" +
		"event: content_block_delta\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}` + "\n\n" +
		"event: message_delta\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":9}}` + "\n\n" +
		"event: message_stop\n" + `data: {"type":"message_stop"}` + "\n\n"
	var sp StreamParser
	// Feed in awkward chunks to prove line reassembly works.
	for i := 0; i < len(stream); i += 7 {
		end := min(i+7, len(stream))
		if _, err := sp.Write([]byte(stream[i:end])); err != nil {
			t.Fatal(err)
		}
	}
	got := sp.Result()
	if got.Usage == nil || got.Usage.CacheRead != 200 || got.Usage.Output != 9 || got.Usage.CacheCreation != 20 {
		t.Fatalf("stream usage wrong: %+v", got.Usage)
	}
	if len(got.Blocks) != 1 || got.Blocks[0].Bytes != len("hello") {
		t.Fatalf("stream blocks wrong: %+v", got.Blocks)
	}
	if ParseResponse([]byte(`{"type":"error","error":{"message":"nope"}}`)).Usage != nil {
		t.Fatal("error responses must not yield usage")
	}
}

func TestStoreRoundTripToSession(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	prompt, _, _, _, err := SummarizeRequest([]byte(sampleRequest), store.Labeler())
	if err != nil {
		t.Fatal(err)
	}
	usage := &Usage{Input: 20, CacheCreation: 500, CacheRead: 1000, Output: 40, Create1h: 500}
	for i := 0; i < 3; i++ {
		rec := Record{Timestamp: base.Add(time.Duration(i) * time.Minute), SessionID: "sess/one", RequestID: "req" + string(rune('a'+i)), Path: "/v1/messages", Model: "claude-opus-5", Prompt: prompt, Response: Response{Usage: usage, Blocks: []Block{{Kind: "text", Bytes: 10, Label: "assistant text"}}}}
		if err := store.Append(rec); err != nil {
			t.Fatal(err)
		}
	}
	// A count_tokens record carries no usage and must be skipped, not fail.
	if err := store.Append(Record{Timestamp: base, SessionID: "sess/one", Path: "/v1/messages/count_tokens"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "sess_one.jsonl")
	if !IsLedgerFile(path) {
		t.Fatal("ledger file not recognized")
	}
	s, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Source != transcript.SourceLedger || !s.PrefixVisible || s.RequestCount() != 3 || s.Skipped != 1 {
		t.Fatalf("session shape wrong: source=%s visible=%v requests=%d skipped=%d", s.Source, s.PrefixVisible, s.RequestCount(), s.Skipped)
	}
	req := s.Lanes[0].Requests[0]
	if req.Context[0].Role != transcript.RoleSystem || req.Context[0].Blocks[0].Bytes != len("You are terse.") {
		t.Fatalf("system prefix not reconstructed: %+v", req.Context[0])
	}
	if req.Usage.CacheRead != 1000 || req.Usage.Create1h != 500 || req.Output == nil || len(req.Output.Blocks) != 1 {
		t.Fatalf("request not reconstructed: %+v", req)
	}
}
