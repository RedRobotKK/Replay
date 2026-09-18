package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The house prose rule, in force since 2026-09-14, bans the em-dash (U+2014)
// and the en-dash (U+2013) from every public surface. Until now nothing in this
// tree enforced it on the file a stranger reads first.
//
// README.md alone carried 36 em-dashes across 31 lines on 2026-09-17, on a
// branch cut for launch. The rule was written down, agreed, and unguarded, so
// the only thing standing between it and the front page was whoever happened to
// read the diff. That is the shape ADR-0014 exists to refuse: a rule with no
// check is a preference, and a preference does not survive a release night.
//
// WHAT THIS TEST DOES NOT COVER. It guards the named list below and nothing
// else. Measured on 2026-09-17, immediately after README.md was cleaned, the
// repository still holds 1792 em-dashes and 54 en-dashes across its tracked
// Markdown: 98 of 156 tracked .md files contain at least one. docs/ holds 1670
// of the em-dashes on its own, and CHANGELOG.md holds 57 and 4 of the
// en-dashes. None of that is fixed by this file and none of it is failing this
// test. Widening the list means cleaning a file first, then naming it here; the
// list is explicit rather than a directory walk precisely so that adding a file
// is a decision somebody makes, not a sweep that silently goes red.
//
// The counts above are a dated reading, not a budget. They are recorded so the
// next person knows the scope of what is still unguarded rather than inferring
// from a green check that the tree is clean.
//
// PASS: every launch-facing document below is free of both dash characters.
// FAIL: the file names itself, the line number, and the offending line, so the
// fix is an edit rather than a hunt.

// launchFacingMarkdown is the explicit guarded list: the documents a stranger
// arriving at this repository reads before they read anything else. README.md
// is first because it is the surface the launch points at.
var launchFacingMarkdown = []string{
	"README.md",
	"CONTRIBUTING.md",
	"SUPPORT.md",
	"CODE_OF_CONDUCT.md",
}

// bannedDashes are the two characters the house rule forbids. The ASCII hyphen
// is deliberately absent: it is the replacement, not the defect.
var bannedDashes = []struct {
	r    rune
	name string
}{
	{'—', "em-dash (U+2014)"},
	{'–', "en-dash (U+2013)"},
}

func TestLaunchFacingMarkdownCarriesNoDashes(t *testing.T) {
	root := repoRoot(t)

	for _, rel := range launchFacingMarkdown {
		rel := rel
		t.Run(rel, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				// A guarded file that has gone missing is a failure, not a
				// skip. A list that quietly stops covering a file is the
				// unfalsifiable check this package exists to prevent.
				t.Fatalf("reading guarded document %s: %v", rel, err)
			}

			var hits int
			for n, line := range strings.Split(string(b), "\n") {
				for _, d := range bannedDashes {
					if !strings.ContainsRune(line, d.r) {
						continue
					}
					hits += strings.Count(line, string(d.r))
					t.Errorf("%s:%d carries an %s, banned on public surfaces since 2026-09-14:\n\t%s",
						rel, n+1, d.name, line)
				}
			}
			if hits > 0 {
				t.Logf("%s: %d banned dash character(s). Replace each by rewriting the "+
					"sentence, not by swapping in a comma everywhere: a colon where the "+
					"second half explains the first, parentheses around a true aside, or "+
					"a full stop where the sentence is doing too much.", rel, hits)
			}
		})
	}
}
