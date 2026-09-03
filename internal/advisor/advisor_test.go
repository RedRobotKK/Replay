package advisor

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

func kinds(ss []Suggestion) map[Kind]Suggestion {
	out := map[Kind]Suggestion{}
	for _, s := range ss {
		if _, ok := out[s.Kind]; !ok {
			out[s.Kind] = s
		}
	}
	return out
}

// The fixture session worked through long Bash inputs with a large first
// turn and one cache break; the advisor must say so, in that order of
// predicted saving, and never invent a hot file or an unused tool.
func TestFixtureProducesTheExpectedSuggestions(t *testing.T) {
	s, err := transcript.ParseClaudeCodeFile("../transcript/testdata/session-redacted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	ob, ok := Observe(s)
	if !ok {
		t.Fatal("fixture must calibrate")
	}
	got := Suggest([]Observation{ob})
	if len(got) == 0 || got[0].Kind != KindToolInputs || got[0].Target != "Bash" {
		t.Fatalf("largest suggestion must be the Bash inputs: %+v", got)
	}
	by := kinds(got)
	for _, k := range []Kind{KindToolInputs, KindFirstTurn, KindCacheBreaks} {
		if _, ok := by[k]; !ok {
			t.Errorf("missing %s suggestion", k)
		}
	}
	for _, k := range []Kind{KindHotFile, KindUnusedTools} {
		if _, ok := by[k]; ok {
			t.Errorf("fixture has no evidence for %s", k)
		}
	}
	first := got[0]
	if first.Sessions != 1 || first.Share < MinShare || first.PredictedTokens != first.PromptTokens/2 || first.Status != Pending || !strings.Contains(first.Action, "heredoc") {
		t.Fatalf("Bash suggestion wrong: %+v", first)
	}
	if by[KindCacheBreaks].Status != AdviceOnly || by[KindCacheBreaks].PromptTokens <= 0 {
		t.Fatalf("cache breaks are advice only: %+v", by[KindCacheBreaks])
	}
}

// synthetic builds a ledger-tier session at a given time whose one lane
// carries tool definitions, calls some of them, and reads files.
func synthetic(at time.Time, defined []string, called []string, reads map[string]int, inputBytes int) *transcript.Session {
	s := &transcript.Session{ID: at.Format(time.RFC3339), Source: transcript.SourceLedger}
	lane := s.Lane("", false)
	var tools []transcript.ToolDef
	for _, name := range defined {
		tools = append(tools, transcript.ToolDef{Name: name, Bytes: 4000})
	}
	prefix := &transcript.Message{UUID: "prefix", Role: transcript.RoleSystem, Blocks: []transcript.Block{{Kind: transcript.KindText, Label: "system prompt", Bytes: 2000}, {Kind: transcript.KindOther, Label: "tool definitions", Bytes: 4000 * len(defined)}}}
	msgs := []*transcript.Message{prefix, {UUID: "u0", Role: transcript.RoleUser, Blocks: []transcript.Block{{Kind: transcript.KindText, Label: transcript.LabelUserText, Bytes: 3000}}}}
	i := 0
	for _, name := range called {
		i++
		msgs = append(msgs,
			&transcript.Message{UUID: fmt.Sprintf("a%d", i), Role: transcript.RoleAssistant, Blocks: []transcript.Block{{Kind: transcript.KindToolUse, Label: transcript.LabelToolCallPrefix + name, ToolName: name, Bytes: inputBytes, ToolUseID: fmt.Sprint(i)}}},
			&transcript.Message{UUID: fmt.Sprintf("r%d", i), Role: transcript.RoleUser, Blocks: []transcript.Block{{Kind: transcript.KindToolResult, Label: transcript.LabelToolResultPrefix + name, ToolName: name, Bytes: 500, ToolUseID: fmt.Sprint(i)}}})
	}
	for path, n := range reads {
		for j := 0; j < n; j++ {
			i++
			label := "Read " + path
			msgs = append(msgs,
				&transcript.Message{UUID: fmt.Sprintf("a%d", i), Role: transcript.RoleAssistant, Blocks: []transcript.Block{{Kind: transcript.KindToolUse, Label: transcript.LabelToolCallPrefix + "Read", ToolName: "Read", Bytes: 40, ToolUseID: fmt.Sprint(i)}}},
				&transcript.Message{UUID: fmt.Sprintf("r%d", i), Role: transcript.RoleUser, Blocks: []transcript.Block{{Kind: transcript.KindToolResult, Label: transcript.LabelToolResultPrefix + label, ToolName: label, Bytes: 6000, ToolUseID: fmt.Sprint(i)}}})
		}
	}
	// One request per assistant turn, each carrying the context so far,
	// with usage that follows the invariant at a fixed 0.25 tokens/byte.
	prev := 0
	for n := 2; n <= len(msgs); n += 2 {
		ctx := msgs[:n]
		bytes := 0
		for _, m := range ctx {
			bytes += m.Bytes()
		}
		total := bytes / 4
		tail := 20
		read := 0
		if prev > 0 {
			read = prev - tail
		}
		lane.Requests = append(lane.Requests, &transcript.Request{ID: fmt.Sprint(n), Model: "claude-opus-5", Timestamp: at.Add(time.Duration(n) * time.Second), Context: ctx, Tools: tools, Usage: transcript.Usage{Input: tail, CacheRead: read, CacheCreation: total - read - tail, Output: 10}})
		prev = total
	}
	return s
}

func TestUnusedToolsAndHotFilesAcrossSessions(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var obs []Observation
	for i := 0; i < 3; i++ {
		s := synthetic(base.AddDate(0, 0, i), []string{"Bash", "Read", "Idle1", "Idle2", "Idle3", "Idle4", "Idle5"}, []string{"Bash", "Bash"}, map[string]int{"main.go": 2}, 200)
		ob, ok := Observe(s)
		if !ok {
			t.Fatalf("session %d must calibrate", i)
		}
		obs = append(obs, ob)
	}
	got := kinds(Suggest(obs))
	unused, ok := got[KindUnusedTools]
	if !ok || unused.Sessions != 3 || !strings.Contains(unused.Title, "5 tool definitions never called") || !strings.Contains(unused.Title, "Idle1") || unused.Status != Pending {
		t.Fatalf("unused tools: %+v", unused)
	}
	hot, ok := got[KindHotFile]
	if !ok || hot.Target != "Read main.go" || !strings.Contains(hot.Title, "read 6 times across 3 sessions") || hot.Status != AdviceOnly || hot.PredictedTokens <= 0 {
		t.Fatalf("hot file: %+v", hot)
	}
}

// AD-2: a suggestion moves from pending to applied when the newest
// sessions show the target shrinking, and to verified when the drop
// reaches half of the prediction.
func TestSuggestionsAreTrackedToClosure(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	tools := []string{"Bash", "Idle1", "Idle2", "Idle3", "Idle4", "Idle5", "Idle6"}
	var obs []Observation
	add := func(i int, defined []string) {
		ob, ok := Observe(synthetic(base.AddDate(0, 0, i), defined, []string{"Bash", "Bash", "Bash"}, nil, 200))
		if !ok {
			t.Fatal("must calibrate")
		}
		obs = append(obs, ob)
	}
	for i := 0; i < 4; i++ {
		add(i, tools)
	}
	if s := kinds(Suggest(obs))[KindUnusedTools]; s.Status != Pending {
		t.Fatalf("unchanged sessions stay pending: %+v", s)
	}
	// Two newest sessions with most idle tools removed: applied and, since
	// the drop exceeds half the predicted halving, verified.
	add(4, []string{"Bash", "Idle1"})
	add(5, []string{"Bash", "Idle1"})
	s := kinds(Suggest(obs))[KindUnusedTools]
	if s.Status != Verified || s.RealizedShare <= 0 {
		t.Fatalf("a large drop must verify: %+v", s)
	}
	// A drop past the applied bar but short of half the prediction counts
	// as applied but not verified: three of six idle tools removed.
	obs = obs[:4]
	add(4, tools[:4])
	add(5, tools[:4])
	s = kinds(Suggest(obs))[KindUnusedTools]
	if s.Status != NotVerified {
		t.Fatalf("a small drop is applied but not verified: %+v", s)
	}
	if s.RealizedShare >= s.PredictedShare*verifyShare {
		t.Fatalf("realized %.3f should be under the verification bar %.3f", s.RealizedShare, s.PredictedShare*verifyShare)
	}
}
