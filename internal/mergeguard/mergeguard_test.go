package mergeguard_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/mergeguard"
)

// The two commits either side of the merge that put main in the red. The
// first is the main that had taken #184; the second is #185's head. Their
// merge is clean, writes a tree, exits zero, and does not compile.
const (
	mainWith184 = "7dd5fa3"
	head185     = "5fb7607"
)

// TestMG1_TheMergeThatBrokeMainIsCaught is the regression test, and it runs
// against the commits that actually did it rather than a constructed pair.
//
// The shape is the point. `git merge-tree` reports NO conflict: the two
// files do not overlap textually, because one is the other renamed and the
// rename's delete side was dropped in a rebase. Both compile alone. They
// collide only at the symbol level, in a _test.go, which is why
// `go build ./...` on the merged tree prints nothing and only a type-check
// that includes tests finds it.
func TestMG1_TheMergeThatBrokeMainIsCaught(t *testing.T) {
	repo := repoRoot(t)
	requireCommits(t, repo, mainWith184, head185)

	out, _ := run(t, repo, "git", "merge-tree", "--write-tree", mainWith184, head185)
	tree, conflicts := mergeguard.Conflicted(out)
	if len(conflicts) != 0 {
		t.Fatalf("expected a CLEAN merge, got conflicts in %v.\n"+
			"If git has started reporting this as a conflict the test is still "+
			"useful, but the finding it documents — that a clean merge can fail "+
			"to compile — needs a different example", conflicts)
	}
	if tree == "" {
		t.Fatal("merge-tree wrote no tree")
	}

	dir := t.TempDir()
	extract(t, repo, tree, dir)

	// Both files land in the merge. That is the defect, and if the fixture
	// ever stops producing it this test proves nothing.
	for _, f := range []string{"zzprof_test.go", "parse_bench_test.go"} {
		if _, err := os.Stat(filepath.Join(dir, "internal", "transcript", f)); err != nil {
			t.Fatalf("%s is not in the merged tree, so this is no longer the incident: %v", f, err)
		}
	}

	// go build is the check the obvious design would have reached for, and
	// it does not catch this. Asserting that keeps the reason in the suite
	// rather than in a comment somebody later deletes.
	if buildOut, _ := run(t, dir, "go", "build", "./..."); strings.TrimSpace(buildOut) != "" {
		t.Fatalf("go build reported something on the merged tree: %s\n"+
			"It did not at the time this was written, which is why Checks() "+
			"does not rely on it", buildOut)
	}

	var found bool
	for _, c := range mergeguard.Checks() {
		out, err := run(t, dir, c[0], c[1:]...)
		if err != nil {
			found = true
			if !strings.Contains(out, "redeclared in this block") {
				t.Errorf("%v failed on the merged tree, but not with the collision:\n%s", c, out)
			}
		}
	}
	if !found {
		t.Fatal("every check passed on a tree that does not compile: " +
			"this is the exact blind spot the tool exists to close")
	}
}

// TestMG2_ACleanMergeIsNotAConflict pins the distinction the caller must not
// collapse. A conflict is the easy case — git says so and nobody merges
// through it. The case worth tooling is the one where git says nothing.
func TestMG2_ACleanMergeIsNotAConflict(t *testing.T) {
	tree, conflicts := mergeguard.Conflicted("fc8c8880217125b746116babd5821bf0d5cc046c\n")
	if tree != "fc8c8880217125b746116babd5821bf0d5cc046c" {
		t.Errorf("tree = %q", tree)
	}
	if len(conflicts) != 0 {
		t.Errorf("a one-line output is a clean merge, got conflicts %v", conflicts)
	}
}

// TestMG3_ConflictedPathsAreNamed covers the other arm, in the format git
// prints: the tree, then mode/oid/stage, a tab, and the path.
func TestMG3_ConflictedPathsAreNamed(t *testing.T) {
	// The blank lines are git's, not decoration: it separates the tree from
	// the conflict section with one and prints another after it.
	out := "abc123\n" +
		"\n" +
		"100644 aaa 1\tinternal/transcript/wire.go\n" +
		"100644 bbb 2\tinternal/transcript/wire.go\n" +
		"100644 ccc 3\tcmd/replay/cost.go\n" +
		"\n"
	tree, conflicts := mergeguard.Conflicted(out)
	if tree != "abc123" {
		t.Errorf("tree = %q, want abc123", tree)
	}
	want := []string{"internal/transcript/wire.go", "cmd/replay/cost.go"}
	if strings.Join(conflicts, ",") != strings.Join(want, ",") {
		t.Errorf("conflicts = %v, want %v (each path once, in order)", conflicts, want)
	}
}

// TestMG4_BuildAloneIsNotAmongTheChecks freezes the finding rather than the
// list. Someone reading Checks() will be tempted to add `go build ./...`
// because it is faster, and it is faster because it does less: it does not
// type-check tests, and the merge that motivated all of this broke in a
// test file.
func TestMG4_BuildAloneIsNotAmongTheChecks(t *testing.T) {
	var vets bool
	for _, c := range mergeguard.Checks() {
		line := strings.Join(c, " ")
		if line == "go build ./..." {
			t.Error("`go build ./...` is in Checks(). It does not type-check test " +
				"files, and the merge this tool exists for broke in one: it printed " +
				"nothing on the merged tree while go vet named the collision")
		}
		if strings.HasPrefix(line, "go vet") {
			vets = true
		}
	}
	if !vets {
		t.Error("no check type-checks test files, so the incident this was built for would pass")
	}
}

func run(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func extract(t *testing.T, repo, tree, dir string) {
	t.Helper()
	archive := exec.Command("git", "archive", tree)
	archive.Dir = repo
	tar := exec.Command("tar", "-x", "-C", dir)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	tar.Stdin = pipe
	if err := archive.Start(); err != nil {
		t.Fatal(err)
	}
	if err := tar.Run(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Wait(); err != nil {
		t.Fatal(err)
	}
}

func requireCommits(t *testing.T, repo string, revs ...string) {
	t.Helper()
	for _, r := range revs {
		if _, err := run(t, repo, "git", "cat-file", "-e", r+"^{commit}"); err != nil {
			t.Skipf("commit %s is not in this clone (shallow checkout?), so the "+
				"historical merge cannot be reconstructed", r)
		}
	}
}

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
