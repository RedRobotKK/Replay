package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/RedRobotKK/Replay/internal/surfaces"
)

// `replay surfaces` answers "what will each agent on this machine tell me about
// its prompt cache", which is a different question from "what did it cost".
//
// Every coding agent measured on 2026-09-15 reports a cache read. One of them
// reports the write that had to have happened for that read to exist. The
// others fail to in three distinct ways, and the difference decides what a
// reader may conclude:
//
//	absent field   claims nothing, so nothing can be wrongly inferred
//	present zero   looks like a measurement and will be summed downstream
//	silent         no counters at all
//
// Collapsing those into one number is what every other tool does. This verb
// exists to keep them apart.

// knownSurface is one agent store and the counters it uses. Adding a surface
// here is the only change required to have it scanned and reported.
var cacheSurfaces = []struct {
	Name  string
	Root  string   // relative to $HOME
	Globs []string // patterns under Root
	Spec  surfaces.FieldSpec
}{
	{"Claude Code", ".claude/projects", []string{"**/*.jsonl"},
		surfaces.FieldSpec{Name: "Claude Code", ReadKey: "cache_read_input_tokens", WriteKey: "cache_creation_input_tokens"}},
	{"Codex", ".codex", []string{"**/rollout-*.jsonl"},
		surfaces.FieldSpec{Name: "Codex", ReadKey: "cached_input_tokens", WriteKey: "cache_creation_input_tokens"}},
	{"Grok", ".grok", []string{"**/*.jsonl"},
		surfaces.FieldSpec{Name: "Grok", ReadKey: "cachedReadTokens", WriteKey: "cacheCreationTokens"}},
	{"OpenClaw", ".openclaw", []string{"**/*.jsonl"},
		// cacheRead appears twice per record, in tokens and again in dollars
		// under cost. Summing both mixes units.
		surfaces.FieldSpec{Name: "OpenClaw", ReadKey: "cacheRead", WriteKey: "cacheWrite", ExcludeUnder: "cost"}},
	{"Cursor", ".cursor", []string{"**/*.jsonl"},
		surfaces.FieldSpec{Name: "Cursor", ReadKey: "cache_read_input_tokens", WriteKey: "cache_creation_input_tokens"}},
}

type surfaceRow struct {
	Name              string `json:"name"`
	State             string `json:"state"`
	Files             int    `json:"files"`
	Records           int    `json:"records"`
	Reads             int64  `json:"reads"`
	Writes            *int64 `json:"writes"` // nil when no write field was seen
	WriteFieldPresent bool   `json:"writeFieldPresent"`
}

func runSurfaces(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("surfaces", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the readings as JSON")
	maxFiles := fs.Int("max-files", 40, "scan at most this many files per surface, newest first")
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "surfaces: takes no positional arguments; got %q\n", fs.Arg(0))
		return errUsage
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locating the home directory: %w", err)
	}
	readings := scanAll(home, *maxFiles)
	if *asJSON {
		rows := make([]surfaceRow, 0, len(readings))
		for _, r := range readings {
			rows = append(rows, toRow(r))
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return fmt.Errorf("writing JSON: %w", err)
		}
		return nil
	}
	renderSurfaces(stdout, readings)
	return nil
}

func toRow(r surfaces.Reading) surfaceRow {
	row := surfaceRow{Name: r.Name, State: string(surfaces.Classify(r)),
		Records: r.Records, Reads: r.Reads, WriteFieldPresent: r.WriteFieldPresent}
	if r.WriteFieldPresent {
		w := r.Writes
		row.Writes = &w
	}
	return row
}

// scanAll walks each known surface. A store that is absent from disk is left
// out entirely rather than reported as empty: not installed and installed but
// silent are different facts.
func scanAll(home string, maxFiles int) []surfaces.Reading {
	var out []surfaces.Reading
	for _, s := range cacheSurfaces {
		root := filepath.Join(home, s.Root)
		// ADR-0018: a store that cannot be stat'd is reported as ABSENT, not as
		// installed-and-silent. Those are different facts and a reader must not
		// see "silent" for a tool that was never installed. An unreadable store
		// is indistinguishable from an absent one here, which is a known limit
		// of this scan rather than a claim about the store.
		if _, err := os.Stat(root); err != nil {
			continue
		}
		files := newestUnder(root, maxFiles)
		r := surfaces.Reading{Name: s.Name}
		for _, f := range files {
			fh, err := os.Open(f)
			// A file that will not open contributes nothing rather than zero:
			// it does not raise Records, so it cannot pull a surface toward
			// "silent" or dilute a read total that other files established.
			if err != nil {
				continue
			}
			part := surfaces.Scan(fh, s.Spec)
			fh.Close()
			r.Records += part.Records
			r.Reads += part.Reads
			r.Writes += part.Writes
			r.WriteFieldPresent = r.WriteFieldPresent || part.WriteFieldPresent
		}
		out = append(out, r)
	}
	return out
}

func newestUnder(root string, max int) []string {
	type ent struct {
		path string
		mod  int64
	}
	var ents []ent
	// Walk's own error is discarded deliberately: a partially readable tree
	// still yields the files it could reach, and a surface scanned from fewer
	// files reports fewer records rather than a wrong state.
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		// An entry that cannot be stat'd is skipped, not counted as empty: it
		// is unknown, and the third value is not the first.
		if err != nil || fi == nil || fi.IsDir() {
			return nil
		}
		if filepath.Ext(p) != ".jsonl" || fi.Size() < 512 {
			return nil
		}
		ents = append(ents, ent{p, fi.ModTime().UnixNano()})
		return nil
	})
	sort.Slice(ents, func(i, j int) bool { return ents[i].mod > ents[j].mod })
	if len(ents) > max {
		ents = ents[:max]
	}
	paths := make([]string, 0, len(ents))
	for _, e := range ents {
		paths = append(paths, e.path)
	}
	return paths
}

func renderSurfaces(w io.Writer, readings []surfaces.Reading) {
	if len(readings) == 0 {
		fmt.Fprintln(w, "No agent store was found on this machine. That is a result, not an error:")
		fmt.Fprintln(w, "nothing here reads anything it was not pointed at.")
		return
	}
	fmt.Fprintf(w, "%-14s %-22s %10s %14s %14s\n", "SURFACE", "STATE", "RECORDS", "CACHE READ", "CACHE WRITE")
	for _, r := range readings {
		st := surfaces.Classify(r)
		records, reads, writes := fmt.Sprint(r.Records), fmt.Sprint(r.Reads), "NOT MEASURED"
		if r.WriteFieldPresent {
			writes = fmt.Sprint(r.Writes)
		}
		if st == surfaces.StateSilent {
			records, reads, writes = "0", "NOT MEASURED", "NOT MEASURED"
		}
		fmt.Fprintf(w, "%-14s %-22s %10s %14s %14s\n", r.Name, st, records, reads, writes)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "A cache read serves a prefix some earlier request wrote. A surface reporting reads")
	fmt.Fprintln(w, "and a zero write is reporting a field it does not have, as a number.")
}
