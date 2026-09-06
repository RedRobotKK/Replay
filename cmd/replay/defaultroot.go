package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// transcriptsEnv lets a user name their corpus when it is somewhere discovery
// does not reach. Colon-separated, like PATH, because a person with sessions in
// two places should not have to pick one.
const transcriptsEnv = "REPLAY_TRANSCRIPTS"

// defaultTranscriptRoots returns every directory that holds transcripts.
//
// It used to return one: $CLAUDE_CONFIG_DIR (or ~/.claude) plus /projects. That
// made an unsearched location indistinguishable from an empty machine, and a
// user with sessions in two places got a cost total over one of them, which is
// worse than no total because it is wrong and looks right.
//
// An explicit REPLAY_TRANSCRIPTS wins outright. Discovery is a convenience; a
// user who has said where their corpus is has ended the question.
func defaultTranscriptRoots(home string) []string {
	if v := strings.TrimSpace(os.Getenv(transcriptsEnv)); v != "" {
		var out []string
		for _, p := range filepath.SplitList(v) {
			if p = strings.TrimSpace(p); p != "" && holdsTranscripts(p) {
				out = append(out, p)
			}
		}
		return out
	}
	var out []string
	seen := map[string]bool{}
	for _, root := range candidateTranscriptRoots(home) {
		if seen[root] || !holdsTranscripts(root) {
			continue
		}
		seen[root] = true
		out = append(out, root)
	}
	return out
}

// candidateTranscriptRoots lists where a transcript is plausibly written.
//
// Order is search order and carries no precedence: every candidate that holds
// transcripts is used, because a corpus split across two of them is still one
// corpus.
func candidateTranscriptRoots(home string) []string {
	if home == "" {
		return nil
	}
	roots := []string{
		filepath.Join(claudeConfigDir(home), "projects"),
		filepath.Join(home, ".claude", "projects"),
		filepath.Join(home, ".config", "claude", "projects"),
	}
	if x := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); x != "" {
		roots = append(roots, filepath.Join(x, "claude", "projects"))
	}
	// claudeConfigDir resolves to ~/.claude unless CLAUDE_CONFIG_DIR overrides
	// it, so the first two candidates are usually the same path. Deduplicate
	// here rather than at each caller: this list is shown to the user, and one
	// directory named twice reads as a broken tool.
	out := make([]string, 0, len(roots))
	seen := make(map[string]bool, len(roots))
	for _, r := range roots {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

// holdsTranscripts reports whether a directory contains at least one .jsonl,
// stopping at the first. Whether a corpus is worth reading is a question for
// the reader; this only answers whether there is one.
func holdsTranscripts(root string) bool {
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return false
	}
	found := false
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".jsonl" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// explainNoCorpus says where Replay looked and how to tell it where to look
// instead.
//
// The previous message named one directory and moved on to the command list. A
// user could not tell "you have no transcripts" from "yours are somewhere I did
// not search", and had no way to say which. Those are different situations and
// only one of them is the user's problem to fix.
func explainNoCorpus(home string, w io.Writer) {
	roots := candidateTranscriptRoots(home)
	if len(roots) == 0 {
		_, _ = fmt.Fprintf(w, "No home directory could be resolved, so there was nowhere to look "+
			"for transcripts.\n")
	} else {
		_, _ = fmt.Fprintf(w, "No transcripts found. Looked in:\n")
		for _, r := range roots {
			_, _ = fmt.Fprintf(w, "  %s\n", r)
		}
	}
	_, _ = fmt.Fprintf(w, "\nIf your sessions are somewhere else, say so:\n")
	_, _ = fmt.Fprintf(w, "  %s=/path/to/projects replay\n", transcriptsEnv)
	_, _ = fmt.Fprintf(w, "  replay cost /path/to/projects\n")
	_, _ = fmt.Fprintf(w, "\nReplay reads Claude Code sessions from disk. For any other agent, "+
		"replay serve proxies it live.\n\n")
}
