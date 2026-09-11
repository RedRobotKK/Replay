package guardcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reviewer's own guards, which it never had.
//
// It carried //go:build ignore, so `go test ./...` could not reach a line of
// it. Every claim it made about neutralising a condition correctly was a claim
// in a comment, checked by nothing — which is the exact shape of the defect it
// exists to find in other people's code.

// writeGo lays down a source file and returns its path.
func writeGo(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "subject.go")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// neutralised returns the file's contents with the guard on `line` forced false.
func neutralised(t *testing.T, path string, line int) string {
	t.Helper()
	gs, err := Conditionals(path, map[int]bool{line: true})
	if err != nil {
		t.Fatalf("Conditionals: %v", err)
	}
	if len(gs) != 1 {
		t.Fatalf("expected one conditional on line %d, found %d", line, len(gs))
	}
	restore, err := Neutralise(gs[0])
	if err != nil {
		t.Fatalf("Neutralise: %v", err)
	}
	defer restore()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// parses reports whether the mutated source still compiles as Go.
func parses(t *testing.T, src string) bool {
	t.Helper()
	path := writeGo(t, src)
	return goParses(path)
}

// GC1: a conditional with an init statement survives neutralisation.
//
// `if x, ok := f(); ok {` is the single most common guard shape in this
// repository, and the reviewer could not check one. Prepending `if false &&`
// to the line produces `if false && (x, ok := f(); ok)`, which is not Go, so
// the mutant is stillborn and the verdict is "not checked" — reported once and
// then quietly excluded from the count. Two of the seven guards in the last
// pull request went unchecked this way.
func TestGC1_AnInitStatementIsNotStillborn(t *testing.T) {
	src := `package p

func f() (int, bool) { return 1, true }

func g() int {
	if x, ok := f(); ok {
		return x
	}
	return 0
}
`
	got := neutralised(t, writeGo(t, src), 6)
	if !parses(t, got) {
		t.Errorf("neutralising an init-statement conditional produced source that is "+
			"not Go, so the mutant is stillborn and the guard goes unchecked:\n%s", got)
	}
}

// GC2: a condition spanning more than one line is neutralised whole.
//
// The line-based rewrite reads only the first line, so the clauses below it
// stay live — the same defect the `||` parenthesising fixed, in a different
// shape and still present.
func TestGC2_AMultiLineConditionIsNeutralisedWhole(t *testing.T) {
	src := `package p

func g(a, b bool) int {
	if a ||
		b {
		return 1
	}
	return 0
}
`
	got := neutralised(t, writeGo(t, src), 4)
	if !parses(t, got) {
		t.Fatalf("the mutant does not parse:\n%s", got)
	}
	// The whole condition must sit inside the forced-false expression. If only
	// the first line was rewritten, `b` is still a live top-level clause.
	if strings.Contains(got, "false && (a ||\n") && !strings.Contains(got, "b)") {
		t.Errorf("only the first line of the condition was neutralised:\n%s", got)
	}
	if !strings.Contains(strings.Join(strings.Fields(got), " "), "false && (a || b)") {
		t.Errorf("the condition was not neutralised as a whole:\n%s", got)
	}
}

// GC3: a `||` guard is neutralised in every clause, not just the first.
//
// Frozen: Go binds && tighter than ||, so `if false && A || B` disables only A.
// The tool reported SURVIVED for guards it had never actually neutralised.
func TestGC3_EveryClauseOfAnOrIsNeutralised(t *testing.T) {
	src := `package p

func g(a, b bool) int {
	if a || b {
		return 1
	}
	return 0
}
`
	got := neutralised(t, writeGo(t, src), 4)
	if !strings.Contains(got, "false && (a || b)") {
		t.Errorf("the || was not parenthesised, so only its first clause was "+
			"disabled:\n%s", got)
	}
}

// GC4: the original file comes back exactly.
//
// The reviewer mutates the working tree of a real checkout. A restore that
// drifts by a byte leaves someone's repository quietly modified.
func TestGC4_RestorePutsTheFileBackExactly(t *testing.T) {
	src := `package p

func g(a bool) int {
	if a {
		return 1
	}
	return 0
}
`
	path := writeGo(t, src)
	gs, err := Conditionals(path, map[int]bool{4: true})
	if err != nil || len(gs) != 1 {
		t.Fatalf("Conditionals: %v (%d found)", err, len(gs))
	}
	restore, err := Neutralise(gs[0])
	if err != nil {
		t.Fatal(err)
	}
	restore()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != src {
		t.Errorf("restore did not put the file back:\n--- want ---\n%s\n--- got ---\n%s", src, b)
	}
}
