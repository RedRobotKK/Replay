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

// The guard gate on PR #334 reported the error and clamp branches of this
// adapter as never entered by any test. Each one below is a contract a
// caller depends on, written RED against the branch neutralised (the
// behaviour already existed; the evidence did not).

// A share below zero is a provider bug or a shape this build does not
// understand. It is clamped to zero so a negative fresh count never flows
// into a cost figure as a saving, and the inclusive input is kept whole.
func TestResponsesUsage_NegativeSharesAreClampedNotSubtracted(t *testing.T) {
	negWrite := -7
	cases := []struct {
		name string
		in   *ResponsesUsage
		want Usage
	}{
		{"negative cached", withInputDetails(1500, -5, nil, 300, 0), Usage{Input: 1500, Output: 300}},
		{"negative write", withInputDetails(1500, 0, &negWrite, 300, 0), Usage{Input: 1500, Output: 300}},
		{"both negative", withInputDetails(1500, -5, &negWrite, 300, 0), Usage{Input: 1500, Output: 300}},
	}
	for _, c := range cases {
		if got := c.in.Usage(); got != c.want {
			t.Errorf("%s: %+v, want %+v (a negative share must not inflate Input or appear as a read or write)", c.name, got, c.want)
		}
	}
}

// A body the parser cannot read is an error, not a partial summary. The
// proxy forwards such a request unsummarised, which is the established
// behaviour on every path, and it can only do that if this returns an error
// rather than an empty request it would guard and ledger as real.
func TestParseResponsesRequest_RefusesWhatItCannotRead(t *testing.T) {
	cases := map[string][]byte{
		"not JSON":                    []byte("<html>"),
		"input is an object":          []byte(`{"model":"m","input":{"bad":1}}`),
		"input item is not an object": []byte(`{"model":"m","input":["just a string"]}`),
		"input item is a bare number": []byte(`{"model":"m","input":[42]}`),
	}
	for name, body := range cases {
		req, err := ParseResponsesRequest(body)
		if err == nil {
			t.Errorf("%s: no error, got %+v; the proxy would guard and ledger a request it could not read", name, req)
		}
		if req != nil {
			t.Errorf("%s: a request was returned beside the error: %+v", name, req)
		}
	}
}

// A content value that is absent, a plain string, or a list of parts is
// measured by the text it carries. Absent is zero; a string is its byte
// length, escapes and multibyte characters included, exactly as a part's
// text would be. ContentBytes already gives both answers, so this pins the
// contract that let the string fast path be removed as redundant.
func TestResponsesContentBytes_MeasuresText(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{``, 0},
		{`""`, 0},
		{`"hello"`, 5},
		{`"a\"b\\c"`, 5},
		{`"日本語"`, len("日本語")},
		{`[{"type":"input_text","text":"hello"},{"type":"input_text","text":"日本語"}]`, 5 + len("日本語")},
		{`[{"type":"input_image","image_url":"data:x"}]`, len("type") + len("input_image") + len("image_url") + len("data:x")},
	}
	for _, c := range cases {
		if got := ResponsesContentBytes([]byte(c.raw)); got != c.want {
			t.Errorf("%s: %d, want %d", c.raw, got, c.want)
		}
	}
	// And through the request reader: an item with no content and a tool
	// result with no output are legal and measure zero.
	req, err := ParseResponsesRequest([]byte(`{"model":"m","input":[{"type":"message","role":"user"},{"type":"function_call_output","call_id":"c"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Items) != 2 || req.Items[0].Block.Bytes != 0 || req.Items[1].Block.Bytes != 0 {
		t.Errorf("content-less items: %+v", req.Items)
	}
}
