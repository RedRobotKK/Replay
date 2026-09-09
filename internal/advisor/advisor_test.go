package advisor

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
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
	if b := by[KindCacheBreaks]; b.Status != AdviceOnly || b.PromptTokens <= 0 || b.PredictedTokens != b.PromptTokens {
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
	// "built-in" entered this string when unused tools gained per-server
	// attribution on 2026-09-09. Idle1..Idle5 carry no mcp__ prefix, so they
	// bucket as built-ins, which is the behaviour under test — the count, the
	// share, the session total and the status are all unchanged.
	if !ok || unused.Sessions != 3 || !strings.Contains(unused.Title, "5 built-in tool definitions never called") || !strings.Contains(unused.Title, "Idle1") || unused.Status != Pending {
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

// Named-server attribution: the last step between a measurement and an action.
//
// docs/WHAT-YOU-GET.md calls this "the single highest-leverage thing left to
// build" and MONEY-PATH section 5 makes it step 3, ahead of anything paid,
// because the gate in step 5 consumes it.
//
// The gap it closes is small and total. Today advise says:
//
//	12 tool definitions never called are 8% of prompt tokens (a, b, c, ...)
//
// which is true, and stops one step short: the reader still has to work out
// WHICH twelve and WHERE they came from, and a comma-joined list of twelve
// mcp__ names is not a thing anybody reads. It needs no config to close, because
// MCP tools are named mcp__<server>__<tool> — the server is already in the name,
// and the ledger already stores each name with its byte size.
//
// So the unit of advice becomes the server, which is also the unit of action: a
// reader cannot disable one tool, but they can disable a server.
func TestUnusedToolsAreAttributedToTheirServer(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	defined := []string{
		"mcp__playwright__click", "mcp__playwright__screenshot", "mcp__playwright__navigate",
		"mcp__jira__search", "mcp__jira__comment",
		"Bash", "Read",
	}
	// Only Bash is ever called. Two servers and one built-in go unused.
	var obs []Observation
	for i := 0; i < 3; i++ {
		s := synthetic(base.AddDate(0, 0, i), defined, []string{"Bash", "Bash"}, map[string]int{"main.go": 2}, 200)
		ob, ok := Observe(s)
		if !ok {
			t.Fatalf("session %d must calibrate", i)
		}
		obs = append(obs, ob)
	}

	var servers []Suggestion
	for _, s := range Suggest(obs) {
		if s.Kind == KindUnusedTools {
			servers = append(servers, s)
		}
	}
	if len(servers) < 2 {
		t.Fatalf("expected one suggestion per unused server, got %d: %+v",
			len(servers), titlesOf(servers))
	}

	byTarget := map[string]Suggestion{}
	for _, s := range servers {
		byTarget[s.Target] = s
	}
	for _, want := range []string{"playwright", "jira"} {
		s, ok := byTarget[want]
		if !ok {
			t.Errorf("no suggestion targets the %q server; targets were %v",
				want, keysOf(byTarget))
			continue
		}
		// The count must be that server's tools, not the corpus-wide total.
		// Reporting 5 against playwright would be the old lumped number wearing
		// a server's name, which is worse than not attributing at all.
		wantN := map[string]string{"playwright": "3", "jira": "2"}[want]
		if !strings.Contains(s.Title, wantN) {
			t.Errorf("%s: title %q does not carry its own tool count %s",
				want, s.Title, wantN)
		}
		if !strings.Contains(s.Title, want) {
			t.Errorf("%s: title %q does not name the server", want, s.Title)
		}
		// The whole point: an action the reader can take.
		if !strings.Contains(strings.ToLower(s.Action), "disable") {
			t.Errorf("%s: action %q does not tell the reader what to do", want, s.Action)
		}
	}

	// Playwright carries three definitions to jira's two, so it must cost more.
	// If the tokens were split evenly the attribution would be decorative.
	if p, j := byTarget["playwright"], byTarget["jira"]; p.PromptTokens <= j.PromptTokens {
		t.Errorf("playwright (3 tools) costs %d prompt tokens against jira's (2 tools) %d; "+
			"the per-server figures are not derived from the per-server bytes",
			p.PromptTokens, j.PromptTokens)
	}
}

// A built-in tool has no server, and must not be invented one.
//
// "Read" does not match mcp__<server>__<tool>. Bucketing it under a fabricated
// server name would be the tool telling the reader to disable something that
// does not exist.
func TestUnusedBuiltinsAreNotGivenAServer(t *testing.T) {
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var obs []Observation
	for i := 0; i < 3; i++ {
		s := synthetic(base.AddDate(0, 0, i),
			[]string{"Bash", "Read", "Glob"}, []string{"Bash", "Bash"}, map[string]int{"main.go": 2}, 200)
		ob, ok := Observe(s)
		if !ok {
			t.Fatalf("session %d must calibrate", i)
		}
		obs = append(obs, ob)
	}
	for _, s := range Suggest(obs) {
		if s.Kind != KindUnusedTools {
			continue
		}
		if strings.Contains(s.Target, "mcp__") || strings.Contains(s.Action, "disable this server") {
			t.Errorf("built-in tools were attributed to a server: target=%q title=%q action=%q",
				s.Target, s.Title, s.Action)
		}
		if !strings.Contains(s.Title, "Glob") {
			t.Errorf("the built-in suggestion names neither the tools nor the count: %q", s.Title)
		}
	}
}

func titlesOf(ss []Suggestion) []string {
	out := []string{}
	for _, s := range ss {
		out = append(out, s.Title)
	}
	return out
}

func keysOf(m map[string]Suggestion) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
