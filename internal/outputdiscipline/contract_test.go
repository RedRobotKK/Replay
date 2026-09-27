package outputdiscipline

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// Fixture identity, pinned by content.
//
// The defect this package exists to fix was an unnamed run set. Naming it in a
// comment would repeat the mistake one level up, so the identity is asserted.
const (
	benchSHA256 = "00896480a43e2c79eb9b6583a47e2273d8b1a50f041aecbea16915877ac71a96"
	n10SHA256   = "ba635faf1104adc7b51f915355f272b4a7cae7f23f903581d4d7d2bc6041e02e"
)

func TestFixtureIdentityIsPinned(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"testdata/bench.json", benchSHA256},
		{"testdata/n10.json", n10SHA256},
	} {
		b, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		if got := hex.EncodeToString(sum[:]); got != tc.want {
			t.Errorf("%s sha256 = %s, want %s\n"+
				"      The run set behind a published table changed. Either this "+
				"is a deliberate re-derivation, in which case update the hash and "+
				"say so in the evidence file, or a published figure is being moved "+
				"by editing its evidence.", tc.path, got, tc.want)
		}
	}
}

// The two run sets are distinct and must not be conflated.
//
// One document, two tables, two fixture sets. Scoring the n=10 table against
// the 18-run set, or vice versa, is the error a single unnamed corpus invites.
func TestTheTwoRunSetsAreDistinct(t *testing.T) {
	bench, err := Load("testdata/bench.json")
	if err != nil {
		t.Fatal(err)
	}
	n10, err := Load("testdata/n10.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(bench) != 18 {
		t.Errorf("bench holds %d runs, want 18 (the document's headline count)", len(bench))
	}
	if len(n10) != 40 {
		t.Errorf("n10 holds %d runs, want 40", len(n10))
	}
	names := map[string]bool{}
	for _, r := range bench {
		names[r.Run] = true
	}
	var overlap int
	for _, r := range n10 {
		if names[r.Run] {
			overlap++
		}
	}
	if overlap != 0 {
		t.Errorf("%d runs appear in both sets; the tables would not be independent", overlap)
	}
	// Scoring one table against the other set must NOT produce the published
	// figure. If it did, naming the fixture set would buy nothing.
	if got := Spreads(bench)["haiku/verbose"].Mean; got == 54413 {
		t.Error("the 18-run set reproduces the n=10 table's mean; the two sets " +
			"are not actually distinguishable and this package proves nothing")
	}
}

// Normalisation 1: published means are ROUNDED, not truncated.
//
// The two real cases from the n10 set, and what this evidence can and cannot
// settle about the rule.
func TestPublishedMeansAreRoundedNotTruncated(t *testing.T) {
	// haiku verbose: exact mean 54,412.60, published 54,413. Truncation gives
	// 54,412, so rounding is load-bearing here.
	if got := roundHalfEven(54412.60); got != 54413 {
		t.Errorf("round(54412.60) = %d, want 54413 (published); truncation gives 54412", got)
	}
	// haiku bounded: exact mean 18,115.5, published 18,116. This is the only
	// published figure that lands on a half, and half-to-even and half-up BOTH
	// give 18,116. The document therefore does not discriminate between them,
	// and this test does not pretend otherwise.
	if got := roundHalfEven(18115.5); got != 18116 {
		t.Errorf("round(18115.5) = %d, want 18116 (published)", got)
	}
	// The convention actually implemented, recorded so a change is visible.
	// Nothing in this document depends on it.
	if got := roundHalfEven(0.5); got != 0 {
		t.Errorf("round(0.5) = %d, want 0 under half-to-even, which is what Go's "+
			"%%.0f does; this is unconstrained by the evidence and is pinned only "+
			"so a silent switch shows up", got)
	}
}

// Normalisation 2: the scored answer is the leading integer.
//
// Six of the 58 runs carry an appended note from an agmsg watcher that failed
// to start. Those runs are NOT excluded: the published table included them,
// and this package preserves what was published.
func TestAnswerReadsALeadingIntegerAndAdmitsWhenItCannot(t *testing.T) {
	bare := Run{Result: "60"}
	noted := Run{Result: "60\n\n(Note: the agmsg inbox watcher could not start.)"}
	bold := Run{Result: "**60**"}
	prose := Run{Result: "Counting the test functions in each file:\n\n1. bare_test.go: 6"}

	for _, r := range []Run{bare, noted} {
		a, ok := r.Answer()
		if !ok || a != 60 {
			t.Errorf("Answer() = %d, %v for %q; want 60, true", a, ok, r.Result)
		}
	}
	// Bold and prose are NOT parsed. No published figure scores them, and a
	// heuristic that dug 60 out of "**60**" would be inventing evidence.
	for _, r := range []Run{bold, prose} {
		if _, ok := r.Answer(); ok {
			t.Errorf("Answer() claimed to read %q; it must report failure rather "+
				"than guess at a form no published table scores", r.Result)
		}
	}
	if !bare.BareAnswer() {
		t.Error("a bare number was not recognised as bare")
	}
	for _, r := range []Run{noted, bold, prose} {
		if r.BareAnswer() {
			t.Errorf("%q was called bare; the surrounding text would be invisible", r.Result)
		}
	}
}

// The contaminated runs really are in the fixture, and really are counted.
func TestNonBareResultsArePresentAndCounted(t *testing.T) {
	bench, err := Load("testdata/bench.json")
	if err != nil {
		t.Fatal(err)
	}
	n10, err := Load("testdata/n10.json")
	if err != nil {
		t.Fatal(err)
	}
	var notBare int
	for _, r := range append(append([]Run{}, bench...), n10...) {
		if !r.BareAnswer() {
			notBare++
		}
	}
	// One appended note in bench, three markdown-bold and one prose answer and
	// one further note in n10. Six of 58. The count is part of what the
	// evidence discloses about how clean these runs were.
	if notBare != 6 {
		t.Errorf("runs whose result is not a bare number = %d, want 6", notBare)
	}
	// And they are inside the published medians, not filtered out.
	if got := Medians(bench)["opus/verbose"].N; got != 2 {
		t.Errorf("opus/verbose n = %d, want 2: the contaminated run is included, "+
			"as the published table included it", got)
	}
}
