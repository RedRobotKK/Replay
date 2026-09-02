package transcript

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

const fixture = "testdata/session-redacted.jsonl"

func TestParseFixture(t *testing.T) {
	s, err := ParseClaudeCodeFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Lanes) != 1 {
		t.Fatalf("lanes = %d, want 1", len(s.Lanes))
	}
	lane := s.Lanes[0]
	if len(lane.Requests) < 40 {
		t.Fatalf("requests = %d, want a full session", len(lane.Requests))
	}
	if s.ClientVersion == "" || s.ID != "redacted" {
		t.Fatalf("session metadata not read: version %q id %q", s.ClientVersion, s.ID)
	}

	first := lane.Requests[0]
	if first.Usage.CacheRead == 0 || first.Usage.CacheCreation == 0 {
		t.Fatalf("first request usage not parsed: %+v", first.Usage)
	}
	if first.Usage.Create1h != first.Usage.CacheCreation {
		t.Fatalf("TTL breakdown not parsed: %+v", first.Usage)
	}
	if first.Usage.ThinkingTokens == 0 {
		t.Fatalf("thinking tokens not parsed: %+v", first.Usage)
	}
	if first.Model == "" || first.Effort == "" || first.Timestamp.IsZero() {
		t.Fatalf("request metadata missing: %+v", first)
	}

	// Output blocks are grouped across lines in API order.
	kinds := make([]string, 0, len(first.Output.Blocks))
	for _, b := range first.Output.Blocks {
		kinds = append(kinds, b.Kind)
	}
	if strings.Join(kinds, ",") != "thinking,text,tool_use" {
		t.Fatalf("first output block order = %v", kinds)
	}

	// Context grows monotonically along the lane and each request's context
	// ends with a user message.
	for i, r := range lane.Requests {
		if len(r.Context) == 0 {
			t.Fatalf("request %d has empty context", i)
		}
		if last := r.Context[len(r.Context)-1]; last.Role != RoleUser {
			t.Fatalf("request %d context ends with %s, want user", i, last.Role)
		}
		if i > 0 && len(r.Context) < len(lane.Requests[i-1].Context) {
			t.Fatalf("request %d context shrank: %d < %d", i, len(r.Context), len(lane.Requests[i-1].Context))
		}
	}
}

func TestToolResultsResolveToToolNames(t *testing.T) {
	s, err := ParseClaudeCodeFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	last := s.Lanes[0].Requests[len(s.Lanes[0].Requests)-1]
	resolved, unresolved := 0, 0
	for _, m := range last.Context {
		for _, b := range m.Blocks {
			if b.Kind != KindToolResult {
				continue
			}
			if b.ToolName == "unknown tool" {
				unresolved++
			} else {
				resolved++
			}
		}
	}
	if resolved == 0 || unresolved > 0 {
		t.Fatalf("tool results resolved %d, unresolved %d", resolved, unresolved)
	}
}

func TestParseRejectsEmptyInput(t *testing.T) {
	if _, err := ParseClaudeCode(strings.NewReader("")); err == nil {
		t.Fatal("expected an error for empty input")
	}
	if _, err := ParseClaudeCode(strings.NewReader(`{"type":"mode","mode":"normal"}` + "\n")); err == nil {
		t.Fatal("expected an error when no requests are present")
	}
}

func TestRedactPreservesAnalysisAndDropsContent(t *testing.T) {
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Redact(bytes.NewReader(raw), &out); err != nil {
		t.Fatal(err)
	}
	// Redacting an already redacted file must be a fixed point: same usage,
	// same structure, same byte sizes.
	before, err := ParseClaudeCode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	after, err := ParseClaudeCode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if before.RequestCount() != after.RequestCount() {
		t.Fatalf("request count changed: %d -> %d", before.RequestCount(), after.RequestCount())
	}
	for i := range before.Lanes[0].Requests {
		b, a := before.Lanes[0].Requests[i], after.Lanes[0].Requests[i]
		if b.Usage != a.Usage {
			t.Fatalf("request %d usage changed: %+v -> %+v", i, b.Usage, a.Usage)
		}
		if len(b.Context) != len(a.Context) {
			t.Fatalf("request %d context length changed", i)
		}
		for j := range b.Context {
			if b.Context[j].Bytes() != a.Context[j].Bytes() {
				t.Fatalf("request %d message %d bytes changed: %d -> %d", i, j, b.Context[j].Bytes(), a.Context[j].Bytes())
			}
		}
	}
	for _, leak := range []string{"/root/", "/home/", "/Users/", "sk-ant-", "ghp_"} {
		if bytes.Contains(out.Bytes(), []byte(leak)) {
			t.Fatalf("redacted output contains %q", leak)
		}
	}
}

func TestParseSkipsMalformedRequest(t *testing.T) {
	good := `{"type":"user","uuid":"u1","parentUuid":null,"timestamp":"2026-09-02T00:00:00Z","sessionId":"s","version":"1","message":{"role":"user","content":"hi"}}` + "\n" +
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:01Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}` + "\n"
	bad := `{"type":"assistant","uuid":"a2","parentUuid":"a1","requestId":"r2","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:02Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"interrupted"}]}}` + "\n"
	s, err := ParseClaudeCode(strings.NewReader(good + bad))
	if err != nil {
		t.Fatal(err)
	}
	if s.RequestCount() != 1 || s.Skipped != 1 {
		t.Fatalf("requests %d skipped %d, want 1 and 1", s.RequestCount(), s.Skipped)
	}
}

func TestTruncateLabelIsRuneAware(t *testing.T) {
	label := strings.Repeat("プ", 10)
	got := TruncateLabel(label, 5)
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 5 || !strings.HasSuffix(got, "…") {
		t.Fatalf("TruncateLabel = %q", got)
	}
	if TruncateLabel("short", 10) != "short" {
		t.Fatal("short labels must be returned unchanged")
	}
}

func TestRedactPreservesToolCallBytes(t *testing.T) {
	input := `{"type":"assistant","uuid":"a1","parentUuid":null,"requestId":"r1","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:01Z","message":{"role":"assistant","model":"m","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/Users/me/very/long/path/to/app.go","limit":10,"nested":{"query":"password.hunter2","flag":true}}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}` + "\n"
	var out bytes.Buffer
	if err := Redact(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	before, err := ParseClaudeCode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	after, err := ParseClaudeCode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	b, a := before.Lanes[0].Requests[0].Output.Blocks[0], after.Lanes[0].Requests[0].Output.Blocks[0]
	if b.Bytes != a.Bytes {
		t.Fatalf("tool call bytes changed by redaction: %d -> %d\n%s", b.Bytes, a.Bytes, out.String())
	}
	got := out.String()
	if strings.Contains(got, "hunter2") || strings.Contains(got, "/Users/me") {
		t.Fatalf("redaction leaked a value:\n%s", got)
	}
	if !strings.Contains(got, ".go") || !strings.Contains(got, `"limit":10`) || !strings.Contains(got, `"flag":true`) {
		t.Fatalf("redaction dropped structure it should keep:\n%s", got)
	}
}

func TestRedactPreservesEqualityWithinFile(t *testing.T) {
	line := func(uuid, parent, req, cmd string) string {
		return `{"type":"assistant","uuid":"` + uuid + `","parentUuid":` + parent + `,"requestId":"` + req + `","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:01Z","message":{"role":"assistant","model":"m","content":[{"type":"tool_use","id":"` + uuid + `","name":"Bash","input":{"command":"` + cmd + `"}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}` + "\n"
	}
	input := line("a1", "null", "r1", "go test ./...") + line("a2", `"a1"`, "r2", "go test ./...") + line("a3", `"a2"`, "r3", "go vet ./...")
	var out bytes.Buffer
	if err := Redact(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	s, err := ParseClaudeCode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	reqs := s.Lanes[0].Requests
	same := reqs[0].Output.Blocks[0].Text == reqs[1].Output.Blocks[0].Text
	different := reqs[1].Output.Blocks[0].Text != reqs[2].Output.Blocks[0].Text
	if !same || !different {
		t.Fatalf("redaction must keep equal inputs equal and distinct inputs distinct: same=%v different=%v", same, different)
	}
}

func TestRedactReplacesTextWithFiller(t *testing.T) {
	line := `{"type":"user","uuid":"u1","parentUuid":null,"timestamp":"2026-09-02T00:00:00Z","sessionId":"s","cwd":"/Users/me","message":{"role":"user","content":[{"type":"text","text":"my secret is sk-ant-abc"}]}}` + "\n" +
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:01Z","message":{"role":"assistant","model":"m","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/Users/me/app.go","limit":10}}],"usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}` + "\n"
	var out bytes.Buffer
	if err := Redact(strings.NewReader(line), &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, leak := range []string{"secret", "sk-ant", "/Users/me", "app.go\""} {
		if strings.Contains(got, leak) {
			t.Fatalf("redacted output still contains %q:\n%s", leak, got)
		}
	}
	if !strings.Contains(got, `"name":"Read"`) || !strings.Contains(got, ".go") {
		t.Fatalf("redaction dropped structure it should keep:\n%s", got)
	}
	if strings.Contains(got, "xxxxxxxx") {
		t.Fatalf("filler must be derived from content, not a constant:\n%s", got)
	}
	if !strings.Contains(got, `"cache_read_input_tokens":3`) {
		t.Fatalf("redaction changed usage:\n%s", got)
	}
}
