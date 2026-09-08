package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Claude's desktop app runs agents in sandboxes that write Claude Code's own
// transcript format, verbatim. Discovery has to find them, and must not
// silently fold them into the local corpus.
func TestDR1(t *testing.T) {
	home := t.TempDir()
	deep := filepath.Join(home, "Library", "Application Support", "Claude",
		"local-agent-mode-sessions", "sess-1", "org-1", "local_abc", ".claude", "projects", "-a-b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "x.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Negative fixtures, without which two guards below cannot fail: a
	// "projects" directory that is NOT under .claude, and a .claude/projects
	// that holds nothing. Every fixture in the first version of this test had
	// both properties set the passing way, so dropping either check changed no
	// outcome.
	decoy := filepath.Join(home, "Library", "Application Support", "Claude",
		"local-agent-mode-sessions", "sess-1", "notclaude", "projects")
	_ = os.MkdirAll(decoy, 0o755)
	_ = os.WriteFile(filepath.Join(decoy, "z.jsonl"), []byte("{}\n"), 0o644)
	empty := filepath.Join(home, "Library", "Application Support", "Claude",
		"local-agent-mode-sessions", "sess-2", "org", "local_e", ".claude", "projects")
	_ = os.MkdirAll(empty, 0o755)

	got := desktopSandboxRoots(home)
	if len(got) == 0 {
		t.Fatal("found no sandbox transcript root under the desktop app")
	}
	for _, r := range got {
		if !strings.Contains(r, "local-agent-mode-sessions") {
			t.Errorf("root outside the sandbox tree: %s", r)
		}
		if strings.Contains(r, "notclaude") {
			t.Errorf("a projects directory outside .claude was accepted: %s", r)
		}
		if r == empty {
			t.Errorf("an empty projects directory was reported as a corpus: %s", r)
		}
	}
}

// An absent desktop app is not an error and not an empty answer that looks
// like a searched one.
func TestDR2(t *testing.T) {
	if got := desktopSandboxRoots(t.TempDir()); len(got) != 0 {
		t.Errorf("no desktop app present, got roots: %v", got)
	}
}

// The sandbox corpus must not be silently merged into the local one.
//
// Two populations with one total is the defect this repository has corrected
// three times: transcripts counted as sessions, lanes counted as tasks, and a
// break table computed over 7% of the corpus. The sandboxes are a different
// machine's worth of work running under the same account, and folding them in
// would change every published figure without saying so.
func TestDR3(t *testing.T) {
	home := t.TempDir()
	deep := filepath.Join(home, "Library", "Application Support", "Claude",
		"local-agent-mode-sessions", "s", "o", "local_x", ".claude", "projects", "-p")
	_ = os.MkdirAll(deep, 0o755)
	_ = os.WriteFile(filepath.Join(deep, "x.jsonl"), []byte("{}\n"), 0o644)
	local := filepath.Join(home, ".claude", "projects", "-q")
	_ = os.MkdirAll(local, 0o755)
	_ = os.WriteFile(filepath.Join(local, "y.jsonl"), []byte("{}\n"), 0o644)

	t.Setenv(transcriptsEnv, "")
	for _, r := range defaultTranscriptRoots(home) {
		if strings.Contains(r, "local-agent-mode-sessions") {
			t.Errorf("sandbox root entered the default corpus without being asked: %s", r)
		}
	}
}
