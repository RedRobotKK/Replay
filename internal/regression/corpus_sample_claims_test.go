package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// FC-CS, frozen. A withdrawn corpus figure was withdrawn in one file and left
// standing in six others.
//
// This is the third time the same defect has been written up here — FC-PX in
// a doc comment, FC-RB in a README section — but it is not the same failure.
// Those two were figures that aged. This one is a figure that was *retracted*
// and went on being published anyway, which is worse, because the project had
// already done the hard part and written the correction down.
//
// The sample size of the calibration corpus has been published four ways:
//
//	2026-09-03  11 sessions, all of them this repository's own development
//	2026-09-05  "Sessions: 1363" — the transcript-FILE count, not sessions
//	2026-09-06  retracted: 1450 transcripts from 78 distinct sessions, and
//	            one session supplied 1020 of those files by itself
//	2026-09-10  re-read: 1751 transcripts from 116 distinct sessions
//
// The 2026-09-06 retraction is thorough and well argued and lives in
// docs/evidence/calibration-corpus-2026-09-06.md. The 2026-09-10 re-reading
// exists for exactly the reason this test does: the README was quoting the
// 09-06 figure in the present tense four days after both the corpus and the
// engine had moved. The README was then fixed. Nothing else was.
//
// So on 2026-09-11, a whole-tree audit found "78" still standing as a
// present-tense claim in SPONSORS.md, docs/ROADMAP.md, docs/requirements.md,
// docs/WHAT-YOU-GET.md and docs/PRODUCT-DIRECTION.md, and "1363-session
// corpus" still naming the corpus in internal/ledger/response.go — a retracted
// number used as a proper noun. A reader meets those files long before they
// meet docs/evidence/, so the retraction reached the archive and never reached
// the audience.
//
// The repair was not the 2026-09-10 figures. 116 will rot the way 78 did and
// 1363 did, and this repo has now settled that twice. Each site states the
// shape instead — one machine, one account, one operator; a session writes one
// transcript per lane — and names docs/evidence/, where every reading carries
// the date it was taken on.
//
// WHICH NUMBER, because two of them look alike and only one is the withdrawal.
// 1363 was never a session count and 78 was: 1363 counted transcript files and
// called them sessions, which overstated the independent sample roughly
// twentyfold, while 78 was the honest session count for its date and became
// wrong only by standing still. Both are banned here for the same reason and
// not for the same mistake, and a reader who conflates them will think this
// test is about arithmetic. It is about what happens after a correction.
//
// SCOPE, stated because the next reader will otherwise overestimate this.
//
//   - It bans two specific withdrawn numbers in one explicit list of files.
//     It would NOT have caught "78" while 78 was current, and it will not
//     catch the next figure to go stale — 116, or whatever replaces it. A
//     figure becomes catchable here only once somebody retracts it and adds
//     a row to withdrawnCorpusFigures.
//   - It compares nothing against a corpus. Nothing in this build does.
//   - It does not read the Go tree beyond response.go. "1363" appears in a
//     dozen other Go files (cmd/replay/corpus.go, internal/analysis/outlier.go,
//     internal/tui/advise.go and others) and almost all of those mentions are
//     legitimately narrating the retraction, or are the unrelated "1363.2x"
//     outlier string. No pattern separates a figure being published from a
//     figure being discussed, so the file list is written by hand and kept
//     short rather than inferred.
//   - Banning the number bans the retraction *narrative* too, in these files.
//     That is deliberate. The narrative belongs in the dated evidence record
//     that owns it, which these files link to; retelling it in the front
//     matter is how the number got six homes in the first place.
//
// PASS: no file in the forward-facing set quotes a withdrawn corpus figure.
// FAIL: a retraction reached one file and not the rest, again.
func TestFCCS_NoForwardFacingDocCarriesAWithdrawnCorpusFigure(t *testing.T) {
	root := repoRoot(t)

	for _, path := range forwardFacingCorpusClaims {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("cannot read %s: %v.\nIf it was renamed, retarget this test; "+
				"if it was deleted, read this test's header before removing its row — "+
				"it is the record of why the figure must not come back.", path, err)
			continue
		}
		body := string(b)
		if strings.TrimSpace(body) == "" {
			t.Errorf("%s is empty, so this test read nothing and proved nothing", path)
			continue
		}

		for _, fig := range withdrawnCorpusFigures {
			for _, line := range matchingLines(path, body, fig.pattern) {
				t.Errorf("%s\n  %s was withdrawn on %s and is published here again.\n  %s\n\n"+
					"State the shape — one machine, one account, one operator, and a session "+
					"writes one transcript per lane — and name docs/evidence/, where each "+
					"reading carries the date it was taken on. A fresher count is not the "+
					"repair; it is the same defect with a later number in it.",
					line, fig.figure, fig.withdrawn, fig.why)
			}
		}
	}
}

// forwardFacingCorpusClaims are the files a reader meets before they meet
// docs/evidence/. README.md is not here: it was corrected on 2026-09-10 to
// name every figure's read-date, which is the other legitimate repair, and a
// ban would fight it.
var forwardFacingCorpusClaims = []string{
	"SPONSORS.md",
	"docs/ROADMAP.md",
	"docs/requirements.md",
	"docs/WHAT-YOU-GET.md",
	"docs/PRODUCT-DIRECTION.md",
	"internal/ledger/response.go",
}

// withdrawnCorpusFigure is one figure this project published, measured again,
// and took back. A retraction that lands in one file is not a retraction, so
// every withdrawal gets a row here and the row is what makes it stick.
type withdrawnCorpusFigure struct {
	figure    string
	withdrawn string
	why       string
	pattern   *regexp.Regexp
	// positive is a real line from the tree before the repair. It exists so
	// that a pattern edited into uselessness fails loudly instead of passing
	// on everything.
	positive string
}

var withdrawnCorpusFigures = []withdrawnCorpusFigure{
	{
		figure:    `"1363 sessions"`,
		withdrawn: "2026-09-06",
		why: "1363 counted transcript FILES and called them sessions. A session " +
			"writes one per agent lane and one session supplied 1020 of them, so " +
			"the published figure overstated the independent sample roughly " +
			"twentyfold.",
		pattern:  regexp.MustCompile(`\b1,?363\b`),
		positive: "// 1363-session corpus comes from. Anthropic has already added fields",
	},
	{
		figure:    `"78 sessions"`,
		withdrawn: "2026-09-10",
		why: "78 was the honest session count on 2026-09-06 and became wrong by " +
			"standing still: docs/evidence/calibration-corpus-2026-09-10.md re-read " +
			"the same corpus four days later and the sample had moved. It is banned " +
			"as a standing claim, not as arithmetic.",
		// Adjacency to the word, in either order, because the claim was written
		// both as "78 distinct sessions" and as a "| Sessions | **78** |" cell.
		// Bounded by the sentence so a 78 three clauses away is not a match.
		pattern: regexp.MustCompile(
			`(?i)(\b78\b[^.\n]{0,40}\bsessions?\b|\bsessions?\b[^.\n]{0,40}\b78\b)`),
		positive: "**Status:** confirmed on **78 distinct sessions**, read across 1450 transcript files",
	},
}

// A pattern that matches nothing matches nothing in the tree either, and this
// whole test would then be a green light for a defect it cannot see. Each row
// carries a line the tree actually held on 2026-09-11 and has to match it.
func TestFCCS_EveryWithdrawnFigurePatternStillMatchesItsOwnDefect(t *testing.T) {
	if len(withdrawnCorpusFigures) == 0 {
		t.Fatal("no withdrawn figures registered, so FC-CS checks nothing")
	}
	for _, fig := range withdrawnCorpusFigures {
		if fig.positive == "" {
			t.Errorf("%s carries no sample of the defect, so its pattern is unproven", fig.figure)
			continue
		}
		if !fig.pattern.MatchString(fig.positive) {
			t.Errorf("the pattern for %s no longer matches the line it was written for:\n  %s\n\n"+
				"It was loosened or broken, and FC-CS is now passing on a tree it cannot read.",
				fig.figure, fig.positive)
		}
	}
}

// matchingLines reports every line of body the pattern hits, with its 1-based
// line number, so a failure names the line rather than the file.
func matchingLines(path, body string, pattern *regexp.Regexp) []string {
	var out []string
	for i, line := range strings.Split(body, "\n") {
		m := pattern.FindString(line)
		if m == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 110 {
			trimmed = trimmed[:107] + "..."
		}
		out = append(out, path+":"+strconv.Itoa(i+1)+": "+m+"   in: "+trimmed)
	}
	return out
}
