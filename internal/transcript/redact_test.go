package transcript

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// An MCP tool name carries the server that provided it, sometimes as a raw
// connector UUID. The published test fixture leaked exactly that.
func TestRedactHidesTheMCPServerButKeepsTheShape(t *testing.T) {
	// Redact drops any line without a uuid, so a synthetic line needs one.
	in := `{"uuid":"11111111-2222-3333-4444-555555555555","type":"assistant","message":{"content":[` +
		`{"type":"tool_use","id":"x","name":"mcp__bf7c680d-5fdc-5ef4-b4a0-abadb619bf0a__get_session","input":{}},` +
		`{"type":"tool_use","id":"y","name":"mcp__bf7c680d-5fdc-5ef4-b4a0-abadb619bf0a__send_later","input":{}},` +
		`{"type":"tool_use","id":"z","name":"Bash","input":{}}]}}` + "\n"
	var out bytes.Buffer
	if err := Redact(strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "bf7c680d") {
		t.Fatalf("connector id survived redaction: %s", got)
	}
	if !strings.Contains(got, `"name":"Bash"`) {
		t.Fatalf("a built-in tool name must be kept: %s", got)
	}
	if !strings.Contains(got, "__get_session") || !strings.Contains(got, "__send_later") {
		t.Fatalf("the operation must survive so analysis still works: %s", got)
	}
	// Two tools from one server must land on one hashed server, or grouping breaks.
	ids := regexp.MustCompile(`mcp__(s_[0-9a-f]+)__`).FindAllStringSubmatch(got, -1)
	if len(ids) != 2 || ids[0][1] != ids[1][1] {
		t.Fatalf("same server must hash the same: %v", ids)
	}
}
