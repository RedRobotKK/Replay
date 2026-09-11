package transcript

import (
	"encoding/json"
	"testing"
)

// countingLabel records every call so a test can assert the label function
// was consulted, or was not. An equality check cannot tell the difference:
// the value it would have produced is the value already in the map.
type countingLabel struct {
	calls  int
	inputs []string
}

func (c *countingLabel) fn(name string, input json.RawMessage) string {
	c.calls++
	c.inputs = append(c.inputs, string(input))
	return "labelled:" + name
}

func toolUse(id, name, input string) RawBlock {
	return RawBlock{Type: KindToolUse, ID: id, Name: name, Input: json.RawMessage(input)}
}

// TestLO1AnIDAlreadyNamedIsNotNamedAgain is the whole point of the change.
// collectToolNames walks every assistant tool_use block and stores a label
// for each id before the decoder builds a single message; DecodeBlock then
// walks the same blocks, with the same map and the same label function, and
// called the label function again for every one of them. The second result
// is the first result — the label is a pure function of the two arguments —
// so only a call count can show that it happened.
func TestLO1AnIDAlreadyNamedIsNotNamedAgain(t *testing.T) {
	c := &countingLabel{}
	names := map[string]string{"tu_1": "Read already/known.go"}

	b := DecodeBlock(toolUse("tu_1", "Read", `{"file_path":"already/known.go"}`), RoleAssistant, names, c.fn)

	if c.calls != 0 {
		t.Fatalf("label function called %d times for an id already in the map, want 0: %v", c.calls, c.inputs)
	}
	if got := names["tu_1"]; got != "Read already/known.go" {
		t.Fatalf("toolNames[tu_1] = %q, want the label that was already there", got)
	}
	// The block itself has never carried the label: it names the tool.
	if b.Label != LabelToolCallPrefix+"Read" || b.ToolName != "Read" {
		t.Fatalf("block = %+v, want it labelled from the tool name", b)
	}
}

// TestLO2NowhereToPutTheNameMeansNothingToCompute covers the two shapes where
// the label was computed and then dropped on the floor: a block with no id,
// and a caller that passed no map. The label function reads the whole tool
// input — a file write carries the file — so this is not a small thing to do
// for a value nobody reads.
func TestLO2NowhereToPutTheNameMeansNothingToCompute(t *testing.T) {
	t.Run("no id", func(t *testing.T) {
		c := &countingLabel{}
		names := map[string]string{}
		DecodeBlock(toolUse("", "Write", `{"file_path":"x.go"}`), RoleAssistant, names, c.fn)
		if c.calls != 0 {
			t.Fatalf("label function called %d times for a block with no id, want 0", c.calls)
		}
		if len(names) != 0 {
			t.Fatalf("toolNames = %v, want nothing written under an empty id", names)
		}
	})
	t.Run("no map", func(t *testing.T) {
		c := &countingLabel{}
		DecodeBlock(toolUse("tu_2", "Write", `{"file_path":"x.go"}`), RoleAssistant, nil, c.fn)
		if c.calls != 0 {
			t.Fatalf("label function called %d times with no map to fill, want 0", c.calls)
		}
	})
}

// TestLO3AnUnknownIDIsStillNamed is the other half: the skip must not turn
// the label function off. A parser whose map is empty — every parser except
// Claude Code's, which is the one that pre-fills it — has to keep working.
func TestLO3AnUnknownIDIsStillNamed(t *testing.T) {
	c := &countingLabel{}
	names := map[string]string{}

	DecodeBlock(toolUse("tu_3", "Bash", `{"command":"ls"}`), RoleAssistant, names, c.fn)

	if c.calls != 1 {
		t.Fatalf("label function called %d times for an unknown id, want 1", c.calls)
	}
	if got := names["tu_3"]; got != "labelled:Bash" {
		t.Fatalf("toolNames[tu_3] = %q, want the label function's answer", got)
	}
	if len(c.inputs) != 1 || c.inputs[0] != `{"command":"ls"}` {
		t.Fatalf("label function saw %v, want the block's input", c.inputs)
	}
}

// TestLO4WithoutALabelFunctionTheToolNameIsTheLabel pins the nil-label arm,
// which is what the ledger's streaming decoder uses.
func TestLO4WithoutALabelFunctionTheToolNameIsTheLabel(t *testing.T) {
	names := map[string]string{}
	DecodeBlock(toolUse("tu_4", "Grep", `{"pattern":"x"}`), RoleAssistant, names, nil)
	if got := names["tu_4"]; got != "Grep" {
		t.Fatalf("toolNames[tu_4] = %q, want the bare tool name", got)
	}
}

// TestLO5AResultStillResolvesThroughTheMap is the reason the map is written
// at all: a tool_result carries only the id of the call it answers.
func TestLO5AResultStillResolvesThroughTheMap(t *testing.T) {
	c := &countingLabel{}
	names := map[string]string{}
	DecodeBlock(toolUse("tu_5", "Read", `{"file_path":"a/b.go"}`), RoleAssistant, names, c.fn)

	res := DecodeBlock(RawBlock{Type: KindToolResult, ToolUseID: "tu_5", Content: json.RawMessage(`"ok"`)}, RoleUser, names, c.fn)
	if res.ToolName != "labelled:Read" {
		t.Fatalf("result ToolName = %q, want the label stored for tu_5", res.ToolName)
	}
	if res.Label != LabelToolResultPrefix+"labelled:Read" {
		t.Fatalf("result Label = %q", res.Label)
	}
}
