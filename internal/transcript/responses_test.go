package transcript

import "testing"

// The Responses API usage object counts INCLUSIVELY, like chat completions:
// input_tokens already contains the cached share, and on GPT-5.6 and later
// the written share too. This package's Usage is exclusive, so both are
// subtracted out of Input. Reasoning tokens are a share of output, carried
// as ThinkingTokens. CacheCreation is zero unless the provider sent a write
// figure: nothing is reconstructed from the uncached remainder.
func TestResponsesUsage_NormalisesInclusiveCounts(t *testing.T) {
	write := 1200
	cases := []struct {
		name string
		in   *ResponsesUsage
		want Usage
	}{
		{"nil receiver", nil, Usage{}},
		{"no details", &ResponsesUsage{InputTokens: 1500, OutputTokens: 300}, Usage{Input: 1500, Output: 300}},
		{"cached share subtracted", withInputDetails(1500, 1000, nil, 300, 200), Usage{Input: 500, CacheRead: 1000, Output: 300, ThinkingTokens: 200}},
		{"write field present", withInputDetails(1500, 0, &write, 50, 0), Usage{Input: 300, CacheCreation: 1200, Output: 50}},
		{"cached above input is clamped", withInputDetails(100, 900, nil, 1, 0), Usage{Input: 0, CacheRead: 100, Output: 1}},
		{"write above the remainder is clamped", withInputDetails(100, 80, &write, 1, 0), Usage{Input: 0, CacheRead: 80, CacheCreation: 20, Output: 1}},
	}
	for _, c := range cases {
		if got := c.in.Usage(); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	if !(&ResponsesUsage{}).Empty() || withInputDetails(1, 0, nil, 0, 0).Empty() {
		t.Error("Empty: an all-zero usage is empty and one with any token is not")
	}
}

func withInputDetails(input, cached int, write *int, output, reasoning int) *ResponsesUsage {
	u := &ResponsesUsage{InputTokens: input, OutputTokens: output, TotalTokens: input + output}
	u.InputDetails = &ResponsesInputDetails{CachedTokens: cached, CacheWriteTokens: write}
	u.OutputDetails = &ResponsesOutputDetails{ReasoningTokens: reasoning}
	return u
}

// The request. Every item kind Codex sends is reduced to a block with a
// kind, a size, a role and, for a tool call, a content-free call key; no
// message text leaves this function.
func TestParseResponsesRequest_ReadsEveryItemKindWithoutKeepingText(t *testing.T) {
	body := []byte(`{"model":"gpt-6-astra","stream":true,"instructions":"be brief","prompt_cache_key":"conv-1",` +
		`"tools":[{"type":"function","name":"shell","parameters":{"type":"object"}},{"type":"web_search"}],` +
		`"input":[` +
		`{"type":"message","role":"user","content":[{"type":"input_text","text":"list the files"}]},` +
		`{"role":"developer","content":"house rules"},` +
		`{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}],"encrypted_content":"abcd"},` +
		`{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{\"command\":[\"ls\"]}"},` +
		`{"type":"function_call_output","call_id":"call_1","output":"total 0"},` +
		`{"type":"item_reference","id":"msg_9"}` +
		`]}`)
	req, err := ParseResponsesRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "gpt-6-astra" || !req.Stream || req.PromptCacheKey != "conv-1" || req.Instructions != len("be brief") || req.Bytes != len(body) {
		t.Errorf("header fields: %+v", *req)
	}
	if len(req.Tools) != 2 || req.Tools[0].Name != "shell" || req.Tools[1].Name != "web_search" || req.Tools[0].Bytes == 0 {
		t.Errorf("tools: %+v", req.Tools)
	}
	want := []struct {
		role, kind string
		bytes      int
	}{
		{RoleUser, KindText, len("list the files")},
		{RoleSystem, KindText, len("house rules")},
		{RoleAssistant, KindThinking, len("thinking") + len("abcd")},
		{RoleAssistant, KindToolUse, len("shell") + len(`{"command":["ls"]}`)},
		{"tool", KindToolResult, len("total 0")},
		{RoleUser, KindOther, len(`{"type":"item_reference","id":"msg_9"}`)},
	}
	if len(req.Items) != len(want) {
		t.Fatalf("items = %d, want %d: %+v", len(req.Items), len(want), req.Items)
	}
	for i, w := range want {
		got := req.Items[i]
		if got.Role != w.role || got.Block.Kind != w.kind || got.Block.Bytes != w.bytes {
			t.Errorf("item %d: role=%q kind=%q bytes=%d, want %q %q %d", i, got.Role, got.Block.Kind, got.Block.Bytes, w.role, w.kind, w.bytes)
		}
		if got.Block.Kind != KindToolUse && got.Block.Text != "" {
			t.Errorf("item %d keeps text %q", i, got.Block.Text)
		}
	}
	call := req.Items[3].Block
	if call.ToolName != "shell" || call.ToolUseID != "call_1" || call.CallKey == "" {
		t.Errorf("tool call identity: %+v", call)
	}
	if req.Items[4].Block.ToolUseID != "call_1" {
		t.Errorf("tool result not joined to its call: %+v", req.Items[4].Block)
	}
	// A plain string input is one user message.
	short, err := ParseResponsesRequest([]byte(`{"model":"gpt-6-astra","input":"hello"}`))
	if err != nil || len(short.Items) != 1 || short.Items[0].Block.Bytes != 5 || short.Items[0].Role != RoleUser {
		t.Errorf("string input: %+v %v", short, err)
	}
}

// Identical calls share a key; a different argument is a different call.
// The loop detector counts repeats of the key and nothing else.
func TestParseResponsesRequest_CallKeysIdentifyRepeatedCalls(t *testing.T) {
	item := func(id, args string) string {
		return `{"type":"function_call","call_id":"` + id + `","name":"shell","arguments":"` + args + `"}`
	}
	req, err := ParseResponsesRequest([]byte(`{"model":"m","input":[` + item("a", `ls`) + `,` + item("b", `ls`) + `,` + item("c", `pwd`) + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	a, b, c := req.Items[0].Block.CallKey, req.Items[1].Block.CallKey, req.Items[2].Block.CallKey
	if a != b {
		t.Errorf("identical calls keyed differently: %q %q", a, b)
	}
	if a == c {
		t.Errorf("different arguments share a key: %q", a)
	}
}
