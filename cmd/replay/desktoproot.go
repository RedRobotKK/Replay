package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Claude's desktop app runs agents in sandboxes, and each sandbox writes Claude
// Code's own transcript format, verbatim.
//
// Found 2026-09-07 while inventorying agent state on disk. The docs excluded
// ~/Library/Application Support/Claude by name as "a different product", and
// 11 GB of it is indeed VM images and browser cache. The payload is not: under
// local-agent-mode-sessions each sandbox home carries a full
// .claude/projects/<slug>/<uuid>.jsonl tree with the same usage fields,
// including the ephemeral_5m and ephemeral_1h cache-creation split, and
// isSidechain. It needs a discovery glob rather than a reader.
//
// It is NOT added to the default corpus, and that is the whole design decision
// here. This repository has corrected the same defect three times in one day:
// transcripts counted as sessions, agent lanes counted as tasks, and a break
// table computed over 7% of the files. Folding a second population into the
// default total would change every figure the tool prints without saying so.
// Discovery reports it; the user asks for it.
const desktopSandboxDir = "local-agent-mode-sessions"

// desktopSandboxRoots returns each .claude/projects directory inside a desktop
// sandbox, or nothing when the app is not installed.
//
// The walk is bounded to the sandbox tree and stops descending at any projects
// directory it finds, so it never enters the transcripts themselves. An absent
// app returns no roots and no error: not looked and looked-and-empty are
// different answers, and the caller distinguishes them by whether the base
// exists at all.
func desktopSandboxRoots(home string) []string {
	if home == "" {
		return nil
	}
	base := filepath.Join(home, "Library", "Application Support", "Claude", desktopSandboxDir)
	if fi, err := os.Stat(base); err != nil || !fi.IsDir() {
		return nil
	}
	seen := map[string]bool{}
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil //nolint:nilerr // an unreadable sandbox is skipped, not fatal
		}
		if d.Name() != "projects" || filepath.Base(filepath.Dir(p)) != ".claude" {
			return nil
		}
		if holdsTranscripts(p) {
			seen[p] = true
		}
		// Do not descend into a corpus: everything below is a project slug.
		return fs.SkipDir
	})
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
