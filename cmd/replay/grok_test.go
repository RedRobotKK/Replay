package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Nothing here reads ~/.grok. The suite's TestMain points HOME at an empty
// directory on purpose, and a test whose result depends on what the developer
// ran yesterday measures the developer.
//
// The fixtures are the real shape, measured 2026-09-17: one directory per
// session under ~/.grok/sessions/<urlencoded-cwd>/<session-id>/, with the
// per-turn usage on a turn_completed update in updates.jsonl.
func grokTurn(input, output, cached, created, reasoning int) string {
	return fmt.Sprintf(`{"timestamp":1789191644,"method":"_x.ai/session/update","params":`+
		`{"sessionId":"s","update":{"sessionUpdate":"turn_completed","usage":`+
		`{"inputTokens":%d,"outputTokens":%d,"totalTokens":%d,"cachedReadTokens":%d,`+
		`"cacheCreationTokens":%d,"reasoningTokens":%d,"modelCalls":3,"apiDurationMs":4927,`+
		`"costUsdTicks":43574400,"modelUsage":{"grok-4.6-build":{"inputTokens":%d}}}}}}`,
		input, output, input+output, cached, created, reasoning, input) + "\n"
}

// writeGrokHome lays out session directories under a fake HOME and returns it.
func writeGrokHome(t *testing.T, sessions map[string]map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for id, files := range sessions {
		dir := filepath.Join(home, ".grok", "sessions", "%2FUsers%2Fx", id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return home
}

// GK1: the roots are the layout that is on disk, not the one that sounds
// right.
//
// Grok keeps sessions under a directory named for the URL-encoded working
// directory they ran in, which is two levels below ~/.grok/sessions and not
// one. A reader that globbed one level down would find nothing and report a
// machine with no Grok usage, which is the failure mode this project has
// already shipped once in its own discovery.
func TestGK1_GrokRootsAreTheLayoutOnDisk(t *testing.T) {
	got := grokRoots("/home/x")
	want := filepath.Join("/home/x", ".grok", "sessions")
	if len(got) != 1 || got[0] != want {
		t.Errorf("grokRoots = %v, want [%s]", got, want)
	}
	if grokRoots("") != nil {
		t.Errorf("an unknown HOME produced roots: %v", grokRoots(""))
	}
}

// GK2: session directories are found through the URL-encoded layer, and a
// directory with nothing to read is not one.
func TestGK2_FindGrokSessionsWalksTheEncodedLayer(t *testing.T) {
	home := writeGrokHome(t, map[string]map[string]string{
		"01a09420-aaa": {"updates.jsonl": grokTurn(1000, 20, 800, 0, 5)},
		"01a09420-bbb": {"updates.jsonl": grokTurn(2000, 40, 0, 0, 0)},
		"01a09420-ccc": {"summary.json": "{}"},
	})
	got := findGrokSessions(grokRoots(home))
	if len(got) != 2 {
		t.Fatalf("found %d session(s), want 2: %v", len(got), got)
	}
	for _, g := range got {
		if strings.HasSuffix(g, "ccc") {
			t.Errorf("a directory with no updates.jsonl was taken for a session: %s", g)
		}
	}
}

// GK3: the cache sits INSIDE the input, so fresh is a subtraction.
//
// This is the whole reason the reader goes through usage.FromInclusive rather
// than copying the provider's number across. Measured on this machine
// 2026-09-17: inputTokens + outputTokens == totalTokens on 2,683 of 2,683
// usage objects, and inputTokens + outputTokens + cachedReadTokens == totalTokens
// on none of them except where the cache was zero. cachedReadTokens is
// therefore contained within inputTokens.
//
// Anthropic counts the other way — its input_tokens is the uncached remainder
// with the cache reported beside it — so an adapter that copies inputTokens
// into Fresh is right for one surface and overstates the other by the whole
// cache. On the real record below that is 6,272 of 17,415 input tokens, or 36%
// of the fresh figure invented out of tokens that were served from cache.
//
// FAIL looks like: Validate returning "usage does not add up: fresh 17415 +
// read 6272 + write 0 = 23687, but prompt is 17415", which is the error
// FromInclusive's own documentation calls "a provider counting inclusively
// must be converted, not copied". Reintroduce it by building the Record by
// hand with Fresh: u.InputTokens.
func TestGK3_TheCacheSitsInsideTheInput(t *testing.T) {
	cases := []struct {
		name                                       string
		input, output, cached, created, reasoning  int
		wantPrompt, wantFresh, wantRead, wantWrite int
	}{
		// The real record, from ~/.grok, retyped here as a fixture.
		{"the measured record", 17415, 35, 6272, 0, 30, 17415, 11143, 6272, 0},
		{"nothing cached", 1000, 20, 0, 0, 0, 1000, 1000, 0, 0},
		{"entirely cached", 2000, 10, 2000, 0, 4, 2000, 0, 2000, 0},
		// cacheCreationTokens is zero on every record in the corpus. If a
		// write ever arrives it is a share of the same input, so it comes out
		// of fresh too.
		{"a cache write inside the input", 1000, 10, 200, 300, 0, 1000, 500, 200, 300},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			turn := transcript.GrokTurn{Usage: transcript.GrokUsage{
				InputTokens:         c.input,
				OutputTokens:        c.output,
				TotalTokens:         c.input + c.output,
				CachedReadTokens:    c.cached,
				CacheCreationTokens: c.created,
				ReasoningTokens:     c.reasoning,
			}}
			r, err := grokRecord(turn)
			if err != nil {
				t.Fatalf("grokRecord refused a record that adds up: %v", err)
			}
			if r.Prompt != c.wantPrompt {
				t.Errorf("prompt %d, want %d: inputTokens IS the whole prompt on this surface",
					r.Prompt, c.wantPrompt)
			}
			if r.Fresh != c.wantFresh {
				t.Errorf("fresh %d, want %d: fresh is inputTokens minus the cached read and "+
					"the cache write, not a copy of inputTokens", r.Fresh, c.wantFresh)
			}
			if r.CachedRead != c.wantRead {
				t.Errorf("cached read %d, want %d", r.CachedRead, c.wantRead)
			}
			if r.CachedWrite != c.wantWrite {
				t.Errorf("cached write %d, want %d", r.CachedWrite, c.wantWrite)
			}
			if r.Output != c.output || r.Reasoning != c.reasoning {
				t.Errorf("output %d/reasoning %d, want %d/%d", r.Output, r.Reasoning, c.output, c.reasoning)
			}
			// The guard the whole conversion exists to satisfy.
			if err := r.Validate(); err != nil {
				t.Errorf("the converted record does not add up: %v", err)
			}
			// And the copy, named directly: a fresh count equal to the prompt
			// while any of the prompt was cached is the defect itself.
			if c.cached > 0 && r.Fresh == r.Prompt {
				t.Errorf("fresh equals the whole prompt (%d) while %d of it was served from "+
					"cache: the inclusive count was copied instead of converted", r.Prompt, c.cached)
			}
			if r.Raw == nil && turn.Raw != nil {
				t.Errorf("the provider's own payload was dropped")
			}
		})
	}
}

// GK4: a record the reader will not believe is counted and explained, not
// zeroed.
//
// `replay codex` reports its 15 refused records for the same reason: a total
// that silently skipped something is a total that looks complete.
func TestGK4_RefusedRecordsAreCountedAndExplained(t *testing.T) {
	home := writeGrokHome(t, map[string]map[string]string{
		"01a09420-aaa": {"updates.jsonl": grokTurn(1000, 20, 800, 0, 5) +
			// cachedReadTokens larger than the input it is part of.
			grokTurn(100, 10, 500, 0, 0)},
	})
	g := readGrok(findGrokSessions(grokRoots(home)))
	if g.Turns != 1 {
		t.Errorf("counted %d turn(s), want 1: the impossible record must not be in the total", g.Turns)
	}
	if g.Refused != 1 {
		t.Errorf("refused %d, want 1", g.Refused)
	}
	if g.Prompt != 1000 || g.CachedRead != 800 || g.Fresh != 200 {
		t.Errorf("totals prompt %d fresh %d read %d, want 1000/200/800", g.Prompt, g.Fresh, g.CachedRead)
	}
	if g.Output != 20 || g.Reasoning != 5 {
		t.Errorf("output %d reasoning %d, want 20/5", g.Output, g.Reasoning)
	}
	if g.Sessions != 1 {
		t.Errorf("sessions %d, want 1", g.Sessions)
	}
	// modelCalls, not turns: a turn is not a request.
	if g.Requests != 3 {
		t.Errorf("requests %d, want 3: the record says the turn made three model calls", g.Requests)
	}
	if len(g.Reasons) == 0 {
		t.Fatalf("a refusal was counted and no reason was kept")
	}
	joined := strings.Join(g.Reasons, " ")
	if !strings.Contains(joined, "cachedReadTokens") {
		t.Errorf("the refusal reasons do not name the field that failed: %q", joined)
	}

	// And the reader is told. A refusal counted into a struct nobody prints is
	// the same silence as no refusal at all.
	note := strings.Join(burnGrok(home, "").problems, " ")
	if !strings.Contains(note, "1 record(s) refused") {
		t.Errorf("the surface report never mentions the refused record:\n%s", note)
	}
	if !strings.Contains(note, "not the same as counting them as zero") {
		t.Errorf("the report does not say what a refusal means for the totals:\n%s", note)
	}
}

// GK5: the surface reports tokens and refuses to report dollars.
//
// Grok records costUsdTicks, an integer whose scale nothing in this repository
// or in the corpus establishes. A plausible factor would be indistinguishable
// from a measurement downstream, so the column stays empty and says why.
func TestGK5_TheSurfaceIsNotPriced(t *testing.T) {
	home := writeGrokHome(t, map[string]map[string]string{
		"01a09420-aaa": {"updates.jsonl": grokTurn(17415, 35, 6272, 0, 30)},
	})
	s := burnGrok(home, "")
	if s.name != "grok" {
		t.Errorf("surface name %q, want grok", s.name)
	}
	if s.tokens != 17450 {
		t.Errorf("tokens %d, want 17450: the prompt with the cache inside it, plus the output", s.tokens)
	}
	if s.pricedReqs != 0 {
		t.Errorf("%d request(s) were priced on a surface with no established unit", s.pricedReqs)
	}
	if got := costCell(s); got != "no price" {
		t.Errorf("cost cell %q, want %q: no dollar figure may be derived from costUsdTicks", got, "no price")
	}
	if s.localOnly {
		t.Errorf("Grok is a hosted API with a real bill; marking it local says nobody is invoiced")
	}
	problems := strings.Join(s.problems, " ")
	if !strings.Contains(problems, "costUsdTicks") {
		t.Errorf("the report never names the counter it declined to price:\n%s", problems)
	}
	// The scale is not unknown, it is unverified, and those are different
	// states. Grok's own client documentation ships at
	// ~/.grok/docs/user-guide/17-sessions.md and states the factor. Nothing
	// here has checked that statement against an invoice, so the surface stays
	// unpriced — but a reader who goes looking must be sent to what was found
	// rather than told nothing exists.
	if !strings.Contains(problems, "17-sessions.md") {
		t.Errorf("the client's own documented tick scale was found and the report does "+
			"not say where:\n%s", problems)
	}
	if !strings.Contains(problems, "not been checked against a statement of account") {
		t.Errorf("the report does not say why a documented scale is still not priced "+
			"here:\n%s", problems)
	}
	for _, banned := range []string{"$", "USD", "cents"} {
		if strings.Contains(problems, banned) {
			t.Errorf("a money unit reached a surface whose scale is unestablished (%q):\n%s",
				banned, problems)
		}
	}
	// Reasoning tokens are reported. Every IDE agent surveyed discards them,
	// and they are replayed as input on the next turn.
	if !strings.Contains(problems, "reasoning") {
		t.Errorf("30 reasoning tokens were read and never surfaced:\n%s", problems)
	}
}

// GK6: a rollup that exceeds the turns it rolls up is reported, not summed.
//
// A parent session's usage.json absorbs the sub-agent sessions it spawned,
// which are on disk as sessions in their own right. Adding the rollups would
// count that work twice; saying nothing would leave a reader who opens
// usage.json unable to reconcile it with this report.
func TestGK6_ARollupThatExceedsItsTurnsIsReported(t *testing.T) {
	home := writeGrokHome(t, map[string]map[string]string{
		"01a09420-parent": {
			"updates.jsonl": grokTurn(1000, 20, 800, 0, 5),
			"usage.json": `{"sessionId":"01a09420-parent","session":{"inputTokens":9000,` +
				`"outputTokens":20,"totalTokens":9020,"cachedReadTokens":800,` +
				`"cacheCreationTokens":0,"reasoningTokens":5,"modelCalls":9}}`,
		},
		"01a09420-child": {"updates.jsonl": grokTurn(8000, 0, 0, 0, 0)},
	})
	g := readGrok(findGrokSessions(grokRoots(home)))
	if g.Prompt != 9000 {
		t.Errorf("prompt %d, want 9000: each session's own turns counted once", g.Prompt)
	}
	if g.RollupDiverged != 1 {
		t.Errorf("%d divergent rollup(s), want 1", g.RollupDiverged)
	}
	if g.RollupExcess != 8000 {
		t.Errorf("rollup excess %d, want 8000: what the parent's own file claims over its turns",
			g.RollupExcess)
	}
	note := strings.Join(burnGrok(home, "").problems, " ")
	if !strings.Contains(note, "usage.json") {
		t.Errorf("the divergence is not reported to the reader:\n%s", note)
	}
}

// GK7: the paragraph that explains why the token columns cannot be added
// names every surface in the table it is explaining.
//
// It named Anthropic, Codex and Ollama, because those were the three rows
// there when it was written. Grok counts the way Codex does, and a reader
// looking for their surface in the sentence that tells them what the column
// means finds three of the four rows described and their own absent.
func TestGK7_TheNotAddableNoteNamesEverySurface(t *testing.T) {
	home := writeGrokHome(t, map[string]map[string]string{
		"01a09420-aaa": {"updates.jsonl": grokTurn(17415, 35, 6272, 0, 30)},
	})
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	var out, errb bytes.Buffer
	if err := runBurn(nil, &out, &errb); err != nil {
		t.Fatalf("runBurn: %v", err)
	}
	got := out.String()
	i := strings.Index(got, "The token columns are not addable")
	if i < 0 {
		t.Fatalf("the explanation is gone entirely:\n%s", got)
	}
	para := got[i:]
	if j := strings.Index(para, "\n\n"); j > 0 {
		para = para[:j]
	}
	if !strings.Contains(para, "Grok") {
		t.Errorf("a grok row is in the table and the paragraph explaining what the "+
			"token column means does not mention it:\n%s", para)
	}
}
