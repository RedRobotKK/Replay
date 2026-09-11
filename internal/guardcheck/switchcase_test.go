package guardcheck

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"
)

// A tagless switch case is a conditional, and the reviewer could not see one.
//
// Conditionals matched *ast.IfStmt and nothing else. This tree holds 91 tagless
// `switch {` statements across 66 files, and every `case cond:` inside them was
// a guard that reached neither survived nor unchecked — it was not seen, and
// the pass still printed a clean line.
//
// It is the same shape as the platform blind spot (#189) and larger. Four of
// this repository's refusals are written as tagless switch cases, which is why
// scripts/refusal-reachability had to grow its own AST neutraliser rather than
// reuse this package: the shape it needed was missing here.
//
// PASS: each case expression is a guard, neutralisable in place.
// FAIL: they are invisible, and a switch-shaped refusal ships unreviewed.

const switchSubject = `package p

func classify(n int, s string) string {
	switch {
	case n < 0:
		return "negative"
	case s == "":
		return "empty"
	default:
		return "ok"
	}
}
`

// SW1: every case expression of a tagless switch is collected.
func TestSW1_TaglessSwitchCasesAreConditionals(t *testing.T) {
	path := writeGo(t, switchSubject)
	// Lines 4-9 cover the switch and both cases.
	lines := map[int]bool{}
	for i := 1; i <= 12; i++ {
		lines[i] = true
	}
	gs, err := Conditionals(path, lines)
	if err != nil {
		t.Fatalf("Conditionals: %v", err)
	}
	var got []string
	for _, g := range gs {
		got = append(got, g.Src)
	}
	if len(gs) < 2 {
		t.Fatalf("a tagless switch with two cases yielded %d conditional(s): %v.\n"+
			"Every case expression is a guard; missing them means a switch-shaped "+
			"refusal is never scored and the pass still reports a clean line", len(gs), got)
	}
}

// SW2: a neutralised case compiles and is forced false.
//
// `case false && (n < 0):` keeps the expression compiled, so a variable only it
// uses does not go unused and turn the mutant stillborn for the wrong reason.
func TestSW2_ANeutralisedCaseStillCompiles(t *testing.T) {
	path := writeGo(t, switchSubject)
	lines := map[int]bool{}
	for i := 1; i <= 12; i++ {
		lines[i] = true
	}
	gs, err := Conditionals(path, lines)
	if err != nil || len(gs) == 0 {
		t.Fatalf("Conditionals: %v (%d found)", err, len(gs))
	}
	// The first case expression.
	var target Guard
	for _, g := range gs {
		if strings.Contains(g.Src, "case n < 0") {
			target = g
			break
		}
	}
	if target.File == "" {
		t.Skipf("no case guard collected; SW1 covers that")
	}

	restore, err := Neutralise(target)
	if err != nil {
		t.Fatalf("Neutralise: %v", err)
	}
	defer restore()

	if !goParses(path) {
		b := readFile(t, path)
		t.Errorf("neutralising a switch case produced source that is not Go, so the "+
			"mutant is stillborn and the guard goes unchecked:\n%s", b)
	}
	if b := readFile(t, path); !strings.Contains(b, "false && (n < 0)") {
		t.Errorf("the case expression was not forced false:\n%s", b)
	}
}

// SW3: a TAGGED switch is not touched.
//
// `switch x { case 1: }` compares values. Rewriting that to
// `case false && (1):` is not a comparison, does not compile, and would be a
// stillborn mutant of this tool's own making — the score inflation it exists
// to prevent, produced by the fix for a different blind spot.
func TestSW3_ATaggedSwitchIsNotAConditional(t *testing.T) {
	path := writeGo(t, `package p

func pick(n int) string {
	switch n {
	case 1:
		return "one"
	case 2:
		return "two"
	}
	return "other"
}
`)
	lines := map[int]bool{}
	for i := 1; i <= 12; i++ {
		lines[i] = true
	}
	gs, err := Conditionals(path, lines)
	if err != nil {
		t.Fatalf("Conditionals: %v", err)
	}
	for _, g := range gs {
		if strings.Contains(g.Src, "case 1") || strings.Contains(g.Src, "case 2") {
			t.Errorf("a tagged switch's case was collected as a conditional (%q). "+
				"Forcing it false does not compile, so this tool would report its own "+
				"mutant as unchecked", g.Src)
		}
	}
}

// SW4: `default:` has no condition and is not collected.
func TestSW4_DefaultIsNotAConditional(t *testing.T) {
	path := writeGo(t, switchSubject)
	lines := map[int]bool{}
	for i := 1; i <= 12; i++ {
		lines[i] = true
	}
	gs, err := Conditionals(path, lines)
	if err != nil {
		t.Fatalf("Conditionals: %v", err)
	}
	for _, g := range gs {
		if strings.Contains(g.Src, "default:") {
			t.Errorf("default: was collected as a conditional (%q); it has no "+
				"expression to force false", g.Src)
		}
	}
}

// SW5: the walk's own refusals, entered directly.
//
// go/parser only ever puts *ast.CaseClause in a switch body, so inside the
// walk these branches cannot be reached through any file this tool is pointed
// at — the reviewer reported them as branches nothing enters, correctly. They
// are reachable here because an AST can be built rather than parsed.
func TestSW5_TheSwitchWalkRefusesWhatItShould(t *testing.T) {
	cond := &ast.BinaryExpr{X: ast.NewIdent("n"), Op: token.LSS, Y: ast.NewIdent("m")}

	t.Run("a tagged switch yields nothing", func(t *testing.T) {
		sw := &ast.SwitchStmt{
			Tag:  ast.NewIdent("x"),
			Body: &ast.BlockStmt{List: []ast.Stmt{&ast.CaseClause{List: []ast.Expr{cond}}}},
		}
		if got := taglessCases(sw); len(got) != 0 {
			t.Errorf("a tagged switch yielded %d case(s); forcing one false does not "+
				"compile, so this tool would author its own unchecked mutant", len(got))
		}
	})

	t.Run("a switch with no body yields nothing", func(t *testing.T) {
		if got := taglessCases(&ast.SwitchStmt{}); len(got) != 0 {
			t.Errorf("a bodyless switch yielded %d case(s)", len(got))
		}
	})

	t.Run("a non-case statement in the body is skipped", func(t *testing.T) {
		sw := &ast.SwitchStmt{Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ExprStmt{X: ast.NewIdent("nonsense")},
			&ast.CaseClause{List: []ast.Expr{cond}},
		}}}
		got := taglessCases(sw)
		if len(got) != 1 {
			t.Errorf("expected the one real case clause, got %d", len(got))
		}
	})

	t.Run("default is returned and contributes no condition", func(t *testing.T) {
		sw := &ast.SwitchStmt{Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.CaseClause{}, // default:
		}}}
		got := taglessCases(sw)
		if len(got) != 1 || len(got[0].List) != 0 {
			t.Errorf("default: should come back with an empty condition list, got %+v", got)
		}
	})
}

// SW6: a switch case outside the diff is not collected.
//
// The reviewer is diff-scoped so it costs seconds rather than the eleven
// minutes the full catalogue needs. A collector that ignored the line set
// would put every case in the file to the suite on every run — and the switch
// path reaches that check through its own closure, so GC17 covering the `if`
// path says nothing about this one.
func TestSW6_OnlyChangedSwitchCasesAreCollected(t *testing.T) {
	path := writeGo(t, switchSubject)

	// Line 5 is `case n < 0:`; line 7 is `case s == "":`.
	gs, err := Conditionals(path, map[int]bool{5: true})
	if err != nil {
		t.Fatalf("Conditionals: %v", err)
	}
	for _, g := range gs {
		if strings.Contains(g.Src, `case s == ""`) {
			t.Errorf("a case outside the changed line set was collected (%q at line %d); "+
				"the whole file would be put to the suite on every run", g.Src, g.Line)
		}
	}
	var found bool
	for _, g := range gs {
		if strings.Contains(g.Src, "case n < 0") {
			found = true
		}
	}
	if !found {
		t.Error("the case ON the changed line was not collected, so this test would " +
			"pass over a collector that returns nothing at all")
	}
}
