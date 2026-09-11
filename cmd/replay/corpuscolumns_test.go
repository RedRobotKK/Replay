package main

import (
	"bytes"
	"strings"
	"testing"
)

// The per-model table's column headings must match what the cells hold.
//
// This is the check the project did not have when it retracted a lane count
// published under a heading that said `Sessions`, and did not have again when
// the correction left the NEXT column over holding lane counts under `Recent
// sessions`. A reviewer reading docs/evidence/calibration-corpus-2026-09-10.md
// found the second one four lines under the note retracting the first.
//
// So the rule is structural rather than a spot check of one number: in the
// per-model table, a heading naming sessions may only sit over
// ModelCalibration.Sessions, and every lane count must be headed for lanes.
// corpus.go:219 is the single line that has to obey it.
func TestPerModelColumnsAreNamedForWhatTheyCount(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"corpus", "../../internal/transcript/testdata"}, &out, &errOut); err != nil {
		t.Fatalf("corpus: %v (stderr: %s)", err, errOut.String())
	}
	header := perModelHeader(t, out.String())

	cells := strings.Split(strings.Trim(header, "|"), "|")
	var named []string
	for _, c := range cells {
		named = append(named, strings.ToLower(strings.TrimSpace(c)))
	}

	// One column named for sessions, and it is the independent count.
	sessionCols := 0
	for _, c := range named {
		if strings.Contains(c, "session") {
			sessionCols++
		}
	}
	if sessionCols != 1 {
		t.Errorf("%d columns name sessions in %q; exactly one does — the independent count. "+
			"A second one is a lane count wearing the word, which is the defect this "+
			"table has shipped twice", sessionCols, header)
	}

	// The recent window slices lanes, so its heading has to say so. It said
	// "Recent sessions" until 2026-09-11 while holding analysis.RecentLanes.
	if !hasColumn(named, "recent lanes") {
		t.Errorf("no `Recent lanes` column in %q: the window slices lanes "+
			"(analysis.StalenessRecentLanes) and the heading is what a reader believes",
			header)
	}
	if hasColumn(named, "recent sessions") {
		t.Errorf("`Recent sessions` is back in %q over a lane count", header)
	}
	if !hasColumn(named, "lanes") {
		t.Errorf("no `Lanes` column in %q: without it a reader cannot tell a fan-out "+
			"corpus from a wide one, which is the whole reason the two counts differ",
			header)
	}
}

// perModelHeader returns the header row of the `## Per model` table.
func perModelHeader(t *testing.T, doc string) string {
	t.Helper()
	_, after, ok := strings.Cut(doc, "## Per model")
	if !ok {
		t.Fatal("the corpus document has no `## Per model` section")
	}
	for _, line := range strings.Split(after, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "| Model |") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatal("the per-model table has no header row starting `| Model |`")
	return ""
}

func hasColumn(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
