package masking

import (
	"os"
	"path/filepath"
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
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if !insideProject(link, filepath.Join(target, "main.go")) {
		t.Error("a root given through a link did not match a real path under it")
	}
	if !insideProject(target, filepath.Join(link, "main.go")) {
		t.Error("a path given through a link did not match a real root")
	}
}

// 6. Case variance on a case-insensitive filesystem, where the platforms
// disagree and the harness found it.
//
// On darwin, filepath.Rel is case-sensitive, so SRC/app.js against a root of
// src returns "../SRC/app.js" and the path is refused. On windows Rel is
// case-insensitive, returns "app.js", and the path is accepted.
//
// Windows is the correct one. Both filesystems are case-insensitive, so
// SRC/app.js and src/app.js are the same file and that file is inside the
// project; darwin is over-refusing, which costs an unrestored placeholder and
// leaks nothing. Neither platform lets a secret out.
//
// So case-sensitivity is not the invariant worth asserting. This is: a path
// that resolves outside the project is refused, whatever its case.
func TestCaseVarianceNeverOpensAnEscape(t *testing.T) {
	project, outside := sandbox(t)
	parent := filepath.Dir(project)
	upperIn := filepath.Join(parent, strings.ToUpper(filepath.Base(project)), "app.js")
	upperOut := filepath.Join(parent, strings.ToUpper(filepath.Base(outside)), "shadow.env")

	// The security property, on every platform.
	if insideProject(project, upperOut) {
		t.Errorf("an out-of-project path was accepted in a different case: %s", upperOut)
	}
	// The platform difference, recorded rather than asserted either way.
	if insideProject(project, upperIn) {
		t.Logf("case-insensitive comparison: %s accepted under %s", upperIn, project)
	} else {
		t.Logf("case-sensitive comparison: %s refused under %s", upperIn, project)
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
