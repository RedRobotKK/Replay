package main

import (
	"bytes"
	"os"
	"path/filepath"
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

// ledgerSessions writes one ledger file per name, each holding the given
// session ids one record per line, plus a blank line and an unparseable one.
//
// Both of those are deliberate. A blank line and a line that is not JSON are
// what a real ledger accumulates, and each is a branch in the erasure walk that
// nothing observed.
func ledgerSessions(t *testing.T, files map[string][]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, ids := range files {
		var b strings.Builder
		b.WriteString("\n")                        // blank line
		b.WriteString("this is not json at all\n") // unparseable, must be kept
		for _, id := range ids {
			b.WriteString(`{"session_id":"` + id + `","bytes":1}` + "\n")
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// PG10: the erasure path had no tests at all, and every guard in it was
// removable without one failing.
//
// `replay purge --session <id>` is the answer to an erasure request. Ten of its
// conditionals were unobserved, including both confirmation gates, both
// nothing-found paths, and the walk filter that decides which files it will
// touch. This is the shape ADR-0014 exists for: the branch that decides whether
// to delete somebody's data is the one that must be able to fail.
func TestPG10_ErasureRemovesOneSessionAndKeepsTheRest(t *testing.T) {
	dir := ledgerSessions(t, map[string][]string{
		"a.jsonl": {"wanted", "other"},
		"b.jsonl": {"other"},
	})
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "wanted", "--yes"}, &out, &errb); err != nil {
		t.Fatalf("erasure failed: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "a.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if strings.Contains(s, "wanted") {
		t.Errorf("the requested session survived the erasure:\n%s", s)
	}
	if !strings.Contains(s, "other") {
		t.Errorf("erasing one session removed another that shared the file:\n%s", s)
	}
	// An unparseable line says nothing about anyone and a purge is not a
	// corpus cleaner.
	if !strings.Contains(s, "this is not json") {
		t.Errorf("an unparseable line was dropped by a deletion command:\n%s", s)
	}
	// A rewritten ledger is still a ledger: JSONL is newline-terminated, and a
	// file whose last record lost its newline is one the next appender
	// corrupts by writing straight onto it.
	if !strings.HasSuffix(s, "\n") {
		t.Errorf("the rewritten ledger does not end in a newline: %q", s)
	}
	// A file carrying none of the requested session must not be rewritten.
	if !strings.Contains(out.String(), "removed") {
		t.Errorf("the erasure did not report what it removed:\n%s", out.String())
	}
	if strings.Contains(out.String(), "b.jsonl") {
		t.Errorf("a file with no matching record was reported as touched:\n%s", out.String())
	}
}

// PG11: without --yes the erasure changes nothing and says so.
func TestPG11_ErasureIsADryRunByDefault(t *testing.T) {
	dir := ledgerSessions(t, map[string][]string{"a.jsonl": {"wanted", "other"}})
	before, err := os.ReadFile(filepath.Join(dir, "a.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "wanted"}, &out, &errb); err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "a.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("an erasure without --yes rewrote the ledger")
	}
	if !strings.Contains(out.String(), "would remove") {
		t.Errorf("the dry run does not say what it would remove:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Nothing was changed") {
		t.Errorf("a dry run must say that nothing changed, or a reader cannot tell it "+
			"from a run that did:\n%s", out.String())
	}
}

// PG12: a session nobody has is said plainly, with the number of records read.
//
// The count matters: "nothing matched" over an empty directory and over a
// hundred thousand records are different answers to an erasure request.
func TestPG12_AnErasureThatMatchesNothingSaysSo(t *testing.T) {
	dir := ledgerSessions(t, map[string][]string{"a.jsonl": {"other"}})
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "absent", "--yes"}, &out, &errb); err != nil {
		t.Fatalf("erasure failed: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "Nothing to remove") {
		t.Errorf("a session that matched nothing was not reported:\n%s", s)
	}
	if !strings.Contains(s, "record(s) read") {
		t.Errorf("the records read must travel with the nothing-found message:\n%s", s)
	}
	if strings.Contains(s, "removed 0") {
		t.Errorf("nothing-found was rendered as a removal:\n%s", s)
	}
}

// PG13: --session with no id is refused rather than matching everything.
func TestPG13_AnEmptySessionIDIsRefused(t *testing.T) {
	dir := ledgerSessions(t, map[string][]string{"a.jsonl": {"other"}})
	var out, errb bytes.Buffer
	err := runPurge([]string{dir, "--session", "   ", "--yes"}, &out, &errb)
	if err == nil {
		t.Fatal("an empty session id was accepted; it would match on a blank id")
	}
	if !strings.Contains(err.Error(), "needs an id") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
}

// PG14: the export is written before anything is removed.
func TestPG14_ExportIsWrittenBeforeRemoval(t *testing.T) {
	dir := ledgerSessions(t, map[string][]string{"a.jsonl": {"wanted", "other"}})
	export := filepath.Join(t.TempDir(), "exported.jsonl")
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "wanted", "--export", export, "--yes"}, &out, &errb); err != nil {
		t.Fatalf("erasure failed: %v", err)
	}
	body, err := os.ReadFile(export)
	if err != nil {
		t.Fatalf("the export was not written: %v", err)
	}
	if !strings.Contains(string(body), "wanted") {
		t.Errorf("the export does not carry the removed records:\n%s", body)
	}
	if !strings.Contains(out.String(), "before removing them") {
		t.Errorf("the export was not reported:\n%s", out.String())
	}
}

// PG15: the walk touches ledger files and nothing else.
//
// The filter is three clauses — a walk error, a directory, and the .jsonl
// suffix — and deleting a file this command does not own is the worst outcome
// available to it.
func TestPG15_ErasureTouchesOnlyLedgerFiles(t *testing.T) {
	dir := ledgerSessions(t, map[string][]string{"a.jsonl": {"wanted"}})
	stranger := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(stranger, []byte(`{"session_id":"wanted"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "nested")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "wanted", "--yes"}, &out, &errb); err != nil {
		t.Fatalf("erasure failed: %v", err)
	}
	body, err := os.ReadFile(stranger)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "wanted") {
		t.Error("a file that is not a ledger record was rewritten by purge")
	}
}

// PG16: the directory argument is required, and a missing one is refused.
func TestPG16_TheLedgerDirectoryIsRequired(t *testing.T) {
	var out, errb bytes.Buffer
	err := runPurge([]string{"--older-than", "30d"}, &out, &errb)
	if err == nil {
		t.Fatal("purge ran with no directory")
	}
	if !strings.Contains(err.Error(), "one ledger directory is required") {
		t.Errorf("refused for the wrong reason: %v", err)
	}

	var out2, errb2 bytes.Buffer
	err = runPurge([]string{filepath.Join(t.TempDir(), "absent"), "--older-than", "30d"}, &out2, &errb2)
	if err == nil {
		t.Fatal("purge ran against a directory that does not exist")
	}
	if !strings.Contains(err.Error(), "reading the ledger directory") {
		t.Errorf("a missing directory must say it could not be read: %v", err)
	}
}

// PG17: --yes changes the verb, not only the outcome.
//
// The dry run says "would remove" and a real run says "removed". Nothing
// observed the branch that switches them, so a run that deleted files could
// have gone on reporting that it would.
func TestPG17_TheVerbFollowsWhetherAnythingWasRemoved(t *testing.T) {
	dir := ledgerWith(t, 90*24*time.Hour)
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--older-than", "30d", "--yes"}, &out, &errb); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "removed") || strings.Contains(s, "would remove") {
		t.Errorf("a run with --yes must say what it removed, not what it would:\n%s", s)
	}
	if strings.Contains(s, "Nothing was changed") {
		t.Errorf("a run that deleted files said nothing changed:\n%s", s)
	}

	dir2 := ledgerWith(t, 90*24*time.Hour)
	var out2, errb2 bytes.Buffer
	if err := runPurge([]string{dir2, "--older-than", "30d"}, &out2, &errb2); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out2.String(), "Nothing was changed") {
		t.Errorf("a dry run must say nothing changed:\n%s", out2.String())
	}
}

// PG18: a ledger file the command cannot read is left alone, not emptied.
//
// The read failure inside the erasure walk was unobserved. Falling through on
// a read error would leave `lines` empty, every record "kept" would be none,
// and the file would be rewritten to nothing — the erasure command deleting a
// file it could not even read.
func TestPG18_AnUnreadableLedgerFileIsLeftAlone(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can read anything")
	}
	dir := ledgerSessions(t, map[string][]string{"a.jsonl": {"wanted"}})
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
		t.Fatalf("an unreadable file ended the erasure: %v", err)
	}
	_ = os.Chmod(locked, 0o600)
	body, err := os.ReadFile(locked)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"session_id":"wanted"}`+"\n" {
		t.Errorf("a file the command could not read was rewritten. It must be left exactly\n"+
			"as found, because a read failure is not evidence about its contents. got: %q", body)
	}
}

// PG19: blank lines are not records, and are not counted as ones.
//
// The scanned count travels with the nothing-found message, so counting blank
// lines would inflate the number of records an erasure request was checked
// against.
func TestPG19_BlankLinesAreNotRecords(t *testing.T) {
	dir := t.TempDir()
	// Three real records, four blank lines.
	body := "\n\n" + `{"session_id":"x"}` + "\n\n" + `{"session_id":"y"}` + "\n\n" + `{"session_id":"z"}` + "\n\n"
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "absent", "--yes"}, &out, &errb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "(3 record(s) read)") {
		t.Errorf("blank lines were counted as records; the count must be 3:\n%s", out.String())
	}
}

// PG20: erasing every record in a file leaves it empty, not holding a newline.
//
// The trailing-newline branch was unobserved. A file rewritten to "\n" is a
// file that still has a line in it, and the next reader has to decide whether
// that is a record.
func TestPG20_ErasingEveryRecordLeavesTheFileEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "only.jsonl"),
		[]byte(`{"session_id":"wanted"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "wanted", "--yes"}, &out, &errb); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "only.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 0 {
		t.Errorf("a file whose every record was erased holds %q, want nothing", body)
	}
}
