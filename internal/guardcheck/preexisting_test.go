package guardcheck

import (
	"os"
	"path/filepath"
	"testing"
)

// The reviewer is diff-scoped by line, and a move has no lines of its own.
//
// Splitting one file into eight presents every relocated line as added, so the
// reviewer re-analyses conditions that have been in the tree for months and
// reports their long-standing holes as if the split had dug them. Measured on
// #210: 136 guards and 42 survivors against origin/main on 2026-09-11, where
// the diagnosis in #232 recorded 125 and 39 against an earlier main. The 94.5%
// coverage on both sides and the same 27 zero-count blocks before and after
// under different file names are that diagnosis's figures, not this file's.
//
// These tests pin the part that can be checked without running a suite: what
// makes two conditionals the same guard, and what happens when the answer is
// ambiguous.

// writeGoIn lays down a source file in a named directory, so a test can put two
// files in one package.
func writeGoIn(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// only returns the single conditional on a line, or fails.
func only(t *testing.T, path string, line int) Guard {
	t.Helper()
	gs, err := Conditionals(path, map[int]bool{line: true})
	if err != nil {
		t.Fatalf("Conditionals(%s): %v", path, err)
	}
	if len(gs) != 1 {
		t.Fatalf("expected one conditional on %s:%d, found %d", path, line, len(gs))
	}
	return gs[0]
}

// PX1: a condition that moved to another file of the same package is the same
// guard.
//
// This is the whole defect. The reviewer's identity for a guard was file and
// line, both of which a move changes, so every relocated conditional read as
// new.
func TestPX1_AMovedConditionKeepsItsIdentity(t *testing.T) {
	dir := t.TempDir()
	before := writeGoIn(t, dir, "server.go", `package p

func handle(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
`)
	// The same function, verbatim, in the file the split moved it to.
	after := writeGoIn(t, dir, "lifecycle.go", `package p

// A comment that was not there before, so the line number moves too.
func handle(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
`)
	old, moved := only(t, before, 4), only(t, after, 5)
	if old.File == moved.File || old.Line == moved.Line {
		t.Fatalf("the fixture does not move the guard: %s:%d vs %s:%d",
			old.File, old.Line, moved.File, moved.Line)
	}
	if old.Identity() != moved.Identity() {
		t.Errorf("a move changed the guard's identity, so the reviewer reports a "+
			"months-old hole as introduced:\n  before %+v\n  after  %+v",
			old.Identity(), moved.Identity())
	}
}

// PX2: the same condition text in another function is not the same guard.
//
// `if err != nil` is written hundreds of times in this repository. An identity
// of condition text alone would let any one of them certify any other, which
// turns the whole exemption into "pass everything".
func TestPX2_TheSameTextInAnotherFunctionIsNotTheSameGuard(t *testing.T) {
	dir := t.TempDir()
	path := writeGoIn(t, dir, "two.go", `package p

func a(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func b(n int) int {
	if n < 0 {
		return 1
	}
	return n
}
`)
	if only(t, path, 4).Identity() == only(t, path, 11).Identity() {
		t.Error("two functions' guards share an identity on identical text, so either " +
			"can certify the other as pre-existing")
	}
}

// PX3: reformatting a condition does not change its identity.
//
// A split is usually accompanied by gofmt, and a condition that was wrapped
// differently must not read as a new guard.
func TestPX3_ReformattingDoesNotChangeIdentity(t *testing.T) {
	dir := t.TempDir()
	one := writeGoIn(t, dir, "one.go", `package p

func f(a, b bool) int {
	if a || b {
		return 1
	}
	return 0
}
`)
	wrapped := writeGoIn(t, dir, "wrapped.go", `package p

func f(a, b bool) int {
	if a ||
		b {
		return 1
	}
	return 0
}
`)
	if only(t, one, 4).Identity() != only(t, wrapped, 4).Identity() {
		t.Error("wrapping a condition over two lines changed its identity")
	}
}

// PX4: a method's identity carries its receiver type.
//
// Two types in one package routinely have a Close, a Write, or a Start with the
// same guard at the top of it. Without the receiver, one certifies the other.
func TestPX4_AReceiverTypeIsPartOfTheIdentity(t *testing.T) {
	dir := t.TempDir()
	path := writeGoIn(t, dir, "methods.go", `package p

type A struct{ n int }
type B struct{ n int }

func (a *A) run() int {
	if a.n < 0 {
		return 0
	}
	return a.n
}

func (b *B) run() int {
	if b.n < 0 {
		return 1
	}
	return b.n
}
`)
	ga, gb := only(t, path, 7), only(t, path, 14)
	if ga.Func == "" || gb.Func == "" {
		t.Fatalf("a method's guard has no function name: %q / %q", ga.Func, gb.Func)
	}
	if ga.Identity() == gb.Identity() {
		t.Errorf("run on two receivers shares one identity (%q)", ga.Func)
	}
}

// PX5: identical guards are paired by count, not by name.
//
// Nothing can say which of three identical `if err != nil` in one function is
// which. What can be said is how many of them survived before and how many
// survive now, and that the excess is the change's doing. The excess must fail:
// a version that passed the whole identity because one of its members was
// pre-existing would hide a new hole behind an old one.
func TestPX5_ExcessSurvivorsOfOneIdentityAreIntroduced(t *testing.T) {
	dir := t.TempDir()
	path := writeGoIn(t, dir, "dupes.go", `package p

func f(err error, n int) int {
	if err != nil {
		return 1
	}
	if err != nil {
		return 2
	}
	return n
}
`)
	first, second := only(t, path, 4), only(t, path, 7)
	if first.Identity() != second.Identity() {
		t.Fatal("the fixture's two guards were meant to be indistinguishable")
	}
	pre, introduced := PairSurvivors([]Guard{first, second},
		map[Identity]int{first.Identity(): 1})
	if len(pre) != 1 || len(introduced) != 1 {
		t.Fatalf("two survivors against one in the base gave %d pre-existing and %d "+
			"introduced, want 1 and 1", len(pre), len(introduced))
	}
	if introduced[0].Line != second.Line {
		t.Errorf("the pairing consumed the base survivor out of order: introduced %d, want %d",
			introduced[0].Line, second.Line)
	}
}

// PX6: a survivor the base tree has no counterpart for still fails.
//
// This is the half that matters. A reviewer that stops failing is worse than
// one that fails noisily, so the exemption has to be earned per guard and the
// empty case has to be the failing one.
func TestPX6_ASurvivorWithNoBaseCounterpartIsIntroduced(t *testing.T) {
	dir := t.TempDir()
	path := writeGoIn(t, dir, "new.go", `package p

func f(n int) int {
	if n == 7 {
		return 0
	}
	return n
}
`)
	g := only(t, path, 4)
	for name, base := range map[string]map[Identity]int{
		"nothing known about the base": nil,
		"the base knows other guards":  {{Pkg: "./elsewhere", Func: "f", Cond: "n == 7"}: 9},
		"the identity is exhausted":    {g.Identity(): 0},
	} {
		pre, introduced := PairSurvivors([]Guard{g}, base)
		if len(pre) != 0 || len(introduced) != 1 {
			t.Errorf("%s: %d pre-existing and %d introduced, want 0 and 1",
				name, len(pre), len(introduced))
		}
	}
}

// PX7: with no line filter, every conditional in a file is collected.
//
// The base side has no diff to scope by — the whole point is that the guard is
// somewhere else in it — so the index is built over entire files.
func TestPX7_ANilLineFilterCollectsEveryConditional(t *testing.T) {
	dir := t.TempDir()
	path := writeGoIn(t, dir, "all.go", `package p

func f(a, b bool) int {
	if a {
		return 1
	}
	switch {
	case b:
		return 2
	}
	if a && b {
		return 3
	}
	return 0
}
`)
	gs, err := Conditionals(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 3 {
		t.Fatalf("a nil line filter collected %d conditionals, want 3", len(gs))
	}
	// And an empty filter still collects nothing, or the diff scoping that
	// makes this reviewer cost seconds is gone.
	gs, err = Conditionals(path, map[int]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 0 {
		t.Fatalf("an empty line filter collected %d conditionals, want 0", len(gs))
	}
}

// PX8: the per-identity count is what bounds the base search.
//
// The reviewer neutralises base candidates until it has found as many survivors
// as the change has; the count is that bound, and an undercount would exempt a
// guard nothing measured.
func TestPX8_SurvivorsAreCountedByIdentity(t *testing.T) {
	dir := t.TempDir()
	path := writeGoIn(t, dir, "count.go", `package p

func f(err error, n int) int {
	if err != nil {
		return 1
	}
	if err != nil {
		return 2
	}
	if n < 0 {
		return 3
	}
	return n
}
`)
	first, second, other := only(t, path, 4), only(t, path, 7), only(t, path, 10)
	got := CountByIdentity([]Guard{first, second, other})
	if got[first.Identity()] != 2 {
		t.Errorf("the duplicated identity counted %d, want 2", got[first.Identity()])
	}
	if got[other.Identity()] != 1 {
		t.Errorf("the lone identity counted %d, want 1", got[other.Identity()])
	}
	if len(got) != 2 {
		t.Errorf("counted %d identities, want 2", len(got))
	}
}

// PX10: a generic receiver names its type, not its instantiation.
//
// `func (s *Store[T]) put` and `func (s *Store[K, V]) put` are written on the
// type, and the type parameter is not part of what distinguishes one method
// from another. Rendering the whole instantiation would make the identity
// depend on how the parameter happened to be spelled.
func TestPX10_AGenericReceiverNamesItsType(t *testing.T) {
	dir := t.TempDir()
	path := writeGoIn(t, dir, "generic.go", `package p

type Store[T any] struct{ n int }
type Pair[K any, V any] struct{ n int }

func (s *Store[T]) put(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func (p Pair[K, V]) put(n int) int {
	if n < 0 {
		return 1
	}
	return n
}
`)
	one, two := only(t, path, 7), only(t, path, 14)
	if one.Func != "(*Store).put" {
		t.Errorf("Func = %q, want %q", one.Func, "(*Store).put")
	}
	if two.Func != "(Pair).put" {
		t.Errorf("Func = %q, want %q", two.Func, "(Pair).put")
	}
	if one.Identity() == two.Identity() {
		t.Error("two generic types' methods share one identity")
	}
}

// PX9: a conditional outside any function still has an identity.
//
// Nothing in this repository puts an `if` at file scope, but a func literal
// assigned to a package variable does, and a guard whose Func is empty must not
// collide with every other guard whose Func is empty on the same text.
func TestPX9_AGuardOutsideAFunctionIsNotAnonymous(t *testing.T) {
	dir := t.TempDir()
	path := writeGoIn(t, dir, "varfunc.go", `package p

var f = func(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
`)
	g := only(t, path, 4)
	if g.Cond != "n < 0" {
		t.Errorf("Cond = %q, want the condition text", g.Cond)
	}
	// It has no enclosing FuncDecl, so Func is empty — and the identity then
	// rests on package and text alone. Pinned so the weakness is visible
	// rather than discovered later: two package-level func literals in one
	// package with the same guard are indistinguishable to this.
	if g.Func != "" {
		t.Errorf("Func = %q for a guard inside a package-level func literal, want \"\"", g.Func)
	}
}
