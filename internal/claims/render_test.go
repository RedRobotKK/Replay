package claims

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CLAIM-REGISTER.md is a projection of this package, not a second source of
// truth. scripts/claim-register/main.go says so in its own header ("The
// register is the source. This only formats it, so the document cannot drift
// from the code: regenerate rather than edit.") and the generator itself
// prints "Do not edit by hand" into the file it writes.
//
// That promise held only as long as someone remembered to run the generator
// after editing Register or ControlsFor. RPL-C019's detector was corrected in
// internal/claims/controls.go (SP-10) without the register being regenerated:
// CLAIM-REGISTER.md went on describing a plain lexical grep for "TenantID"
// for two commits after the detector had been narrowed to exempt
// internal/tenancy.TenantID specifically. The document was not wrong about
// what Replay claims; it was wrong about how the claim is checked, which is
// exactly the distinction this register exists to preserve.
//
// PASS: CLAIM-REGISTER.md, byte for byte, is what Render() produces right now.
// FAIL: the register and the code it claims to project have drifted apart.
func TestRegisterMatchesRenderedSource(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "CLAIM-REGISTER.md")
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	rendered := Render()
	if string(onDisk) != rendered {
		t.Errorf("CLAIM-REGISTER.md does not match claims.Render(). The register is a "+
			"generated projection of internal/claims (scripts/claim-register/main.go): "+
			"run `go run scripts/claim-register/main.go` from the repository root and "+
			"commit the result, rather than hand-editing %s.", path)
	}
}

// The rendered register ended "---\n\n": a horizontal rule and then a blank
// line that nothing follows. Markdown lint reads that as a second consecutive
// blank line at the end of the file (MD012 at CLAIM-REGISTER.md:939, the line
// after the last one), and the only honest fix is in the generator, because
// the file itself is regenerated rather than edited.
//
// PASS: Render() ends with exactly one newline.
// FAIL: the generator leaves a trailing blank line in the file it writes.
func TestRenderEndsWithExactlyOneNewline(t *testing.T) {
	out := Render()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("Render() does not end with a newline")
	}
	if strings.HasSuffix(out, "\n\n") {
		t.Fatalf("Render() ends with a blank line after the final rule; the generated file carries it as MD012")
	}
}
