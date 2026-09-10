package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// A cap on a session must be drawn from what sessions cost.
//
// `advise --guards` and the guards screen fence with Tukey's upper fence over a
// spread, and print the result as `--max-session-usd`. The spread was one value
// per TRANSCRIPT FILE. A session writes one file per sub-agent lane, so on a
// fanned-out corpus the population was lanes: 1,803 of them where the corpus
// holds 116 sessions, and the printed derivation said "over 1803 sessions".
//
// The two distributions are not the same distribution, and they differ in the
// direction that matters. A lane is a fraction of its session, so a fence over
// lanes sits below the session distribution it is about to be applied to. On
// the corpus this was found on, `cost` reported median session $0.77 and p90
// $3.41 while the lane fence recommended $2.23 — a cap advertised as an outlier
// threshold, sitting under the 90th percentile of the thing it caps.
//
// That the two numbers were close here is a property of this corpus, not a
// defence: the session with 1,030 lanes cost $1,056.14 and contributed 1,030
// values below a dollar.

func TestSpreadFoldsLanesIntoSessions(t *testing.T) {
	// Three lanes of one session, one lane of another.
	in := []laneSpend{
		{ID: "aaaa1111", USD: 1.0, Tokens: 100},
		{ID: "aaaa1111", USD: 2.0, Tokens: 200},
		{ID: "aaaa1111", USD: 3.0, Tokens: 300},
		{ID: "bbbb2222", USD: 0.5, Tokens: 50},
	}
	usd, toks, sessions := foldSpread(in)

	if sessions != 2 {
		t.Errorf("four lanes of two sessions folded to %d sessions", sessions)
	}
	if len(usd) != 2 || len(toks) != 2 {
		t.Fatalf("the spread has %d dollar values and %d token values for 2 sessions",
			len(usd), len(toks))
	}
	// The folded session must carry the SUM of its lanes. A session that ran
	// three lanes spent what all three spent; capping it at what one lane spent
	// is the defect.
	var total float64
	for _, v := range usd {
		total += v
	}
	if total != 6.5 {
		t.Errorf("the spread totals $%.2f, want $6.50: lanes were dropped rather than "+
			"folded", total)
	}
	found := false
	for _, v := range usd {
		if v == 6.0 {
			found = true
		}
	}
	if !found {
		t.Errorf("no session in the spread costs $6.00, so the three lanes of "+
			"aaaa1111 were counted separately: %v", usd)
	}
}

// And a spread with no fan-out is unchanged, so the fold cannot be a no-op that
// happens to pass on flat corpora.
func TestSpreadWithoutFanOutIsUnchanged(t *testing.T) {
	in := []laneSpend{
		{ID: "a", USD: 1.0, Tokens: 10},
		{ID: "b", USD: 2.0, Tokens: 20},
		{ID: "c", USD: 3.0, Tokens: 30},
	}
	usd, _, sessions := foldSpread(in)
	if sessions != 3 || len(usd) != 3 {
		t.Errorf("three single-lane sessions folded to %d sessions / %d values",
			sessions, len(usd))
	}
}

// A lane with no session id cannot be folded into a session, and must not
// become one of its own: an unattributable value in a spread that is about to
// set a refusal threshold is a value nobody can check.
func TestSpreadDropsLanesWithNoSession(t *testing.T) {
	usd, _, sessions := foldSpread([]laneSpend{
		{ID: "", USD: 99.0, Tokens: 1},
		{ID: "a", USD: 1.0, Tokens: 10},
	})
	if sessions != 1 || len(usd) != 1 || usd[0] != 1.0 {
		t.Errorf("an unattributed lane reached the spread: %v over %d sessions", usd, sessions)
	}
}

// The fold has to be wired, not merely written.
//
// The tests above pin foldSpread. They passed with the fold removed from
// guardsState entirely, because they call the helper directly and nothing
// checked the call site — the same unguarded-wiring shape that let the original
// defect ship. This one walks a real corpus through guardsState.
func TestGuardsStateFoldsTheCorpusIntoSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REPLAY_TRANSCRIPTS", "")

	const sess = "aaaa1111-2222-3333-4444-555555555555"
	proj := filepath.Join(home, ".claude", "projects", "proj-a")
	lanes := filepath.Join(proj, sess, "subagents")
	if err := os.MkdirAll(lanes, 0o755); err != nil {
		t.Fatal(err)
	}
	// One session transcript and four sub-agent lanes, all carrying the parent
	// session id — which is how Claude Code writes them, and what makes the
	// fold possible at all.
	writeTranscript(t, filepath.Join(proj, sess+".jsonl"), sess, "req-main")
	for i, name := range []string{"agent-a1", "agent-a2", "agent-a3", "agent-a4"} {
		writeTranscript(t, filepath.Join(lanes, name+".jsonl"), sess, fmt.Sprintf("req-l%d", i))
	}

	_, sessions := guardsState()
	if sessions == 0 {
		t.Fatal("guardsState read nothing; the fixture is not being walked and this " +
			"test is checking nothing")
	}
	if sessions != 1 {
		t.Errorf("five transcripts of one session produced a spread over %d sessions. "+
			"The spread feeds a fence printed as --max-session-usd, so its population "+
			"has to be sessions", sessions)
	}
}

// writeTranscript writes the smallest Claude Code transcript the parser accepts:
// one assistant line with a request id, a model and usage.
func writeTranscript(t *testing.T, path, session, reqID string) {
	t.Helper()
	line := map[string]any{
		"type":      "assistant",
		"requestId": reqID,
		"sessionId": session,
		"uuid":      reqID + "-uuid",
		"timestamp": "2026-09-01T00:00:00.000Z",
		"version":   "2.1.233",
		"message": map[string]any{
			"role":  "assistant",
			"type":  "message",
			"model": "claude-opus-5",
			"content": []any{
				map[string]any{"type": "text", "text": "hello"},
			},
			"usage": map[string]any{
				"input_tokens":                10,
				"output_tokens":               20,
				"cache_read_input_tokens":     1000,
				"cache_creation_input_tokens": 500,
			},
		},
	}
	b, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
