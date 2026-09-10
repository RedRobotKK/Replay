package guardcheck

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
)

// Coverage is a parsed Go cover profile, read to answer one question: did any
// test ever enter this guard's body?
//
// The answer splits a verdict that used to be one word. A guard survives
// neutralisation either because no test makes its condition true — nothing
// exercises the branch, and the fix is a test — or because the branch does run
// and nothing depends on the difference, in which case the statement is
// redundant with the code below it and the fix is to delete it.
//
// Writing a test to keep dead code is the wrong move, and telling the two
// apart by hand cost three separate diagnoses on a single pull request.
type Coverage struct {
	// blocks are keyed by file base name, since a profile names files by
	// import path and a Guard names them relative to the repository root.
	blocks map[string][]coverBlock

	// Unparsed counts content lines this parser did not understand.
	//
	// It exists because the profile format is an assumption, and an
	// assumption that goes stale silently is the failure this repository
	// names record lag (FD-9). If Go changes how it writes a profile, every
	// block stops matching, every survivor is reported UNOBSERVED, and the
	// reviewer degrades into saying "NOT MEASURED" about a suite it could
	// measure perfectly well the week before. A count makes that visible
	// instead of quiet.
	Unparsed int
}

// coverBlock is one counted region of a cover profile.
type coverBlock struct {
	file                string // the profile's own path, for suffix matching
	startLine, startCol int
	endLine, endCol     int
	count               int
}

// ParseCoverage reads a profile written by `go test -coverprofile`.
func ParseCoverage(profile string) (*Coverage, error) {
	f, err := os.Open(profile)
	if err != nil {
		// Named, not passed through bare. Scanning a nil file yields
		// ErrInvalid, so a missing profile ends in an error either way — but
		// "invalid argument" describes a programming mistake, and the truth
		// is that a file is not there.
		return nil, fmt.Errorf("reading %s: %w", profile, err)
	}
	defer func() { _ = f.Close() }()

	c := &Coverage{blocks: map[string][]coverBlock{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		b, ok := parseCoverLine(line)
		if !ok {
			// A line this parser does not understand is skipped rather than
			// guessed at: a misparsed block would answer "never taken" for a
			// branch the suite covers, and send someone to write a test they
			// already have. It is counted, so that a format change shows up
			// as a number rather than as a quiet loss of every verdict.
			c.Unparsed++
			continue
		}
		key := path.Base(b.file)
		c.blocks[key] = append(c.blocks[key], b)
	}
	return c, sc.Err()
}

// parseCoverLine reads `import/path/file.go:l.c,l.c stmts count`.
func parseCoverLine(line string) (coverBlock, bool) {
	colon := strings.LastIndexByte(line, ':')
	if colon < 0 {
		return coverBlock{}, false
	}
	b := coverBlock{file: line[:colon]}

	fields := strings.Fields(line[colon+1:])
	if len(fields) != 3 {
		return coverBlock{}, false
	}
	span := strings.Split(fields[0], ",")
	if len(span) != 2 {
		return coverBlock{}, false
	}
	var err error
	if b.startLine, b.startCol, err = parsePos(span[0]); err != nil {
		return coverBlock{}, false
	}
	if b.endLine, b.endCol, err = parsePos(span[1]); err != nil {
		return coverBlock{}, false
	}
	if b.count, err = strconv.Atoi(fields[2]); err != nil {
		return coverBlock{}, false
	}
	return b, true
}

// parsePos reads a `line.column` pair.
func parsePos(s string) (int, int, error) {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		return 0, 0, fmt.Errorf("no line.column in %q", s)
	}
	l, err := strconv.Atoi(s[:dot])
	if err != nil {
		return 0, 0, err
	}
	col, err := strconv.Atoi(s[dot+1:])
	if err != nil {
		return 0, 0, err
	}
	return l, col, nil
}

// BranchTaken reports whether any test entered the guard's body.
//
// The second return is whether coverage knows at all. Absence, zero and
// unknown are three values (ADR-0018): a guard the profile carries no block
// for must not be reported as an unentered branch, or a parsing miss becomes a
// finding and the reviewer sends someone to test a branch their suite already
// covers.
func (c *Coverage) BranchTaken(g Guard) (taken bool, known bool) {
	if c == nil || g.BodyStart.Line == 0 {
		return false, false
	}
	// Blocks are keyed by base name because the profile names files by import
	// path and a Guard names them relative to the repository root: the two
	// agree on the file name and on nothing else. The lookup below therefore
	// already establishes the match — a second name comparison here could
	// never be false, and said so when this tool was pointed at itself.
	for _, b := range c.blocks[path.Base(g.File)] {
		if !within(b, g) {
			continue
		}
		// Several blocks can sit inside one body; the branch was entered if
		// any of them ran.
		if b.count > 0 {
			return true, true
		}
		known = true
	}
	return false, known
}

// within reports whether a cover block lies inside the guard's body braces.
//
// Go opens the block just after `{` and closes it at `}`, so the comparison is
// on positions, not lines: a one-line `if x { return }` has a body whose start
// and end share a line with the condition.
func within(b coverBlock, g Guard) bool {
	after := b.startLine > g.BodyStart.Line ||
		(b.startLine == g.BodyStart.Line && b.startCol >= g.BodyStart.Column)
	before := b.endLine < g.BodyEnd.Line ||
		(b.endLine == g.BodyEnd.Line && b.endCol <= g.BodyEnd.Column+1)
	return after && before
}
