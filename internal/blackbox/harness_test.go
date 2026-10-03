//go:build mutation

// Package blackbox exercises the production binary from outside.
//
// Everything in cmd/replay's TestE2E_* goes in through dispatch() in the test
// process. That proves the dispatch path; it does not prove the binary a user
// runs. This package builds ./cmd/replay once, runs each surface as a child
// process with its own HOME and fixtures, and asserts on exit status, stdout,
// stderr and machine-readable output. Then, for every surface, it thaws the
// tree into a scratch copy, applies the frozen mutant whose killer is that
// surface's E2E test, builds that binary, and requires the same black-box check
// to fail. A surface is PRODUCTION-GRADE here only when the real binary passes
// and every mutated binary fails.
//
// It sits behind the `mutation` build tag because it imports os/exec, which
// cmd/replay/x402_test.go confines to that tag: the shipped binary must not be
// able to start processes, and a test that starts them must not be in any
// ordinary build. Run it with:
//
//	go test -tags mutation -timeout 30m ./internal/blackbox/
package blackbox

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// binary is the production binary built once from the tree under test.
type binary struct {
	Path    string
	Version string
	Commit  string
}

var (
	buildOnce sync.Once
	built     binary
	buildErr  error
)

func productionBinary(t *testing.T) binary {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "replay-blackbox-bin-")
		if err != nil {
			buildErr = err
			return
		}
		built.Path = filepath.Join(dir, "replay")
		cmd := exec.Command("go", "build", "-o", built.Path, "./cmd/replay")
		cmd.Dir = repoRoot(t)
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = err
			built.Version = string(out)
			return
		}
		v := exec.Command(built.Path, "version")
		out, _ := v.Output()
		built.Version = strings.TrimSpace(string(out))
		c := exec.Command("git", "rev-parse", "--short", "HEAD")
		c.Dir = repoRoot(t)
		sha, _ := c.Output()
		built.Commit = strings.TrimSpace(string(sha))
	})
	if buildErr != nil {
		t.Fatalf("cannot build the production binary: %v\n%s", buildErr, built.Version)
	}
	return built
}

// result is what one invocation of the binary produced.
type result struct {
	Args   []string
	Exit   int
	Stdout string
	Stderr string
}

// run executes the binary as a user would: a child process with the given
// HOME, CLAUDE_CONFIG_DIR unset, and the arguments on the command line.
func run(bin string, home string, stdin string, args ...string) result {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(baseEnv(), "HOME="+home, "USERPROFILE="+home, "CLAUDE_CONFIG_DIR=", "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "NO_COLOR=1")
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
			errb.WriteString(err.Error())
		}
	}
	return result{Args: args, Exit: code, Stdout: out.String(), Stderr: errb.String()}
}

// baseEnv keeps PATH and the Go toolchain's own variables and drops everything
// that could point the binary at the developer's real state.
func baseEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		switch k {
		case "HOME", "USERPROFILE", "CLAUDE_CONFIG_DIR", "XDG_CONFIG_HOME", "ANTHROPIC_BASE_URL", "ANTHROPIC_API_KEY", "REPLAY_TOKEN", "REPLAY_UPSTREAM", "REPLAY_DISABLED":
			continue
		}
		env = append(env, kv)
	}
	return env
}

// discoverSurfaces reads the dispatch switch in cmd/replay/main.go. It is the
// same inventory cmd/replay/wiring_gate_test.go derives, re-derived here so
// this package cannot drift from the binary it tests.
type surface struct {
	Names []string
	Entry string
	Line  int
}

func (s surface) Canon() string { return strings.TrimLeft(s.Names[0], "-") }
func (s surface) E2E() string {
	c := s.Canon()
	return "TestE2E_" + strings.ToUpper(c[:1]) + c[1:]
}

func discoverSurfaces(t *testing.T) []surface {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(repoRoot(t), "cmd", "replay", "main.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var sw *ast.SwitchStmt
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "dispatch" || fd.Recv != nil {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if s, ok := n.(*ast.SwitchStmt); ok && sw == nil {
				sw = s
			}
			return sw == nil
		})
	}
	if sw == nil {
		t.Fatal("no dispatch switch in main.go")
	}
	var out []surface
	for _, stmt := range sw.Body.List {
		cc := stmt.(*ast.CaseClause)
		if cc.List == nil {
			continue
		}
		s := surface{Line: fset.Position(cc.Pos()).Line, Entry: "inline"}
		for _, e := range cc.List {
			v, _ := strconv.Unquote(e.(*ast.BasicLit).Value)
			s.Names = append(s.Names, v)
		}
		for _, b := range cc.Body {
			ast.Inspect(b, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && s.Entry == "inline" && (strings.HasPrefix(id.Name, "run") || strings.HasPrefix(id.Name, "print")) {
						s.Entry = id.Name
					}
				}
				return true
			})
		}
		out = append(out, s)
	}
	if len(out) < 20 {
		t.Fatalf("discovered %d surfaces; the walk is broken", len(out))
	}
	return out
}

// frozen mutants, as the catalogue holds them.
type edit struct {
	File        string `json:"file"`
	Anchor      string `json:"anchor"`
	Replacement string `json:"replacement"`
}
type mutant struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	File        string   `json:"file"`
	Anchor      string   `json:"anchor"`
	Replacement string   `json:"replacement"`
	Also        []edit   `json:"also"`
	KilledBy    []string `json:"killedBy"`
}

func (m mutant) edits() []edit {
	return append([]edit{{File: m.File, Anchor: m.Anchor, Replacement: m.Replacement}}, m.Also...)
}

func loadMutants(t *testing.T) []mutant {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "mutation", "testdata", "mutants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Mutants []mutant `json:"mutants"`
	}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatal(err)
	}
	return c.Mutants
}

// mutantsKilledBy returns the frozen mutants whose named killer is the test.
func mutantsKilledBy(ms []mutant, test string) []mutant {
	var out []mutant
	for _, m := range ms {
		for _, k := range m.KilledBy {
			if k == test {
				out = append(out, m)
			}
		}
	}
	return out
}

// thaw copies the tree (tracked and untracked-but-not-ignored files) into a
// scratch directory, exactly as internal/mutation does, so a mutant is never
// applied to the working copy.
func thaw(t *testing.T, root string) string {
	t.Helper()
	dst := t.TempDir()
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("cannot enumerate the tree: %v", err)
	}
	for _, name := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if name == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		target := filepath.Join(dst, name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

// mutatedBinary applies one frozen mutant to a thawed tree and builds it.
func mutatedBinary(t *testing.T, m mutant) (string, error) {
	t.Helper()
	dir := thaw(t, repoRoot(t))
	for _, e := range m.edits() {
		path := filepath.Join(dir, e.File)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s targets %s: %v", m.ID, e.File, err)
		}
		if n := strings.Count(string(body), e.Anchor); n != 1 {
			t.Fatalf("%s anchors %d times in %s; the catalogue requires exactly one", m.ID, n, e.File)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(string(body), e.Anchor, e.Replacement, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "replay-mutated")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/replay")
	cmd.Dir = dir
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", &buildFailure{m.ID, string(b)}
	}
	return out, nil
}

type buildFailure struct {
	ID  string
	Out string
}

func (b *buildFailure) Error() string { return b.ID + " did not build (stillborn):\n" + b.Out }

// runEnv is run with extra environment variables for the child.
func runEnv(bin string, home string, stdin string, extra []string, args ...string) result {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(append(baseEnv(), "HOME="+home, "USERPROFILE="+home, "CLAUDE_CONFIG_DIR=", "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "NO_COLOR=1"), extra...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
			errb.WriteString(err.Error())
		}
	}
	return result{Args: args, Exit: code, Stdout: out.String(), Stderr: errb.String()}
}
