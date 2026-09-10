package observation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Writing a corpus contribution, and the promise that goes with it.
//
// The probe path prints "no prompts, no paths, no spend". That sentence is the
// bargain a contributor accepts, and a corpus payload breaks it — deliberately,
// because it carries aggregate money by design. So it cannot reuse that path or
// that message: the contributor has to be told what they are sending, in the
// same breath they are told nothing was sent.
//
// Same refusals as the probe writer. A submission that overwrites an earlier
// unsent one, or writes through a symlink, is the same defect whatever it
// carries.

func sampleCorpus() Corpus {
	return Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-10T00:00Z",
		Tasks: 115, TotalUSD: 3382.13, AvoidableUSD: 161.66,
		AvoidableShare: 0.0478, MedianTaskUSD: 0.77,
		PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		Unpriced: 6, SourceTag: "abc123", TagBasis: "local",
	}.Digested()
}

// CW1: the payload lands on disk and round-trips.
//
// PASS: a file whose JSON parses back to the same five figures.
// FAIL: a payload the contributor cannot read before deciding to send it.
func TestCW1_TheCorpusIsWrittenAndReadable(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteCorpus(dir, sampleCorpus())
	if err != nil {
		t.Fatalf("WriteCorpus: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got Corpus
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("the written payload is not JSON: %v", err)
	}
	if got.Tasks != 115 || got.TotalUSD != 3382.13 || got.AvoidableUSD != 161.66 {
		t.Errorf("the figures did not survive the round trip: %+v", got)
	}
	if got.PricedAt == "" || got.RulesVersion == "" {
		t.Error("the basis did not survive; a pooled figure cannot be defended without it")
	}
}

// CW2: an invalid corpus is refused before it reaches disk.
//
// PASS: nothing is written.
// FAIL: a file a contributor might send that aggregates to a measured zero.
func TestCW2_AnInvalidCorpusIsNotWritten(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteCorpus(dir, Corpus{Schema: CorpusSchema}); err == nil {
		t.Error("an empty corpus was written")
	}
	e, _ := os.ReadDir(dir)
	if len(e) != 0 {
		t.Errorf("the refusal left %d file(s) behind", len(e))
	}
}

// CW3: an existing submission is never replaced.
//
// The probe writer's rule, for the same reason: a submission that has not been
// sent is somebody's pending decision, and overwriting it destroys the copy
// they were reading.
func TestCW3_AnUnsentSubmissionIsNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteCorpus(dir, sampleCorpus())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"marker":"the copy they were reading"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteCorpus(dir, sampleCorpus()); err == nil {
		t.Fatal("a second write replaced an unsent submission")
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "the copy they were reading") {
		t.Error("the earlier submission was overwritten anyway")
	}
}

// CW4: a symlinked destination is refused.
func TestCW4_ASymlinkedDestinationIsRefused(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(t.TempDir(), "elsewhere.json")
	link := filepath.Join(dir, corpusFileName(sampleCorpus()))
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := WriteCorpus(dir, sampleCorpus()); err == nil {
		t.Error("a submission was written through a symlink")
	}
	if _, err := os.Stat(real); err == nil {
		t.Error("the payload landed at the symlink's target")
	}
}
