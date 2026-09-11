//go:build ignore

// merge-guard compiles the result of merging this branch into its base,
// which no CI job here does.
//
// Every check in this repo runs against a branch head. Nothing runs against
// the merge, so two changes that are each green can produce a main that does
// not build. It has happened twice:
//
//	#157 with #160  a TUI row budget each respected alone, exceeded together
//	#184 with #185  one carried a file to main, the other added it renamed
//
// The second is the instructive one, and it is the test fixture. Its merge
// is CLEAN: git reports no conflict, writes a tree and exits zero. The two
// files do not overlap textually, because one is the other renamed and the
// rename's delete side was dropped in a rebase — git said so, in a
// "skipped previously applied commit" warning in the middle of an otherwise
// successful rebase. They collide only at the symbol level, and only in a
// _test.go, so `go build ./...` on the merged tree prints nothing.
//
// Usage:
//
//	go run scripts/merge-guard/main.go [base]
//
// base defaults to origin/main. Nothing is written to the working tree: the
// merge is computed with `git merge-tree --write-tree`, which creates a tree
// object and no commit, and the tree is extracted to a temporary directory
// that is removed on every exit path.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/RedRobotKK/Replay/internal/mergeguard"
)

func main() {
	base := "origin/main"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}

	head, err := output("git", "rev-parse", "HEAD")
	if err != nil {
		fail("cannot read HEAD: %v", err)
	}
	head = strings.TrimSpace(head)

	out, mergeErr := output("git", "merge-tree", "--write-tree", base, head)
	tree, conflicts := mergeguard.Conflicted(out, mergeErr)
	if len(conflicts) > 0 {
		fmt.Printf("merge-guard: %s and %s conflict in %d file(s):\n", base, short(head), len(conflicts))
		for _, p := range conflicts {
			fmt.Printf("  %s\n", p)
		}
		os.Exit(1)
	}
	if tree == "" {
		fail("merge-tree wrote no tree: %v\n%s", mergeErr, out)
	}

	dir, err := os.MkdirTemp("", "merge-guard-")
	if err != nil {
		fail("cannot make a scratch directory: %v", err)
	}
	// Unconditionally, including on every failure below.
	defer os.RemoveAll(dir)

	if err := extract(tree, dir); err != nil {
		fail("cannot extract the merged tree: %v", err)
	}

	fmt.Printf("merge-guard: %s into %s merges clean; checking that it compiles\n", short(head), base)
	bad := false
	for _, c := range mergeguard.Checks() {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		got, err := cmd.CombinedOutput()
		if err != nil {
			bad = true
			fmt.Printf("\n  %s FAILED on the merged tree:\n%s\n", strings.Join(c, " "), indent(string(got)))
			continue
		}
		fmt.Printf("  %s ok\n", strings.Join(c, " "))
	}
	if bad {
		fmt.Println("\nBoth branches are fine on their own. Their merge is not, and no other")
		fmt.Println("check in this repo looks at the merge. Rebase onto the base and read what")
		fmt.Println("git says it skipped.")
		os.Exit(1)
	}
	fmt.Println("merge-guard: the merge compiles")
}

func extract(tree, dir string) error {
	archive := exec.Command("git", "archive", tree)
	tar := exec.Command("tar", "-x", "-C", dir)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	tar.Stdin = pipe
	tar.Stderr = os.Stderr
	if err := archive.Start(); err != nil {
		return err
	}
	if err := tar.Run(); err != nil {
		_ = archive.Wait()
		return err
	}
	return archive.Wait()
}

func output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var b strings.Builder
	cmd.Stdout = &b
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return b.String(), err
}

func indent(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("    " + line + "\n")
	}
	return b.String()
}

func short(rev string) string {
	if len(rev) > 8 {
		return rev[:8]
	}
	return rev
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "merge-guard: "+format+"\n", args...)
	os.Exit(2)
}
