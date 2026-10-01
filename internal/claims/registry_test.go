package claims

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The register is only worth having if it cannot drift away from the code.
// These are the tests that make it a harness rather than a list, in the same
// spirit as internal/regression's frozen-defect register.

func TestEveryClaimStatesItsBoundary(t *testing.T) {
	for _, c := range Register {
		if len(c.DoesNotEstablish) == 0 {
			t.Errorf("%s has no DoesNotEstablish. The boundary is the first thing to go missing "+
				"and the most expensive thing to lose; a claim without one is marketing copy.", c.ID)
		}
		if strings.TrimSpace(c.Why) == "" {
			t.Errorf("%s has no Why. A result without a reason cannot be audited.", c.ID)
		}
		if strings.TrimSpace(c.Scope) == "" {
			t.Errorf("%s has no Scope, so there is no condition under which it could be false.", c.ID)
		}
	}
}

func TestClaimIDsAreUniqueAndWellFormed(t *testing.T) {
	shape := regexp.MustCompile(`^RPL-C\d{3}$`)
	seen := map[string]bool{}
	for _, c := range Register {
		if !shape.MatchString(c.ID) {
			t.Errorf("malformed claim id %q", c.ID)
		}
		if seen[c.ID] {
			t.Errorf("duplicate claim id %q", c.ID)
		}
		seen[c.ID] = true
	}
}

// A claim marked ESTABLISHED on the strength of the thing it is about is not
// established. This is rule 10 of the campaign, enforced rather than promised.
func TestNothingIsEstablishedOnItsOwnAuthority(t *testing.T) {
	for _, c := range Register {
		if c.Result != Established {
			continue
		}
		if strings.TrimSpace(c.Oracle) == "" || strings.EqualFold(c.Oracle, "self") {
			t.Errorf("%s is ESTABLISHED with oracle %q. An oracle that is the system under test "+
				"cannot establish anything.", c.ID, c.Oracle)
		}
	}
}

// A claim that asserts something must point at where it is asserted, or be
// explicitly marked as a non-claim. A claim with neither is one somebody
// believes without the product ever having said it.
func TestAClaimEitherCitesItsAssertionOrIsANonClaim(t *testing.T) {
	for _, c := range Register {
		nonClaim := c.Result == NoEndpoint || c.Result == NotMeasured || c.Result == DeliberateNonClaim
		if len(c.Asserted) == 0 && !nonClaim {
			t.Errorf("%s cites no assertion site but is not a non-claim (result %s). "+
				"Either the product says this somewhere, or nobody should believe it.", c.ID, c.Result)
		}
	}
}

// Every test name a claim leans on must exist. Without this the register
// decays into aspiration the first time a test is renamed.
func TestEveryClaimNamesTestsThatExist(t *testing.T) {
	have := allTestFuncNames(t)
	for _, c := range Register {
		if len(c.Tests) == 0 {
			t.Errorf("%s names no test", c.ID)
			continue
		}
		for _, name := range c.Tests {
			if !have[name] {
				t.Errorf("%s names test %s, which does not exist in the tree", c.ID, name)
			}
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found above working directory")
	return ""
}

func allTestFuncNames(t *testing.T) map[string]bool {
	t.Helper()
	root := repoRoot(t)
	names := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") {
				names[fn.Name.Name] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 100 {
		t.Fatalf("only found %d test functions; the walk is broken and this check "+
			"would pass vacuously", len(names))
	}
	return names
}

// Every claim must state how it could fail. A claim with no controls is an
// opinion with an id.
func TestEveryClaimHasControls(t *testing.T) {
	for _, c := range Register {
		ctl, ok := ControlsFor[c.ID]
		if !ok {
			t.Errorf("%s has no entry in ControlsFor. Nobody has said how it could fail.", c.ID)
			continue
		}
		if ctl.Basis == "" {
			t.Errorf("%s states no evidence basis, so a reader cannot tell whether the "+
				"figure it concerns is observed or reconstructed", c.ID)
		}
		if ctl.Layer == "" {
			t.Errorf("%s does not say which vocabulary decides it", c.ID)
		}
		// A claim that concluded something positive needs both controls. A
		// non-claim legitimately has neither, because there is nothing to
		// discriminate.
		nonClaim := c.Result == NoEndpoint || c.Result == DeliberateNonClaim
		if !nonClaim {
			if ctl.Positive == "" {
				t.Errorf("%s has no positive control", c.ID)
			}
			if ctl.Negative == "" {
				t.Errorf("%s has no negative control, so its check may pass against "+
					"anything", c.ID)
			}
		}
	}
	// And nothing may be registered in ControlsFor that is not a claim.
	ids := map[string]bool{}
	for _, c := range Register {
		ids[c.ID] = true
	}
	for id := range ControlsFor {
		if !ids[id] {
			t.Errorf("ControlsFor has an entry for %s, which is not a registered claim", id)
		}
	}
}

// An assumption is an invariant the claim rests on that Replay does not
// verify. Where one exists it must be stated in the claim's boundary too, so
// a reader of DoesNotEstablish alone is not misled.
func TestAnAssumptionIsVisibleInTheBoundary(t *testing.T) {
	for _, c := range Register {
		ctl := ControlsFor[c.ID]
		if len(ctl.Assumption) == 0 {
			continue
		}
		var boundary string
		for _, d := range c.DoesNotEstablish {
			boundary += d + " "
		}
		if !strings.Contains(strings.ToLower(boundary), "assum") &&
			!strings.Contains(strings.ToLower(c.Why), "assum") {
			t.Errorf("%s rests on a named assumption but neither its boundary nor its "+
				"Why mentions one. A reader of the claim alone would take it as "+
				"established.", c.ID)
		}
	}
}
