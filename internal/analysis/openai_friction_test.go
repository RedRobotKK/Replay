package analysis

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The OpenAI loop guard sees every tool call in a message. Analysis does not,
// and must not start to by accident: ErrorCosts feeds the friction report
// (report.go) and, through learn.Score, the ErrorShare that learn's graduation
// guard compares against control. On the OpenAI ledger path a call after the
// first in its message was never part of the ledger's blocks, so a session
// that differs only by such a call has the same error accounting.
//
// Synthetic: two proxy-shaped sessions built from chat/completions bodies
// through the real summariser and session builder.

func openAIFrictionSession(t *testing.T, firstMessageCalls string) *transcript.Session {
	t.Helper()
	call := func(id, cmd string) string {
		return `{"id":"` + id + `","type":"function","function":{"name":"Bash","arguments":"{\"cmd\":\"` + cmd + `\"}"}}`
	}
	calls := call("c1", "pwd")
	if firstMessageCalls == "pwd+ls" {
		calls += "," + call("c2", "ls")
	}
	turn1 := []string{
		`{"role":"system","content":"sys"}`, `{"role":"user","content":"go"}`,
		`{"role":"assistant","content":null,"tool_calls":[` + calls + `]}`,
		`{"role":"tool","tool_call_id":"c1","content":"ok"}`, `{"role":"tool","tool_call_id":"c2","content":"ok"}`,
	}
	turn2 := append(append([]string{}, turn1...),
		`{"role":"assistant","content":null,"tool_calls":[`+call("c3", "ls")+`]}`,
		`{"role":"tool","tool_call_id":"c3","content":"ok"}`)
	b := ledger.NewSessionBuilder("friction", "friction.jsonl")
	at := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	for i, msgs := range [][]string{turn1, turn2} {
		sum, err := ledger.SummarizeOpenAIRequest([]byte(`{"model":"gpt-x","messages":[`+strings.Join(msgs, ",")+`]}`), nil)
		if err != nil {
			t.Fatal(err)
		}
		b.Add(ledger.Record{
			Timestamp: at.Add(time.Duration(i) * time.Minute), SessionID: "s", RequestSummary: sum,
			Response: ledger.Response{Usage: &transcript.Usage{Input: 200 + 50*i, CacheRead: 1000 * i, Output: 20}},
		})
	}
	return b.Session()
}

func TestOpenAILaterCallsLeaveErrorAccountingAlone(t *testing.T) {
	report := func(s *transcript.Session) (*LaneReport, []ErrorCost) {
		lane := MainLane(s)
		if lane == nil || len(lane.Requests) != 2 {
			t.Fatalf("lane %+v", lane)
		}
		rep := AnalyzeLane(s, lane)
		return rep, ErrorCosts(rep.Calibration, rep.Fit)
	}
	withLater, learnInput := report(openAIFrictionSession(t, "pwd+ls"))
	without, learnInputWithout := report(openAIFrictionSession(t, "pwd"))
	if !reflect.DeepEqual(withLater.Errors, without.Errors) {
		t.Errorf("friction report moved by a later call in a message:\n with    %+v\n without %+v", withLater.Errors, without.Errors)
	}
	// The exact figures learn.Score sums into ErrorShare.
	if !reflect.DeepEqual(learnInput, learnInputWithout) {
		t.Errorf("learn's error input moved:\n with    %+v\n without %+v", learnInput, learnInputWithout)
	}
	for _, e := range learnInput {
		if e.Class == ErrorRepeatedCommand {
			t.Errorf("a repeat was counted from a call the ledger never keyed: %+v", e)
		}
	}
}
