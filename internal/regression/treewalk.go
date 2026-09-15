package regression

import (
	"os"
	"path/filepath"
)

// skipTreeDir reports whether a directory is outside this repository's own
// tree and should not be audited by the guards that walk it.
//
// Two rules, and the second is the one that was missing.
//
// The named directories are build and dependency output: nothing in them is
// this project's source.
//
// A directory carrying its own `.git` is the root of a DIFFERENT checkout.
// Agent worktrees land in `.claude/worktrees/`, each a complete copy of this
// repository, usually at an older commit that legitimately predates whatever
// a guard is looking for. Auditing them reports packages that are not this
// tree's, at revisions nobody asked about.
//
// It is a rule about what a directory IS rather than a deny-list of names,
// because the next nested checkout will not be called `.claude`. A worktree's
// `.git` is a file rather than a directory, so both are accepted here.
//
// Measured 2026-09-15: 16 worktrees on this machine, and FC-COSIGN failed
// identically on origin/main with none of that day's commits in it. CI never
// saw it, because `.claude/worktrees/` is gitignored and a clean checkout has
// none. The false alarm therefore lands only on the machine of whoever is
// actually working, which is the worst place for it.
//
// root is excepted, and it has to be: the repository root carries a `.git` of
// its own, so a rule that did not except it skipped the whole tree. The first
// version did exactly that, and FC-COSIGN caught it in one run because it
// asserts it found at least two packages rather than trusting an empty walk.
// A guard that reports nothing and a guard that finds nothing wrong look
// identical without that assertion.
func skipTreeDir(root, path, name string) bool {
	switch name {
	case ".git", "node_modules", "dist":
		return true
	}
	if path == root {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		return true
	}
	return false
}
