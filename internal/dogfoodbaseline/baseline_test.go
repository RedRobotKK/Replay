package dogfoodbaseline

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// The frozen block is internally consistent: its parts sum to its total.
//
// This is the strongest guarantee available for this artifact. Its source, the
// live ~/.claude/projects tree, has changed and the measurement cannot be
// re-run. What survives is whether the published numbers are consistent with
// each other, and that is checkable forever without the tree.
func TestFrozenBlockSumsToItsTotal(t *testing.T) {
	b := Frozen()
	if got := b.ComponentSum(); !sameCents(got, b.Total) {
		t.Errorf("components sum to %.2f, published total is %.2f", got, b.Total)
	}
}

// The three metrics the document says a forward comparison is allowed to use
// are entailed by the block, not independent assertions.
func TestPublishedRatesFollowFromTheBlock(t *testing.T) {
	b := Frozen()
	for _, tc := range []struct {
		name string
		got  float64
		want float64
	}{
		{"re-billed share of total spend", b.RebilledShare(), 3.768},
		{"cache-write share of cached spend", b.WriteShareOfCached(), 16.083},
		{"read-to-write ratio", b.ReadToWrite(), 5.22},
	} {
		if !closeTo(tc.got, tc.want) {
			t.Errorf("%s: computed %.4f, published %.3f", tc.name, tc.got, tc.want)
		}
	}
}

// The document states one total twice and they disagree by 17 cents.
//
// $17,738.82 is the one the components sum to. $17,738.65 appears once, beside
// the dominant session. The frozen evidence is not edited to resolve this; the
// disagreement is recorded so a reader meets it deliberately rather than by
// accident.
func TestTheDocumentsSecondTotalIsInconsistentAndRecorded(t *testing.T) {
	b := Frozen()
	if sameCents(b.SecondaryTotalAsWritten, b.Total) {
		t.Fatal("the two totals now agree; if the document was corrected, this " +
			"test and the erratum recording the discrepancy should go with it")
	}
	if diff := b.Total - b.SecondaryTotalAsWritten; !closeTo(diff, 0.17) {
		t.Errorf("the two totals differ by %.2f, want 0.17", diff)
	}
}

// The measurement is NOT reproducible, and that is a property of the artifact
// rather than a gap to be closed.
//
// Nothing in this package reads ~/.claude/projects. A test that did would be
// measuring today's tree and calling it the baseline, which is the exact error
// the document itself warns against: comparing totals across time measures the
// calendar.
func TestNothingHereReadsTheLiveTree(t *testing.T) {
	if Frozen().SourceReproducible {
		t.Error("the baseline claims to be reproducible; its source is a mutable " +
			"tree that has since changed and no harness can honestly re-derive it")
	}
}

// The no-harness rule, enforced against the import graph.
//
// The temptation this artifact invites is to point a reader at
// ~/.claude/projects and call the result the baseline. A comment saying "do
// not" is not a guarantee.
//
// An earlier version of this guard scanned the source for banned strings and
// failed on the package comment, which names the path legitimately. Imports
// are the right subject: a package that cannot reach the filesystem or the
// environment cannot grow a harness, whatever its prose says.
func TestPackageCannotGrowAReproductionHarness(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{`"math"`: true}
	var checked int
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			checked++
			for _, imp := range f.Imports {
				if !allowed[imp.Path.Value] {
					t.Errorf("%s imports %s. This package must not be able to read "+
						"the live tree: its source population no longer exists, and a "+
						"harness pointed at today's tree would measure a different "+
						"corpus and call it the 2026-09-25 baseline.", name, imp.Path.Value)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no non-test source files were parsed; this guard proves nothing")
	}
}
