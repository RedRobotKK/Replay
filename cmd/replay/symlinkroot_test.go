package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A transcript root reached through a symlink is still a transcript root.
//
// holdsTranscripts calls os.Stat, which FOLLOWS a symlink and answers IsDir
// true, then hands the same path to filepath.WalkDir, which LSTATS it, sees a
// symlink rather than a directory, and descends into nothing. The function
// returns false having looked at no files.
//
// doctor takes the other route: os.ReadDir opens through the link and finds
// everything. On a machine with a symlinked ~/.claude, bare `replay` said
// "Claude Code is here, but has recorded no sessions yet" while `replay doctor`
// counted 1,681 transcripts in the same shell.
//
// That is not an error a reader can recognise as one. It is an affirmative
// false statement about the user's own machine, in the first thing the tool
// ever prints to them.
//
// countTranscripts already carries a comment explaining that a promise derived
// by a second, subtly different walk is how these numbers drift apart, and its
// test asserts two walks agree. That discipline covered countTranscripts and
// transcriptFiles. It did not cover doctor and holdsTranscripts.

// SL-1: a symlinked root holds its transcripts.
func TestSL1_ASymlinkedRootHoldsItsTranscripts(t *testing.T) {
	real := t.TempDir()
	proj := filepath.Join(real, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "s.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(t.TempDir(), "projects")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	if !holdsTranscripts(link) {
		t.Fatal("holdsTranscripts said a symlinked root holds nothing; the same directory read directly holds one transcript")
	}
}

// SL-2: the two walks agree, on a plain root and on a symlinked one.
//
// This is the assertion that would have caught the defect. Either walk alone
// looks correct against its own fixture; only comparing them shows that they
// disagree about what a symlink is.
func TestSL2_TheTwoWalksAgreeAboutWhatIsThere(t *testing.T) {
	real := t.TempDir()
	proj := filepath.Join(real, "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "s.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(t.TempDir(), "projects")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	for _, root := range []string{real, link} {
		held := holdsTranscripts(root)
		counted := countTranscripts(root)
		found := counted.sessions+counted.lanes > 0
		if held != found {
			t.Errorf("root %q: holdsTranscripts says %v, countTranscripts finds %d sessions and %d lanes; the two walks must agree about the same directory", root, held, counted.sessions, counted.lanes)
		}
	}
}

// SL-3: an empty symlinked root is still empty.
//
// The fix must not reach the right answer by abandoning the question. A root
// that genuinely holds nothing has to keep reporting nothing, or SL-1 could be
// satisfied by returning true unconditionally.
func TestSL3_AnEmptySymlinkedRootIsStillEmpty(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "projects")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	if holdsTranscripts(link) {
		t.Fatal("holdsTranscripts found transcripts in an empty symlinked directory")
	}
}
