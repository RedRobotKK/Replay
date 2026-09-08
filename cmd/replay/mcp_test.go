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

// One server, not two. The local server must serve the whole vocabulary the
// hosted one advertises, so a user configures one endpoint.
func TestMC6(t *testing.T) {
	got := rpc(t, `{"jsonrpc":"2.0","id":6,"method":"tools/list"}`)
	r, _ := got[0]["result"].(map[string]any)
	have := map[string]bool{}
	for _, x := range r["tools"].([]any) {
		have[x.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{
		"replay_price_check", "replay_rules_free", "replay_mcp_overhead",
		"replay_surfaces", "replay_quota",
	} {
		if !have[want] {
			t.Errorf("merged server does not serve %s", want)
		}
	}
}

// A tool that needs the network must say so rather than returning a figure
// that looks compiled-in. Provenance is the product.
func TestMC7(t *testing.T) {
	for _, name := range []string{"replay_rules_latest", "replay_installer_release"} {
		out := rpc(t, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"`+name+`","arguments":{}}}`)
		r, _ := out[0]["result"].(map[string]any)
		if r == nil {
			continue // an error is an acceptable answer; silence is not
		}
		txt := r["content"].([]any)[0].(map[string]any)["text"].(string)
		if !strings.Contains(strings.ToLower(txt), "network") && !strings.Contains(strings.ToLower(txt), "redrobot.jp") {
			t.Errorf("%s did not say it needs a remote source: %q", name, txt)
		}
	}
}

// mcp_overhead prices what a client's tool definitions cost to carry, and the
// arithmetic must be checkable from the numbers it prints.
func TestMC8(t *testing.T) {
	out := rpc(t, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"replay_mcp_overhead","arguments":{"model":"claude-opus-5","bytes":40000,"requests":100}}}`)
	r, _ := out[0]["result"].(map[string]any)
	if r == nil {
		t.Fatalf("no result: %v", out[0])
	}
	txt := r["content"].([]any)[0].(map[string]any)["text"].(string)
	for _, want := range []string{"40,000", "100", "$"} {
		if !strings.Contains(txt, want) {
			t.Errorf("overhead answer missing %q:\n%s", want, txt)
		}
	}
	if !strings.Contains(strings.ToLower(txt), "estimat") {
		t.Error("a token count derived from bytes is an estimate and must say so")
	}
}

// `mcp --install` prints a config a person can paste, and it must be valid
// JSON naming this binary. A setup snippet that does not parse is worse than
// no snippet: it fails inside somebody else's config file.
func TestMC9(t *testing.T) {
	var out, errb bytes.Buffer
	if err := run([]string{"mcp", "--install"}, &out, &errb); err != nil && err != errHelpShown {
		t.Fatalf("run: %v (%s)", err, errb.String())
	}
	s := out.String()
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j < i {
		t.Fatalf("no JSON object in the snippet:\n%s", s)
	}
	var cfg struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(s[i:j+1]), &cfg); err != nil {
		t.Fatalf("snippet is not valid JSON: %v\n%s", err, s[i:j+1])
	}
	e, ok := cfg.MCPServers["replay"]
	if !ok {
		t.Fatalf("no replay server in the snippet: %v", cfg.MCPServers)
	}
	if e.Command == "" || len(e.Args) == 0 || e.Args[0] != "mcp" {
		t.Errorf("snippet does not invoke `replay mcp`: %+v", e)
	}
}

// TestMC10: the snippet is valid JSON for a Windows path.
//
// The defect this pins was invisible on Unix. mcpInstallSnippet concatenated
// the binary path into a JSON string literal, so on Windows the command read
// "C:\Users\...\replay.exe", and \U is not a valid JSON escape. The snippet
// the reader is told to paste into their agent's configuration would be
// rejected by any parser.
//
// TestMC9 could not catch it, because it runs with whatever path the host
// produces and every developer machine here is Unix. Only CI on
// windows-latest failed. This test supplies the path instead of inheriting
// it, so the case is checked everywhere.
func TestMC10_SnippetSurvivesAWindowsPath(t *testing.T) {
	for _, bin := range []string{
		`C:\Users\runneradmin\go\bin\replay.exe`,
		`C:\Program Files\Replay\replay.exe`,
		`/usr/local/bin/replay`,
		`/home/a b/replay`,
	} {
		s := mcpInstallSnippet(bin)
		i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
		if i < 0 || j < i {
			t.Fatalf("no JSON object in the snippet for %q", bin)
		}
		var cfg struct {
			MCPServers map[string]struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal([]byte(s[i:j+1]), &cfg); err != nil {
			t.Errorf("snippet for %q is not valid JSON: %v\n%s", bin, err, s[i:j+1])
			continue
		}
		if got := cfg.MCPServers["replay"].Command; got != bin {
			t.Errorf("snippet for %q round-tripped the command as %q", bin, got)
		}
	}
}
