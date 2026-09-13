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

// audit.jsonl sits beside these transcripts and is deliberately not read.
//
// Recorded here because it is a trap that looks like a find. The same sandbox
// tree carries a second population of usage-bearing JSONL under the name
// audit.jsonl, and it is not extra data: it is the same traffic logged again.
// MEASURED on this machine, 2026-09-12, under this same base directory:
//
//	audit.jsonl              540 files   41,004 usage rows   5,566,921,095 cache_read
//	.claude/projects/*.jsonl 787 files   74,021 usage rows   7,150,518,356 cache_read
//
// The audit records carry cache_read_input_tokens, cache_creation_input_tokens,
// input_tokens and output_tokens under exactly the field names the transcript
// parser already understands, plus duration_ms, iterations and tool_uses. So
// they will parse cleanly, produce plausible figures, and inflate Claude
// Desktop's cache-read total by roughly 78% for anyone who globs *.jsonl here
// rather than the .claude/projects tree specifically.
//
// That is why desktopSandboxRoots matches on a `projects` directory whose
// parent is `.claude` rather than on the file extension. Widening the match is
// the failure mode, and it would not announce itself: every number would still
// look like a number. Whoever next extends this walk should treat a rise in
// the desktop totals as evidence of double counting until proven otherwise.
//
// The figure was handed over as "roughly half" and measured here at 78%. The
// direction was right and the magnitude was not, which is the sort of thing
// that only shows up when somebody counts.

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
