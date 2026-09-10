package regression

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// No test may run against the machine's real home directory.
//
// It happened. `go test ./cmd/replay/` rewrote the operator's own
// ~/.replay/advice.json, because TestS1 runs every value-delivering command —
// `advise` among them — against a fixture corpus with HOME left alone. Their
// real findings, from 1744 transcripts, were replaced by three from a
// two-session fixture, and the applied markers they had set were gone.
//
// A TestMain in cmd/replay fixed the package where the damage was observed.
// That is not the same as fixing the class, and reporting it as though it were
// is how a defect comes back somewhere else. This is the class.
//
// The scope is computed rather than assumed, which matters because the first
// estimate was wrong in the alarming direction. Listing every package with no
// TestMain named fourteen; almost none of them can reach a home directory at
// all, and counting them as exposed would have made this look larger than it
// is. Reachability is what counts: a package whose production code resolves
// HOME, or which imports one that does, transitively.
//
// The walk is AST-based rather than `go list -deps` for the reason
// unwired_packages_test.go gives: os/exec is confined to the mutation build
// tag, so this package cannot shell out to the toolchain to answer a question
// about itself.

var homeResolution = regexp.MustCompile(`UserHomeDir\(\)|Getenv\("HOME"\)|Getenv\("USERPROFILE"\)`)

type pkgInfo struct {
	imports  map[string]bool
	resolves bool // production code reads a home directory
	hasTests bool
	isolates bool // a TestMain that replaces HOME
}

// HI1: every package that can reach a home directory isolates HOME in its tests.
func TestHI1_NoPackageTestsAgainstTheRealHome(t *testing.T) {
	pkgs := scanPackages(t)
	if len(pkgs) == 0 {
		t.Fatal("no packages scanned, so this guard asserts nothing")
	}

	// The seed must not be empty, or reachability is vacuous and every package
	// passes for the wrong reason.
	var seeds []string
	for name, p := range pkgs {
		if p.resolves {
			seeds = append(seeds, name)
		}
	}
	if len(seeds) == 0 {
		t.Fatal("no package resolves a home directory, so nothing is reachable and this " +
			"guard would pass however many tests wrote to the reader's files")
	}

	var exposed []string
	for name, p := range pkgs {
		if !p.hasTests || !reaches(pkgs, name, map[string]bool{}) {
			continue
		}
		if !p.isolates {
			exposed = append(exposed, name)
		}
	}
	if len(exposed) > 0 {
		sort.Strings(exposed)
		sort.Strings(seeds)
		t.Errorf("these packages have tests that can reach a home directory and do not "+
			"replace HOME for the run, so a command under test can write to the reader's "+
			"own files:\n  %s\n\nHome resolution starts in: %s\n\n"+
			"Add a TestMain whose body calls os.Setenv for BOTH \"HOME\" and \"USERPROFILE\", "+
			"pointing them at a temporary directory. cmd/replay/home_test.go is the worked "+
			"example. USERPROFILE is not optional: os.UserHomeDir reads it on Windows and "+
			"ignores HOME.",
			strings.Join(exposed, "\n  "), strings.Join(seeds, ", "))
	}
}

// isolatesHome reports whether a file declares a TestMain that replaces BOTH
// home-directory variables.
//
// Parsed, not grepped, and the difference is the finding. This used to be two
// substring checks over the raw source — "func TestMain(" and `Setenv("HOME"`
// anywhere in the file — and an audit satisfied it with a comment:
//
//	// was: os.Setenv("HOME", dir)
//	func TestMain(m *testing.M) { ... sets nothing ... }
//
// A comment about isolation is not isolation. It is the same edit that
// defeated MC1, MC2 and GR1 — three guards written days apart, all reading a
// file as text and taking a match as proof — and this is the fourth.
//
// USERPROFILE is required too, which the old check never asked for. os.UserHomeDir
// reads it on Windows and ignores HOME, so a package isolating only HOME
// reintroduces FD-4 on the one platform nobody here develops on, and the old
// guard would have passed it.
func isolatesHome(src string) bool {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
	if err != nil {
		return false
	}
	var home, profile bool
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "TestMain" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Setenv" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			switch lit.Value {
			case `"HOME"`:
				home = true
			case `"USERPROFILE"`:
				profile = true
			}
			return true
		})
	}
	return home && profile
}

func reaches(pkgs map[string]*pkgInfo, name string, seen map[string]bool) bool {
	if seen[name] {
		return false
	}
	seen[name] = true
	p, ok := pkgs[name]
	if !ok {
		return false
	}
	if p.resolves {
		return true
	}
	for imp := range p.imports {
		if reaches(pkgs, imp, seen) {
			return true
		}
	}
	return false
}

func scanPackages(t *testing.T) map[string]*pkgInfo {
	t.Helper()
	root := repoRoot(t)
	const mod = "github.com/RedRobotKK/Replay"
	importRe := regexp.MustCompile(`"` + regexp.QuoteMeta(mod) + `/([^"]+)"`)
	pkgs := map[string]*pkgInfo{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			// .claude holds agent worktrees: whole copies of this repository,
			// which would be walked as if they were source.
			case ".git", ".claude", "vendor", "node_modules", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		rel, rerr := filepath.Rel(root, filepath.Dir(path))
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		p := pkgs[rel]
		if p == nil {
			p = &pkgInfo{imports: map[string]bool{}}
			pkgs[rel] = p
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		src := string(b)
		for _, m := range importRe.FindAllStringSubmatch(src, -1) {
			p.imports[m[1]] = true
		}
		if strings.HasSuffix(path, "_test.go") {
			p.hasTests = true
			if isolatesHome(src) {
				p.isolates = true
			}
			return nil
		}
		if homeResolution.MatchString(src) {
			p.resolves = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return pkgs
}
