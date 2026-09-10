package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A background check on what the ADRs claim versus what the tree actually
// holds — built, or hearsay.
//
// Back-checking the rules-feed ADRs on 2026-09-10 found ADR-0013 (Accepted)
// true to the code line for line and ADR-0019 (Proposed) entirely paper. That
// check was done by hand. This makes it automatic, and it fails in BOTH
// directions:
//
//   - an ADR that claims a symbol is built, where the tree has no such symbol,
//     is hearsay wearing an Accepted badge;
//   - an ADR marked Proposed/unbuilt whose symbol now EXISTS is built work
//     wearing the wrong status — the record has gone stale under the code.
//
// Freeform ADR prose cannot be parsed into claims without noise, and a noisy
// check that cries wolf gets switched off. So the claims are DECLARED here,
// the way the frozen-defect register declares its guards: an ADR that asserts
// something about the code earns a row, and the row is what this verifies. The
// discipline going forward is one line — an ADR that claims code carries a
// manifest entry, or this test has nothing to check and says so.

type adrClaim struct {
	adr    string // which ADR
	symbol string // a token that must appear (or not) in non-test Go source
	// present is what the ADR asserts: true = the ADR claims this is built,
	// false = the ADR states it does not exist yet.
	present bool
	why     string // the sentence the row defends
}

// The manifest. Seeded 2026-09-10 from the hand back-check; every entry was
// verified true when written. Add a row when an ADR makes a checkable claim
// about the code.
var adrClaims = []adrClaim{
	// ADR-0013, Accepted and built — verified present in non-test source.
	// (The pinned wallet address lives in cmd/replay/funding_address_test.go,
	// a test, and is guarded there; it is deliberately not asserted here,
	// which scans non-test source only.)
	{"0013", "func ParsePaymentRequired", true, "x402.go parses a 402's payment terms"},
	{"0013", "type PaymentRequired", true, "x402.go models the 402 response"},
	{"0013", "x402-json", true, "rules --update exposes --x402-json and never pays"},
	// ADR-0019, Proposed and unbuilt — verified ABSENT. When the capability
	// probe is built, these appear and this test goes red, forcing 0019's
	// status to move rather than drift out of sync with the code.
	{"0019", "type Capability", false, "0019's capability descriptor is not built yet"},
	{"0019", "ProbedAt", false, "0019's probe-date field is not built yet"},
}

// nonTestGoSource returns every non-test .go file's contents, concatenated.
func nonTestGoSource(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	var b strings.Builder
	for _, dir := range []string{"cmd", "internal"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, rerr := os.ReadFile(p)
			if rerr == nil {
				b.Write(body)
				b.WriteByte('\n')
			}
			return nil
		})
	}
	return b.String()
}

// ADR-1: every declared ADR claim matches the tree, built or absent.
func TestADR1_ClaimsMatchTheTree(t *testing.T) {
	src := nonTestGoSource(t)
	if len(src) < 10000 {
		t.Fatalf("read only %d bytes of source; the walk is not seeing the tree, so this "+
			"check would pass by looking at nothing", len(src))
	}
	for _, c := range adrClaims {
		found := strings.Contains(src, c.symbol)
		switch {
		case c.present && !found:
			t.Errorf("ADR-%s claims %q is built (%s), but no non-test source contains it. "+
				"Either the ADR is hearsay, or the symbol was renamed and the ADR — and this "+
				"row — must follow it.", c.adr, c.symbol, c.why)
		case !c.present && found:
			t.Errorf("ADR-%s states %q does not exist (%s), but the tree now contains it. "+
				"The work got built and the ADR's status is stale: move ADR-%s off Proposed "+
				"and flip this row to present.", c.adr, c.symbol, c.why, c.adr)
		}
	}
}

// ADR-2: every ADR that carries a "What was built" section names only files
// that exist. A build-claim naming a path the tree does not have is the
// cheapest possible hearsay to catch.
func TestADR2_BuildClaimFilesExist(t *testing.T) {
	root := repoRoot(t)
	adrDir := filepath.Join(root, "docs", "adr")
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		t.Fatalf("reading ADR dir: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(adrDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		// Only the sections that assert something was built.
		if !strings.Contains(text, "## What was built") && !strings.Contains(text, "## What is built") {
			continue
		}
		for _, tok := range goPathTokens(text) {
			checked++
			if _, err := os.Stat(filepath.Join(root, tok)); err != nil {
				t.Errorf("%s names %q in a build-claim section, and the tree has no such file: %v",
					e.Name(), tok, err)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no .go paths found in any build-claim section; the extractor is looking at " +
			"nothing, which is a check that cannot fail")
	}
}

// goPathTokens pulls backtick-quoted repo paths ending in .go out of prose.
func goPathTokens(text string) []string {
	var out []string
	for _, part := range strings.Split(text, "`") {
		if strings.HasSuffix(part, ".go") && strings.Contains(part, "/") &&
			!strings.ContainsAny(part, " \t\n") {
			out = append(out, part)
		}
	}
	return out
}
