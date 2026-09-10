package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Three claims the README makes about itself that nothing could check.
//
// On 2026-09-10 four front-page claims were found stale at once: the command
// count, the number of network requests the binary originates, the calibration
// figure, and a promise about how evidence files are edited. One of the four —
// the command count — was catchable, because docs/CLI.md is generated from the
// binary. The other three were prose, and prose is not read by any test here,
// so they drifted for as long as nobody re-checked them by hand.
//
// These guards do not verify that the README is right. They verify that it
// cannot disagree with a generated document, with the evidence it cites, or
// with a retraction this project has already published.

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

var cliOutboundRow = regexp.MustCompile(`\|\s*\[` + "`" + `([a-z-]+)` + "`" + `\]\([^)]*\)\s*\|[^|]*\|([^|]*)\|`)

// DC-1: every command docs/CLI.md marks outbound is named where the footprint
// is described.
//
// docs/CLI.md is generated from the binary, so its command table cannot drift
// from what the tool accepts. The README's footprint bullet and the surface map
// are hand-written, and on 2026-09-10 both were wrong in different directions:
// the README said the binary "originates two network requests" and named
// neither `upgrade` nor `rules --update`, while docs/SURFACES.md documented
// `upgrade` in its table and denied it in its own summary paragraph. Three
// documents, three different answers, and the one generated from the binary was
// not consulted by any of them.
func TestDC1_OutboundCommandsAreNamedInTheFootprint(t *testing.T) {
	cli := readDoc(t, "docs/CLI.md")
	readme := readDoc(t, "README.md")
	surfaces := readDoc(t, "docs/SURFACES.md")

	var outbound []string
	for _, m := range cliOutboundRow.FindAllStringSubmatch(cli, -1) {
		if strings.Contains(strings.ToLower(m[2]), "outbound") {
			outbound = append(outbound, m[1])
		}
	}
	// Anti-vacuity: this guard is worthless if the table stopped parsing.
	// Four commands carried an outbound marking when it was written.
	if len(outbound) < 3 {
		t.Fatalf("parsed %d outbound commands from docs/CLI.md (%v); the table format "+
			"changed and this guard stopped reading it", len(outbound), outbound)
	}
	for _, cmd := range outbound {
		named := regexp.MustCompile(`\b` + regexp.QuoteMeta(cmd) + `\b`)
		if !named.MatchString(readme) {
			t.Errorf("docs/CLI.md marks %q outbound and README.md never names it. "+
				"The generated reference is the one read from the binary; a footprint "+
				"section that omits a command it lists is the drift that made "+
				"\"two network requests\" survive four commands", cmd)
		}
		if !named.MatchString(surfaces) {
			t.Errorf("docs/CLI.md marks %q outbound and docs/SURFACES.md never names it. "+
				"That page's whole purpose is completeness", cmd)
		}
	}
}

var (
	trustFigure   = regexp.MustCompile(`reproduces the provider's own cache reads on \*\*([0-9.]+)%\*\* of compared turns across ([0-9,]+)\s*\ntranscripts`)
	trustSessions = regexp.MustCompile(`\*\*([0-9,]+) distinct sessions`)
	evidenceLink  = regexp.MustCompile(`\(docs/evidence/(calibration-corpus-[0-9-]+\.md)\)`)
)

// DC-2: the calibration figure on the front page must appear in a dated
// evidence file the same page links.
//
// The README carried "97.46% of compared turns across 1450 transcripts" in the
// present tense for four days after both the corpus and the engine had moved.
// Nothing was wrong with the arithmetic; the figure was a snapshot presented as
// a current reading, and no test could tell the difference because no test read
// either number. Re-running the engine gave 97.79% across 1751 transcripts from
// 116 sessions.
//
// This does not check that the figure is current — nothing offline can, since
// reproducing it needs the maintainer's own transcripts. It checks that the
// number on the front page is a number some dated file actually recorded, so
// changing one without the other fails.
func TestDC2_TheCalibrationFigureIsCitedFromEvidence(t *testing.T) {
	readme := readDoc(t, "README.md")

	fig := trustFigure.FindStringSubmatch(readme)
	if fig == nil {
		t.Fatalf("no calibration figure found in README.md; if the sentence was reworded, " +
			"reword this guard with it rather than deleting it")
	}
	rate, transcripts := fig[1], strings.ReplaceAll(fig[2], ",", "")

	sess := trustSessions.FindStringSubmatch(readme)
	if sess == nil {
		t.Fatalf("the calibration paragraph names no distinct-session count")
	}
	sessions := strings.ReplaceAll(sess[1], ",", "")

	links := evidenceLink.FindAllStringSubmatch(readme, -1)
	if len(links) == 0 {
		t.Fatalf("the calibration paragraph cites no docs/evidence/calibration-corpus-*.md file")
	}

	var cited []string
	for _, l := range links {
		cited = append(cited, l[1])
		body := readDoc(t, "docs/evidence/"+l[1])
		if strings.Contains(body, rate+"%") &&
			strings.Contains(body, transcripts) &&
			strings.Contains(body, sessions) {
			return // one cited file records all three; that is the citation.
		}
	}
	t.Errorf("README.md reports %s%% across %s transcripts from %s sessions, and none of the "+
		"evidence files it cites (%v) records all three. Either the front page moved without "+
		"the evidence, or a new reading needs a new dated file",
		rate, transcripts, sessions, cited)
}

// DC-3, frozen. "Never edited after the fact" was not true of the history.
//
// The README and docs/evidence/README.md both promised that evidence files are
// dated and never edited after the fact, and that corrections are new files.
// Sixteen of the twenty-five dated files carry more than one commit. Most of
// those edits append a retraction, which is the intended shape; several replace
// instead — rehydration-boundary-2026-09-05.md lost an eight-line section at
// ad7e884, and routing-baseline-2026-09-06.md lost a published 85.10% at
// a6b259b.
//
// This package cannot shell out to git to re-measure that — os/exec is confined
// to the mutation build tag — so the guard freezes the retraction instead: the
// promise must not come back except inside the sentence that withdraws it.
//
// PASS: the phrase appears only in a paragraph that also withdraws it.
// FAIL: it is asserted again as a promise.
func TestDC3_TheEvidenceImmutabilityPromiseStaysRetracted(t *testing.T) {
	const claim = "never edited after the fact"
	// A paragraph carrying any of these is withdrawing the promise, not making it.
	withdrawn := []string{"is not true", "has not held", "not true of the history",
		"was retracted", "until 2026-09-10"}
	// The promise is only this project's to break about its own measurements.
	// docs/maintainers.md states the same rule for PRDs and reviews, which is a
	// policy for a different class of document and is not what drifted.
	aboutEvidence := []string{"docs/evidence", "evidence file", "calibration", "measurement"}

	for path, body := range textFiles(t, ".md") {
		for _, p := range paragraphs(body) {
			if _, ok := containsAny(p, claim); !ok {
				continue
			}
			if _, ok := containsAny(p, withdrawn...); ok {
				continue
			}
			if _, ok := containsAny(p, aboutEvidence...); !ok {
				continue
			}
			t.Errorf("%s asserts %q as a promise:\n\n%s\n\n"+
				"Sixteen of twenty-five dated evidence files carry more than one commit, "+
				"and several replace the original claim rather than appending to it. If the "+
				"practice has since been made true, delete this guard in the commit that "+
				"makes it true and say how it is enforced", path, claim, p)
		}
	}
}
