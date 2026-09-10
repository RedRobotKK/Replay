package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Deleting ledger records the reader no longer wants to keep.
//
// The gap this closes is a compliance one and it was found by looking rather
// than by a failure: nothing in this tool ever removed a ledger record. A
// proxy that records every request indefinitely, with no documented period and
// no way to delete, fails the Confidentiality criterion in a SOC 2 audit and
// has no answer to an erasure request under GDPR Article 17.
//
// The ledger holds no message content by design — record.go says "Counts and
// thresholds only, never content" — which keeps the exposure low. It is not
// zero: session ids, request ids, paths and timestamps are pseudonymous
// identifiers, and "we kept it forever because it was only metadata" is not a
// retention policy.
//
// Deleting is the one operation in this tool that cannot be undone by running
// it again, so the design is shaped around that:
//
//	--older-than is required     no default window, because a default would
//	                             eventually delete something for somebody who
//	                             never chose one
//	--dry-run is the default     it reports and removes nothing until --yes
//	whole files only             a ledger file is one session; partial rewrites
//	                             risk leaving a half-written record behind
//
// Every test below states what passes and what fails, because a destructive
// command tested only on its happy path is how data goes missing quietly.

// ledgerWith writes n session files with the given ages, and returns the dir.
func ledgerWith(t *testing.T, ages ...time.Duration) string {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()
	for i, age := range ages {
		p := filepath.Join(dir, "session-"+string(rune('a'+i))+".jsonl")
		if err := os.WriteFile(p, []byte(`{"schema":1,"ts":"2026-09-01T00:00:00Z"}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		when := now.Add(-age)
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func countLedger(t *testing.T, dir string) int {
	t.Helper()
	e, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range e {
		if strings.HasSuffix(f.Name(), ".jsonl") {
			n++
		}
	}
	return n
}

// PG1: without --yes, nothing is deleted.
//
// PASS: the command reports what it would remove and every file survives.
// FAIL: a file is gone after a run the reader did not confirm.
func TestPG1_DryRunIsTheDefault(t *testing.T) {
	dir := ledgerWith(t, 90*24*time.Hour, 1*time.Hour)
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--older-than", "30d"}, &out, &errb); err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if n := countLedger(t, dir); n != 2 {
		t.Errorf("a run without --yes removed files: %d of 2 remain", n)
	}
	if !strings.Contains(out.String(), "would remove") {
		t.Errorf("the dry run does not say what it would remove:\n%s", out.String())
	}
}

// PG2: with --yes, only records older than the window go.
//
// PASS: the old file is gone, the recent one is untouched.
// FAIL: a file inside the window is deleted, or one outside it survives.
func TestPG2_OnlyRecordsOutsideTheWindowAreRemoved(t *testing.T) {
	dir := ledgerWith(t, 90*24*time.Hour, 1*time.Hour)
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--older-than", "30d", "--yes"}, &out, &errb); err != nil {
		t.Fatalf("purge failed: %v", err)
	}
	if n := countLedger(t, dir); n != 1 {
		t.Fatalf("%d file(s) remain, expected exactly the one inside the window", n)
	}
	e, _ := os.ReadDir(dir)
	for _, f := range e {
		if f.Name() == "session-a.jsonl" {
			t.Error("the 90-day-old file survived a 30-day window")
		}
	}
}

// PG3: --older-than is required.
//
// PASS: a run without a window refuses and deletes nothing.
// FAIL: a default window silently decides what to delete for somebody who
// never chose one.
func TestPG3_TheWindowIsRequired(t *testing.T) {
	dir := ledgerWith(t, 90*24*time.Hour)
	var out, errb bytes.Buffer
	err := runPurge([]string{dir, "--yes"}, &out, &errb)
	if err == nil {
		t.Fatal("purge ran with no --older-than; a destructive default is not a default")
	}
	if n := countLedger(t, dir); n != 1 {
		t.Error("the refusal deleted something anyway")
	}
}

// PG4: nothing to remove is reported, not silently succeeded.
//
// PASS: a corpus with nothing outside the window says so.
// FAIL: silence, which reads identically to a purge that worked.
func TestPG4_NothingToRemoveIsSaidPlainly(t *testing.T) {
	dir := ledgerWith(t, 1*time.Hour)
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--older-than", "30d", "--yes"}, &out, &errb); err != nil {
		t.Fatalf("purge failed: %v", err)
	}
	if !strings.Contains(strings.ToLower(out.String()), "nothing") {
		t.Errorf("a run that removed nothing does not say so:\n%s", out.String())
	}
	if n := countLedger(t, dir); n != 1 {
		t.Error("a file inside the window was removed")
	}
}

// PG5: only ledger files are touched.
//
// PASS: an unrelated file in the directory survives whatever its age.
// FAIL: the command deletes something it does not own, which is the worst
// outcome available to it.
func TestPG5_OnlyLedgerFilesAreRemoved(t *testing.T) {
	dir := ledgerWith(t, 90*24*time.Hour)
	other := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(other, []byte("not a ledger"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(other, old, old); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--older-than", "30d", "--yes"}, &out, &errb); err != nil {
		t.Fatalf("purge failed: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Error("purge deleted a file that is not a ledger record")
	}
}

// PG6: a malformed window is refused rather than guessed at.
//
// PASS: "30 days", "-30d" and "" are all refused, and nothing is deleted.
// FAIL: any of them is interpreted, because a misread window on a destructive
// command deletes the wrong set.
func TestPG6_AnUnparseableWindowIsRefused(t *testing.T) {
	for _, bad := range []string{"30 days", "-30d", "0d", "thirty"} {
		dir := ledgerWith(t, 90*24*time.Hour)
		var out, errb bytes.Buffer
		if err := runPurge([]string{dir, "--older-than", bad, "--yes"}, &out, &errb); err == nil {
			t.Errorf("--older-than %q was accepted", bad)
		}
		if n := countLedger(t, dir); n != 1 {
			t.Errorf("--older-than %q deleted something before refusing", bad)
		}
	}
}

// PG22: an erasure names what it could not read.
//
// The walk skipped an unreadable ledger file and said nothing, then printed
// "removed N record(s)" or "Nothing to remove". A subject asking for erasure was
// told it completed over a corpus the command had not fully examined. Absence,
// zero and unknown are three values, and silence collapsed the third into the
// first.
func TestPG22_AnErasureNamesWhatItCouldNotRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Unix mode bits on this platform; a file cannot be made unreadable here")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read anything")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"),
		[]byte(`{"session_id":"wanted"}`+"\n"+`{"session_id":"other"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "locked.jsonl")
	if err := os.WriteFile(locked, []byte(`{"session_id":"wanted"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Skipf("cannot remove file permissions: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })

	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "wanted", "--yes"}, &out, &errb); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "could not be read") {
		t.Errorf("the erasure did not say a file was skipped, so its report reads as a "+
			"statement about the whole directory:\n%s", s)
	}
	if !strings.Contains(s, "not a statement about those") {
		t.Errorf("the report must bound its own claim:\n%s", s)
	}
}
