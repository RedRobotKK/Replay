package outcome

// The tests in this file run git, as the independent reference for what this
// package computes: git archive materialises the pinned substrate, git
// write-tree is the oracle for TreeID. They are the only tests here that exec,
// and cmd/replay/x402_test.go exempts this one file by name for that reason.

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func needGit(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	return p
}

// The real substrate, when the clone carries commit 6871022 (a shallow CI clone
// does not, and the test says so rather than passing).
func TestPinnedTruthIsDerivedFromTheSubstrate(t *testing.T) {
	needModes(t)
	git := needGit(t)
	m, _, err := Pinned()
	if err != nil {
		t.Fatal(err)
	}
	if exec.Command(git, "cat-file", "-e", m.SubstrateCommit+"^{commit}").Run() != nil {
		t.Skipf("commit %s is not in this clone (shallow checkout); the truth derivation was not exercised", m.SubstrateCommit)
	}
	// git archive is relative to the working directory, so it must run at the
	// repository root: from this package's directory, which did not exist at
	// the substrate commit, it archives nothing.
	top, err := exec.Command(git, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(git, "archive", "--format=tar", m.SubstrateCommit)
	cmd.Dir = strings.TrimSpace(string(top))
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "substrate")
	if files := extractTar(t, out, root); files != 1014 {
		t.Fatalf("extracted %d files from %s, git ls-tree lists 1014", files, m.SubstrateCommit)
	}
	id, err := TreeID(root)
	if err != nil {
		t.Fatal(err)
	}
	if id != m.SubstrateTree {
		t.Fatalf("TreeID of the archived substrate = %s, git says %s", id, m.SubstrateTree)
	}
	n, err := DeriveAnswer(root, m.Glob)
	if err != nil {
		t.Fatal(err)
	}
	if n != 60 {
		t.Fatalf("derived %d test functions at %s, the documented truth is 60", n, m.SubstrateCommit)
	}
}

// extractTar writes a git archive under root and returns how many regular
// files it wrote. An archive that yields none fails here, not later.
func extractTar(t *testing.T, b []byte, root string) int {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(b))
	files := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			if files == 0 {
				t.Fatal("the archive held no files")
			}
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(root, filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			mode := os.FileMode(h.Mode).Perm()
			if err := os.WriteFile(p, body, mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(p, mode); err != nil {
				t.Fatal(err)
			}
			files++
		case tar.TypeXGlobalHeader:
		default:
			t.Fatalf("unexpected archive entry %s type %c", h.Name, h.Typeflag)
		}
	}
}

// TreeID must be git's own tree id, or a match against the pinned commit means
// nothing. git is the oracle here, not this package.
func TestTreeIDMatchesGitWriteTree(t *testing.T) {
	needModes(t)
	git := needGit(t)
	src := filepath.Join(t.TempDir(), "src")
	files := substrate()
	files["a.b"] = spec{body: "1"}
	files["a-b/x"] = spec{body: "2"}
	files["a/z"] = spec{body: "3"}
	files["ab"] = spec{body: "4"}
	writeTree(t, src, files)
	if err := os.MkdirAll(filepath.Join(src, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := TreeID(src)
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(t.TempDir(), "repo")
	copyTree(t, src, repo)
	for _, args := range [][]string{{"init", "-q"}, {"-c", "core.fileMode=true", "add", "-A"}} {
		cmd := exec.Command(git, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	cmd := exec.Command(git, "write-tree")
	cmd.Dir = repo
	want, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("TreeID = %s, git write-tree = %s", got, strings.TrimSpace(string(want)))
	}
}

// gitWriteTree is the reference: the tree id git itself records for src.
func gitWriteTree(t *testing.T, git, src string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	cp := exec.Command("cp", "-Rp", src+"/.", repo)
	if out, err := cp.CombinedOutput(); err != nil {
		t.Fatalf("cp: %v %s", err, out)
	}
	for _, args := range [][]string{{"init", "-q"}, {"-c", "core.fileMode=true", "-c", "core.symlinks=true", "add", "-A"}} {
		cmd := exec.Command(git, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	cmd := exec.Command(git, "write-tree")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestTreeIDMatchesGitOnEdgeCases(t *testing.T) {
	needModes(t)
	git := needGit(t)
	for name, build := range map[string]func(string){
		"symlinks store their target": func(root string) {
			writeTree(t, root, map[string]spec{"dir/f": {body: "x"}})
			_ = os.Symlink("dir/f", filepath.Join(root, "rel-link"))
			_ = os.Symlink("../outside/target", filepath.Join(root, "dir", "up-link"))
		},
		"only the owner execute bit counts": func(root string) {
			writeTree(t, root, map[string]spec{"group-x": {body: "g"}, "owner-x": {body: "o"}})
			_ = os.Chmod(filepath.Join(root, "group-x"), 0o654)
			_ = os.Chmod(filepath.Join(root, "owner-x"), 0o744)
		},
		"empty files and nested trees": func(root string) {
			writeTree(t, root, map[string]spec{"e": {}, "d/e/f/g": {body: "deep"}, "d.txt": {body: "1"}, "d-x": {body: "2"}})
		},
	} {
		root := filepath.Join(t.TempDir(), "src")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		build(root)
		got, err := TreeID(root)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := gitWriteTree(t, git, root); got != want {
			t.Errorf("%s: TreeID %s, git write-tree %s", name, got, want)
		}
	}
	// An empty root is git's empty tree.
	if got, err := TreeID(t.TempDir()); err != nil || got != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		t.Errorf("empty root: %s %v", got, err)
	}
}

// A special file is refused, not hashed: git cannot record one.
func TestTreeIDRefusesASpecialFile(t *testing.T) {
	needModes(t)
	root := t.TempDir()
	if err := exec.Command("mkfifo", filepath.Join(root, "pipe")).Run(); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	if _, err := TreeID(root); err == nil || !strings.Contains(err.Error(), "unsupported entry") {
		t.Errorf("fifo hashed: %v", err)
	}
}
