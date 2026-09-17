package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/version"
)

// The submission must not carry a commit this binary does not have.
//
// `version.Commit` defaults to the literal "unknown", which is not empty, so
// the `omitempty` on observation.Corpus.Commit cannot elide it. Production
// refuses the value: functions/_lib/contribute.js lists `commit` in OPTIONAL
// and shapes it /^[0-9a-f]{7,40}$/, so an absent field is accepted and
// "unknown" is not. Read at source 2026-09-17, and observed as a 400 on
// 2026-09-15 (docs/evidence/contribution-path-2026-09-15.md).
//
// This exercises contributeCorpus, the real construction path, and reads the
// file it writes, so the assertion is about bytes a contributor would send.
func writtenSubmission(t *testing.T, commit string) map[string]json.RawMessage {
	t.Helper()
	orig := version.Commit
	version.Commit = commit
	t.Cleanup(func() { version.Commit = orig })

	home := withSeries(t)
	writeConsent(t, home, "corpus_opt_in = true\n")

	dir := t.TempDir()
	path, _, err := contributeCorpus(testCampaign, dir, corpusFigures{
		Tasks: 2, TotalUSD: 1.50, RebilledUSD: 0.25,
		RebilledShare: 0.1667, MedianTaskUSD: 0.75,
	}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatalf("contributeCorpus: %v", err)
	}
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("reading the submission: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("the submission is not a JSON object: %v", err)
	}
	return m
}

func TestAnUnknownCommitNeverReachesTheSubmission(t *testing.T) {
	m := writtenSubmission(t, "unknown")
	if raw, present := m["commit"]; present {
		t.Errorf("the submission carries commit=%s; production refuses it and the binary does not know it", raw)
	}
}

func TestAKnownCommitDoesReachTheSubmission(t *testing.T) {
	const sha = "b3667e3"
	m := writtenSubmission(t, sha)
	raw, present := m["commit"]
	if !present {
		t.Fatal("a real commit was dropped from the submission")
	}
	var got string
	if err := json.Unmarshal(raw, &got); err != nil || got != sha {
		t.Errorf("commit = %v (err %v), want %q exactly", got, err, sha)
	}
}

// The two facts are independent: a proxy install has a version and no commit.
func TestTheVersionIsStillSubmittedWithoutACommit(t *testing.T) {
	origV := version.Version
	version.Version = "v0.6.0"
	t.Cleanup(func() { version.Version = origV })

	m := writtenSubmission(t, "unknown")
	if _, present := m["commit"]; present {
		t.Error("commit present")
	}
	var bv string
	if err := json.Unmarshal(m["binaryVersion"], &bv); err != nil || bv != "v0.6.0" {
		t.Errorf("binaryVersion = %v (err %v), want v0.6.0", bv, err)
	}
}
