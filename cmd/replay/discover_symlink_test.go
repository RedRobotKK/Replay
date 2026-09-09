package main

import (
	"os"
	"path/filepath"
	"testing"
)

// DS8: a symlinked store is found.
//
// WalkDir lstats the root and answers about the link rather than the directory,
// so a symlinked store walked as given descends into nothing and reports zero —
// telling a user they do not have a tool they use daily, which is DS2 inverted.
// defaultroot.go:90-102 documents this exact bug and fixes it for transcripts;
// discovery reintroduced it.
func TestDS8_ASymlinkedStoreIsFound(t *testing.T) {
	real := t.TempDir()
	inner := filepath.Join(real, "sessions", "2026")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "rollout-a.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(real, "sessions"), filepath.Join(home, ".codex", "sessions")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	found := discoverAgents(home)
	if len(found) != 1 || found[0].Name != "Codex" || found[0].Files != 1 {
		t.Fatalf("a symlinked Codex store was not found: %+v", found)
	}
}

// DS9: files that are not evidence are not counted.
//
// The mutation that escaped every other test: delete the glob loop in
// countStoreFiles and count every file under the root. All seven passed. The
// production effect is a `~/.cursor` holding only `argv.json` and a lock file
// reporting "Cursor 2 files" — a tool the reader does not use, named
// confidently, which is DS2 defeated through the back door.
//
// DS6 pins Codex's count to what its reader sees, and that is the only test
// that constrained Files to a value at all. This constrains the other three.
func TestDS9_NonEvidenceFilesAreNotCounted(t *testing.T) {
	home := fakeHome(t, map[string]string{
		// One real rollout, and three files that are not evidence of anything.
		".codex/sessions/2026/rollout-a.jsonl": "{}\n",
		".codex/sessions/argv.json":            "{}",
		".codex/sessions/config.toml":          "x = 1",
		".codex/sessions/.DS_Store":            "junk",
		// Ollama keeps app*.log beside server*.log; only the latter carries
		// the usage lines `replay burn` reads.
		".ollama/logs/server.log": "line\n",
		".ollama/logs/app.log":    "line\n",
		".ollama/logs/app-1.log":  "line\n",
	})
	byName := map[string]agentFinding{}
	for _, f := range discoverAgents(home) {
		byName[f.Name] = f
	}
	if got := byName["Codex"].Files; got != 1 {
		t.Errorf("Codex counted %d files; only one is a rollout. Counting argv.json "+
			"and .DS_Store inflates every store and names tools nobody uses.", got)
	}
	if got := byName["Ollama"].Files; got != 1 {
		t.Errorf("Ollama counted %d files; `replay burn` globs server*.log and reads 1. "+
			"app*.log carries no usage.", got)
	}
}
