package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The tool already knows where the transcripts are. It should use them.
//
// `replay doctor` finds them unaided - "90 sessions across 12 projects under
// ~/.claude/projects" - while `replay cost` with no argument returns
// "one or more transcript directories are required: invalid usage".
//
// That is the first command a person runs after `curl | sh`, and it fails. The
// data was on disk before the install and the tool located it in a different
// subcommand a second later. This is the whole funnel dying at step one over an
// argument the binary can infer.

// DR-1: the default root is found when it exists.
func TestDR1_DefaultRootIsFoundWhenItExists(t *testing.T) {
	home := t.TempDir()
	p := filepath.Join(home, ".claude", "projects", "proj")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "s.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := defaultTranscriptRoots(home)
	if len(got) != 1 || got[0] != filepath.Join(home, ".claude", "projects") {
		t.Errorf("did not find the transcript root: %v", got)
	}
}

// DR-2: nothing invented when the directory is absent.
//
// PASS: empty, so the caller still shows its usage error.
// FAIL: a path that does not exist, which turns a clear "tell me where" into an
// obscure "no such file".
func TestDR2_NothingIsInventedWhenAbsent(t *testing.T) {
	if got := defaultTranscriptRoots(t.TempDir()); len(got) != 0 {
		t.Errorf("invented a root that does not exist: %v", got)
	}
}

// DR-3: an empty transcript directory is not a root.
//
// A directory with no transcripts in it produces a report about nothing, which
// reads as "the tool does not work" rather than "there is nothing here yet".
func TestDR3_AnEmptyDirectoryIsNotARoot(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude", "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := defaultTranscriptRoots(home); len(got) != 0 {
		t.Errorf("an empty projects directory is not a usable default: %v", got)
	}
}
