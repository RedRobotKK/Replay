//go:build ignore

// refusal-reachability neutralises the guard in front of each refusal and
// reports the refusals no test can tell apart from any other outcome.
//
// A refusal is where this tool declines to answer: it prints NOT MEASURED,
// returns an error rather than a guess, or says a figure cannot be computed.
// Every one of them is a promise made to a reader. A refusal nothing exercises
// is an unchecked promise, and a refusal whose guard can be disabled with the
// suite still green is worse — the promise is not merely unchecked, the suite
// actively vouches for it.
//
// This is the sibling of scripts/guard-reachability/main.go and shares its
// neutralisation: `if false && (<cond>)`, parenthesised, because Go binds &&
// tighter than || and the unparenthesised form disables only the first clause.
// The difference is selection. guard-reachability is scoped to a diff and asks
// "is this NEW conditional observed". This one is scoped to the refusal
// vocabulary and asks "is this STANDING refusal distinguishable", which is a
// question about the whole tree and answerable in one run because there are
// tens of refusals rather than thousands of conditionals.
//
// Usage:
//
//	go run scripts/refusal-reachability/main.go [package-glob...]
//
// With no arguments it audits every non-test Go file in the module.
package main

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// refusalWords are the phrases this project uses when it declines to answer.
// Matched against string literals in a return statement, never against
// comments — a comment describing a refusal is not one.
var refusalWords = []string{
	"NOT MEASURED",
	"not measured",
	"refusing to",
	"refuses to",
	"cannot be read",
	"nothing to measure",
	"is not computed",
}

// flakySkip names tests excluded from every run this tool makes, baseline and
// mutant alike, and it is printed on every run so the exclusion cannot hide.
//
// TestRescore_SessionCostDoesNotGrowWorseThanQuadratic asserts a wall-clock
// ratio between two workloads. Measured on 2026-09-10 it failed 1 run in 3 on
// an unloaded machine and more often under the parallel load this tool creates,
// because a mutant run competes with everything else on the box. A baseline
// that is red by coin flip makes every mutant look caught, which is the exact
// failure ADR-0014 refuses to run into. It is a performance bound and cannot
// observe a refusal guard either way, so excluding it costs this audit nothing.
//
// It is excluded HERE and left alone in the suite: weakening a real test to
// make a tool quieter is the trade this whole exercise exists to refuse.
const flakySkip = "TestRescore_SessionCostDoesNotGrowWorseThanQuadratic"

type site struct {
	file      string // file holding the refusal
	pkg       string // package to run tests for
	refusal   string // the literal that makes it a refusal
	guardLine int    // line of the condition that gates it
	guardSrc  string
	// condStart and condEnd are byte offsets of the gating condition in the
	// file, taken from the AST rather than found by searching the line.
	//
	// The line-search form this replaced could not neutralise three shapes it
	// met here: `if info, err := os.Lstat(p); err != nil` (the init statement
	// made splitting on `if ` wrong, and splitting on `;` risks a semicolon
	// inside a string literal), a condition wrapped across lines, and a
	// tagless `switch { case cond: }`, which is how four of this repository's
	// refusals are written. Offsets have none of those cases: the parser has
	// already decided where the condition begins and ends.
	condStart, condEnd int
	// unmutable, when set, is why this site cannot be neutralised. ADR-0014
	// requires such a site be reported, never quietly skipped.
	unmutable string
}

func main() {
	roots := os.Args[1:]
	if len(roots) == 0 {
		roots = []string{"cmd", "internal"}
	}

	files, err := goFiles(roots)
	if err != nil {
		fail("walking %v: %v", roots, err)
	}

	var sites []site
	for _, f := range files {
		if excluded, _ := excludedFromBuild(f); excluded {
			continue
		}
		ss, err := refusalsIn(f)
		if err != nil {
			fail("parsing %s: %v", f, err)
		}
		sites = append(sites, ss...)
	}

	guarded := 0
	for _, s := range sites {
		if s.unmutable == "" {
			guarded++
		}
	}
	fmt.Printf("refusal-reachability: %d refusal site(s), %d behind a condition this tool can force false\n",
		len(sites), guarded)
	fmt.Printf("excluded from every run (baseline and mutant): -skip %s\n\n", flakySkip)

	// A red baseline makes every mutant look caught. ADR-0014 refuses to run
	// against one, and so does this.
	if out, ok := runTests(pkgsOf(sites)); !ok {
		fail("the baseline is red, so every mutant would look caught:\n%s", tail(out, 20))
	}

	var survivors, stillborn, unverified []site
	i := 0
	for _, s := range sites {
		if s.unmutable != "" {
			// ADR-0014: a conditional that cannot be neutralised is reported
			// as unverified, never skipped.
			unverified = append(unverified, s)
			continue
		}
		i++
		fmt.Printf("  [%d/%d] %s:%d  %s\n", i, guarded, s.file, s.guardLine, short(s.guardSrc))
		fmt.Printf("          refuses: %s\n", short(s.refusal))
		restore, err := neutralise(s)
		if err != nil {
			fmt.Printf("          UNVERIFIED: %v\n", err)
			s.unmutable = err.Error()
			unverified = append(unverified, s)
			continue
		}
		out, green := runTests([]string{s.pkg})
		restore()
		switch {
		case strings.Contains(out, "build failed") || strings.Contains(out, "cannot use") ||
			strings.Contains(out, "[build failed]") || strings.Contains(out, "syntax error"):
			fmt.Printf("          stillborn: the neutralised form does not compile\n")
			stillborn = append(stillborn, s)
		case green:
			fmt.Printf("          SURVIVED: no test tells this refusal from any other outcome\n")
			survivors = append(survivors, s)
		default:
			fmt.Printf("          caught\n")
		}
	}

	fmt.Printf("\nrefusal-reachability: %d mutated, %d survived, %d stillborn, %d unverified\n",
		guarded, len(survivors), len(stillborn), len(unverified))
	for _, s := range survivors {
		fmt.Printf("  SURVIVED    %s:%d  %s\n", s.file, s.guardLine, short(s.refusal))
	}
	for _, s := range unverified {
		why := s.unmutable
		if why == "" {
			why = "not neutralised"
		}
		fmt.Printf("  UNVERIFIED  %s  %s\n              (%s)\n", s.file, short(s.refusal), why)
	}
	if len(survivors) > 0 {
		os.Exit(1)
	}
}

func goFiles(roots []string) ([]string, error) {
	var out []string
	for _, root := range roots {
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || filepath.Ext(p) != ".go" || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			out = append(out, p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// refusalsIn finds return statements carrying a refusal literal and the
// innermost `if` that gates each.
func refusalsIn(file string) ([]site, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	byLine := strings.Split(string(src), "\n")

	// A gate is whatever decides that this refusal, and not the next outcome,
	// is what the caller gets: an `if` condition or one `case` of a tagless
	// switch. Both are expressions that can be forced false.
	type gate struct {
		cond      ast.Expr
		unmutable string
	}

	var out []site
	var stack []gate
	var walk func(n ast.Node) bool
	walk = func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.IfStmt:
			stack = append(stack, gate{cond: v.Cond})
			ast.Inspect(v.Body, walk)
			stack = stack[:len(stack)-1]
			// The else branch and the init statement are outside this gate.
			if v.Else != nil {
				ast.Inspect(v.Else, walk)
			}
			if v.Init != nil {
				ast.Inspect(v.Init, walk)
			}
			return false
		case *ast.SwitchStmt:
			// A tagless `switch { case cond: }` is a chain of ifs and is
			// treated as one. A switch WITH a tag compares against the tag,
			// and forcing a case false there is not a one-expression edit.
			tagless := v.Tag == nil
			for _, stmt := range v.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				g := gate{}
				switch {
				case len(cc.List) == 0:
					g.unmutable = "the `default` clause of a switch has no condition to force false"
				case !tagless:
					g.unmutable = "a case of a switch with a tag is not a boolean this tool can invert"
				case len(cc.List) > 1:
					g.unmutable = "a case with several expressions needs each forced false"
				default:
					g.cond = cc.List[0]
				}
				stack = append(stack, g)
				for _, st := range cc.Body {
					ast.Inspect(st, walk)
				}
				stack = stack[:len(stack)-1]
			}
			if v.Init != nil {
				ast.Inspect(v.Init, walk)
			}
			return false
		case *ast.ReturnStmt:
			lit, ok := refusalLiteral(v)
			if !ok {
				return true
			}
			s := site{
				file:      file,
				pkg:       "./" + filepath.Dir(file),
				refusal:   lit,
				unmutable: "the refusal is not behind any condition",
			}
			if len(stack) > 0 {
				g := stack[len(stack)-1]
				if g.unmutable != "" {
					s.unmutable = g.unmutable
				} else {
					pos := fset.Position(g.cond.Pos())
					s.guardLine = pos.Line
					s.condStart = fset.Position(g.cond.Pos()).Offset
					s.condEnd = fset.Position(g.cond.End()).Offset
					s.unmutable = ""
					if pos.Line <= len(byLine) {
						s.guardSrc = strings.TrimSpace(byLine[pos.Line-1])
					}
				}
			}
			out = append(out, s)
			return true
		}
		return true
	}
	ast.Inspect(f, walk)
	return out, nil
}

// refusalLiteral reports the refusal phrase a return statement carries, if any.
// Only string literals count: a comment naming a refusal is not one.
func refusalLiteral(r *ast.ReturnStmt) (string, bool) {
	found := ""
	for _, e := range r.Results {
		ast.Inspect(e, func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return true
			}
			for _, w := range refusalWords {
				if strings.Contains(bl.Value, w) {
					if found == "" {
						found = strings.Trim(bl.Value, "\"`")
					}
					return false
				}
			}
			return true
		})
	}
	return found, found != ""
}

func excludedFromBuild(file string) (bool, string) {
	b, err := os.ReadFile(file)
	if err != nil {
		return false, ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return false, ""
		}
		if !constraint.IsGoBuild(line) {
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			return false, ""
		}
		if !expr.Eval(func(string) bool { return false }) {
			return true, "build constraints exclude it: " + line
		}
	}
	return false, ""
}

// neutralise forces one gating condition false and returns a restore func.
//
// The condition stays compiled, so a variable only it uses does not go unused
// and turn the mutant stillborn for the wrong reason. It is parenthesised
// because Go binds && tighter than ||: `false && A || B` parses as
// `(false && A) || B` and disables only the first clause, which reports
// SURVIVED for a guard that was never neutralised.
func neutralise(s site) (func(), error) {
	orig, err := os.ReadFile(s.file)
	if err != nil {
		return nil, err
	}
	if s.condStart <= 0 || s.condEnd > len(orig) || s.condEnd <= s.condStart {
		return nil, fmt.Errorf("condition offsets %d..%d are not inside the file", s.condStart, s.condEnd)
	}
	cond := string(orig[s.condStart:s.condEnd])
	var b []byte
	b = append(b, orig[:s.condStart]...)
	b = append(b, "false && ("+cond+")"...)
	b = append(b, orig[s.condEnd:]...)
	if err := os.WriteFile(s.file, b, 0o644); err != nil {
		return nil, err
	}
	return func() { _ = os.WriteFile(s.file, orig, 0o644) }, nil
}

func runTests(pkgs []string) (string, bool) {
	args := append([]string{"test", "-count=1", "-skip", flakySkip}, pkgs...)
	out, err := exec.Command("go", args...).CombinedOutput()
	return string(out), err == nil
}

func pkgsOf(ss []site) []string {
	seen, out := map[string]bool{}, []string{}
	for _, s := range ss {
		if !seen[s.pkg] {
			seen[s.pkg] = true
			out = append(out, s.pkg)
		}
	}
	return out
}

func short(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 76 {
		return s[:73] + "..."
	}
	return s
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "refusal-reachability: "+format+"\n", a...)
	os.Exit(2)
}
