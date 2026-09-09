//go:build ignore

// guard-reachability neutralises each conditional a change touches and reports
// the ones whose removal changes nothing observable.
//
// The gap it fills is not a missing test, it is a missing reader. On
// 2026-09-09 this repository merged 23 pull requests in fourteen hours, every
// one of them authored and merged by the same agent, with CI green throughout.
// Three carried defects. All three were found by disabling a guard and watching
// the suite stay green — none by review, and none by the tests that shipped
// alongside them:
//
//   - an advisor fix that left Verified unreachable, reported green from a
//     filtered run that excluded the failing test
//   - an absence fix whose four tests all passed with half the fix reverted,
//     because every one called the inner function directly and none crossed
//     the join
//   - a guard written to close a vacuous test which was itself vacuous on
//     arrival, asserting on wording a different refusal also produces
//
// A reviewer would plausibly have caught none of those by reading. A machine
// that disables each new conditional and reruns the tests catches all three,
// and cannot get bored on the twenty-third pull request of the day.
//
// ADR-0014 names this practice and describes an implementation in a sibling
// repository. This is the Go one, scoped to a diff so it costs seconds rather
// than the eleven minutes the full mutant catalogue needs.
//
// Usage:
//
//	go run scripts/guard-reachability/main.go [base-ref]
//
// It compares the working tree against base-ref (default origin/main), finds
// conditionals added or changed in non-test Go files, and for each one runs the
// packages that could observe it with the condition forced false.
//
// A conditional whose neutralisation leaves every test passing is reported. It
// is either unreachable or untested, and ADR-0014 is explicit that both matter.
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type guard struct {
	file string
	line int
	src  string
	pkg  string
}

func main() {
	base := "origin/main"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}

	changed, err := changedGoFiles(base)
	if err != nil {
		fail("listing changed files: %v", err)
	}
	if len(changed) == 0 {
		fmt.Println("guard-reachability: no non-test Go files changed against " + base)
		return
	}

	var guards []guard
	for file, lines := range changed {
		// A file excluded from every build has no guard that can affect
		// anyone, and its package cannot be tested at all.
		//
		// Found by running this tool on the pull request that adds it. It
		// analysed its own source, found 28 conditionals, and tried to
		// `go test ./scripts/guard-reachability` — a package carrying
		// //go:build ignore, so the toolchain reports "build constraints
		// exclude all Go files" and the baseline is red. The refusal worked;
		// the analysis should never have got that far.
		if excluded, why := excludedFromBuild(file); excluded {
			fmt.Printf("guard-reachability: skipping %s (%s)\n", file, why)
			continue
		}
		gs, err := conditionalsIn(file, lines)
		if err != nil {
			fail("parsing %s: %v", file, err)
		}
		guards = append(guards, gs...)
	}
	if len(guards) == 0 {
		fmt.Printf("guard-reachability: %d changed file(s), no new or changed conditionals\n", len(changed))
		return
	}

	// A red baseline makes every guard look caught, which is the failure this
	// tool exists to prevent in others. ADR-0014 refuses to run against one.
	fmt.Printf("guard-reachability: baseline over %d guard(s) in %d file(s)\n", len(guards), len(changed))
	if out, ok := runTests(pkgsOf(guards)); !ok {
		fail("the baseline is red, so every mutant would look caught:\n%s", out)
	}

	var survivors []guard
	for i, g := range guards {
		fmt.Printf("  [%d/%d] %s:%d  %s\n", i+1, len(guards), g.file, g.line, short(g.src))
		restore, err := neutralise(g)
		if err != nil {
			fmt.Printf("        skipped: %v\n", err)
			continue
		}
		out, green := runTests([]string{g.pkg})
		restore()
		switch {
		case strings.Contains(out, "build failed") || strings.Contains(out, "cannot use"):
			// A mutant the compiler rejected was never put to the suite.
			// Counting it as caught is how a score is inflated.
			fmt.Printf("        stillborn: the neutralised form does not compile\n")
		case green:
			fmt.Printf("        SURVIVED: nothing observed this guard\n")
			survivors = append(survivors, g)
		default:
			fmt.Printf("        caught\n")
		}
	}

	fmt.Printf("\nguard-reachability: %d guard(s), %d survived\n", len(guards), len(survivors))
	if len(survivors) == 0 {
		return
	}
	fmt.Println("\nEach of these can be removed without any test noticing. It is either")
	fmt.Println("unreachable or untested, and both matter:")
	for _, g := range survivors {
		fmt.Printf("  %s:%d  %s\n", g.file, g.line, short(g.src))
	}
	os.Exit(1)
}

// changedGoFiles maps a changed non-test .go file to the lines the diff touched.
func changedGoFiles(base string) (map[string]map[int]bool, error) {
	// Committed work first, then the working tree, and the union of both.
	//
	// The first version fell back only when the three-dot diff ERRORED. On a
	// branch with no commits yet that diff succeeds and is empty, so
	// uncommitted changes were invisible and the tool reported "no non-test Go
	// files changed" over a file it had just been pointed at. Found by planting
	// a guard and watching it say nothing.
	files := map[string]map[int]bool{}
	var out []byte
	for _, args := range [][]string{
		{"diff", "-U0", base + "...HEAD"},
		{"diff", "-U0", base},
	} {
		b, err := exec.Command("git", args...).Output()
		if err != nil {
			continue
		}
		out = append(out, b...)
	}
	if len(out) == 0 {
		return files, nil
	}
	var cur string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "+++ b/") {
			cur = strings.TrimPrefix(line, "+++ b/")
			if filepath.Ext(cur) != ".go" || strings.HasSuffix(cur, "_test.go") {
				cur = ""
			}
			continue
		}
		if cur == "" || !strings.HasPrefix(line, "@@") {
			continue
		}
		// @@ -old,+new @@
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		spec := strings.TrimPrefix(parts[2], "+")
		start, count := spec, "1"
		if i := strings.IndexByte(spec, ','); i >= 0 {
			start, count = spec[:i], spec[i+1:]
		}
		s, err1 := strconv.Atoi(start)
		n, err2 := strconv.Atoi(count)
		if err1 != nil || err2 != nil {
			continue
		}
		if files[cur] == nil {
			files[cur] = map[int]bool{}
		}
		for l := s; l < s+max(n, 1); l++ {
			files[cur][l] = true
		}
	}
	return files, sc.Err()
}

// excludedFromBuild reports whether a file's //go:build line keeps it out of
// an ordinary build.
//
// Evaluated with no tags set, which is what `go test ./pkg` does. A file
// guarded by `ignore` — the conventional tag for a standalone tool — is
// excluded, and so is anything behind a tag this run does not set. Both are
// correct to skip: the conditionals inside them are not in the binary and no
// ordinary test run can observe them.
func excludedFromBuild(file string) (bool, string) {
	f, err := os.Open(file)
	if err != nil {
		return false, ""
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		// Constraints sit above the package clause; stop once it is reached.
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

// conditionalsIn finds if-statements whose condition sits on a changed line.
func conditionalsIn(file string, lines map[int]bool) ([]guard, error) {
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

	var out []guard
	ast.Inspect(f, func(n ast.Node) bool {
		is, ok := n.(*ast.IfStmt)
		if !ok || is.Cond == nil {
			return true
		}
		pos := fset.Position(is.Pos())
		if !lines[pos.Line] || pos.Line > len(byLine) {
			return true
		}
		out = append(out, guard{
			file: file,
			line: pos.Line,
			src:  strings.TrimSpace(byLine[pos.Line-1]),
			pkg:  "./" + filepath.Dir(file),
		})
		return true
	})
	return out, nil
}

// neutralise forces one conditional false and returns a restore func.
//
// `if false && <cond>` keeps the condition compiled, so a variable it uses does
// not become unused and turn the mutant stillborn for the wrong reason.
func neutralise(g guard) (func(), error) {
	orig, err := os.ReadFile(g.file)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(orig), "\n")
	if g.line > len(lines) {
		return nil, fmt.Errorf("line %d past end of file", g.line)
	}
	line := lines[g.line-1]
	idx := strings.Index(line, "if ")
	if idx < 0 {
		return nil, fmt.Errorf("no `if` on the line")
	}
	if strings.Contains(line, "if false &&") {
		return nil, fmt.Errorf("already neutralised")
	}
	lines[g.line-1] = line[:idx] + "if false && " + strings.TrimSpace(line[idx+3:])
	if err := os.WriteFile(g.file, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return nil, err
	}
	return func() { _ = os.WriteFile(g.file, orig, 0o644) }, nil
}

func runTests(pkgs []string) (string, bool) {
	args := append([]string{"test", "-count=1"}, pkgs...)
	out, err := exec.Command("go", args...).CombinedOutput()
	return string(out), err == nil
}

func pkgsOf(gs []guard) []string {
	seen, out := map[string]bool{}, []string{}
	for _, g := range gs {
		if !seen[g.pkg] {
			seen[g.pkg] = true
			out = append(out, g.pkg)
		}
	}
	return out
}

func short(s string) string {
	if len(s) > 72 {
		return s[:69] + "..."
	}
	return s
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "guard-reachability: "+format+"\n", a...)
	os.Exit(2)
}
