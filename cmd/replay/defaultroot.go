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

// explainNoCorpus says what kind of empty this is, and what to do about it.
//
// Three situations produce no transcripts and they are not the same. The agent
// was never installed here. It is installed and has recorded nothing yet. Or
// there is a corpus somewhere Replay did not look. The next step differs for
// each, and printing one message over all three tells the reader that the tool
// does not know which they are in.
//
// This is the installer's advertised first command, so on a fresh machine this
// function IS the product's first impression. It leads with what Replay would
// have shown, because a reader who cannot see the point will not go and get a
// corpus to prove it.
func explainNoCorpus(home string, w io.Writer) {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
	roots := candidateTranscriptRoots(home)

	if len(roots) == 0 {
		p("No transcripts found: no home directory could be resolved, so there was\n")
		p("nowhere to look.\n\n")
		p("Point Replay at your sessions directly:\n")
		p("  replay cost /path/to/projects\n\n")
		return
	}

	switch {
	case !anyRootExists(roots):
		p("No transcripts found. Claude Code is not installed here, or has never run.\n\n")
		p("Replay reads its session files and shows what your agent's prompt cache\n")
		p("cost you: which turns re-billed the whole context, and why.\n\n")
		p("If you use a different agent, Replay can measure it live instead:\n")
		p("  replay serve\n\n")
	default:
		p("No transcripts found. Claude Code is here, but has recorded no sessions yet.\n\n")
		p("Run it once, then come back. Replay will show what each task cost and\n")
		p("where the prompt cache broke.\n\n")
	}

	p("Already have sessions somewhere else?\n")
	p("  %s=/path/to/projects replay\n\n", transcriptsEnv)
	p("Looked in:\n")
	for _, r := range roots {
		p("  %s\n", r)
	}
	p("\n")
}

// anyRootExists reports whether any candidate directory is present, which is
// what separates "never installed" from "installed and empty".
func anyRootExists(roots []string) bool {
	for _, r := range roots {
		if fi, err := os.Stat(r); err == nil && fi.IsDir() {
			return true
		}
	}
	return false
}
