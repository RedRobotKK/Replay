package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Two terminal agents installed on this machine that no row covered, plus the
// correction of a shipped row that blamed the wrong thing.
//
// Both were opened here. Neither is a vendor-source derivation.

// OS15: a surface whose records live one directory down is detected.
//
// This is the shipped-row version of the hasEntries control. Oracle keeps
// `~/.oracle/sessions/<slug>/meta.json`, so the probe root holds only
// directories and no loose file at all. Before hasEntries learned to look past
// its direct children this row could never fire, and the failure was silent:
// the reader is told nothing was found, and the report then goes on to imply
// their bill has no blind spot.
//
// Every surface shipped before this happens to keep a file directly at its
// probe root (~/.grok, ~/.cursor, ~/.ollama/logs, ~/.codex), which is exactly
// why nothing caught it.
func TestOS15_ASurfaceOneDirectoryDownIsDetected(t *testing.T) {
	home := withHome(t)
	// The real shape: a slug directory per session, meta.json inside it, and
	// nothing loose at the sessions root.
	mustWrite(t, filepath.Join(home, ".oracle", "sessions", "say-ok", "meta.json"), "{}\n")

	got := surfaceNamed(findOtherSurfaces(home), "Oracle")
	if got == nil {
		t.Fatalf("a session directory holding meta.json one level down was not "+
			"found, so a present surface is reported absent: %+v",
			findOtherSurfaces(home))
	}
	if want := filepath.Join(home, ".oracle", "sessions"); got.dir != want {
		t.Errorf("the reader is pointed at %q, not %q", got.dir, want)
	}
	if got.cmd != "" {
		t.Errorf("Oracle was offered the command %q, and this build cannot price "+
			"it", got.cmd)
	}
}

// OS16: the Oracle reason names the measured inconsistency, not just an
// absence.
//
// MEASURED, 2026-09-12, Oracle 0.8.6, ~/.oracle/sessions, 3 sessions. Only one
// carries a `usage` object at all, and that one reads
// {inputTokens: 4256, outputTokens: 0, reasoningTokens: 0, totalTokens: 6,
// cost: 0.089376}. A totalTokens of 6 against an inputTokens of 4256 is not a
// figure with a rounding problem, it is a record that does not agree with
// itself, and no arithmetic recovers a bill from it. There is also no cache
// field of any kind.
//
// Saying that out loud matters more than saying "no cache field". A reader who
// hears only the latter might reasonably try to price the input and output
// columns. They do not add up either.
func TestOS16_TheOracleReasonNamesTheInconsistency(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".oracle", "sessions", "say-ok", "meta.json"), "{}\n")

	got := surfaceNamed(findOtherSurfaces(home), "Oracle")
	if got == nil {
		t.Fatal("Oracle was not detected")
	}
	low := strings.ToLower(got.why)
	for _, want := range []string{"inputtokens", "cache", "totaltokens"} {
		if !strings.Contains(low, want) {
			t.Errorf("the Oracle reason does not name %q: %q", want, got.why)
		}
	}
}

// OS17: Grok is found with the command that reads it, and carries no `why`.
//
// This test used to pin two retracted claims in sequence: first that Replay
// could not read Grok's wire (it never posted to a wire this build parses;
// the data was on disk all along), then that it had no reader for the file
// that held it. `replay grok` (grok.go) is that reader now. A surface with a
// command carries no `why` — `why` is only for the ones offering none — so
// this checks `cmd` directly rather than re-deriving the old prose assertion
// against a field that no longer holds anything.
func TestOS17_GrokIsFoundWithItsCommand(t *testing.T) {
	home := withHome(t)
	mustWrite(t, filepath.Join(home, ".grok", "updates.jsonl"), "{}\n")

	got := surfaceNamed(findOtherSurfaces(home), "Grok")
	if got == nil {
		t.Fatalf("Grok data is on disk and the surface was not found: %+v",
			findOtherSurfaces(home))
	}
	if got.cmd != "replay grok" {
		t.Errorf("Grok's command = %q, want %q", got.cmd, "replay grok")
	}
	if got.why != "" {
		t.Errorf("Grok has a command and should carry no why, got %q", got.why)
	}
}

// OS18: the state directory moves with OPENCLAW_STATE_DIR.
//
// VERIFIED from the installed bundle, openclaw 2026.2.15 at
// /opt/homebrew/lib/node_modules/openclaw: `NEW_STATE_DIRNAME = ".openclaw"`,
// and OPENCLAW_STATE_DIR appears 217 times, including in the CLI's own help
// text ("Write completion scripts to $OPENCLAW_STATE_DIR/completions"). So the
// whole tree relocates, and a probe that only knows ~/.openclaw goes blind for
// anyone who set it. That is the same class of miss as hardcoding the agent id.
func TestOS18_TheStateDirectoryMovesWithItsEnvironmentVariable(t *testing.T) {
	home := withHome(t)
	moved := filepath.Join(t.TempDir(), "relocated-state")
	if err := os.MkdirAll(moved, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(moved, "agents", "main", "sessions", "s.jsonl"), "{}\n")
	t.Setenv("OPENCLAW_STATE_DIR", moved)

	got := surfaceNamed(findOtherSurfaces(home), "OpenClaw")
	if got == nil {
		t.Fatalf("OPENCLAW_STATE_DIR points at a tree with a session in it and the "+
			"surface was not found: %+v", findOtherSurfaces(home))
	}
	if want := filepath.Join(moved, "agents", "main", "sessions"); got.dir != want {
		t.Errorf("the reader is pointed at %q, not at the relocated tree (%q)",
			got.dir, want)
	}
}
