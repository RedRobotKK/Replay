package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/advisor"
)

// Marking a finding must outlive the keystroke.
//
// Selection without persistence is a cursor, not triage. A reader who marks
// three findings applied, quits, and comes back to the same three pending has
// been given a toy — and worse, has been told their work was recorded when it
// was not.
//
// advice.json already carries a status per suggestion and `replay advise`
// already tracks pending/applied/verified. The screen just never wrote to it.

func adviceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".replay"), 0o700); err != nil {
		t.Fatal(err)
	}
	f := adviceFile{
		Schema:   advisor.AdviceFileSchema,
		Sessions: 12,
		Suggestions: []advisor.Suggestion{
			{ID: "one", Title: "Bash results are 30% of prompt tokens", Status: advisor.Pending},
			{ID: "two", Title: "first-turn instructions are 24%", Status: advisor.Pending},
		},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".replay", adviceFileName)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TG1: marking a finding writes it to disk.
func TestTG1_MarkingAFindingPersists(t *testing.T) {
	path := adviceFixture(t)
	if err := markAdvice("two", advisor.Applied); err != nil {
		t.Fatalf("markAdvice: %v", err)
	}
	var got adviceFile
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	// Count first. Both assertions below live inside a loop filtered on ID, so
	// a markAdvice that KEPT ONLY the marked suggestion would delete every
	// other finding from the reader's own advice.json and this test would pass
	// — the loop body for "one" simply never runs. Verified: an audit made
	// exactly that change and `go test ./cmd/replay/` was green.
	//
	// The file is the reader's record of what they have decided. Losing the
	// rest of it while reporting success is worse than failing to record the
	// mark at all.
	if len(got.Suggestions) != 2 {
		t.Fatalf("marking one finding left %d suggestion(s) in the file, was 2. Marking a "+
			"finding must not remove the others.", len(got.Suggestions))
	}
	seen := map[string]bool{}
	for _, s := range got.Suggestions {
		seen[s.ID] = true
		if s.ID == "two" && s.Status != advisor.Applied {
			t.Errorf("finding two is %q after being marked applied; the reader was told "+
				"their decision was recorded and it was not", s.Status)
		}
		if s.ID == "one" && s.Status != advisor.Pending {
			t.Errorf("marking one finding changed another: one is now %q", s.Status)
		}
	}
	for _, id := range []string{"one", "two"} {
		if !seen[id] {
			t.Errorf("finding %q is missing from the file after marking", id)
		}
	}
}

// TG2: an unknown id is refused rather than silently doing nothing.
//
// A no-op that reports success is how a reader comes to trust a screen that is
// not recording anything.
func TestTG2_AnUnknownFindingIsRefused(t *testing.T) {
	adviceFixture(t)
	if err := markAdvice("nope", advisor.Applied); err == nil {
		t.Error("marking a finding that does not exist reported success")
	}
}

// TG3: with no advice file, marking fails loudly.
func TestTG3_NoAdviceFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	if err := markAdvice("one", advisor.Applied); err == nil {
		t.Error("marking against a missing advice file reported success")
	}
}

// TG4: a cache built from a much smaller corpus is not used.
//
// Reading advice.json made the screen instant and, on this machine, wrong: it
// showed "3 changes across 1 transcript" while the corpus held 140 across
// 1,729. The file was left by an earlier run against a fixture, and nothing
// compared it to what is on disk now.
//
// Instant and wrong is a worse trade than slow and right, because the reader
// cannot tell. The timestamp does not save it either — advice from an hour ago
// over one transcript is not stale, it is about a different corpus.
//
// So the rule is coverage, not age: use the cache when it was built from
// substantially the same number of transcripts as exist now. The corpus grows
// while the tool runs, so this cannot be an equality test.
func TestTG4_ACacheFromASmallerCorpusIsRejected(t *testing.T) {
	for _, c := range []struct {
		name         string
		cached, disk int
		want         bool
	}{
		{"same corpus", 1729, 1729, true},
		{"grown a little, as it always does mid-session", 1700, 1729, true},
		{"a fixture run against one transcript", 1, 1729, false},
		{"half the corpus", 800, 1729, false},
		{"cache is larger, so files were removed", 1729, 800, false},
		{"nothing on disk yet", 12, 0, false},
	} {
		if got := cacheCoversCorpus(c.cached, c.disk); got != c.want {
			t.Errorf("%s: cached=%d disk=%d gave %v, want %v",
				c.name, c.cached, c.disk, got, c.want)
		}
	}
}
