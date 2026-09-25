package main

import (
	"bytes"
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

// writeAdvice puts an arbitrary advice file under a temporary HOME and returns
// its path. Separate from adviceFixture because these tests care about the
// statuses and the decisions in the file, not about a two-finding list.
func writeAdvice(t *testing.T, f adviceFile) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".replay"), 0o700); err != nil {
		t.Fatal(err)
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

// TG5: a status the verifier computed is not a decision the reader made.
//
// appliedIDs feeds `applied` into advisor.Suggest, and applied is the single
// gate on track(): without it every suggestion is Pending, with it a suggestion
// can be promoted to Verified. Verified and NotVerified are produced BY track,
// never by a reader. Reading them back as applied closes the loop that 8f32a31
// opened: a status the old verifier inferred from corpus drift is re-read as a
// human decision forever after, and a suggestion promotes itself.
//
// Measured on the machine this was written for: advice.json held 147 records,
// 20 of them "not verified" and 1 "verified", with ZERO reader dispositions
// ever recorded. Regenerating against the same corpus moved one of them from
// "not verified" to "verified" with nobody having marked anything.
func TestTG5_AComputedStatusIsNotAReaderDecision(t *testing.T) {
	writeAdvice(t, adviceFile{
		Schema: advisor.AdviceFileSchema,
		Suggestions: []advisor.Suggestion{
			{ID: "verified-by-the-verifier", Status: advisor.Verified},
			{ID: "not-verified-by-the-verifier", Status: advisor.NotVerified},
			{ID: "untouched", Status: advisor.Pending},
			{ID: "unobservable", Status: advisor.AdviceOnly},
		},
	})
	got := appliedIDs()
	for id := range got {
		t.Errorf("%q counts as applied, but no reader ever marked it: its status "+
			"was written by the verifier and read back as a human decision", id)
	}
}

// TG6: a decision the reader recorded is the thing that counts as applied.
//
// The positive control for TG5. A guard over an empty set passes for the wrong
// reason, and an appliedIDs that had simply been stubbed to return nothing
// would satisfy TG5 while breaking the one input the verifier has.
func TestTG6_ARecordedDecisionIsWhatCounts(t *testing.T) {
	writeAdvice(t, adviceFile{
		Schema: advisor.AdviceFileSchema,
		Suggestions: []advisor.Suggestion{
			{ID: "marked", Status: advisor.Pending},
			{ID: "dismissed", Status: advisor.Pending},
			{ID: "untouched", Status: advisor.Pending},
		},
		Decisions: map[string]advisor.Decision{
			"marked":    advisor.DecisionApplied,
			"dismissed": advisor.DecisionDismissed,
		},
	})
	got := appliedIDs()
	if !got["marked"] {
		t.Error("a finding the reader marked applied does not count as applied; the " +
			"verifier has no input at all and nothing can ever be judged")
	}
	// A dismissal is a decision and it is not an application. Counting it
	// would be this defect rebuilt out of a different constant.
	if got["dismissed"] {
		t.Error("a dismissed finding counts as applied: the reader said they were NOT " +
			"doing it, so nothing moved and there is nothing to verify")
	}
	if got["untouched"] {
		t.Error("a finding with no decision counts as applied")
	}
	if len(got) != 1 {
		t.Errorf("appliedIDs returned %d ids, want exactly 1", len(got))
	}
}

// TG7: a status invented inside the decisions object is not honoured.
//
// The file is plain JSON under the reader's own home and a status that the
// verifier produces has no business being a decision. Accepting one would walk
// back into TG5 one level down.
func TestTG7_OnlyRealDecisionsAreRead(t *testing.T) {
	writeAdvice(t, adviceFile{
		Schema:      advisor.AdviceFileSchema,
		Suggestions: []advisor.Suggestion{{ID: "one", Status: advisor.Pending}, {ID: "two", Status: advisor.Pending}},
		Decisions: map[string]advisor.Decision{
			"one": advisor.Decision(advisor.Verified),
			"two": advisor.DecisionApplied,
		},
	})
	got := readerDecisions()
	if _, ok := got["one"]; ok {
		t.Error(`"verified" was accepted as a reader decision`)
	}
	if got["two"] != advisor.DecisionApplied {
		t.Errorf(`the real decision was dropped along with the invented one: %+v`, got)
	}
}

// TG8: a file from an older schema contributes no decisions, and cannot be
// marked in place.
//
// Its statuses have unknown provenance. On the machine this was written for,
// 21 of them were written by a verifier that has since been proved wrong, and
// nothing in the file distinguishes those from a status a reader caused. The
// honest move is to discard them and let the reader mark again, which is the
// rule the ledger already follows at its own schema 2.
func TestTG8_AnOlderSchemaContributesNothing(t *testing.T) {
	writeAdvice(t, adviceFile{
		Schema: advisor.AdviceFileSchema - 1,
		Suggestions: []advisor.Suggestion{
			{ID: "one", Status: advisor.Verified},
			{ID: "two", Status: advisor.NotVerified},
		},
	})
	if got := readerDecisions(); len(got) != 0 {
		t.Errorf("a schema %d file produced decisions %+v: statuses of unknown "+
			"provenance were promoted into reader decisions",
			advisor.AdviceFileSchema-1, got)
	}
	if err := markAdvice("one", advisor.Applied); err == nil {
		t.Error("marking into a file this build does not understand reported success; " +
			"the old statuses would be carried forward under a schema number that " +
			"asserts they are trustworthy")
	}

	// The other direction, and the one that makes the schema check do work
	// rather than agree with the absent field. A file from a LATER build has a
	// decisions object, and this build has no idea what its entries mean: the
	// next bump is the one that changes them. Equality, not "at least", which
	// is the rule adviceFromCache already applies before it renders one.
	writeAdvice(t, adviceFile{
		Schema:      advisor.AdviceFileSchema + 1,
		Suggestions: []advisor.Suggestion{{ID: "one", Status: advisor.Pending}},
		Decisions:   map[string]advisor.Decision{"one": advisor.DecisionApplied},
	})
	if got := readerDecisions(); len(got) != 0 {
		t.Errorf("a schema %d file produced decisions %+v: this build read a decisions "+
			"object written under a contract it does not have",
			advisor.AdviceFileSchema+1, got)
	}
}

// TG12: a dismissal is recorded as a dismissal.
//
// The keystroke is where this defect gets a second chance. `x` and `a` arrive
// at the same function, and mapping `x` onto an application would hand the
// verifier a suggestion the reader explicitly refused: nothing about the
// corpus moved, so any drop it then measured would be drift, which is the
// exact failure 8f32a31 removed.
func TestTG12_ADismissalIsNotAnApplication(t *testing.T) {
	writeAdvice(t, adviceFile{
		Schema:      advisor.AdviceFileSchema,
		Suggestions: []advisor.Suggestion{{ID: "one", Status: advisor.Pending}, {ID: "two", Status: advisor.Pending}},
	})
	if err := markAdvice("one", advisor.Dismissed); err != nil {
		t.Fatalf("markAdvice dismissed: %v", err)
	}
	// Control: the same path DOES record an application, so the assertion
	// below is not passing because nothing is ever recorded.
	if err := markAdvice("two", advisor.Applied); err != nil {
		t.Fatalf("markAdvice applied: %v", err)
	}
	got := readerDecisions()
	if got["two"] != advisor.DecisionApplied {
		t.Fatalf("control: an application was not recorded at all: %+v", got)
	}
	if got["one"] != advisor.DecisionDismissed {
		t.Errorf("a dismissal was recorded as %q", got["one"])
	}
	if appliedIDs()["one"] {
		t.Error("a dismissed finding is handed to the verifier as applied: the reader " +
			"refused it, so any drop measured afterwards is corpus drift")
	}
}

// TG9: a computed status cannot be recorded as a reader decision.
//
// track produces Verified, NotVerified, AdviceOnly and Pending. A reader
// produces Applied and Dismissed. Nothing should be able to put the first four
// into the only field on disk that claims a human author, and the check is here
// rather than at the keystroke so that a second caller cannot skip it.
func TestTG9_AComputedStatusCannotBeMarked(t *testing.T) {
	for _, st := range []advisor.Status{advisor.Verified, advisor.NotVerified, advisor.AdviceOnly, advisor.Pending} {
		adviceFixture(t)
		if err := markAdvice("one", st); err == nil {
			t.Errorf("markAdvice accepted %q, which no reader can set", st)
		}
	}
}

// TG10: marking a finding survives the status being recomputed.
//
// The whole point of the split. `replay advise` rewrites every status from the
// corpus on every run, so a decision stored in the status field is destroyed by
// the first run after it is made. This asserts the round trip at the level the
// two functions meet: mark, then read back as the verifier's input.
func TestTG10_AMarkSurvivesTheStatusBeingRecomputed(t *testing.T) {
	path := writeAdvice(t, adviceFile{
		Schema:      advisor.AdviceFileSchema,
		Suggestions: []advisor.Suggestion{{ID: "one", Status: advisor.Pending}},
	})
	if err := markAdvice("one", advisor.Applied); err != nil {
		t.Fatalf("markAdvice: %v", err)
	}
	// What advise does next: recompute the status and write the file back,
	// carrying the decisions forward.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f adviceFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if f.Decisions["one"] != advisor.DecisionApplied {
		t.Fatalf("the mark was not recorded as a decision: %+v", f.Decisions)
	}
	f.Suggestions[0].Status = advisor.Verified
	nb, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nb, 0o600); err != nil {
		t.Fatal(err)
	}
	if !appliedIDs()["one"] {
		t.Error("the reader's mark did not survive the verifier acting on it: the " +
			"lifecycle pending to applied to verified collapses back to pending on " +
			"the next run and the keystroke is lost")
	}
}

// TG11: `replay advise` carries the reader's decisions forward.
//
// Every run rewrites advice.json wholesale from the corpus. The decisions
// object is the one part of it that is not derived from the corpus, so a run
// that does not copy it forward deletes the reader's record and makes the mark
// they were told was saved last exactly until they used the tool again.
//
// End to end on purpose: this is the seam where the two halves meet, and the
// unit tests either side of it would both stay green while the write dropped
// the field.
func TestTG11_AdviseCarriesDecisionsForward(t *testing.T) {
	corpus(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(home, ".replay", adviceFileName)
	dir := os.Getenv("REPLAY_TRANSCRIPTS")

	advise := func() adviceFile {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if err := run([]string{"advise", dir}, &stdout, &stderr); err != nil {
			t.Fatalf("advise: %v\n%s", err, stderr.String())
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("advise wrote no file at %s: %v", path, err)
		}
		var f adviceFile
		if err := json.Unmarshal(b, &f); err != nil {
			t.Fatal(err)
		}
		return f
	}

	first := advise()
	// The positive control. Everything below filters on an id, and over an
	// empty list every loop body is skipped and the test passes having
	// asserted nothing.
	if len(first.Suggestions) == 0 {
		t.Fatal("the fixture corpus produced no suggestions, so this test would pass " +
			"without exercising anything")
	}
	if first.Schema != advisor.AdviceFileSchema {
		t.Fatalf("advise wrote schema %d, want %d", first.Schema, advisor.AdviceFileSchema)
	}
	// A pending one. AdviceOnly is a statement about what the tool can observe
	// at all and a keystroke does not change it, so marking one of those would
	// assert the overlay against the case it deliberately leaves alone.
	id := ""
	for _, s := range first.Suggestions {
		if s.Status == advisor.Pending {
			id = s.ID
			break
		}
	}
	if id == "" {
		t.Fatalf("no pending suggestion in the fixture corpus (%d suggestions), so there "+
			"is nothing here the verifier can reach and this test asserts nothing",
			len(first.Suggestions))
	}
	if err := markAdvice(id, advisor.Applied); err != nil {
		t.Fatalf("markAdvice: %v", err)
	}

	second := advise()
	if second.Decisions[id] != advisor.DecisionApplied {
		t.Fatalf("the reader's decision did not survive the next advise run: decisions are %+v", second.Decisions)
	}
	found := false
	for _, s := range second.Suggestions {
		if s.ID != id {
			continue
		}
		found = true
		if s.Status != advisor.Applied {
			t.Errorf("a finding the reader marked applied reads back as %q after the next "+
				"run, so the screen tells them their keystroke was not recorded", s.Status)
		}
	}
	if !found {
		t.Errorf("the marked finding %q is gone from the second run's suggestions", id)
	}
}
