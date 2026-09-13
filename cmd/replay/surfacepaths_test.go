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

// A tree of EMPTY directories is not evidence the surface has run.
//
// The name and the comment here used to say "a directory holding only
// directories", which stopped being true on 2026-09-13 and was never quite
// what this fixture tested. `hasEntries` now looks up to entryProbeDepth
// levels down, because a surface that keeps its records one directory below
// the probe root (Oracle's sessions/<slug>/meta.json, OpenClaw's
// agents/<id>/sessions/) was otherwise reported absent while sitting in front
// of the tool. HE1 in hasentries_test.go is the control for that direction.
//
// What this test pins is the other direction, which did NOT change and is the
// easier one to lose while fixing the first: a directory of directories with
// nothing underneath is still not a corpus. Several agents create their store
// eagerly on install and write nothing until first use, and reporting that as
// "detected" names a blind spot that is not there and sends a reader after
// data that does not exist.
//
// Note the fixture only ever built EMPTY subdirectories, so it passed both
// before and after the change. That is worth saying out loud rather than
// leaving as a coincidence: this test did not catch the direct-children defect
// and was never able to, despite a name that sounded like it would.
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
