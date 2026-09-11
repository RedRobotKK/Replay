package guardcheck_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/guardcheck"
)

// TestGC6_NoNeutralisedConditionalIsCommitted refuses the reviewer's own
// scratch work in the tree.
//
// guard-reachability neutralises a conditional by rewriting it as
// `if false && (<cond>)`, runs the suite, and puts the file back. When it is
// interrupted — a killed process, a machine that went to sleep, a terminal
// closed — it puts nothing back, and what is left compiles, looks almost
// right, and disables a guard.
//
// This has reached a commit twice. Both times the mutation was the same one,
// on the same line, and both times it turned a bounded scan into an infinite
// loop rather than a wrong answer: with the guard off, a malformed object key
// returns an index of zero, the walk restarts from the beginning of the input
// and never ends. The first time it was caught by a test timing out at ten
// minutes. The second time it was pushed.
//
// `git status` cannot catch this. The file is legitimately modified on a
// branch that changes it, so the leftover hides inside the diff the author
// means to commit.
//
// A grep cannot catch it either: five files in this repo carry the literal
// text `if false && ` inside comments and string literals, because they
// document the very form this looks for. So this walks the AST and looks for
// the expression, which a comment cannot produce.
func TestGC6_NoNeutralisedConditionalIsCommitted(t *testing.T) {
	// The reviewer neutralises a guard and then runs this suite. Failing
	// here on its own scratch work would fail every one of its runs, so
	// every mutant would look caught and its verdict would mean nothing.
	// It says so by setting this; nothing else does, and the baseline run
	// does not, so the tree the reviewer starts from is still guarded.
	if os.Getenv(guardcheck.NeutralisingEnv) != "" {
		t.Skip("guard-reachability is neutralising a conditional right now; " +
			"it restores the file when it is done")
	}
	root := repoRoot(t)
	fset := token.NewFileSet()

	var found []string
	walked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			// A file that does not parse is not this test's business; the
			// build says so more clearly.
			return nil
		}
		walked++
		rel, _ := filepath.Rel(root, path)
		ast.Inspect(f, func(n ast.Node) bool {
			for _, cond := range conditionsOf(n) {
				if lit, side := constantShortCircuit(cond); lit != "" {
					found = append(found, rel+":"+
						ffline(fset, cond.Pos())+": "+side+" is always "+lit)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// A walk that found nothing because it visited nothing would pass this
	// test while checking no file at all.
	if walked < 50 {
		t.Fatalf("walked %d Go files under %s, expected the whole repo: this test is not looking at anything", walked, root)
	}
	if len(found) > 0 {
		t.Fatalf("neutralised conditional(s) left in the tree — guard-reachability was interrupted "+
			"and did not restore the file:\n  %s\n\nRestore with `git checkout -- <file>`, or remove the "+
			"`false &&` by hand. Do not commit it: the guard it disables is not decoration.",
			strings.Join(found, "\n  "))
	}
	t.Logf("%d Go files, no neutralised conditional", walked)
}

// conditionsOf returns every expression a node decides a branch on.
//
// The case clauses matter as much as the if statements, and missing them is
// not hypothetical: a `case false && (...)` in a tagless switch is exactly
// the form the reviewer writes for a switch arm, and the first version of
// this test walked only if/for/switch conditions. A fourth leftover went
// through it unnoticed, in the same file as the other three.
func conditionsOf(n ast.Node) []ast.Expr {
	switch s := n.(type) {
	case *ast.IfStmt:
		return []ast.Expr{s.Cond}
	case *ast.ForStmt:
		return []ast.Expr{s.Cond}
	case *ast.SwitchStmt:
		return []ast.Expr{s.Tag}
	case *ast.CaseClause:
		// A tagless switch decides on each case expression, and that is
		// where a neutralised switch arm lands.
		return s.List
	}
	return nil
}

// constantShortCircuit reports whether expr is an && or || whose outcome one
// operand already decides: `false && x` can never run x's branch, and
// `true || x` always does. Those are the two forms the reviewer writes.
//
// It deliberately does not flag `x && false` or `x || true`, which nobody
// writes by accident and the reviewer never writes at all.
func constantShortCircuit(expr ast.Expr) (lit, side string) {
	b, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return "", ""
	}
	id, ok := b.X.(*ast.Ident)
	if !ok {
		return "", ""
	}
	switch {
	case b.Op == token.LAND && id.Name == "false":
		return "false", "the left operand of &&"
	case b.Op == token.LOR && id.Name == "true":
		return "true", "the left operand of ||"
	}
	return "", ""
}

func ffline(fset *token.FileSet, p token.Pos) string {
	pos := fset.Position(p)
	return itoa(pos.Line)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// repoRoot walks up from the test's directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("no go.mod above the test directory")
	return ""
}
