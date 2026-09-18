package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// replay_surfaces answers a machine, not a reader. Its description promises
// "which AI agents leave readable state on this machine", and an agent that
// believes it will stop looking at whatever the answer omits.
//
// It reported Claude Code transcript roots and nothing else, so on a Codex
// machine it answered zero and named no surface at all. That is not a missing
// feature, it is a wrong answer: the two commands that do read that machine,
// `replay codex` and `replay burn`, became invisible to the agent asking.
//
// The check is written so that a cosmetic fix cannot pass it. It builds two
// homes, one carrying rollouts and one carrying none, and requires the report
// to tell them apart. A hardcoded sentence naming Codex satisfies a spelling
// test and would fail this one.
func writeCodexHome(t *testing.T, rollouts int) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude", "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	sessions := filepath.Join(home, ".codex", "sessions")
	archived := filepath.Join(home, ".codex", "archived_sessions")
	for _, d := range []string{sessions, archived} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Split across both roots. `codex archive` moves a session from one to the
	// other without changing its relevance to a bill, so a reader that knows
	// only sessions/ reports a fraction as though it were the whole.
	for i := 0; i < rollouts; i++ {
		dir := sessions
		if i%2 == 1 {
			dir = archived
		}
		p := filepath.Join(dir, "rollout-"+string(rune('a'+i))+".jsonl")
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestSurfacesReportsCodexRollouts(t *testing.T) {
	four := writeCodexHome(t, 4)
	none := writeCodexHome(t, 0)

	report := func(home string) string {
		t.Helper()
		t.Setenv("HOME", home)
		out, err := mcpCall("replay_surfaces", nil)
		if err != nil {
			t.Fatalf("replay_surfaces: %v", err)
		}
		return out
	}

	withRollouts := report(four)
	withNone := report(none)

	if !strings.Contains(strings.ToLower(withRollouts), "codex") {
		t.Errorf("replay_surfaces never names the Codex surface:\n%s", withRollouts)
	}
	// The count has to come from the disk, not from a constant. Four rollouts
	// split across sessions/ and archived_sessions/ must read as four.
	if !strings.Contains(withRollouts, "4") {
		t.Errorf("replay_surfaces did not report the 4 rollouts it was given:\n%s", withRollouts)
	}
	if withRollouts == withNone {
		t.Errorf("replay_surfaces cannot tell a Codex machine from a machine without one;\n"+
			"both answered:\n%s", withRollouts)
	}
}
