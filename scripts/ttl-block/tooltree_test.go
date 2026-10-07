package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Same tree, hashed twice, must agree: the hash is a pure function of the
// file contents, nothing else.
func TestToolTreeSHA256Deterministic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")
	writeFile(t, filepath.Join(root, "sub", "b.go"), "package b\n")
	sum1, _, err := toolTreeSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	sum2, _, err := toolTreeSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	if sum1 != sum2 || sum1 == "" {
		t.Fatalf("hash not deterministic: %q vs %q", sum1, sum2)
	}
}

// File order must not affect the hash: passing the same two roots in the
// opposite order produces the same aggregate.
func TestToolTreeSHA256OrderIndependent(t *testing.T) {
	d1, d2 := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(d1, "a.go"), "package a\n")
	writeFile(t, filepath.Join(d2, "b.go"), "package b\n")
	forward, _, err := toolTreeSHA256(d1, d2)
	if err != nil {
		t.Fatal(err)
	}
	backward, _, err := toolTreeSHA256(d2, d1)
	if err != nil {
		t.Fatal(err)
	}
	if forward != backward {
		t.Fatalf("hash depends on root order: %q vs %q", forward, backward)
	}
}

// A real change to a tracked file must be detected.
func TestToolTreeSHA256DetectsContentChange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	writeFile(t, path, "package a\n")
	before, _, err := toolTreeSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "package a\n\nfunc changed() {}\n")
	after, _, err := toolTreeSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("a content change to a tracked file must change the hash")
	}
}

// A stray non-Go file (an editor artifact, a build byproduct) must not
// change the hash: the measurement-tool tree is its Go source.
func TestToolTreeSHA256IgnoresNonGoFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")
	before, _, err := toolTreeSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".DS_Store"), "junk")
	writeFile(t, filepath.Join(root, "a.test"), "binary-ish junk")
	after, _, err := toolTreeSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("a non-.go file must not change the measurement-tool hash")
	}
}

// An unreadable tracked file must fail the hash explicitly, not be hashed
// as if it were not there.
func TestToolTreeSHA256FailsExplicitlyOnUnreadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not apply the same way on windows")
	}
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	writeFile(t, path, "package a\n")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(path, 0o600) }()
	if _, _, err := toolTreeSHA256(root); err == nil {
		t.Fatal("an unreadable file must produce an explicit error, not a hash that silently omits it")
	}
}

// The per-file map is keyed by a path that identifies which file changed,
// for the disclosure list a mismatch produces.
func TestToolTreeSHA256FilesMapNamesEveryFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")
	writeFile(t, filepath.Join(root, "sub", "b.go"), "package b\n")
	_, files, err := toolTreeSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %v, want 2 entries", files)
	}
	found := map[string]bool{}
	for p := range files {
		found[filepath.Base(p)] = true
	}
	if !found["a.go"] || !found["b.go"] {
		t.Fatalf("files map does not name both tracked files: %v", files)
	}
}

// toolTreeDiff names exactly the paths that changed, were added, or were
// removed between two snapshots, and nothing that stayed the same.
func TestToolTreeDiffNamesChangedAddedAndRemovedFiles(t *testing.T) {
	before := map[string]string{"a.go": "h1", "b.go": "h2", "c.go": "h3"}
	after := map[string]string{"a.go": "h1", "b.go": "DIFFERENT", "d.go": "h4"}
	got := toolTreeDiff(before, after)
	want := []string{"b.go", "c.go", "d.go"} // b changed, c removed, d added; a unchanged
	if len(got) != len(want) {
		t.Fatalf("toolTreeDiff = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("toolTreeDiff = %v, want %v", got, want)
		}
	}
}

// BLOCK_STARTED captures the tool-tree hash of exactly the roots the caller
// named, under the new field, independent of the settings-file machinery.
func TestStartBlockCapturesToolTreeSHA256(t *testing.T) {
	p := fixture(t, map[string]any{})
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")
	p.toolTreeRoots = []string{root}
	s := readSched(t, p)
	rec, err := startBlock(p, 1, s.Blocks[0], time.Now())
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := toolTreeSHA256(root)
	if err != nil {
		t.Fatal(err)
	}
	if rec.ToolTreeSHA256 != want || rec.ToolTreeSHA256 == "" {
		t.Fatalf("ToolTreeSHA256 = %q, want %q", rec.ToolTreeSHA256, want)
	}
	if len(rec.ToolTreeFiles) != 1 {
		t.Fatalf("ToolTreeFiles = %v, want one entry", rec.ToolTreeFiles)
	}
}

// An unchanged tool tree between BLOCK_STARTED and BLOCK_ENDED is disclosed
// as intact, with no changed files named.
func TestEndBlockToolTreeIntactWhenUnchanged(t *testing.T) {
	p := fixture(t, map[string]any{})
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.go"), "package a\n")
	p.toolTreeRoots = []string{root}
	s := readSched(t, p)
	if _, err := startBlock(p, 1, s.Blocks[0], time.Now()); err != nil {
		t.Fatal(err)
	}
	rec, err := endBlock(p, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if rec.ToolTreeIntact == nil || !*rec.ToolTreeIntact {
		t.Fatalf("ToolTreeIntact = %v, want true", rec.ToolTreeIntact)
	}
	if len(rec.ToolTreeChangedFiles) != 0 {
		t.Fatalf("ToolTreeChangedFiles = %v, want none", rec.ToolTreeChangedFiles)
	}
}

// A tool tree that changed during the block is disclosed, with the changed
// file named, but the block still ends successfully: a changed tool does
// not void the block, mirroring the existing settings-span rule.
func TestEndBlockDisclosesToolTreeMismatchWithoutVoidingBlock(t *testing.T) {
	p := fixture(t, map[string]any{})
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	writeFile(t, path, "package a\n")
	p.toolTreeRoots = []string{root}
	s := readSched(t, p)
	if _, err := startBlock(p, 1, s.Blocks[0], time.Now()); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "package a\n\nfunc changed() {}\n")
	rec, err := endBlock(p, 1, time.Now())
	if err != nil {
		t.Fatalf("a tool-tree mismatch must not fail the block end: %v", err)
	}
	if rec.ToolTreeIntact == nil || *rec.ToolTreeIntact {
		t.Fatalf("ToolTreeIntact = %v, want false", rec.ToolTreeIntact)
	}
	if len(rec.ToolTreeChangedFiles) != 1 || rec.ToolTreeChangedFiles[0] != path {
		t.Fatalf("ToolTreeChangedFiles = %v, want [%s]", rec.ToolTreeChangedFiles, path)
	}
}

// A tool-tree hash that cannot be computed (an unreadable file) is disclosed
// as UNAVAILABLE rather than aborting the block: block validity depends on
// the settings-file verification, never on whether this disclosure succeeded.
func TestToolTreeHashFailureDoesNotAbortBlockStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not apply the same way on windows")
	}
	p := fixture(t, map[string]any{})
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	writeFile(t, path, "package a\n")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(path, 0o600) }()
	p.toolTreeRoots = []string{root}
	s := readSched(t, p)
	rec, err := startBlock(p, 1, s.Blocks[0], time.Now())
	if err != nil {
		t.Fatalf("a tool-tree hash failure must not abort block start: %v", err)
	}
	if rec.ToolTreeSHA256 != "UNAVAILABLE" {
		t.Fatalf("ToolTreeSHA256 = %q, want UNAVAILABLE", rec.ToolTreeSHA256)
	}
	if rec.ToolTreeError == "" {
		t.Fatal("a tool-tree hash failure must be disclosed in ToolTreeError")
	}
}
