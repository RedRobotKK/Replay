// Package analysis, source provenance.
//
// This file answers one question about a session: did the evidence come from
// more than one place?
//
// It exists because of a failure on 2026-09-07. A question of the form "what
// was actually sent" was answered by reading five files, every one of them
// under a single directory, and the answer was confidently wrong: the
// authoritative record lived in a different tree entirely. All five sources
// agreed. The agreement carried no information, because the sources shared an
// origin.
//
// The warning here does not judge a conclusion. It states a fact about the
// search that produced it, and leaves the judgement to the reader. That is
// deliberate: a tool that guessed which conclusions were wrong would be wrong
// itself, often, and would be ignored within a week.
package analysis

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// minPathsForProvenance is the fewest distinct files that can say anything
// about search breadth.
//
// One or two reads are not a search. Warning on them would fire the signal
// constantly, and a warning that fires constantly is furniture. Three is the
// smallest count where "all from one place" is a choice rather than an
// accident of the question being small.
const minPathsForProvenance = 3

// Provenance is where a session's evidence came from.
type Provenance struct {
	// Paths is the number of DISTINCT files read. A file read five times is
	// one source; counting reads would let a loop look like a broad search.
	Paths int
	// Dirs is the number of distinct directories those files sit in.
	Dirs int
	// TopDir is the directory holding the most of them, and TopDirPaths how
	// many. Named so the warning can point somewhere.
	TopDir      string
	TopDirPaths int
	// Concentration is TopDirPaths over Paths, so 1.0 means every file came
	// from one directory.
	Concentration float64
}

// ReadProvenance summarises where a set of read paths came from.
//
// Order does not matter and duplicates are collapsed. Paths are taken as
// given: this does not touch the filesystem, because it runs against
// transcripts of sessions that may have happened on another machine.
func ReadProvenance(paths []string) Provenance {
	seen := map[string]bool{}
	byDir := map[string]int{}
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		byDir[path.Dir(p)]++
	}
	pv := Provenance{Paths: len(seen), Dirs: len(byDir)}
	if pv.Paths == 0 {
		return pv
	}
	// Sorted so the top directory is stable when two tie, rather than
	// whichever key Go's map iteration happens to yield first.
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, d := range dirs {
		if byDir[d] > pv.TopDirPaths {
			pv.TopDir, pv.TopDirPaths = d, byDir[d]
		}
	}
	pv.Concentration = float64(pv.TopDirPaths) / float64(pv.Paths)
	return pv
}

// Warning is what to tell a reader, or empty when there is nothing to say.
//
// It fires only when every file came from one directory. A merely lopsided
// search is not reported: a session working inside one package legitimately
// reads mostly that package, and this cannot know which case it is looking at.
// Total concentration is the one shape where "my sources agree" is guaranteed
// to carry no information about completeness.
func (p Provenance) Warning() string {
	if p.Paths < minPathsForProvenance || p.Dirs != 1 {
		return ""
	}
	return fmt.Sprintf(
		"all %d files read came from %s. Sources that share a directory agree by "+
			"construction, so their agreement says nothing about whether the answer is "+
			"complete. If this question had an authoritative record elsewhere, this "+
			"search could not have found it.",
		p.Paths, p.TopDir)
}

// pathTools are the tools whose label carries a file path as its argument.
//
// Bash is deliberately absent. Its label carries a command line, and a command
// line mentions paths in too many shapes to parse without guessing. A guessed
// path here would inflate the directory count and silence the warning, which is
// the failure direction that matters: a false all-clear is worse than no
// reading at all.
var pathTools = map[string]bool{
	"Read": true, "Edit": true, "Write": true, "NotebookEdit": true,
}

// SessionReadPaths pulls the file paths a session read, from its tool labels.
//
// Labels are used rather than tool inputs because the parser keeps the label
// and deliberately does not retain raw inputs. That is a privacy decision and
// this works within it: the path is the argument, and the argument is in the
// label.
func SessionReadPaths(s *transcript.Session) []string {
	if s == nil {
		return nil
	}
	var out []string
	for _, lane := range s.Lanes {
		for _, r := range lane.Requests {
			for _, m := range r.Context {
				out = append(out, pathsFromBlocks(m.Blocks)...)
			}
			if r.Output != nil {
				out = append(out, pathsFromBlocks(r.Output.Blocks)...)
			}
		}
	}
	return out
}

func pathsFromBlocks(blocks []transcript.Block) []string {
	var out []string
	for _, b := range blocks {
		if !pathTools[b.ToolName] {
			continue
		}
		label := b.Label
		for _, p := range []string{transcript.LabelToolResultPrefix, transcript.LabelToolCallPrefix} {
			label = strings.TrimPrefix(label, p)
		}
		// What remains is "<Tool> <argument>". Anything without an argument is
		// a call whose input the parser did not record, and is skipped rather
		// than counted as a read of nothing.
		name, arg, ok := strings.Cut(label, " ")
		if !ok || !pathTools[name] || strings.TrimSpace(arg) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(arg))
	}
	return out
}
