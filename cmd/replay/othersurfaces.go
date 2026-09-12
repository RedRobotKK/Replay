package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Before declaring that there is nothing to read, look at the other agents.
//
// `defaultTranscriptRoots` knows only where Claude Code writes. `codexRoots`
// and the Ollama log glob have existed for as long as `replay codex` and
// `replay burn` have, and `discover` has known `~/.grok` and `~/.cursor`
// longer still. The empty-state path consulted none of them. So a reader with
// hundreds of Codex rollout logs on disk installed Replay, ran it, and was
// told "Claude Code is not installed here, or has never run" — true, and the
// wrong answer to the question they asked.
//
// It is the worst case to get wrong, because Codex is the richest surface
// available: it reports a cache-write field Anthropic's own wire does not,
// separates reasoning tokens, and carries a quota counter that actually moves.
// Sending that reader away is sending away the one user whose data can answer
// the most.

// codexHome is where Codex keeps its rollout logs, relative to the user's home.
//
// It lives here rather than in codex.go so that the set of agent homes this
// build knows about has exactly one home. FD-11 reads this file against every
// other `filepath.Join(home, ".<agent>")` in cmd/replay; a root defined
// somewhere else is a root the empty state can go blind to.
const codexHome = ".codex"

// otherSurface is an agent whose records are on this machine but which the
// default transcript roots do not cover.
//
// `cmd` is empty for a surface this build cannot yet read. That distinction is
// the whole point of the type: naming a surface tells the reader their data
// was seen, and offering a command that returns nothing would be the same
// defect as pointing them at an empty directory — a second empty report, this
// time with the tool's word behind it.
type otherSurface struct {
	name string // what the reader calls it
	dir  string // where its records were found
	cmd  string // the command that reads it, or empty if none does yet
	why  string // why there is no command, when there is none
}

// knownSurfaces is the full set of agent homes this build knows about.
//
// It is deliberately the single place that set is written down. FD-11 checks
// this file against every `filepath.Join(home, ".<agent>")` in cmd/replay, so
// a surface taught to `burn` or `discover` and forgotten here fails the suite
// rather than silently reintroducing the blind spot.
func knownSurfaces(home string) []otherSurface {
	codex := otherSurface{name: "Codex", cmd: "replay codex"}
	for _, dir := range codexRoots(home) {
		if hasEntries(dir) {
			codex.dir = dir
			break
		}
	}
	return []otherSurface{
		codex,
		{
			name: "Ollama",
			dir:  firstWithEntries(filepath.Join(home, ".ollama", "logs")),
			cmd:  "replay burn",
		},
		{
			name: "Grok",
			dir:  firstWithEntries(filepath.Join(home, ".grok")),
			why:  "Replay cannot read Grok's wire yet: it posts to /responses, which this build does not parse",
		},
		{
			name: "Cursor",
			dir:  firstWithEntries(filepath.Join(home, ".cursor")),
			why:  "Replay cannot read Cursor yet: its local history carries structure but no token usage",
		},
	}
}

// findOtherSurfaces reports the non-Claude-Code agents with records on disk.
//
// A directory that exists but is empty does not count. That is what a machine
// which once installed an agent looks like, and pointing its owner at a second
// command only to show them a second empty report is worse than saying nothing.
func findOtherSurfaces(home string) []otherSurface {
	// Without a home there is nothing to look in, and looking anyway is worse
	// than not looking: filepath.Join("", ".codex") is the relative path
	// ".codex", so every probe below would resolve against the working
	// directory and report whatever the reader happened to `cd` into.
	if home == "" {
		return nil
	}
	var found []otherSurface
	for _, s := range knownSurfaces(home) {
		if s.dir != "" {
			found = append(found, s)
		}
	}
	// Readable surfaces first: a reader with both wants the one they can act
	// on at the top, not sorted under an apology for one they cannot.
	sort.SliceStable(found, func(i, j int) bool {
		return found[i].cmd != "" && found[j].cmd == ""
	})
	return found
}

// firstWithEntries returns dir when it holds at least one file, else "".
func firstWithEntries(dir string) string {
	if hasEntries(dir) {
		return dir
	}
	return ""
}

// hasEntries reports whether a directory exists and holds at least one file.
//
// The read error is deliberately not branched on. ReadDir returns no entries
// for a directory that is missing or unreadable, so the loop below already
// answers false; and where it returns partial entries alongside an error,
// those entries are a true answer to the question being asked.
func hasEntries(dir string) bool {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			return true
		}
	}
	return false
}

// writeOtherSurfaces names what was found, or writes nothing.
//
// Nothing is the common case and must stay silent: a machine with no other
// agent data would otherwise carry this paragraph on every empty run.
func writeOtherSurfaces(found []otherSurface, w io.Writer) {
	for _, s := range found {
		if s.cmd != "" {
			_, _ = fmt.Fprintf(w, "%s records are on this machine, and Replay reads them:\n", s.name)
			_, _ = fmt.Fprintf(w, "  %s\n", s.cmd)
		} else {
			_, _ = fmt.Fprintf(w, "%s records are on this machine. %s.\n", s.name, s.why)
		}
		_, _ = fmt.Fprintf(w, "  found in %s\n\n", s.dir)
	}
}
