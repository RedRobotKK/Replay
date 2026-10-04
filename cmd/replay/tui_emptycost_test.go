package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Production wiring for the empty cost screen: through dispatch, on a HOME
// with Codex rollouts and no Claude Code transcripts, the screen the
// installer opens names both next steps. The TUI package test proves the
// renderer; this proves the command fills the field it renders.
func TestE2E_TuiEmptyCostScreenNamesTheNextSteps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("REPLAY_TRANSCRIPTS", "")
	sessions := filepath.Join(home, ".codex", "sessions", "2026", "10", "04")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessions, "rollout-x.jsonl"), []byte(`{"type":"session_meta"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errb, err := e2e(t, "tui", "-once", "-screen", "cost")
	if err != nil {
		t.Fatalf("%v\n%s", err, errb)
	}
	if !strings.Contains(out, "No transcripts to add up") {
		t.Fatalf("fixture: expected the empty cost screen:\n%s", out)
	}
	for _, want := range []string{"replay codex", "replay serve"} {
		if !strings.Contains(out, want) {
			t.Errorf("the empty cost screen reached through dispatch does not name %q:\n%s", want, out)
		}
	}
}
