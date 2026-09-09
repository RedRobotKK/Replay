package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// What else is on this desk.
//
// `replay doctor` probed three places: Claude Code transcripts, the proxy
// environment variable, and its own ledger. A survey on 2026-09-09 found five
// agent CLIs installed on the author's machine that nothing in this repository
// recorded, two of which write usage data the readers here can already parse.
// The tool was measuring one agent on a desk running six, and not because the
// data was hard to find.
//
// # Two rules, and the second is the one that matters
//
// **Report a store only when its files were counted.** Not when the directory
// exists — a dotfile outlives the tool that made it, and someone who tried an
// agent for an afternoon in 2024 still has the folder. Counting that as found
// names a tool the reader does not have, and a reader who checks one claim and
// finds it hollow stops believing the transcript count on the line above.
//
// **Say what can be read, not just that something is there.** Cursor has 118
// transcripts on this machine and zero usage fields, confirmed twice. Listing
// it beside Codex without that distinction promises a cost measurement its
// files cannot support, which is the overclaim docs/SURFACES.md was corrected
// for the same day this was written.
//
// # What this deliberately does not do
//
// It does not read $PATH, run `brew list`, or execute any binary. Discovery is
// a directory walk over a fixed list of known store locations under the user's
// own home. Enumerating installed software is a different and more invasive
// act than counting files an agent already wrote, and nothing here needs it:
// the store is what carries the data, and a tool installed but never run has
// nothing to measure anyway.

// agentFinding is one agent store found on disk.
type agentFinding struct {
	// Name is the tool as its own documentation spells it.
	Name string
	// Path is where this was found, so the reader can go and look.
	Path string
	// Files is how many store files were counted. Never zero in a finding.
	Files int
	// Capped is true when the walk stopped early, so Files is a floor.
	//
	// Separate from Files because the cap is per root and the report is per
	// finding: a two-root agent with 700 known files was printed "500+" when
	// neither walk had been truncated, and one with 1200 reported 1000 as
	// though it were exact. A limit rendered as a measurement is the defect
	// this repository keeps finding.
	Capped bool
	// Reads says what Replay can take from this surface, in the reader's terms.
	Reads string
	// Next is the exact command to run.
	Next string
	// Priceable is whether the surface carries usage the tool can cost.
	// False means conversation only, and Reads must say so.
	Priceable bool
}

// agentStore is one place worth looking, and what is true of it.
type agentStore struct {
	name string
	// roots are every absolute directory this agent keeps a store in.
	//
	// Plural because Codex keeps two: `codex archive` moves a session from
	// sessions/ to archived_sessions/ without changing its format or its
	// relevance to a bill. Scanning only the first reported 29 files on this
	// machine where `replay codex` reads 150 — and doctor prints that very
	// command on the next line. Two commands disagreeing about the same disk
	// discredits every other line in the report.
	roots     []string
	patterns  []string // file globs, matched against the base name
	reads     string
	next      string
	priceable bool
}

// knownStores is the fixed list.
//
// Roots and patterns come from the readers themselves wherever one exists,
// rather than being re-spelled here. That is not tidiness. Re-spelling Codex's
// two directories as literals is exactly how this file shipped counting only
// `sessions/` and missing `archived_sessions/` — 29 files where `replay codex`
// reads 150, with doctor printing that very command on the next line. The
// comment on codexRoots had already recorded that defect as one "this project
// fixed in its own discovery once already", and discovery reintroduced it
// within the hour.
//
// Ollama had the same shape and DS6 did not cover it: doctor counted every
// `*.log`, `replay burn` globs `server*.log`, so 12 against 6. A count that
// disagrees with the command beside it is worse than no count.
func knownStores(home string) []agentStore {
	return []agentStore{
		{
			name: "Codex", roots: codexRoots(home), patterns: []string{"*.jsonl"},
			reads:     "rollout logs: per-turn usage, and rate-limit events",
			next:      "replay codex",
			priceable: true,
		},
		{
			name:  "Grok",
			roots: []string{filepath.Join(home, ".grok")}, patterns: []string{"updates.jsonl"},
			// No reader exists yet, and the next step says so rather than
			// naming a command. An earlier draft pointed at
			// `replay doctor --agents`, a flag that does not exist — the
			// overpromise docs/design/unwired-3-branches-and-docs.md
			// catalogues, written into the tool while cataloguing it.
			// TestDS7 now fails if any next step names a command that is not
			// dispatched, so that cannot recur silently.
			reads:     "usage fields are present: inputTokens, cachedReadTokens, costUsdTicks",
			next:      "no reader built yet. The data is there; the dollar scale is unverified",
			priceable: true,
		},
		{
			name:  "Ollama",
			roots: []string{filepath.Join(home, ".ollama", "logs")},
			// server*.log, matching burn.go:184. app*.log carries no usage.
			patterns:  []string{"server*.log"},
			reads:     "server logs: prompt sizes and eval durations. Local, so no dollars",
			next:      "replay burn",
			priceable: false, // no dollars, and the line above says so
		},
		{
			name:      "Cursor",
			roots:     []string{filepath.Join(home, ".cursor")},
			patterns:  []string{"*.db", "*.jsonl", "*.sqlite"},
			reads:     "conversation transcripts only, no usage fields",
			next:      "nothing to run: this is not a spend surface and cannot be priced",
			priceable: false,
		},
	}
}

// discoverAgents walks the known stores under home and returns those that hold
// files, sorted by name so the report does not reorder between runs.
func discoverAgents(home string) []agentFinding {
	var out []agentFinding
	for _, s := range knownStores(home) {
		n, capped := 0, false
		var roots []string
		for _, root := range s.roots {
			c, hit := countStoreFiles(root, s.patterns)
			if c > 0 {
				n += c
				roots = append(roots, root)
			}
			capped = capped || hit
		}
		if n == 0 {
			// Either not installed, or installed and never used. Both are
			// "nothing to measure", and neither is worth a line.
			continue
		}
		out = append(out, agentFinding{
			Name: s.name, Path: strings.Join(roots, ", "), Files: n, Capped: capped,
			Reads: s.reads, Next: s.next, Priceable: s.priceable,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// countStoreFiles counts matching files anywhere under root.
//
// Bounded rather than exhaustive: a store can hold hundreds of thousands of
// files, and doctor is a command someone runs while waiting. The count stops
// at the cap and the caller reports "at least", which answers the question
// being asked — is there anything here — without walking 3.8 GB to do it.
func countStoreFiles(root string, patterns []string) (n int, capped bool) {
	// maxFiles, not cap: `cap` shadows the builtin, and this is the only place
	// in the repository that would.
	const maxFiles = 500
	// Resolve first. os.Stat follows a link and answers about its target while
	// WalkDir lstats and answers about the link itself, so on a symlinked
	// store the walk descends into nothing and reports zero. defaultroot.go
	// documents that exact bug and fixes it; this file reintroduced it.
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable is not found; say nothing rather than guess
		}
		if n >= maxFiles {
			capped = true
			return filepath.SkipAll
		}
		if d.IsDir() {
			return nil
		}
		for _, p := range patterns {
			if ok, _ := filepath.Match(p, d.Name()); ok {
				n++
				return nil
			}
		}
		return nil
	})
	return n, capped
}
