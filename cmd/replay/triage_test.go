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
	for _, s := range got.Suggestions {
		if s.ID == "two" && s.Status != advisor.Applied {
			t.Errorf("finding two is %q after being marked applied; the reader was told "+
				"their decision was recorded and it was not", s.Status)
		}
		if s.ID == "one" && s.Status != advisor.Pending {
			t.Errorf("marking one finding changed another: one is now %q", s.Status)
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
