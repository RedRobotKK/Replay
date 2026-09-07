// Package analysis, source discovery.
//
// This is the other half of provenance.go. That file notices, after the fact,
// that every file a session read came from one directory. This one runs
// BEFORE the question is asked and says where the project's records actually
// live, so the narrow search never happens.
//
// The order matters. On 2026-09-07 the detector would have fired correctly and
// still been too late: the answer was already written. What would have changed
// the outcome is knowing, at the start, that a directory of sent-application
// records existed at all.
//
// The hazard in building this is obvious and worth naming. A list of sources
// implies completeness. If this scan misses a record, it does not merely fail
// to help, it licenses the same narrow search with more confidence than
// before. So the rules below are deliberately narrow and mechanical, and
// Render always states what it did not look at. A false all-clear is the
// failure direction that matters.
package analysis

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Why a directory was reported. Kept as a value the caller can print, because
// "this is a record" is a claim that should always come with its reason.
const (
	WhyNamed  = "named like a record"
	WhyAppend = "append-only"
	WhySeries = "dated series"
)

// recordWords are the whole-word tokens that make a document a record.
//
// Matched as tokens, never substrings. "logger" contains "log" and is not a
// log; "catalog_index" contains "index" and is not an index. Substring
// matching on these words turns every source tree into a wall of false
// positives, and a list nobody reads prevents nothing.
var recordWords = map[string]bool{
	"ledger": true, "registry": true, "index": true, "manifest": true,
	"history": true, "sent": true, "journal": true, "record": true,
	"records": true, "roster": true, "inventory": true, "log": true,
	"logs": true, "changelog": true, "audit": true, "ledgers": true,
}

// docExts limits every rule to documents.
//
// A record is something a person could cite. Program source is not, and
// admitting it is what makes the word list unusable: index.html and logger.go
// both match on the word alone. Requiring a document extension removes that
// entire class with one mechanical rule rather than a growing list of
// exceptions.
var docExts = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".csv": true, ".tsv": true,
	".json": true, ".jsonl": true, ".ndjson": true, ".yaml": true, ".yml": true,
	".org": true, ".rst": true,
}

// appendExts are shapes that are records regardless of their name, because
// they are append-only by construction: one event per line, never rewritten.
var appendExts = map[string]bool{".jsonl": true, ".ndjson": true}

// skipDirs are trees that hold other people's records, never the project's.
//
// Every one of these is disclosed by Render. An undisclosed skip is exactly
// the silent narrowing this file exists to prevent.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	"testdata": true,
	".git":     true, "target": true, "__pycache__": true, ".venv": true,
}

// minSeriesFiles is the fewest dated documents in one directory that make a
// series rather than a coincidence.
//
// Two files sharing a date shape happens by accident: a note and a meeting
// agenda. Three is where someone is keeping something on a cadence, and a
// cadence is a record even when no filename says so.
const minSeriesFiles = 3

var datedName = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// SourceDir is one directory holding records, and why it counts.
type SourceDir struct {
	Dir   string   // slash-separated, relative to the scan root
	Files []string // matching basenames, sorted
	Why   string   // WhyNamed, WhyAppend or WhySeries
}

// SourceMap is the result of a scan.
type SourceMap struct {
	Root    string
	Dirs    []SourceDir
	Scanned int // documents examined, so a zero result can be told from a failed walk
}

// ScanSources finds the directories under root that hold records.
//
// Unreadable directories are skipped rather than failing the scan, because a
// partial map is useful and an error here would leave the caller with nothing.
// The cost is that the map can be quietly short, which is the reason Render
// never claims to be complete.
func ScanSources(root string) (SourceMap, error) {
	m := SourceMap{Root: root}
	type acc struct {
		named  []string
		append []string
		dated  []string
	}
	byDir := map[string]*acc{}

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (skipDirs[name] || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		if !docExts[ext] {
			return nil
		}
		m.Scanned++
		rel, rerr := filepath.Rel(root, filepath.Dir(p))
		if rerr != nil {
			return nil
		}
		dir := filepath.ToSlash(rel)
		a := byDir[dir]
		if a == nil {
			a = &acc{}
			byDir[dir] = a
		}
		switch {
		case appendExts[ext]:
			a.append = append(a.append, name)
		case hasRecordWord(name):
			a.named = append(a.named, name)
		}
		if datedName.MatchString(name) {
			a.dated = append(a.dated, name)
		}
		return nil
	})
	if err != nil {
		return m, err
	}

	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, d := range dirs {
		a := byDir[d]
		// Order is deliberate. An explicitly named ledger is a stronger claim
		// than "these filenames carry dates", so it wins the label when both
		// hold, and the weakest rule is only reported when it is the only one.
		var files []string
		var why string
		switch {
		case len(a.named) > 0:
			files, why = a.named, WhyNamed
		case len(a.append) > 0:
			files, why = a.append, WhyAppend
		case len(a.dated) >= minSeriesFiles:
			files, why = a.dated, WhySeries
		default:
			continue
		}
		sort.Strings(files)
		m.Dirs = append(m.Dirs, SourceDir{Dir: d, Files: files, Why: why})
	}
	return m, nil
}

func hasRecordWord(name string) bool {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	for _, tok := range strings.FieldsFunc(strings.ToLower(base), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if recordWords[tok] {
			return true
		}
	}
	return false
}

// Render is the text to put in front of a model before it starts looking.
//
// It closes with what the scan did not do. That closing paragraph is not
// politeness, it is the point: a list of sources read without its limits is a
// claim of completeness, and this scan cannot make that claim.
func (m SourceMap) Render() string {
	var b strings.Builder
	if len(m.Dirs) == 0 {
		fmt.Fprintf(&b, "No record files found under %s (%d documents examined).\n", m.Root, m.Scanned)
	} else {
		fmt.Fprintf(&b, "Records under %s, %d directories from %d documents examined:\n\n",
			m.Root, len(m.Dirs), m.Scanned)
		for _, d := range m.Dirs {
			fmt.Fprintf(&b, "  %-34s %s\n", d.Dir+"/", d.Why)
			for _, f := range d.Files {
				fmt.Fprintf(&b, "  %-34s   %s\n", "", f)
			}
		}
		b.WriteString("\n")
	}
	skipped := make([]string, 0, len(skipDirs))
	for d := range skipDirs {
		skipped = append(skipped, d)
	}
	sort.Strings(skipped)
	fmt.Fprintf(&b,
		"This is not a complete list of where the answer might be. It matches "+
			"documents only, by filename, using three rules: a record word in the "+
			"name, an append-only extension, or %d+ dated files in one directory. "+
			"A record held in a database, in prose, under an unremarkable name, or "+
			"in a skipped tree (%s, and any dot-directory) does not appear above and "+
			"is not thereby absent.\n",
		minSeriesFiles, strings.Join(skipped, ", "))
	return b.String()
}
