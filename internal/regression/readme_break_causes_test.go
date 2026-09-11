package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// FC-RB, frozen. The README's "What actually broke the cache" section
// carried a five-row table of break-cause shares. Three readings in five
// days, the last two hours apart on the same day:
//
//	2026-09-06  1,506 transcripts  735 breaks  re-render 50.8%  TTL 33.9%
//	2026-09-11  1,816 transcripts  806 breaks  re-render 42.4%  TTL 42.6%
//	2026-09-11  1,874 transcripts  828 breaks  re-render 41.0%  TTL 41.2%
//
// The corpus is live — it is this machine's own transcripts — so it grew by
// 58 between the last two, which is why a same-day re-read moved the figures
// at all. That is the point rather than a caveat: a share measured on a
// corpus that grows while you work has a shelf life measured in hours.
//
// The last reading puts the two leading causes in the OPPOSITE order to the
// one the README published, and the section's conclusion — that the shapes matter more than
// the ranking — was right about the half that held and wrong to wave away
// the half that flipped.
//
// This is the same finding as FC-PX one file over, and it takes the same
// repair, for the same reason: the answer was not fresher numbers. Fresher
// numbers age too, and these had already been replaced once. The section now
// argues from mechanism — frequent-and-small against rare-and-enormous —
// and names `replay diff` as the command that produces figures dated by the
// run that made them.
//
// SCOPE, stated because the next reader will otherwise overestimate this.
// It looks for a percentage inside one section of one file. It would NOT
// have caught the prose claim "the rarest break cause", which was false
// without containing a digit, and it does not check any figure against the
// corpus — nothing in this build does. It stops the table coming back. That
// is all it does, and a guard believed to cover more than it does is worse
// than none.
func TestFCRB_TheBreakCauseSectionCarriesNoShares(t *testing.T) {
	const heading = "## What actually broke the cache"

	path := filepath.Join("..", "..", "README.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)

	start := strings.Index(body, heading)
	if start < 0 {
		t.Fatalf("README.md no longer has the %q section.\nIf it was renamed, "+
			"retarget this test; if it was deleted, this test is the only record "+
			"of why the table must not come back — read it before removing it.", heading)
	}
	section := body[start+len(heading):]
	// The next top-level heading ends the section. Without this bound the
	// test would scan the whole README and fail on unrelated figures.
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	if strings.TrimSpace(section) == "" {
		t.Fatal("the section is empty, so this test read nothing and proved nothing")
	}

	// A percentage is the signature. The table's shares were all of this
	// shape, and so is any replacement someone writes in prose.
	pct := regexp.MustCompile(`\d+(?:\.\d+)?\s*%`)
	var found []string
	for _, line := range strings.Split(section, "\n") {
		// Dates are allowed and contain digits; percentages are not.
		for _, m := range pct.FindAllString(line, -1) {
			found = append(found, m+"  in: "+strings.TrimSpace(line))
		}
	}
	if len(found) > 0 {
		t.Errorf("the break-cause section quotes a share again:\n  %s\n\n"+
			"Those figures moved three times in five days and the last re-run "+
			"reversed the order of the two leading causes. State the mechanism "+
			"and name `replay diff`, which prints figures dated by the run that "+
			"made them.", strings.Join(found, "\n  "))
	}
}
