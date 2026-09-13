package main

import (
	"os"
	"path/filepath"
	"testing"
)

// firstWithEntries has to mean what its name says.
//
// It was named for picking the first of several candidates and took exactly
// one, which held only while every surface this build knew about kept its
// records in one place. It does not hold for 2026: the same agent writes to
// ~/Library/Application Support on macOS, ~/.config or ~/.local/share on Linux,
// and a dotfile home on older installs. A detector that checks one of three
// tells a reader their bill has no blind spot when it has one, which is worse
// than not detecting the surface at all.

func TestFirstWithEntriesPicksTheFirstThatHasAny(t *testing.T) {
	base := t.TempDir()
	empty := filepath.Join(base, "empty")
	missing := filepath.Join(base, "missing")
	full := filepath.Join(base, "full")
	second := filepath.Join(base, "second")
	for _, d := range []string{empty, full, second} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{full, second} {
		if err := os.WriteFile(filepath.Join(d, "a.log"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if got := firstWithEntries(missing, empty, full, second); got != full {
		t.Errorf("firstWithEntries skipped past the first candidate with entries: got %q, want %q", got, full)
	}
	if got := firstWithEntries(missing, empty); got != "" {
		t.Errorf("no candidate has entries, so the answer is none: got %q", got)
	}
	if got := firstWithEntries(); got != "" {
		t.Errorf("no candidates at all is not a surface: got %q", got)
	}
	// The single-argument form is every existing call site and must not change.
	if got := firstWithEntries(full); got != full {
		t.Errorf("one candidate that has entries: got %q, want %q", got, full)
	}
}

// A directory holding only directories is not evidence the surface has run.
// Several agents create their home eagerly on install and write nothing until
// first use, and reporting that as "detected" would name a blind spot that is
// not there.
func TestADirectoryOfDirectoriesIsNotEvidence(t *testing.T) {
	base := t.TempDir()
	shell := filepath.Join(base, "shell")
	if err := os.MkdirAll(filepath.Join(shell, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := firstWithEntries(shell); got != "" {
		t.Errorf("a directory containing only directories was treated as a used surface: %q", got)
	}
}

// An unset path is a normal argument, not a special case.
//
// Callers build candidates from environment variables that may not be set, and
// having each one branch first is how a candidate gets forgotten.
func TestAnEmptyCandidateIsSkippedNotFatal(t *testing.T) {
	base := t.TempDir()
	full := filepath.Join(base, "full")
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "a.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := firstWithEntries("", "", full); got != full {
		t.Errorf("an unset candidate was not skipped: got %q, want %q", got, full)
	}
	if got := firstWithEntries("", ""); got != "" {
		t.Errorf("only unset candidates is none: got %q", got)
	}
}
