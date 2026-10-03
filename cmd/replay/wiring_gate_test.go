package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/claims"
)

// The production surface gate.
//
// Every test in this file starts from the same fact: the list of subcommands
// is the switch in dispatch(), in main.go, and nothing else. There is no
// hand-written inventory to drift from it. A new case in that switch is a new
// supported surface the moment it is added, and this file turns red until that
// surface has, in order:
//
//   - an end-to-end test, TestE2E_<Name>, that reaches the surface through
//     dispatch() the way main() does, on an isolated HOME, and asserts what the
//     user sees;
//   - a frozen mutant in internal/mutation/testdata/mutants.json whose
//     killedBy names that test, applied to a file the shipped binary links;
//   - an entry function that exists in a non-test file of this package.
//
// The register of claims is held to the same standard: a claim marked
// ESTABLISHED or BOUNDED may only cite tests that live in a package the binary
// links, because a test in a package the binary does not link proves nothing
// about the binary. The PROOF document may only cite shipped packages in a row
// it marks PROVEN.
//
// The matrix in docs/evidence/wiring-matrix-1.0.md is rendered from these facts
// and compared byte for byte, so the document cannot say PASS about a tree
// that would fail here. Regenerate it with
//
//	go test ./cmd/replay -run TestWiringGate -update-wiring-matrix
//
// What this file cannot do is run a mutant. Whether each frozen mutant still
// dies is TestFrozenMutantsStillDie in internal/mutation, behind the mutation
// build tag because it copies the tree once per mutant. This file checks the
// static half: that the mutant exists, names this surface's E2E test as its
// killer, and still anchors to a line in a shipped file. A mutant whose anchor
// has moved is a guard that no longer guards, and that is caught here.

var updateWiringMatrix = flag.Bool("update-wiring-matrix", false,
	"rewrite docs/evidence/wiring-matrix-1.0.md from the discovered surfaces")

const (
	wiringModule     = "github.com/RedRobotKK/Replay"
	wiringMatrixPath = "docs/evidence/wiring-matrix-1.0.md"
	wiringMutants    = "internal/mutation/testdata/mutants.json"
	wiringProof      = "docs/PROOF-1.0.md"
)

// wiredSurface is one case of the dispatch switch.
type wiredSurface struct {
	Names []string // every spelling the case accepts, first is canonical
	Entry string   // the function the case hands control to, or "inline"
	Line  int      // where in main.go
}

func (s wiredSurface) Canon() string { return strings.TrimLeft(s.Names[0], "-") }

// TestName is the end-to-end test this surface must have.
func (s wiredSurface) TestName() string {
	c := s.Canon()
	return "TestE2E_" + strings.ToUpper(c[:1]) + c[1:]
}

func wiringRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// discoverSurfaces reads the dispatch switch. It is the inventory.
func discoverSurfaces(t *testing.T) []wiredSurface {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var dispatchFn *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "dispatch" && fd.Recv == nil {
			dispatchFn = fd
		}
	}
	if dispatchFn == nil {
		t.Fatal("main.go has no dispatch(); the inventory has nothing to read")
	}
	var sw *ast.SwitchStmt
	ast.Inspect(dispatchFn.Body, func(n ast.Node) bool {
		if s, ok := n.(*ast.SwitchStmt); ok && sw == nil {
			sw = s
		}
		return sw == nil
	})
	if sw == nil {
		t.Fatal("dispatch() has no switch; the inventory has nothing to read")
	}
	var out []wiredSurface
	for _, stmt := range sw.Body.List {
		cc := stmt.(*ast.CaseClause)
		if cc.List == nil {
			continue // default: a path argument routed to "replay", already a case
		}
		var s wiredSurface
		s.Line = fset.Position(cc.Pos()).Line
		for _, e := range cc.List {
			lit, ok := e.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Fatalf("main.go:%d: a case that is not a string literal cannot be inventoried", s.Line)
			}
			v, _ := strconv.Unquote(lit.Value)
			s.Names = append(s.Names, v)
		}
		s.Entry = "inline"
		for _, b := range cc.Body {
			ast.Inspect(b, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				if strings.HasPrefix(id.Name, "run") && s.Entry == "inline" {
					s.Entry = id.Name
				} else if strings.HasPrefix(id.Name, "print") && s.Entry == "inline" {
					s.Entry = id.Name
				}
				return true
			})
		}
		out = append(out, s)
	}
	if len(out) < 20 {
		t.Fatalf("discovered only %d surfaces from dispatch(); the walk is broken", len(out))
	}
	return out
}

// shippedClosure is every repository package cmd/replay links, by parsing
// imports of non-test files. go list is not used: a test in this package may
// not import os/exec (x402_test.go).
func shippedClosure(t *testing.T) map[string]bool {
	t.Helper()
	root := wiringRepoRoot(t)
	imports := func(dir string) []string {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, filepath.Join(root, dir), func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ImportsOnly)
		if err != nil {
			return nil
		}
		var out []string
		for _, p := range pkgs {
			for _, f := range p.Files {
				for _, im := range f.Imports {
					v, _ := strconv.Unquote(im.Path.Value)
					if strings.HasPrefix(v, wiringModule+"/") {
						out = append(out, strings.TrimPrefix(v, wiringModule+"/"))
					}
				}
			}
		}
		return out
	}
	seen := map[string]bool{"cmd/replay": true}
	queue := []string{"cmd/replay"}
	for len(queue) > 0 {
		d := queue[0]
		queue = queue[1:]
		for _, im := range imports(d) {
			if !seen[im] {
				seen[im] = true
				queue = append(queue, im)
			}
		}
	}
	if !seen["internal/proxy"] || !seen["internal/analysis"] {
		t.Fatalf("closure walk found %d packages and misses internal/proxy or internal/analysis; it is broken", len(seen))
	}
	return seen
}

// testFuncs maps every Test* function under the repository to the package
// directory it lives in, and keeps its body for inspection.
type testFunc struct {
	Dir  string
	File string
	Decl *ast.FuncDecl
	Fset *token.FileSet
}

func allTestFuncs(t *testing.T) map[string]testFunc {
	t.Helper()
	root := wiringRepoRoot(t)
	out := map[string]testFunc{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", ".claude", "tmp":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !strings.HasPrefix(fd.Name.Name, "Test") {
				continue
			}
			out[fd.Name.Name] = testFunc{Dir: filepath.ToSlash(rel), File: path, Decl: fd, Fset: fset}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 100 {
		t.Fatalf("found %d Test functions; the walk is broken", len(out))
	}
	return out
}

// callsDispatchWith reports whether a test body calls dispatch (or run, which
// main() calls and which calls dispatch) with the surface's name as a literal
// somewhere in the body.
func callsDispatchWith(tf testFunc, names []string) (calls bool, named bool) {
	ast.Inspect(tf.Decl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "dispatch" || id.Name == "run" || id.Name == "e2e") {
				calls = true
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING {
				v, _ := strconv.Unquote(x.Value)
				for _, n := range names {
					if v == n {
						named = true
					}
				}
			}
		}
		return true
	})
	return
}

type frozenMutant struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Anchor   string   `json:"anchor"`
	KilledBy []string `json:"killedBy"`
}

func loadFrozenMutants(t *testing.T) []frozenMutant {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(wiringRepoRoot(t), wiringMutants))
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Mutants []frozenMutant `json:"mutants"`
	}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatal(err)
	}
	return c.Mutants
}

func funcDeclared(t *testing.T, name string) bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == name {
					return true
				}
			}
		}
	}
	return false
}

// gateRow is one surface judged. Every field is derived.
type gateRow struct {
	Surface    string
	Shipped    bool   // the entry function is declared in cmd/replay
	Caller     string // dispatch case → entry
	Deciding   string // the frozen mutant's file and anchor
	Functional string // the E2E test, which is also the functional test
	Oracle     string // tests the frozen mutant names as killers
	Mutants    string // mutant ids
	E2E        string
	Failures   []string
}

func (r gateRow) Status() string {
	if len(r.Failures) == 0 {
		return "PASS"
	}
	return "FAIL"
}

func judgeSurfaces(t *testing.T) []gateRow {
	t.Helper()
	surfaces := discoverSurfaces(t)
	closure := shippedClosure(t)
	tests := allTestFuncs(t)
	mutants := loadFrozenMutants(t)
	root := wiringRepoRoot(t)

	var rows []gateRow
	for _, s := range surfaces {
		r := gateRow{Surface: s.Canon(), Caller: fmt.Sprintf("dispatch main.go:%d → %s", s.Line, s.Entry)}
		r.Shipped = s.Entry == "inline" || funcDeclared(t, s.Entry)
		if !r.Shipped {
			r.Failures = append(r.Failures, "entry "+s.Entry+" is not declared in cmd/replay")
		}
		name := s.TestName()
		tf, ok := tests[name]
		switch {
		case !ok:
			r.Failures = append(r.Failures, "no "+name)
		case tf.Dir != "cmd/replay":
			r.Failures = append(r.Failures, name+" is in "+tf.Dir+", not cmd/replay")
		default:
			calls, named := callsDispatchWith(tf, s.Names)
			if !calls {
				r.Failures = append(r.Failures, name+" does not go through dispatch()")
			}
			if !named {
				r.Failures = append(r.Failures, name+" never passes "+strconv.Quote(s.Names[0]))
			}
			r.Functional, r.E2E = name, name
		}
		var ids, files, killers []string
		for _, m := range mutants {
			kills := false
			for _, k := range m.KilledBy {
				if k == name {
					kills = true
				}
			}
			if !kills {
				continue
			}
			dir := filepath.ToSlash(filepath.Dir(m.File))
			if !closure[dir] {
				r.Failures = append(r.Failures, m.ID+" mutates "+m.File+", which the binary does not link")
			}
			body, err := os.ReadFile(filepath.Join(root, m.File))
			if err != nil || strings.Count(string(body), m.Anchor) != 1 {
				r.Failures = append(r.Failures, m.ID+" no longer anchors once in "+m.File)
			}
			ids = append(ids, m.ID)
			files = append(files, m.File+" @ "+strconv.Quote(firstLine(m.Anchor)))
			killers = append(killers, m.KilledBy...)
		}
		if len(ids) == 0 {
			r.Failures = append(r.Failures, "no frozen mutant is killed by "+name)
		}
		sort.Strings(killers)
		r.Mutants = strings.Join(ids, ", ")
		r.Deciding = strings.Join(files, "; ")
		r.Oracle = strings.Join(uniq(killers), ", ")
		rows = append(rows, r)
	}
	return rows
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	if len(s) > 60 {
		s = s[:57] + "…"
	}
	return s
}

func uniq(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// TestWiringGate_MainReachesDispatch pins the path the E2E tests stand on:
// main() → run() → dispatch(). If that chain changes, every TestE2E_ test
// is exercising something main() no longer calls, and this says so.
func TestWiringGate_MainReachesDispatch(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := func(fn, callee string) bool {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Name.Name != fn || fd.Recv != nil {
				continue
			}
			found := false
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					if id, ok := c.Fun.(*ast.Ident); ok && id.Name == callee {
						found = true
					}
				}
				return !found
			})
			return found
		}
		return false
	}
	if !calls("main", "run") {
		t.Error("main() does not call run()")
	}
	if !calls("run", "dispatch") {
		t.Error("run() does not call dispatch(); the E2E tests would bypass main's path")
	}
}

// TestWiringGate_EverySurfaceIsWired is the gate over the inventory.
func TestWiringGate_EverySurfaceIsWired(t *testing.T) {
	rows := judgeSurfaces(t)
	if len(rows) == 0 {
		t.Fatal("no surfaces")
	}
	for _, r := range rows {
		for _, f := range r.Failures {
			t.Errorf("%-12s FAIL: %s", r.Surface, f)
		}
	}
}

// TestWiringGate_RegisteredOraclesRunInTheShippedClosure: a claim the register
// marks ESTABLISHED or BOUNDED cites only tests that exist and live in a
// package the binary links.
func TestWiringGate_RegisteredOraclesRunInTheShippedClosure(t *testing.T) {
	closure := shippedClosure(t)
	tests := allTestFuncs(t)
	n := 0
	for _, c := range claims.Register {
		if c.Result != claims.Established && c.Result != claims.Bounded {
			continue
		}
		n++
		if len(c.Tests) == 0 {
			t.Errorf("%s is %s with no test", c.ID, c.Result)
		}
		for _, name := range c.Tests {
			// A registered entry may be a family, "TestC036_*", or a single name.
			if strings.HasSuffix(name, "*") {
				prefix := strings.TrimSuffix(name, "*")
				found := 0
				for tn, tf := range tests {
					if strings.HasPrefix(tn, prefix) {
						found++
						if !closure[tf.Dir] {
							t.Errorf("%s cites %s, which lives in %s; the binary does not link that package, so the test proves nothing about it", c.ID, tn, tf.Dir)
						}
					}
				}
				if found == 0 {
					t.Errorf("%s cites %s and no test matches", c.ID, name)
				}
				continue
			}
			tf, ok := tests[name]
			if !ok {
				t.Errorf("%s cites %s, which does not exist", c.ID, name)
				continue
			}
			if !closure[tf.Dir] {
				t.Errorf("%s cites %s, which lives in %s; the binary does not link that package, so the test proves nothing about it", c.ID, name, tf.Dir)
			}
		}
	}
	if n == 0 {
		t.Fatal("no ESTABLISHED or BOUNDED claims; the register walk is broken")
	}
}

// TestWiringGate_ProvenRowsCiteShippedPackages: a PROOF-1.0 row marked PROVEN
// may name only packages the binary links as its evidence.
func TestWiringGate_ProvenRowsCiteShippedPackages(t *testing.T) {
	closure := shippedClosure(t)
	body, err := os.ReadFile(filepath.Join(wiringRepoRoot(t), wiringProof))
	if err != nil {
		t.Fatal(err)
	}
	pkg := regexp.MustCompile("`(internal/[a-z0-9_/]+)`")
	proven := 0
	for i, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "|") || !(strings.Contains(line, "| ESTABLISHED |") || strings.Contains(line, "PROVEN")) {
			continue
		}
		proven++
		for _, m := range pkg.FindAllStringSubmatch(line, -1) {
			if !closure[m[1]] {
				t.Errorf("%s:%d: an ESTABLISHED row cites %s, which the binary does not link", wiringProof, i+1, m[1])
			}
		}
	}
	if proven == 0 {
		t.Fatal("no ESTABLISHED rows found; the scan is broken")
	}
}

// TestWiringGate_MatrixIsCurrent renders the matrix and compares it with the
// committed document, so the document cannot claim what the tree does not.
func TestWiringGate_MatrixIsCurrent(t *testing.T) {
	rows := judgeSurfaces(t)
	want := renderWiringMatrix(rows)
	path := filepath.Join(wiringRepoRoot(t), wiringMatrixPath)
	if *updateWiringMatrix {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	have, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (run with -update-wiring-matrix)", wiringMatrixPath, err)
	}
	if string(have) != want {
		t.Errorf("%s is stale against the discovered surfaces; run\n  go test ./cmd/replay -run TestWiringGate_MatrixIsCurrent -update-wiring-matrix", wiringMatrixPath)
	}
}

func renderWiringMatrix(rows []gateRow) string {
	var b strings.Builder
	b.WriteString("# Production surface wiring matrix\n\n")
	b.WriteString("Generated by `TestWiringGate_MatrixIsCurrent` in `cmd/replay/wiring_gate_test.go` from the\n")
	b.WriteString("`dispatch()` switch in `cmd/replay/main.go`, the `TestE2E_*` tests, and\n")
	b.WriteString("`internal/mutation/testdata/mutants.json`. Do not edit by hand; the test fails when this\n")
	b.WriteString("file and the tree disagree. Whether each mutant still dies is `TestFrozenMutantsStillDie`\n")
	b.WriteString("(`go test -tags mutation ./internal/mutation/`); this file records which mutant and which\n")
	b.WriteString("killer each surface is held to.\n\n")
	pass, fail := 0, 0
	for _, r := range rows {
		if r.Status() == "PASS" {
			pass++
		} else {
			fail++
		}
	}
	b.WriteString("| Surface | Shipped | Production caller | Deciding code | Functional test | Production oracle | Mutation | E2E | Status |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		shipped := "yes"
		if !r.Shipped {
			shipped = "NO"
		}
		cell := func(s string) string {
			if s == "" {
				return "MISSING"
			}
			return "`" + strings.ReplaceAll(s, "|", "\\|") + "`"
		}
		status := r.Status()
		if status == "FAIL" {
			status = "**FAIL** (" + strings.Join(r.Failures, "; ") + ")"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			r.Surface, shipped, cell(r.Caller), cell(r.Deciding), cell(r.Functional), cell(r.Oracle), cell(r.Mutants), cell(r.E2E), status)
	}
	fmt.Fprintf(&b, "\n**%d surfaces discovered: %d PASS, %d FAIL.**\n\n", len(rows), pass, fail)
	if fail == 0 && pass > 0 {
		b.WriteString("PRODUCTION SURFACE GATE: PASS\n")
	} else {
		b.WriteString("PRODUCTION SURFACE GATE: FAIL\n")
	}
	return b.String()
}
