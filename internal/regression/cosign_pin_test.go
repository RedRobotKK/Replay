package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-COSIGN: every test package that can reach the signature check pins cosign.
//
// The v0.6.0 release build failed twice, on two different days, for one reason.
// `replay upgrade` decides whether to verify a Sigstore signature by asking the
// PATH for cosign. Tests that do not pin that decision are not testing the
// code, they are testing the machine:
//
//   - every CI runner in the matrix has no cosign, so they passed
//   - the release job installs cosign in order to sign, so they failed
//
// The release runner is therefore the only machine in this project's history
// that runs the suite with cosign present, and it is the machine that
// publishes. A green matrix said nothing about it.
//
// The first fix pinned the seam in internal/selfupdate. cmd/replay reaches the
// same code through the upgrade command, could not pin an unexported variable,
// and took the release down a second time. This test is the difference between
// fixing a defect where it was found and fixing it where it lives.
//
// PASS: every package whose tests import selfupdate also pins LookCosign.
// FAIL: a package can reach the signature check and inherits the machine, which
// is a green suite that says nothing about the one runner that matters.
func TestFCCOSIGN_EveryPackageReachingTheSignatureCheckPinsIt(t *testing.T) {
	root := repoRoot(t)

	type pkg struct{ imports, pins bool }
	pkgs := map[string]*pkg{}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipTreeDir(root, path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := string(body)
		dir := filepath.Dir(path)
		p := pkgs[dir]
		if p == nil {
			p = &pkg{}
			pkgs[dir] = p
		}
		// The package under test declares the seam itself, so an in-package
		// reference counts as reaching it just as an import does.
		if strings.Contains(src, "internal/selfupdate") || dir == filepath.Join(root, "internal", "selfupdate") {
			p.imports = true
		}
		// A pin is an assignment to the seam inside TestMain. Checking for the
		// assignment rather than for the word means a comment mentioning
		// cosign cannot satisfy this test, which is the way the last two of
		// these were satisfied by accident.
		if strings.Contains(src, "func TestMain(") &&
			(strings.Contains(src, "selfupdate.LookCosign = func(") ||
				strings.Contains(src, "LookCosign = func(")) {
			p.pins = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for dir, p := range pkgs {
		if !p.imports {
			continue
		}
		checked++
		if !p.pins {
			rel, _ := filepath.Rel(root, dir)
			t.Errorf("%s reaches the Sigstore check and does not pin cosign in a TestMain.\n"+
				"Its result is decided by whether cosign is on the PATH of the machine "+
				"running it, so it passes on every CI runner and fails on the release job, "+
				"which installs cosign in order to sign. Pin it:\n"+
				"    restore := selfupdate.LookCosign\n"+
				"    selfupdate.LookCosign = func() (string, error) { return \"\", errors.New(\"pinned absent\") }",
				rel)
		}
	}
	// Without this the test passes when the walk finds nothing, which is the
	// vacuous shape this repository keeps catching in its own guards.
	if checked < 2 {
		t.Fatalf("only %d package(s) were found to reach the signature check; expected at "+
			"least internal/selfupdate and cmd/replay, so this check is not looking where it thinks", checked)
	}
}
