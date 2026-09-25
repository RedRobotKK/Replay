package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/advisor"
)

// SWR. An older binary silently replaced a newer advice file and destroyed the
// only reader-authored state in it.
//
// Demonstrated on 2026-09-24 against two real binaries and a real corpus,
// recorded in docs/evidence/advisor-status-provenance-2026-09-23.md. A schema-2
// file carrying `decisions: {"f1f075ac845a": "applied"}` was overwritten by a
// build whose adviceFile struct has no Decisions field at all. The key did not
// survive, because the older writer marshals its own struct wholesale: it does
// not fail to understand the field, it never had one. The next current read
// then saw schema 1, discarded it as untrusted per ADR-0027, and the reader's
// mark was gone from `applied` back to `pending`.
//
// ADR-0027 closes the read side and says a schema-1 file contributes no
// decisions. This is the write side of the same rule: a build must not replace
// a file it could not have written.
//
// IT CANNOT REPAIR WHAT ALREADY HAPPENED. A guard added here runs only in
// builds that contain it, so it protects nothing against the binaries already
// installed on a disk today. It bounds the next bump, 2 to 3, and every one
// after. That is the whole claim, and the evidence file says so too.

// writeSchemaFile puts an advice file at a chosen schema into a fresh HOME and
// returns its path and its bytes, so a test can prove the file did not move.
func writeSchemaFile(t *testing.T, schema int, decisions map[string]advisor.Decision) (string, []byte) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".replay"), 0o700); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(adviceFile{
		Schema:      schema,
		Sessions:    12,
		Suggestions: []advisor.Suggestion{{ID: "one", Status: advisor.Applied}},
		Decisions:   decisions,
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".replay", adviceFileName)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, b
}

// SWR1: a writer refuses a file whose schema it could not have written.
//
// The boundary is strictly newer. Equal is the ordinary case and older is the
// upgrade path ADR-0027 already describes, where the old statuses are
// deliberately discarded.
func TestSWR1_AWriterRefusesAFileNewerThanItself(t *testing.T) {
	for _, tc := range []struct {
		name   string
		schema int
		refuse bool
		whyNot string
	}{
		{"one newer than this build", advisor.AdviceFileSchema + 1, true, ""},
		{"far newer", advisor.AdviceFileSchema + 9, true, ""},
		{"this build's own schema", advisor.AdviceFileSchema, false,
			"the ordinary case: every run rewrites its own file"},
		{"older", advisor.AdviceFileSchema - 1, false,
			"the upgrade path. ADR-0027 discards those statuses on purpose"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, _ := writeSchemaFile(t, tc.schema, nil)
			err := refuseNewerAdviceFile(path)
			if tc.refuse && err == nil {
				t.Errorf("a schema-%d file was replaced by a schema-%d writer.\n"+
					"      The newer file holds state this build cannot represent, and "+
					"rewriting it destroys that state with no warning", tc.schema, advisor.AdviceFileSchema)
			}
			if !tc.refuse && err != nil {
				t.Errorf("refused a schema-%d file: %v\n      %s", tc.schema, err, tc.whyNot)
			}
		})
	}
}

// SWR2: the refusal is actionable, and it names the way out.
//
// A reader whose only binary is the old one must not be stranded. `--out`
// already writes somewhere else, so the error says so rather than leaving them
// to guess. An error that only states a fact is a dead end wearing a message.
func TestSWR2_TheRefusalNamesBothSchemasAndTheWayOut(t *testing.T) {
	path, _ := writeSchemaFile(t, advisor.AdviceFileSchema+1, nil)
	err := refuseNewerAdviceFile(path)
	if err == nil {
		t.Fatal("a newer file must be refused")
	}
	for _, want := range []string{"--out", "upgrade"} {
		if !strings.Contains(strings.ToLower(err.Error()), want) {
			t.Errorf("the refusal does not mention %q, so it says what is wrong and "+
				"not what to do about it:\n      %v", want, err)
		}
	}
}

// SWR3: the file does not move.
//
// The property the whole guard exists for, stated as bytes rather than as a
// sentence. An error returned after the write has already happened would pass
// every assertion above and lose the decision anyway.
func TestSWR3_NewerStateSurvivesTheAttemptedWrite(t *testing.T) {
	path, before := writeSchemaFile(t, advisor.AdviceFileSchema+1,
		map[string]advisor.Decision{"one": advisor.DecisionApplied})

	if err := refuseNewerAdviceFile(path); err == nil {
		t.Fatal("a newer file must be refused")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file this guard exists to preserve is unreadable: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("the file changed under a refused write.\n  before %s\n  after  %s", before, after)
	}
	var f adviceFile
	if err := json.Unmarshal(after, &f); err != nil {
		t.Fatal(err)
	}
	if f.Decisions["one"] != advisor.DecisionApplied {
		t.Errorf("the reader's decision did not survive: decisions=%v.\n"+
			"      This is the exact loss reproduced against the 0.5.4 binary", f.Decisions)
	}
}

// SWR4: nothing else is blocked.
//
// A guard that refuses a missing or unreadable file would break the first run
// on a new machine and every run after a corrupted write, which is a far more
// common state than a downgrade. Garbage carries no schema to be newer than
// anything, so it is not this guard's business: `replay advise` rewrites it,
// as it always has.
func TestSWR4_AMissingOrUnreadableFileIsNotThisGuardsBusiness(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".replay"), 0o700); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, ".replay", adviceFileName)
	if err := refuseNewerAdviceFile(missing); err != nil {
		t.Errorf("a first run on a machine with no advice file was refused: %v", err)
	}
	if err := os.WriteFile(missing, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refuseNewerAdviceFile(missing); err != nil {
		t.Errorf("an unreadable file was refused rather than rewritten: %v", err)
	}
}

// SWR5: the guard is wired into the write path.
//
// The four tests above call refuseNewerAdviceFile directly, and a mutant that
// deleted the call from runAdvise left every one of them green. A guard nothing
// reaches is the defect this repository keeps finding, so this one asserts the
// wiring rather than the function: runAdvise itself must refuse, and the file
// must still be on disk unchanged afterwards.
func TestSWR5_RunAdviseItselfRefusesAndLeavesTheFileAlone(t *testing.T) {
	path, before := writeSchemaFile(t, advisor.AdviceFileSchema+1,
		map[string]advisor.Decision{"one": advisor.DecisionApplied})

	corpus := t.TempDir()
	writeTranscript(t, filepath.Join(corpus, "s1.jsonl"), "sess-swr5", "req-swr5")

	var stdout, stderr bytes.Buffer
	err := runAdvise([]string{corpus, "--out", path}, &stdout, &stderr)
	if err == nil {
		t.Fatal("runAdvise replaced an advice file newer than itself. The guard " +
			"exists but nothing calls it, which is the same as not having one")
	}
	if !strings.Contains(err.Error(), "refusing to replace") {
		t.Errorf("runAdvise failed for some other reason: %v", err)
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("the file the guard exists to preserve is gone: %v", readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("the file changed under a refused run.\n  before %s\n  after  %s", before, after)
	}
}
