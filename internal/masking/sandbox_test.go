package masking

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// An adversarial matrix against the project boundary that decides whether a
// real credential may be written to a path an agent chose.
//
// The threat is a poisoned agent, not a local attacker: content the model read
// from a web page or a dependency persuades it to write a secret somewhere it
// should not go. Every vector below is something such an agent can express in
// an ordinary tool call.
//
// The boundary fails closed. A wrong rejection leaves a placeholder in a file,
// which is visible and annoying. A wrong acceptance writes a live credential
// outside the project, which is neither.

// sandbox builds a project with an escape route beside it.
func sandbox(t *testing.T) (project, outside string) {
	t.Helper()
	root := t.TempDir()
	project = filepath.Join(root, "src")
	outside = filepath.Join(root, "unsafe")
	for _, d := range []string{project, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "shadow.env"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return project, outside
}

// 1. Dot-dot traversal, the obvious one, in several spellings.
func TestBoundaryRefusesDotDotTraversal(t *testing.T) {
	project, outside := sandbox(t)
	for _, p := range []string{
		filepath.Join(project, "..", "unsafe", "shadow.env"),
		project + "/../unsafe/shadow.env",
		project + "/a/b/../../../unsafe/shadow.env",
		"../unsafe/shadow.env", // relative, taken from the project
		filepath.Join(outside, "shadow.env"),
	} {
		if insideProject(project, p) {
			t.Errorf("escaped the project: %s", p)
		}
	}
}

// 2. A symlink inside the project pointing out of it. The agent creates the
// link with one tool call and writes through it with the next, so the path it
// submits looks entirely in-project.
func TestBoundaryRefusesASymlinkOutOfTheProject(t *testing.T) {
	project, outside := sandbox(t)
	link := filepath.Join(project, "innocent")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	target := filepath.Join(link, "shadow.env")
	if insideProject(project, target) {
		t.Errorf("wrote through a symlink out of the project: %s", target)
	}
	// The link itself is in-project, so naive prefix matching would accept it.
	if !strings.HasPrefix(target, project) {
		t.Fatal("fixture wrong: the submitted path should look in-project")
	}
}

// 3. A path whose tail does not exist yet, which is the normal case for a file
// the agent is about to create. The escape must still be seen.
func TestBoundaryRefusesAnEscapeThroughAPathThatDoesNotExistYet(t *testing.T) {
	project, _ := sandbox(t)
	for _, p := range []string{
		filepath.Join(project, "ghost", "..", "..", "unsafe", "shadow.env"),
		filepath.Join(project, "a", "b", "c", "..", "..", "..", "..", "unsafe", "x"),
	} {
		if insideProject(project, p) {
			t.Errorf("a non-existent tail hid an escape: %s", p)
		}
	}
	// And a genuinely new file inside the project is still allowed, or the
	// boundary would refuse every file an agent creates.
	if !insideProject(project, filepath.Join(project, "new", "file.go")) {
		t.Error("a new in-project file was refused; the boundary is unusable")
	}
}

// 4. A placeholder in the path. An agent that echoes a masked value into a
// filename must not have it restored on the way past.
func TestBoundaryRefusesAPlaceholderInThePath(t *testing.T) {
	project, _ := sandbox(t)
	p := filepath.Join(project, PlaceholderPrefix+"0123456789abcdef")
	if insideProject(project, p) {
		t.Errorf("a placeholder in the path was accepted: %s", p)
	}
}

// 5. The project root reached through a link, which is the ordinary macOS
// case: /var is a link to /private/var, so a root given one way and a tool
// path given the other must still compare equal or nothing works there.
func TestTheProjectRootItselfMayBeALink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if !insideProject(link, filepath.Join(real, "main.go")) {
		t.Error("a root given through a link did not match a real path under it")
	}
	if !insideProject(real, filepath.Join(link, "main.go")) {
		t.Error("a path given through a link did not match a real root")
	}
}

// 6. Case variance on a case-insensitive filesystem. This test records what the
// boundary does rather than asserting what would be convenient.
//
// On macOS, writing to SRC/x writes to src/x, so a case-insensitive check would
// be more permissive and a case-sensitive one refuses. Refusing is the correct
// direction for a boundary that decides where a live credential may be written:
// the cost is an unrestored placeholder in a file, which someone will notice.
// Widening the comparison to match the filesystem would trade that for the
// chance of writing a credential somewhere the operator did not name.
func TestCaseVarianceFailsClosed(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("case-sensitive filesystem; there is no collision to test")
	}
	project, _ := sandbox(t)
	upper := filepath.Join(filepath.Dir(project), strings.ToUpper(filepath.Base(project)))
	got := insideProject(project, filepath.Join(upper, "app.js"))
	if got {
		t.Logf("case-insensitive: %s accepted under %s", upper, project)
	} else {
		t.Logf("fails closed: %s refused under %s", upper, project)
	}
	// Either behaviour is safe to ship; only one is safe to be wrong about,
	// so the boundary must never widen silently. Pin whichever it is.
	if got {
		t.Error("case variance was accepted: the boundary is wider than the operator wrote, " +
			"and a credential can be written through a path they did not name")
	}
}

// 7. The decoy field. A file-edit tool carries its own path key plus another
// path-like field; the real target must not be laundered by an innocent decoy.
func TestEveryPathFieldMustBeInsideNotJustTheToolsOwn(t *testing.T) {
	project, outside := sandbox(t)
	in := filepath.Join(project, "ok.go")
	out := filepath.Join(outside, "shadow.env")
	if allInside(project, map[string]string{"file_path": in, "path": out}) {
		t.Error("a decoy in-project path opened the scope for an out-of-project one")
	}
	if !allInside(project, map[string]string{"file_path": in, "path": filepath.Join(project, "b.go")}) {
		t.Error("two in-project paths were refused")
	}
}
