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
			case ".git", "testdata", "node_modules":
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
		out[rel] = string(body)
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
