package regression

import (
	"os"
	"path/filepath"
	"testing"
)

// FC-COSIGN walked into other checkouts of this repository and failed on them.
//
// The guard audits THIS tree's test packages. `.claude/worktrees/` holds agent
// worktrees: complete checkouts of the same repository, usually at older
// commits that legitimately predate whatever the guard is looking for. Walking
// them means the guard reports packages that are not this tree's, at revisions
// nobody is asking about.
//
// Measured on this machine 2026-09-15: 16 worktrees present, and the guard
// failed identically on origin/main with none of that day's commits. CI never
// saw it because `.claude/worktrees/` is gitignored and a clean checkout has
// none, so the failure lands only on the machine of whoever is actually
// working, which is the worst place for a false alarm to live.
//
// The rule is not a deny-list of names. A directory holding its own `.git` is
// the root of a different checkout, whatever it is called, and nothing under
// it belongs to this tree.
func TestTreeWalkSkipsOtherCheckouts(t *testing.T) {
	root := t.TempDir()

	// A nested checkout, as a worktree appears: a .git FILE, not a directory.
	nested := filepath.Join(root, ".claude", "worktrees", "agent-abc", "cmd", "replay")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Join(root, ".claude", "worktrees", "agent-abc")
	if err := os.WriteFile(filepath.Join(checkout, ".git"), []byte("gitdir: /elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !skipTreeDir(root, checkout, "agent-abc") {
		t.Error("a directory carrying its own .git is another checkout and must be skipped")
	}
	// And this tree's own directories must still be walked.
	for _, keep := range []string{"cmd", "internal", "scripts"} {
		p := filepath.Join(root, keep)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if skipTreeDir(root, p, keep) {
			t.Errorf("%s is part of this tree and must be walked", keep)
		}
	}
}

// The existing exclusions must survive the change.
func TestTreeWalkStillSkipsTheUsualSuspects(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{".git", "node_modules", "dist"} {
		p := filepath.Join(root, n)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if !skipTreeDir(root, p, n) {
			t.Errorf("%s must still be skipped", n)
		}
	}
}

// The repository root carries a .git of its own. A rule that did not except it
// skipped the entire tree, and the walk then found nothing at all. FC-COSIGN
// caught that in one run because it asserts a minimum package count rather
// than trusting an empty result, which is the difference between a guard that
// found nothing wrong and one that looked nowhere.
func TestTreeWalkDoesNotSkipTheRootItself(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if skipTreeDir(root, root, filepath.Base(root)) {
		t.Fatal("the walk root was skipped because it carries .git; that skips the whole tree")
	}
}
