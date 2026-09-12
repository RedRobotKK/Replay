package advisor

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
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
	got := Suggest([]Observation{ob}, nil)
	if len(got) == 0 {
		t.Fatal("no suggestions")
	}
	// Cache-breaks re-bill at input price; Bash inputs sit in the prefix and
	// bill at the read multiple. Token ranking put Bash first; dollar ranking
	// puts the re-bills first. That is the change.
	if got[0].Kind != KindCacheBreaks {
		t.Fatalf("largest by cache-traffic dollars must be cache-breaks, got %s: %+v", got[0].Kind, got)
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
	bash := by[KindToolInputs]
	if bash.Sessions != 1 || bash.Share < MinShare || bash.PredictedTokens != bash.PromptTokens/2 || bash.Status != Pending || !strings.Contains(bash.Action, "heredoc") {
		t.Fatalf("Bash suggestion wrong: %+v", bash)
	}
	if b := by[KindCacheBreaks]; b.Status != AdviceOnly || b.PromptTokens <= 0 || b.PredictedTokens != b.PromptTokens {
		t.Fatalf("cache breaks are advice only: %+v", by[KindCacheBreaks])
	}
}

func TestCacheTrafficUSD_ZeroTokensAndUnknownModelAreZero(t *testing.T) {
	now := time.Now()
	if cacheTrafficUSD(KindLargeResults, 0, "claude-opus-5", now) != 0 {
		t.Fatal("zero tokens must price as 0, not as a free-looking miss")
	}
	if cacheTrafficUSD(KindLargeResults, 1_000_000, "not-a-priced-model", now) != 0 {
		t.Fatal("an unpriced model must price as 0, excluded not free")
	}
	if cacheTrafficUSD(KindLargeResults, 1_000_000, "claude-opus-5", now) <= 0 {
		t.Fatal("opus-5 cache-read of 1M tokens must be positive")
	}
}

func TestSuggest_RanksByWriteReadDollarsNotTokenShare(t *testing.T) {
	now := time.Now()
	// More tokens of Bash results than of cache-breaks, but the breaks
	// are priced as re-bills (input) and the results as cache reads.
	obs := []Observation{
		{
			at: now, prompt: 2_000_000,
			targets: map[string]evidence{
				key(KindLargeResults, "Bash"): {at: now, share: 0.5, tokens: 1_000_000, usd: 1.00},
			},
		},
		{
			at: now.Add(time.Second), prompt: 200_000,
			targets: map[string]evidence{
				key(KindCacheBreaks, "cache breaks"): {at: now, share: 0.4, tokens: 80_000, usd: 10.00},
			},
		},
	}
	got := Suggest(obs, nil)
	if len(got) < 2 {
		t.Fatalf("want two suggestions, got %d", len(got))
	}
	if got[0].Kind != KindCacheBreaks {
		t.Fatalf("dollar ranking put %s first (tokens %d $%.2f); cache-breaks should lead",
			got[0].Kind, got[0].PredictedTokens, got[0].PredictedUSD)
	}
	if got[1].Kind != KindLargeResults {
		t.Fatalf("second = %s, want Bash results", got[1].Kind)
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
	got := kinds(Suggest(obs, nil))
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
	if s := kinds(Suggest(obs, nil))[KindUnusedTools]; s.Status != Pending {
		t.Fatalf("unchanged sessions stay pending: %+v", s)
	}
	// Two newest sessions with most idle tools removed. Verified now requires
	// the reader to have SAID they applied it: the drop alone used to be taken
	// as the application, and track() then measured that same drop to decide
	// whether the prediction held. See TK1 — that circularity reported pure
	// corpus drift as a confirmed saving.
	add(4, []string{"Bash", "Idle1"})
	add(5, []string{"Bash", "Idle1"})
	appliedHere := map[string]bool{id(KindUnusedTools, "built-in"): true}
	if s := kinds(Suggest(obs, nil))[KindUnusedTools]; s.Status != Pending {
		t.Fatalf("a drop nobody claimed must stay pending, not verify itself: %+v", s)
	}
	s := kinds(Suggest(obs, appliedHere))[KindUnusedTools]
	if s.Status != Verified || s.RealizedShare <= 0 {
		t.Fatalf("a large drop must verify: %+v", s)
	}
	// A drop past the applied bar but short of half the prediction counts
	// as applied but not verified: three of six idle tools removed.
	obs = obs[:4]
	add(4, tools[:4])
	add(5, tools[:4])
	s = kinds(Suggest(obs, appliedHere))[KindUnusedTools]
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
	for _, s := range Suggest(obs, nil) {
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
	for _, s := range Suggest(obs, nil) {
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

// The two guards in cacheTrafficUSD, made load-bearing.
//
// guard-reachability reported both INERT: the branches run, and deleting
// either one changed no test's result. That verdict was right, and the reason
// is the shape of the test above this one. It asserts zero tokens price as 0
// and an unknown model prices as 0 — both true whether or not the guard is
// there, because `0 * rate` is already 0, and an unknown model's Price is the
// zero value, so `mtok * 0 * 0` is 0 too. Two correct assertions that between
// them certify nothing about the code they name.
//
// An INERT verdict is not a request to delete the branch. Read it first: here
// each guard is load-bearing on an input the old tests did not reach, and the
// fix is a test, not a deletion.

// A negative token count is the input that separates the guard from the
// arithmetic.
//
// Nothing in this package should produce one, which is the point: the guard is
// there for the case nobody planned. Without it a negative count does not
// return zero, it returns NEGATIVE DOLLARS — and this figure is summed into a
// ranking, so one negative row does not look like an error, it looks like a
// target worth less than nothing and sorts to the bottom where nobody reads it.
func TestCacheTrafficUSD_ANegativeTokenCountIsZeroNotNegativeDollars(t *testing.T) {
	got := cacheTrafficUSD(KindLargeResults, -1, "claude-opus-5", time.Time{})
	if got != 0 {
		t.Fatalf("a negative token count priced at %v; it must be 0, because a negative "+
			"dollar figure summed into a ranking reads as a cheap target rather than as "+
			"the impossible input it is", got)
	}
}

// A row that carries numbers and is not priced.
//
// `PriceForAt` returns (Price, bool) and the bool is not "the Price is zero".
// activeRow's ok means A ROW MATCHED; the row's own Priced field is what comes
// back as the second value. So an installed rules document can return a fully
// populated Price alongside false — real InputPerMTok, real ReadMult, and a
// flag saying do not bill from this.
//
// That is the exact shape of #212, where unknown models were priced as known.
// Here, deleting `if !ok` bills 1,000,000 tokens at 10 * 0.1 = $1.00 from a row
// whose entire purpose is to say it is not for billing. Zero-value prices hide
// this; a populated unpriced row is the only input that shows it.
func TestCacheTrafficUSD_AnUnpricedRowWithRealNumbersDoesNotBill(t *testing.T) {
	restore := cachemodel.Override(&cachemodel.Rules{
		Schema:   "replay.rules/v1",
		Version:  "test-unpriced-populated",
		Provider: "test",
		Models: []cachemodel.ModelRule{{
			Match:        "unpriced-but-populated",
			InputPerMTok: 10,
			ReadMult:     0.1,
			Priced:       false,
		}},
	})
	defer restore()

	// The premise, asserted rather than assumed: this really is the populated
	// -but-false pair. If a later change makes an unpriced row return a zero
	// Price, the test below stops being able to fail and this line says so.
	p, ok := cachemodel.PriceForAt("unpriced-but-populated", time.Time{})
	if ok {
		t.Fatal("the fixture row reports priced; it cannot exercise the !ok guard")
	}
	if p.InputPerMTok == 0 || p.ReadMult == 0 {
		t.Fatalf("the fixture row came back with a zero Price (%+v), so deleting the "+
			"guard would price it at 0 anyway and this test could not fail", p)
	}

	if got := cacheTrafficUSD(KindLargeResults, 1_000_000, "unpriced-but-populated", time.Time{}); got != 0 {
		t.Fatalf("an unpriced row billed %v for 1M tokens; a row flagged not-for-billing "+
			"must price as excluded, not as %v of real money", got, got)
	}
	// The break arm prices differently and must also refuse.
	if got := cacheTrafficUSD(KindCacheBreaks, 1_000_000, "unpriced-but-populated", time.Time{}); got != 0 {
		t.Fatalf("an unpriced row billed %v on the cache-break arm", got)
	}
}
