package main

import (
	"os"
	"path/filepath"
	"testing"
)

// HE. hasEntries reported "nothing here" for a directory full of data.
//
// It counted only non-directory DIRECT children, so a root holding
// project/chats/session.jsonl and nothing else at its top level answered
// false. Every surface shipped before 2026-09-13 happens to keep a file
// directly at its probe root, which is why this never bit: ~/.grok, ~/.cursor,
// ~/.ollama/logs and ~/.codex all do.
//
// The failure is the one the rule at the top of AGENTS_STATE.md names, and it
// is worse than a crash because it is silent. A detector that answers "nothing
// found" for a surface that is present does not merely miss it. It lets the
// report go on to imply the reader's bill has no blind spot, when it has one
// and the tool was standing in front of it.
//
// Several candidate surfaces keep exactly that shape: ~/.openclaw/agents holds
// only agent directories, ~/.oracle/sessions only session directories, and
// the Gemini CLI, OpenCode and OpenHands layouts all put a directory level
// between the root and the data.

// HE1: a directory holding only subdirectories, with data underneath, counts.
//
// This is the control that found the defect. Before the fix the first
// assertion returned false while the second returned true, from the same tree.
//
// PASS: both the root and the leaf report data.
// FAIL: a present surface is reported absent, silently.
func TestHE1_ADirectoryOfDirectoriesWithDataInsideCounts(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "projectA", "chats")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "session.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !hasEntries(deep) {
		t.Error("a directory holding a file reported no entries")
	}
	if !hasEntries(root) {
		t.Error("a directory holding only subdirectories, with a session file inside one of " +
			"them, reported no entries. A present surface is reported absent, and the report " +
			"then implies the reader's bill has no blind spot.")
	}
}

// HE2: genuinely empty is still empty, and so is a tree of empty directories.
//
// The fix must not become "any directory that exists counts". A surface whose
// store has been created but never written to is absent for this purpose, and
// announcing it would be the opposite defect: naming a surface with nothing in
// it, which sends a reader looking for data that is not there.
func TestHE2_EmptyIsStillEmpty(t *testing.T) {
	empty := t.TempDir()
	if hasEntries(empty) {
		t.Error("an empty directory reported entries")
	}

	hollow := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hollow, "a", "b", "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if hasEntries(hollow) {
		t.Error("a tree of empty directories reported entries. Nothing has been written to " +
			"this surface, and naming it would send a reader looking for data that is not there.")
	}
}

// HE3: a path that does not exist is not an error and is not a finding.
//
// os.ReadDir's error is deliberately discarded, and this is what makes that
// safe: an absent surface and an unreadable one both answer false, which is
// the honest answer to "is there data here for me to read".
func TestHE3_AnAbsentPathIsNotAFinding(t *testing.T) {
	if hasEntries(filepath.Join(t.TempDir(), "no", "such", "path")) {
		t.Error("a path that does not exist reported entries")
	}
}

// HE4: the walk is bounded.
//
// A home directory handed to this by mistake must not become a full filesystem
// crawl on a tool whose first promise is that it is cheap to run. The depth
// limit is the guard, and this asserts it exists by burying the file deeper
// than any real surface layout puts it.
func TestHE4_TheSearchIsBounded(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c", "d", "e", "f", "g", "h")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "buried.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if hasEntries(root) {
		t.Error("a file eight directories down was found. No surface layout nests that far, " +
			"and an unbounded walk from a mistaken root is a filesystem crawl on a tool " +
			"whose first promise is that it is cheap to run.")
	}
}
