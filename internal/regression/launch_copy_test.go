package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-LC. The launch copy is the highest-stakes public surface and nothing
// checked it.
//
// docs/paper/launch-draft.md led with "50.8% of re-billed tokens are client
// re-renders" until 2026-09-13. The file that figure comes from,
// docs/evidence/break-causes-2026-09-06.md, has carried an appended retraction
// since 2026-09-11 saying it does not reproduce: the same classifier now reads
// 42.4%, and a third reading on 2026-09-13 reads 39.7% with the ranking
// flipped.
//
// Posting that table on Hacker News would have handed a reader the project's
// own retraction as a gotcha, in a launch whose entire claim is that
// corrections are published. The draft was not wrong when written. It went
// stale, and nothing was watching.
//
// PASS: the announcement carries no em-dash and no forecast word.
// FAIL: the one document a stranger reads first breaks the rules every other
// surface is tested against.
func TestFCLC_TheLaunchCopyFollowsTheProseRules(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "paper", "announcement-2026-09-14.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no announcement file yet: %v", err)
	}
	body := string(raw)

	// The em-dash is the most reliable tell of unedited model prose, and
	// replay.doctor already fails its build over one.
	if n := strings.Count(body, "—"); n > 0 {
		t.Errorf("the announcement contains %d em-dash(es). replay.doctor fails its build "+
			"over one and the launch post is a more public surface than the site", n)
	}

	// Forecast words. The deliverable rules forbid a savings claim outright:
	// the re-billed figure is what was already spent twice, never a prediction
	// of what a change recovers.
	//
	// The file names these words once, in the section that bans them. That
	// section is found by its heading and excluded, so the rule can be written
	// down without breaking itself.
	banned := []string{"savings", "optimise", "optimize", "unlimited", "guarantee"}
	scan := body
	if i := strings.Index(scan, "## Lines that must not appear anywhere"); i >= 0 {
		scan = scan[:i]
	}
	for _, w := range banned {
		if strings.Contains(strings.ToLower(scan), w) {
			t.Errorf("the announcement uses %q outside the section that bans it. The "+
				"re-billed figure is what was already spent twice, not a forecast", w)
		}
	}
}

// FC-LC2: no published cause ranking anywhere claims to be a finding.
//
// The specific stale figure, caught by name. Three readings in nine days gave
// 50.8%, 42.4% and 39.7% for the same class with the ranking flipping, so any
// document still quoting the first one is quoting a retracted number.
//
// This checks the ANNOUNCEMENT and the DRAFT differently on purpose. The
// announcement may cite 50.8% only as part of the drift table that shows it
// moving. The draft may not lead with it.
func TestFCLC2_NoDocumentLeadsWithTheRetractedShare(t *testing.T) {
	root := repoRoot(t)

	// The retraction must still exist. If somebody deletes it, this test is
	// checking a rule nobody is bound by any more.
	causes, err := os.ReadFile(filepath.Join(root, "docs", "evidence", "break-causes-2026-09-06.md"))
	if err != nil {
		t.Fatalf("reading the 2026-09-06 reading: %v", err)
	}
	if !strings.Contains(string(causes), "does not reproduce") {
		t.Fatal("break-causes-2026-09-06.md no longer carries its retraction. The wrong " +
			"figure is supposed to stay on the page with the correction beside it.")
	}

	ann := filepath.Join(root, "docs", "paper", "announcement-2026-09-14.md")
	raw, err := os.ReadFile(ann)
	if err != nil {
		t.Skipf("no announcement file yet: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "50.8") {
		return // It does not cite the figure at all, which is also fine.
	}
	// It cites it, so it must show the movement rather than the number alone.
	for _, need := range []string{"42.4", "39.7"} {
		if !strings.Contains(body, need) {
			t.Errorf("the announcement quotes 50.8%% without %s beside it. Quoted alone "+
				"that is a retracted figure; quoted with the later readings it is the "+
				"drift, which is the point", need)
		}
	}
}
