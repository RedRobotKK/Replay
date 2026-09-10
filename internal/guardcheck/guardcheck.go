// Package guardcheck is the machinery behind the guard-reachability reviewer.
//
// It lives here, rather than inside the script, because a tool that judges
// whether everyone else's conditionals are observed had none of its own. The
// script carried //go:build ignore, so `go test ./...` could not reach a line
// of it, and every claim it made about its own correctness was a claim in a
// comment. Extracting it makes those claims testable.
//
// See scripts/guard-reachability/main.go for what the reviewer is for.
package guardcheck

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	os "os"
	"path/filepath"
	"strconv"
	"strings"
)

// Guard is one conditional the reviewer will put to the suite.
type Guard struct {
	File string
	Line int
	Src  string
	Pkg  string

	// CondStart and CondEnd are byte offsets bounding the condition
	// expression, and BodyStart and BodyEnd bound the block it guards.
	//
	// They come from the parser rather than from looking at the line, because
	// a conditional is not a line. It can carry an init statement, span
	// several lines, or hold a string containing the text "if ". Every one of
	// those defeated the line-based rewrite that came before, and defeated it
	// silently: the mutant did not compile, so the guard was never put to the
	// suite and the verdict was neither caught nor survived.
	CondStart, CondEnd int
	BodyStart, BodyEnd token.Position
}

// ParseDiff maps a changed non-test .go file to the lines a diff touched.
//
// It takes the diff as text rather than running git, and the reason is a rule
// this repository enforces in CI: os/exec is confined to the mutation build
// tag, so a package anyone can import may not start a process. Keeping the
// analysis pure and leaving `git diff` to the caller satisfies that, and has
// the better side effect that every branch below is reachable from a test
// with a string.
func ParseDiff(diff string) map[string]map[int]bool {
	files := map[string]map[int]bool{}
	var cur string
	sc := bufio.NewScanner(strings.NewReader(diff))
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
	return files
}

// ExcludedFromBuild reports whether a file's //go:build line keeps it out of
// an ordinary build.
//
// Evaluated with no tags set, which is what `go test ./pkg` does. A file
// guarded by `ignore` — the conventional tag for a standalone tool — is
// excluded, and so is anything behind a tag this run does not set. Both are
// correct to skip: the conditionals inside them are not in the binary and no
// ordinary test run can observe them.
func ExcludedFromBuild(file string) (bool, string) {
	// A read failure needs no branch: os.Open returns a nil *os.File, whose
	// Read reports ErrInvalid, so the scanner yields nothing and the function
	// returns "not excluded" — which is the right answer for a file that
	// could not be read. Excluding on a read error would silently drop a real
	// source file from the analysis.
	f, _ := os.Open(file)
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

// Conditionals finds if-statements whose condition sits on a changed line.
func Conditionals(file string, lines map[int]bool) ([]Guard, error) {
	// Read once and hand the bytes to the parser, rather than letting the
	// parser open the file and then opening it again for the source lines.
	// The second read had an error branch no test could reach — the parser
	// had just succeeded on the same path — and an unreachable branch is a
	// claim about failure handling that nothing can check.
	src, err := os.ReadFile(file)
	if err != nil {
		// Named, not passed through bare. Handing nil source to the parser
		// makes it open the path itself, so an unreadable file still ends in
		// an error — but one phrased as a parse failure, which sends a reader
		// looking for a syntax problem in a file that was never read.
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		return nil, err
	}
	byLine := strings.Split(string(src), "\n")

	var out []Guard
	ast.Inspect(f, func(n ast.Node) bool {
		is, ok := n.(*ast.IfStmt)
		if !ok || is.Cond == nil {
			return true
		}
		pos := fset.Position(is.Pos())
		if !lines[pos.Line] || pos.Line > len(byLine) {
			return true
		}
		out = append(out, Guard{
			File:      file,
			Line:      pos.Line,
			Src:       strings.TrimSpace(byLine[pos.Line-1]),
			Pkg:       "./" + filepath.Dir(file),
			CondStart: fset.Position(is.Cond.Pos()).Offset,
			CondEnd:   fset.Position(is.Cond.End()).Offset,
			BodyStart: fset.Position(is.Body.Lbrace),
			BodyEnd:   fset.Position(is.Body.Rbrace),
		})
		return true
	})
	return out, nil
}

// Neutralise forces one conditional false and returns a restore func.
//
// It rewrites the condition expression by byte range, `<cond>` becoming
// `false && (<cond>)`, and touches nothing else on the line.
//
// Two properties follow from working on the expression rather than the line,
// and the reviewer was wrong about both before it did.
//
// An init statement survives. `if x, ok := f(); ok {` becomes
// `if x, ok := f(); false && (ok) {`, which compiles. Prepending `if false &&`
// to that line produced `if false && (x, ok := f(); ok)`, which is not Go, so
// the mutant was stillborn and the guard went unchecked — silently, since a
// stillborn mutant is reported once and then left out of the verdict. Two of
// the seven guards in the pull request that prompted this went unchecked that
// way.
//
// A condition spanning lines is neutralised whole. The line-based rewrite read
// the first line only, so any clause below it stayed live: the same defect as
// the unparenthesised `||`, wearing a different hat.
//
// The condition keeps compiling, so a variable it uses does not become unused
// and turn the mutant stillborn for the wrong reason. It is parenthesised
// because Go binds && tighter than ||: `false && A || B` parses as
// `(false && A) || B` and disables only the first clause, which is how this
// tool once reported SURVIVED for guards it had never actually neutralised.
func Neutralise(g Guard) (func(), error) {
	orig, err := os.ReadFile(g.File)
	if err != nil {
		// Named rather than passed through bare, because the offset check
		// below would otherwise catch a missing file too and report it as
		// "offsets do not fit a 0-byte file" — true, and the wrong diagnosis.
		return nil, fmt.Errorf("reading %s: %w", g.File, err)
	}
	if g.CondStart <= 0 || g.CondEnd > len(orig) || g.CondStart >= g.CondEnd {
		return nil, fmt.Errorf("condition offsets [%d,%d) do not fit a %d-byte file",
			g.CondStart, g.CondEnd, len(orig))
	}
	cond := string(orig[g.CondStart:g.CondEnd])
	if strings.HasPrefix(cond, "false &&") {
		return nil, fmt.Errorf("already neutralised")
	}
	mutant := string(orig[:g.CondStart]) + "false && (" + cond + ")" + string(orig[g.CondEnd:])
	if err := os.WriteFile(g.File, []byte(mutant), 0o644); err != nil {
		return nil, err
	}
	return func() { _ = os.WriteFile(g.File, orig, 0o644) }, nil
}
