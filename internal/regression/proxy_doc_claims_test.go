package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// F2, frozen. The proxy's package comment denied what the package does.
//
// internal/proxy/server.go opened with "Nothing here rewrites a request body
// or removes a client header" while the same file masked secrets, applied the
// context-edit policy, added include-usage, and stripped two client headers.
// It was recorded in docs/design/surface-taxonomy-1-request.md as a
// disagreement between code and doc and left standing for days.
//
// It mattered beyond tidiness. A reader deciding whether to put this binary on
// their wire reads that paragraph first, and it told them the proxy is a
// pass-through. Every mutation it performs is defensible; none of them
// survives being discovered by a reader who was told they did not happen.
//
// The claim is easy to reintroduce, because it is what a careful engineer
// WANTS to be true of a measuring proxy, and it was true when first written.
//
// PASS: no source file claims the proxy leaves requests untouched.
// FAIL: the denial is back, in a package that still mutates.
func TestF2_TheProxyDocDoesNotDenyWhatTheProxyDoes(t *testing.T) {
	root := repoRootFor(t)

	// Each pattern carries a positive: the line the tree actually held before
	// the repair. A pattern edited into uselessness then fails loudly instead
	// of passing on everything. Borrowed from the withdrawn-figure gate, which
	// is the only scan in this repository that had it first.
	claims := []struct {
		pattern  *regexp.Regexp
		positive string
		why      string
	}{{
		pattern:  regexp.MustCompile(`(?i)nothing here rewrites a request`),
		positive: "// Nothing here rewrites a request body or removes a client header. The",
		why:      "masking, context-edit and include-usage all rewrite the body",
	}}

	// ONE PATTERN, not two, and the second one is worth its epitaph.
	//
	// I wrote a pattern for "forwards requests to the provider byte for byte",
	// the other half of the same false claim. It could not work, for two
	// reasons that are both this test's subject:
	//
	//   - The sentence WRAPS. "byte for byte" begins the second line, so no
	//     single line holds the phrase and a line-scanner cannot see it. That
	//     is the scan-scope defect this repository has now shipped twice.
	//   - "byte for byte" is TRUE elsewhere: three sites use it correctly to
	//     say an unparsed body passes through unchanged. Guarding the words
	//     would flag honest prose.
	//
	// A pattern that cannot match its own defect, and would flag correct text
	// if it could, is worse than no pattern. It is dropped rather than
	// weakened, and the drop is recorded so the next person does not spend the
	// same ten minutes discovering it.

	// The patterns have to still match the defect they were written for, or
	// this test is a green light for a claim it can no longer see.
	for _, c := range claims {
		if !c.pattern.MatchString(c.positive) {
			t.Errorf("pattern %v no longer matches its own defect:\n  %s", c.pattern, c.positive)
		}
	}

	dir := filepath.Join(root, "internal", "proxy")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	scanned := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		scanned++
		for i, line := range strings.Split(string(body), "\n") {
			trimmed := strings.TrimSpace(line)
			// Comments only. The corrected doc QUOTES the old sentence to
			// explain it, so a whole-file match would flag the repair itself —
			// which is the hole two other scans in this repo shipped with.
			if !strings.HasPrefix(trimmed, "//") {
				continue
			}
			// The quoting line is the one place the old words may still appear.
			if strings.Contains(trimmed, "said it did") || strings.Contains(trimmed, "The line read") {
				continue
			}
			for _, c := range claims {
				if c.pattern.MatchString(trimmed) {
					t.Errorf("%s:%d claims the proxy leaves requests untouched: %s\n"+
						"      %s", e.Name(), i+1, trimmed, c.why)
				}
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no Go files scanned in internal/proxy; this test asserts nothing")
	}
}
