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
		// Keyed with forward slashes on every platform.
		//
		// filepath.Rel returns the OS separator, so on Windows this map was keyed
		// `cmd\replay\x.go` while every guard that looks a file up writes the
		// literal `cmd/replay/x.go`. The lookup missed, and FD-11 reported the
		// detector it could not find as DELETED — a frozen guard failing on a
		// defect nobody had reintroduced, on the one platform nobody reads the
		// logs for. Every consumer of this map compares against forward slashes,
		// including the paths printed in their failure messages.
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

// textFiles keys with forward slashes, and every guard looks up that way.
//
// This is the contract FD-11 broke on. filepath.Rel returns the OS separator,
// so on Windows the map was keyed `cmd\replay\othersurfaces.go` while the guard
// looked up the literal `cmd/replay/othersurfaces.go`. The lookup missed and the
// guard reported the file as DELETED — a frozen defect failing on a defect
// nobody had reintroduced. It failed on main for at least two days before
// anybody read a windows-latest log.
//
// **Half of this test cannot fail on Unix and that is stated rather than
// hidden.** filepath.ToSlash is the identity function where the separator is
// already '/', so the no-backslash assertion below is vacuous on macOS and
// Linux and is real only on the windows-latest runner. The other half — that
// the paths guards actually look up resolve — fails anywhere, and is what
// catches a rename.
func TestKeysAreSlashSeparatedAndResolve(t *testing.T) {
	files := textFiles(t, ".go")
	if len(files) < 50 {
		t.Fatalf("textFiles returned %d files; it is not reading the tree and every "+
			"assertion below is vacuous", len(files))
	}

	// Vacuous on Unix, load-bearing on Windows.
	for path := range files {
		if strings.ContainsRune(path, '\\') {
			t.Errorf("%q is keyed with a backslash; every guard in this package looks up "+
				"a forward-slash literal and will miss it", path)
		}
	}

	// Falsifiable everywhere: the lookups guards actually perform.
	for _, want := range []string{
		"cmd/replay/othersurfaces.go",
		"internal/analysis/predictor.go",
		"internal/proxy/preflight.go",
	} {
		if _, ok := files[want]; !ok {
			t.Errorf("%s does not resolve in the file map. Either it was renamed and a "+
				"guard that looks it up now fails for the wrong reason, or the keying "+
				"changed and every path lookup in this package is broken", want)
		}
	}
}
