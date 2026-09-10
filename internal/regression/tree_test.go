package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the test's working directory to the module root.
//
// Tests here read the repository as a whole — source comments, documents,
// release config — because that is where the claims being frozen live. A
// relative "../.." would break the moment this package moved.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the working directory; cannot locate the repository root")
		}
		dir = parent
	}
}

// textFiles returns every tracked text file with one of the given extensions,
// keyed by its path relative to the repository root.
//
// testdata is excluded on purpose. A fixture is a recording of what somebody
// else sent, not a claim this project is making, and holding fixtures to the
// prose rules here would either corrupt the recordings or teach everyone to
// silence the check.
func textFiles(t *testing.T, exts ...string) map[string]string {
	t.Helper()
	root := repoRoot(t)
	want := map[string]bool{}
	for _, e := range exts {
		want[e] = true
	}
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			// .claude holds agent worktrees: whole copies of this
			// repository. Without it every frozen claim is checked once per
			// worktree, and a claim retracted here is reported as live because
			// a copy of the retraction still contains the words it retracts.
			// Every other tree-walk in this project already excludes it.
			case ".git", ".claude", "testdata", "node_modules", "vendor", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if !want[filepath.Ext(path)] {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// Keys are forward-slash on every host. Callers compare them against
		// literals like "cmd/replay/othersurfaces.go", and a backslash key
		// makes that lookup miss on Windows — silently, since a missing key
		// reads as a missing file. FD-11 shipped with exactly that bug and
		// reported the file it was looking for as deleted, which is a
		// confident answer to a question nobody asked.
		out[filepath.ToSlash(rel)] = string(body)
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("found no %v files under %s; the walk is broken, not the tree", exts, root)
	}
	return out
}

// paragraphs splits a file into blocks separated by blank lines, treating a
// bare comment marker as blank so a Go doc comment breaks where it reads as
// breaking.
//
// Paragraph scope, not file scope and not line scope. File scope passes as soon
// as the right word appears anywhere, which is how a check stops being able to
// fail; line scope misses a claim spread over two lines, which is how a claim
// gets past one.
func paragraphs(body string) []string {
	var out []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "//" || trimmed == "#" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return out
}

// containsAny reports whether s holds any of the needles, case-insensitively.
func containsAny(s string, needles ...string) (string, bool) {
	low := strings.ToLower(s)
	for _, n := range needles {
		if strings.Contains(low, strings.ToLower(n)) {
			return n, true
		}
	}
	return "", false
}

// textFiles keys are forward-slash on every host.
//
// Callers compare those keys against literals like
// "cmd/replay/othersurfaces.go". On Windows the walk produced backslash keys,
// so the lookup missed and FD-11 reported the file it was looking for as
// deleted — a confident diagnosis of something that had not happened, which is
// worse than a plain failure because it sends the reader after the wrong
// defect. It passed on macOS and Linux and failed only on the host nobody
// runs locally.
//
// The fix belongs in textFiles rather than at each call site, and this is what
// makes that true for the next caller as well as the current ones.
//
// PASS: no key carries a host separator.
// FAIL: one does, and every literal path comparison in this package is a
// coin flip decided by which machine ran it.
func TestTextFilesKeysAreSlashSeparated(t *testing.T) {
	files := textFiles(t, ".go")
	// A walk that found nothing would pass this by measuring nothing.
	if len(files) < 10 {
		t.Fatalf("textFiles returned %d files; the walk is broken and this test "+
			"proves nothing", len(files))
	}

	var nested int
	for path := range files {
		if strings.ContainsRune(path, '\\') {
			t.Errorf("%q carries a backslash, so a literal path lookup misses on this "+
				"host and reads as a missing file", path)
		}
		if strings.Contains(path, "/") {
			nested++
		}
	}
	// Keys must be relative paths into subdirectories, not bare file names:
	// if the walk returned only base names, the check above would pass while
	// every path comparison in this package still failed.
	if nested == 0 {
		t.Error("no key names a subdirectory, so these are not repository-relative " +
			"paths and the separator check above asserts nothing")
	}
}
