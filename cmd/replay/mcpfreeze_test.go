package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The local MCP tool surface is frozen.
//
// These seven names are an interface an agent's configuration depends on, and
// two of them are refusals rather than features: `replay_rules_latest` and
// `replay_installer_release` exist to decline and hand back a URL, because
// Replay never pays and never fetches on a caller's behalf.
//
// The freeze is a decision recorded in CLAUDE-INGEST.md, not a style
// preference. Three things it forbids, each of which would look like an
// improvement in a diff:
//
//   - `replay_diff`, blame, or anything carrying a FINDING. A finding returned
//     over MCP becomes the next model turn's input, so the instrument would be
//     writing into the workload it measures.
//   - `search_tools` / `invoke_tool` / any meta-tool that rewrites `tools/list`.
//     That is a compression proxy, which is a different product and one this
//     project has explicitly declined to clone.
//   - Anything returning message text. The whole surface runs over the user's
//     own transcripts, and a tool that hands an agent the content of an earlier
//     conversation is an exfiltration path with a friendly name.
//
// Adding a tool is also a prefix tax on every session that connects: the
// measurement this project keeps quoting is 226,829 bytes of tool definitions
// against 6,815 bytes of CLAUDE.md on one ledger. A server that grows its own
// `tools/list` weekly is charging its users for the privilege.
//
// Lifting the freeze is allowed. Doing it by accident is not — change this
// list and say why in the same commit.
func TestMCPFreeze_TheSevenToolNames(t *testing.T) {
	want := []string{
		"replay_surfaces",
		"replay_price_check",
		"replay_rules_free",
		"replay_mcp_overhead",
		"replay_rules_latest",
		"replay_installer_release",
		"replay_quota",
	}

	got := mcpTools()
	if len(got) != len(want) {
		var names []string
		for _, tl := range got {
			names = append(names, tl.Name)
		}
		t.Fatalf("the local MCP surface has %d tools, frozen at %d.\n"+
			"      got:  %s\n"+
			"      want: %s\n"+
			"      Lifting the freeze is allowed; doing it silently is not.",
			len(got), len(want), strings.Join(names, ", "), strings.Join(want, ", "))
	}

	// Order is part of the contract. `tools/list` bytes are what a client
	// caches, and reordering them changes the prefix for every connected
	// session without changing a single capability.
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("tool %d is %q, frozen as %q", i, got[i].Name, w)
		}
	}
}

// Every tool must carry the three fields a client needs to render and call it.
func TestMCPFreeze_EveryToolIsWellFormed(t *testing.T) {
	for _, tl := range mcpTools() {
		if tl.Name == "" {
			t.Error("a tool has no name")
			continue
		}
		if strings.TrimSpace(tl.Description) == "" {
			t.Errorf("%s has no description; a client renders this to a human", tl.Name)
		}
		b, err := json.Marshal(tl.InputSchema)
		if err != nil {
			t.Errorf("%s: input schema does not marshal: %v", tl.Name, err)
			continue
		}
		var schema map[string]any
		if err := json.Unmarshal(b, &schema); err != nil {
			t.Errorf("%s: input schema is not an object: %v", tl.Name, err)
			continue
		}
		if schema["type"] != "object" {
			t.Errorf("%s: inputSchema type is %v, want \"object\". The MCP tool envelope "+
				"requires it and a client that validates will refuse the tool", tl.Name, schema["type"])
		}
	}
}

// The two refusals must actually refuse, and name where the answer lives.
//
// A caller reads the description to decide whether to call, then acts on the
// result. `replay_rules_latest` and `replay_installer_release` both concern
// something only the network knows, and both decline: Replay never pays and
// never fetches on a caller's behalf.
//
// This asserts the RESULT, not the wording. A first version of this test
// matched the descriptions for "not", "refuse" or "never" and failed
// replay_installer_release, whose description refuses in the positive —
// "names the source instead of answering from a compiled-in value". That was
// a test about vocabulary, and the vocabulary was fine. What a caller can
// rely on is that the tool returns a pointer rather than a stale fact, and
// that is what is checked here.
func TestMCPFreeze_TheRefusalsReturnAPointerNotAnAnswer(t *testing.T) {
	for _, name := range []string{"replay_rules_latest", "replay_installer_release"} {
		out, err := mcpCall(name, nil)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !strings.Contains(out, "https://") {
			t.Errorf("%s declines to answer and does not say where the answer is:\n      %s",
				name, out)
		}
		// The refusal is the feature. A build that started answering from a
		// compiled-in value would silently go stale, which is the failure the
		// whole dated-rules design exists to prevent.
		low := strings.ToLower(out)
		if !strings.Contains(low, "does not") && !strings.Contains(low, "never") {
			t.Errorf("%s no longer states that it is declining:\n      %s", name, out)
		}
	}
}
