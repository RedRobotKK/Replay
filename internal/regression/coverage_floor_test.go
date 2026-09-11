package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// FC-CG, frozen. The README shows a coverage badge, and a badge is a claim.
//
// The claim it makes is deliberately the FLOOR rather than the measurement.
// A badge reading "coverage 86%" is a number with no owner: it is true on the
// day it is pasted and drifts every commit after, which is the defect this
// repository has now corrected in a doc comment, a README table and five
// separate documents. A badge reading "coverage >=85%" is a claim CI enforces
// on every pull request, so it cannot rot — as long as the number the badge
// shows is the number the gate uses.
//
// This test is that "as long as". Two files hold one number:
//
//	.coverage-floor            read by scripts/coverage-gate.sh, which fails the build
//	README.md                  the badge a reader believes
//
// Raising the floor without updating the badge understates the guarantee.
// Raising the badge without the floor is worse: it advertises an enforcement
// that is not happening, which is a check that cannot fail wearing a green
// shield.
//
// SCOPE. This compares two files to each other. It does NOT run the suite,
// measure coverage, or verify that the gate is wired into CI — the workflow
// owns that, and a gate removed from ci.yml would leave both files agreeing
// about a floor nothing applies. That gap is real and named here rather than
// left to be discovered.
func TestFCCG_TheCoverageBadgeStatesTheEnforcedFloor(t *testing.T) {
	root := repoRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, ".coverage-floor"))
	if err != nil {
		t.Fatalf("reading .coverage-floor: %v\nThe badge in the README promises a "+
			"floor. If the file holding it is gone, nothing is enforcing one.", err)
	}
	floor := strings.TrimSpace(string(raw))
	if floor == "" {
		t.Fatal(".coverage-floor is empty, so the gate has no number to enforce")
	}

	readmeBytes, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(readmeBytes)

	// The badge URL encodes the floor: coverage-%E2%89%A5<n>%25 is "coverage-≥n%".
	m := regexp.MustCompile(`coverage-%E2%89%A5(\d+)%25`).FindStringSubmatch(text)
	if m == nil {
		t.Fatal("the README has no coverage badge of the expected shape, so this " +
			"test is comparing nothing. If the badge was deliberately removed, " +
			"remove this test with it and say so; if its URL changed, retarget " +
			"the pattern rather than deleting the check.")
	}
	if m[1] != floor {
		t.Errorf("the README badge advertises a floor of %s%% and "+
			".coverage-floor enforces %s%%.\n"+
			"If the badge is higher, it promises an enforcement that is not "+
			"happening. If it is lower, the project is understating a guarantee "+
			"it already meets. Either way the two must move together.", m[1], floor)
	}

	// And the gate must actually read that file, not a literal of its own.
	gate, err := os.ReadFile(filepath.Join(root, "scripts", "coverage-gate.sh"))
	if err != nil {
		t.Fatalf("reading the gate: %v", err)
	}
	if !strings.Contains(string(gate), ".coverage-floor") {
		t.Error("scripts/coverage-gate.sh does not mention .coverage-floor. If it " +
			"has grown its own hardcoded floor, the file this test compares " +
			"against is no longer the number being enforced.")
	}
}
