package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func rpc(t *testing.T, in string) []map[string]any {
	t.Helper()
	var out, errb bytes.Buffer
	if err := runMCP(strings.NewReader(in), &out, &errb); err != nil {
		t.Fatalf("runMCP: %v (stderr %q)", err, errb.String())
	}
	var got []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("not JSON-RPC: %q", line)
		}
		got = append(got, m)
	}
	return got
}

// TestMC1: initialize answers with a protocol version and server name.
func TestMC1(t *testing.T) {
	got := rpc(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if len(got) != 1 {
		t.Fatalf("want one response, got %d", len(got))
	}
	r, _ := got[0]["result"].(map[string]any)
	if r == nil {
		t.Fatalf("no result: %v", got[0])
	}
	if r["protocolVersion"] == nil {
		t.Error("no protocolVersion in initialize result")
	}
}

// TestMC2: tools/list names tools, each with a description and a schema.
func TestMC2(t *testing.T) {
	got := rpc(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	r, _ := got[0]["result"].(map[string]any)
	tools, _ := r["tools"].([]any)
	if len(tools) == 0 {
		t.Fatal("no tools advertised")
	}
	for _, x := range tools {
		m := x.(map[string]any)
		if m["name"] == nil || m["description"] == nil || m["inputSchema"] == nil {
			t.Errorf("tool missing name, description or inputSchema: %v", m)
		}
	}
}

// TestMC3: an unknown method is a JSON-RPC error, not a crash and not silence.
func TestMC3(t *testing.T) {
	got := rpc(t, `{"jsonrpc":"2.0","id":3,"method":"nope/nope"}`)
	e, _ := got[0]["error"].(map[string]any)
	if e == nil {
		t.Fatalf("unknown method did not error: %v", got[0])
	}
	if e["code"] == nil {
		t.Error("error carries no code")
	}
}

// TestMC4: a notification, which has no id, gets no response.
//
// JSON-RPC requires silence on notifications. A server that answers them
// corrupts every client that batches.
func TestMC4(t *testing.T) {
	got := rpc(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if len(got) != 0 {
		t.Errorf("notification produced a response: %v", got)
	}
}

// TestMC5: no tool may return prompt content.
//
// The whole surface runs on the user's own transcripts. A tool that hands an
// agent back the text of a previous conversation is an exfiltration path with
// a friendly name.
func TestMC5(t *testing.T) {
	got := rpc(t, `{"jsonrpc":"2.0","id":5,"method":"tools/list"}`)
	r, _ := got[0]["result"].(map[string]any)
	tools, _ := r["tools"].([]any)
	for _, x := range tools {
		m := x.(map[string]any)
		d := strings.ToLower(m["description"].(string))
		for _, bad := range []string{"message text", "prompt text", "conversation content"} {
			if strings.Contains(d, bad) {
				t.Errorf("tool %v advertises returning content: %q", m["name"], d)
			}
		}
	}
}
