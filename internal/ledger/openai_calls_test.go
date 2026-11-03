package ledger

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A chat/completions assistant message can carry several tool calls. The loop
// guard has to see every one of them, and the ledger has to keep the shape it
// has always written: one message per message, one block each, the first call
// keyed. Records are never re-summarised, so a new shape would make every
// session that spans an upgrade look edited at the boundary, and analysis
// (ErrorCosts, ErrorShare, the friction report, learn's guard) reads this shape.

func oaiCallJSON(id, cmd string) string {
	return `{"id":"` + id + `","type":"function","function":{"name":"Bash","arguments":"{\"cmd\":\"` + cmd + `\"}"}}`
}

func oaiBody(messages ...string) []byte {
	return []byte(`{"model":"gpt-x","messages":[` + strings.Join(messages, ",") + `]}`)
}

func oaiAssistantJSON(calls ...string) string {
	return `{"role":"assistant","content":null,"tool_calls":[` + strings.Join(calls, ",") + `]}`
}

const (
	oaiSystem = `{"role":"system","content":"sys"}`
	oaiUser   = `{"role":"user","content":"go"}`
)

func oaiToolJSON(id string) string {
	return `{"role":"tool","tool_call_id":"` + id + `","content":"ok"}`
}

func mustSummarizeOpenAI(t *testing.T, body []byte) RequestSummary {
	t.Helper()
	sum, err := SummarizeOpenAIRequest(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

// The persisted shape for a message with two calls is the shape main wrote:
// one ledger message for it, one block, keyed by its first call only.
func TestOpenAIMultiCallMessageKeepsTheLedgerShape(t *testing.T) {
	sum := mustSummarizeOpenAI(t, oaiBody(oaiSystem, oaiUser,
		oaiAssistantJSON(oaiCallJSON("c1", "pwd"), oaiCallJSON("c2", "ls")), oaiToolJSON("c1"), oaiToolJSON("c2")))
	if len(sum.Prompt.Messages) != 5 {
		t.Fatalf("%d ledger messages for 5 request messages", len(sum.Prompt.Messages))
	}
	for i, m := range sum.Prompt.Messages {
		if len(m.Blocks) != 1 {
			t.Errorf("message %d has %d blocks, want 1", i, len(m.Blocks))
		}
	}
	b := sum.Prompt.Messages[2].Blocks[0]
	if want := transcript.CallKey("Bash", json.RawMessage(`"{\"cmd\":\"pwd\"}"`)); b.Kind != transcript.KindToolUse || b.CallKey != want {
		t.Errorf("assistant block %+v, want a tool_use keyed by its first call %s", b, want)
	}
}

// Session identity is the fallback hash of the first two blocks. A second call
// in the FIRST message must not move it, or a session spanning the upgrade
// splits in two.
func TestOpenAIFirstMessageCallsLeaveSessionIdentityAlone(t *testing.T) {
	one := mustSummarizeOpenAI(t, oaiBody(oaiAssistantJSON(oaiCallJSON("c1", "ls")), oaiUser))
	two := mustSummarizeOpenAI(t, oaiBody(oaiAssistantJSON(oaiCallJSON("c1", "ls"), oaiCallJSON("c2", "pwd")), oaiUser))
	if one.SessionHash != two.SessionHash || one.PrefixHash != two.PrefixHash || one.Prompt.SystemBytes != two.Prompt.SystemBytes {
		t.Errorf("a second call in the first message moved identity: session %q/%q prefix %q/%q system %d/%d",
			one.SessionHash, two.SessionHash, one.PrefixHash, two.PrefixHash, one.Prompt.SystemBytes, two.Prompt.SystemBytes)
	}
	// Alone, too: a request that is only that message.
	a := mustSummarizeOpenAI(t, oaiBody(oaiAssistantJSON(oaiCallJSON("c1", "ls"))))
	b := mustSummarizeOpenAI(t, oaiBody(oaiAssistantJSON(oaiCallJSON("c1", "ls"), oaiCallJSON("c2", "pwd"))))
	if a.SessionHash != b.SessionHash {
		t.Errorf("single-message request: session %q/%q", a.SessionHash, b.SessionHash)
	}
}

// An upgrade boundary. The first record is written by hand in the shape main
// writes for this conversation; the second is the next turn summarised by this
// build. The second request's history must carry the first's messages under
// the same identities, or the analysis sees an edited history at the upgrade.
func TestOpenAIUpgradeBoundaryKeepsMessageIdentity(t *testing.T) {
	bytesOf := func(raw string) int { return transcript.ContentBytes(json.RawMessage(raw)) }
	text := func(role, raw string) Message {
		return Message{Role: "user", Blocks: []Block{{Kind: transcript.KindText, Bytes: bytesOf(raw), Label: transcript.TextLabel(role)}}}
	}
	result := func(id string) Message {
		return Message{Role: "tool", Blocks: []Block{{Kind: transcript.KindToolResult, Bytes: bytesOf(`"ok"`), ToolUseID: id, Label: transcript.LabelToolResultPrefix + "tool"}}}
	}
	args := `"{\"cmd\":\"pwd\"}"`
	old := Prompt{
		SystemBytes: bytesOf(`"sys"`),
		Messages: []Message{
			text("system", `"sys"`),
			text("user", `"go"`),
			{Role: "assistant", Blocks: []Block{{Kind: transcript.KindToolUse, ToolName: "Bash", ToolUseID: "c1",
				Label: transcript.LabelToolCallPrefix + "Bash", CallKey: transcript.CallKey("Bash", json.RawMessage(args)),
				Bytes: bytesOf(`null`) + bytesOf(args)}}},
			result("c1"),
			result("c2"),
		},
	}
	next := mustSummarizeOpenAI(t, oaiBody(oaiSystem, oaiUser,
		oaiAssistantJSON(oaiCallJSON("c1", "pwd"), oaiCallJSON("c2", "ls")), oaiToolJSON("c1"), oaiToolJSON("c2"),
		oaiAssistantJSON(oaiCallJSON("c3", "ls")), oaiToolJSON("c3")))

	at := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	usage := &transcript.Usage{Input: 100, Output: 10}
	// Main writes the prefix hash from the first block's structure.
	sys := old.Messages[0].Blocks[0]
	oldPrefix := hashOf("prefix-", blockIdentity(transcript.Block{Kind: sys.Kind, Bytes: sys.Bytes}))
	b := NewSessionBuilder("boundary", "boundary.jsonl")
	b.Add(Record{Timestamp: at, SessionID: "s", RequestSummary: RequestSummary{Model: "gpt-x", Prompt: old, PrefixHash: oldPrefix}, Response: Response{Usage: usage}})
	b.Add(Record{Timestamp: at.Add(time.Minute), SessionID: "s", RequestSummary: next, Response: Response{Usage: usage}})
	lanes := b.Session().Lanes
	if len(lanes) != 1 || len(lanes[0].Requests) != 2 {
		t.Fatalf("lanes %d", len(lanes))
	}
	first, second := lanes[0].Requests[0].Context, lanes[0].Requests[1].Context
	for i := range first {
		if i >= len(second) || first[i].UUID != second[i].UUID {
			got := "missing"
			if i < len(second) {
				got = second[i].UUID
			}
			t.Errorf("history position %d: old record %s, next record %s", i, first[i].UUID, got)
		}
	}
}

// Whatever the loop guard is given beyond the persisted shape stays in memory:
// a later call's key never reaches the record.
func TestOpenAILaterCallKeysAreNotPersisted(t *testing.T) {
	sum := mustSummarizeOpenAI(t, oaiBody(oaiSystem, oaiUser,
		oaiAssistantJSON(oaiCallJSON("c1", "pwd"), oaiCallJSON("c2", "zz-later-only"))))
	later := transcript.CallKey("Bash", json.RawMessage(`"{\"cmd\":\"zz-later-only\"}"`))
	out, err := json.Marshal(Record{RequestSummary: sum})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), later) || strings.Contains(string(out), "zz-later-only") {
		t.Errorf("the later call reached the persisted record:\n%s", out)
	}
}
