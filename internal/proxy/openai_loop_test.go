package proxy

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The loop guard on the OpenAI chat path. A chat/completions assistant message
// can carry several tool_calls, and a repeated call is a repeat wherever it
// sits in the message. These drive the real summariser into the real
// detector, so they say what the guard concludes, not how the summary is laid
// out.

func oaiCall(id, cmd string) string {
	return `{"id":"` + id + `","type":"function","function":{"name":"Bash","arguments":"{\"cmd\":\"` + cmd + `\"}"}}`
}

func oaiAssistant(calls ...string) string {
	return `{"role":"assistant","content":null,"tool_calls":[` + strings.Join(calls, ",") + `]}`
}

func oaiTool(id string) string {
	return `{"role":"tool","tool_call_id":"` + id + `","content":"ok"}`
}

func oaiLoopVerdict(t *testing.T, limits LoopLimits, messages ...string) LoopVerdict {
	t.Helper()
	body := `{"model":"gpt-x","messages":[{"role":"system","content":"sys"},{"role":"user","content":"go"},` +
		strings.Join(messages, ",") + `]}`
	// The guard's own check (passthrough.go: guard), fed as handle feeds it.
	sum, calls, err := ledger.SummarizeOpenAIRequestWithCalls([]byte(body), nil)
	if err != nil {
		t.Fatal(err)
	}
	return loopVerdict(sum.Prompt, calls, limits)
}

func TestOpenAILoopGuardSeesEveryCallInAMessage(t *testing.T) {
	limits := LoopLimits{Warn: 2, Block: 3}
	cases := []struct {
		name     string
		messages []string
		repeats  int
		warn     bool
		block    bool
	}{
		// The defect: a repeat in second or later position was invisible.
		{"the same call twice in one message",
			[]string{oaiAssistant(oaiCall("c1", "ls"), oaiCall("c2", "ls")), oaiTool("c1"), oaiTool("c2")},
			2, true, false},
		{"a repeat of a call that was second in its message",
			[]string{oaiAssistant(oaiCall("c1", "pwd"), oaiCall("c2", "ls")), oaiTool("c1"), oaiTool("c2"),
				oaiAssistant(oaiCall("c3", "ls")), oaiTool("c3")},
			2, true, false},
		{"three identical calls in one message reach the block",
			[]string{oaiAssistant(oaiCall("c1", "ls"), oaiCall("c2", "ls"), oaiCall("c3", "ls"))},
			3, true, true},

		// Behaviour that holds before and after.
		{"single calls repeated across messages",
			[]string{oaiAssistant(oaiCall("c1", "ls")), oaiTool("c1"), oaiAssistant(oaiCall("c2", "ls")), oaiTool("c2")},
			2, true, false},
		{"distinct calls in one message are not a repeat",
			[]string{oaiAssistant(oaiCall("c1", "ls"), oaiCall("c2", "pwd")), oaiTool("c1"), oaiTool("c2")},
			1, false, false},
		{"an earlier repeat broken by a different call does not count",
			[]string{oaiAssistant(oaiCall("c1", "ls"), oaiCall("c2", "ls")), oaiTool("c1"), oaiTool("c2"),
				oaiAssistant(oaiCall("c3", "pwd")), oaiTool("c3")},
			1, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := oaiLoopVerdict(t, limits, c.messages...)
			if v.Repeats != c.repeats || v.Warn != c.warn || v.Block != c.block {
				t.Errorf("verdict %+v, want repeats %d warn %v block %v", v, c.repeats, c.warn, c.block)
			}
			if v.Label != "Bash" {
				t.Errorf("label %q, want the tool name", v.Label)
			}
		})
	}
}

// Seeing the further calls changes detection only. A second call in a later
// message leaves the system size, both hashes and the summed block bytes where
// they were: those calls' arguments were never sized, and are not now.
func TestOpenAIFurtherCallsLeaveSizesAndIdentityAlone(t *testing.T) {
	summary := func(calls ...string) ledger.RequestSummary {
		t.Helper()
		body := `{"model":"gpt-x","messages":[{"role":"system","content":"sys"},{"role":"user","content":"go"},` +
			oaiAssistant(calls...) + `]}`
		sum, err := ledger.SummarizeOpenAIRequest([]byte(body), nil)
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}
	bytes := func(s ledger.RequestSummary) int {
		n := 0
		for _, m := range s.Prompt.Messages {
			for _, b := range m.Blocks {
				n += b.Bytes
			}
		}
		return n
	}
	one, two := summary(oaiCall("c1", "ls")), summary(oaiCall("c1", "ls"), oaiCall("c2", "pwd"))
	if one.Prompt.SystemBytes != two.Prompt.SystemBytes || one.PrefixHash != two.PrefixHash ||
		one.SessionHash != two.SessionHash || bytes(one) != bytes(two) {
		t.Errorf("a further call moved sizing or identity: system %d/%d prefix %q/%q session %q/%q bytes %d/%d",
			one.Prompt.SystemBytes, two.Prompt.SystemBytes, one.PrefixHash, two.PrefixHash,
			one.SessionHash, two.SessionHash, bytes(one), bytes(two))
	}
}

// Through the server: the guard in passthrough.go is what has to see the
// calls, not only the detector in isolation. Three identical calls in one
// message at block=3 are refused before the provider is asked.
func TestOpenAILoopGuardBlocksRepeatsInOneMessageThroughTheProxy(t *testing.T) {
	base, _ := startOpenAIProxy(t, Config{Loops: LoopLimits{Block: 3}})
	body := `{"model":"gpt-x","messages":[{"role":"system","content":"sys"},{"role":"user","content":"go"},` +
		oaiAssistant(oaiCall("c1", "ls"), oaiCall("c2", "ls"), oaiCall("c3", "ls")) + `]}`
	req, err := http.NewRequest(http.MethodPost, base+chatCompletionsPath, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(got), "replay_loop") {
		t.Fatalf("three identical calls in one message were not stopped: %d %s", resp.StatusCode, got)
	}
}

// A chat/completions body that is not JSON does not summarise, so the guard is
// handed no calls and reads an empty prompt, as it did before calls existed.
// Even at the strictest thresholds it is not taken for a loop: no refusal, no
// loop warning, and the request reaches the provider.
func TestOpenAIMalformedBodyIsNotTakenForALoop(t *testing.T) {
	var hits atomic.Int32
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":1}}`))
	})
	base, _, _ := startProxyWith(t, up, Config{Loops: LoopLimits{Warn: 1, Block: 1}})
	req, err := http.NewRequest(http.MethodPost, base+chatCompletionsPath, strings.NewReader(`{not json`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if strings.Contains(string(got), "replay_loop") {
		t.Errorf("a malformed body was refused as a loop: %d %s", resp.StatusCode, got)
	}
	if w := resp.Header.Get(HeaderWarning); strings.Contains(w, "loop") {
		t.Errorf("a malformed body carried a loop warning: %q", w)
	}
	if hits.Load() != 1 {
		t.Errorf("upstream hits = %d, want 1: the malformed body did not reach the provider (status %d, %s)",
			hits.Load(), resp.StatusCode, got)
	}
}

// A valid body with no tool calls also hands the guard nil, and the verdict is
// the one DetectLoop gives on its prompt: nothing to count.
func TestOpenAINoToolCallsGivesThePromptVerdict(t *testing.T) {
	limits := LoopLimits{Warn: 1, Block: 1}
	sum, calls, err := ledger.SummarizeOpenAIRequestWithCalls(
		[]byte(`{"model":"gpt-x","messages":[{"role":"system","content":"sys"},{"role":"user","content":"go"}]}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != nil {
		t.Errorf("calls %v, want nil for a body with no tool calls", calls)
	}
	if got, want := loopVerdict(sum.Prompt, calls, limits), DetectLoop(sum.Prompt, limits); got != want || got.Repeats != 0 {
		t.Errorf("verdict %+v, prompt verdict %+v: want both empty", got, want)
	}
}
