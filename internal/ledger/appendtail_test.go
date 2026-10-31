package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

// An Append that succeeds must write a record a reader can read.
//
// Append writes one line per call with O_APPEND. When a write is cut short
// (the disk fills part-way, the file size limit is reached, the machine loses
// power after part of the page reached disk), the file is left ending in half
// a record with no newline, and that Append returns its error. The next
// Append used to write straight onto that fragment, so its own complete
// record became the second half of one unreadable line: a call that returned
// nil lost the record it wrote, and ReadRecords counted it as skipped.
//
// The torn fragment is planted rather than produced, because nothing in a
// portable test can make the kernel stop a write part-way; it is byte for byte
// what a cut-short Append leaves.

// AT1: a record appended after a torn tail starts on its own line.
//
// PASS: the new record reads back; the fragment is one skipped line.
// FAIL: no record reads back, because it was glued onto the fragment.
func TestAT1_AnAppendAfterATornTailIsStillReadable(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	const id = "sess"
	path := filepath.Join(dir, sessionFileName(id)+".jsonl")
	// What a cut-short Append of an earlier record leaves behind.
	if err := os.WriteFile(path, []byte(`{"schema":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(Record{SessionID: id}); err != nil {
		t.Fatalf("append: %v", err)
	}
	recs, skipped, _, incomplete, err := ReadRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1: an Append that returned nil wrote a record no reader can read", len(recs))
	}
	if skipped != 1 || incomplete {
		t.Errorf("skipped = %d, incomplete = %v; want the fragment counted once as skipped and the file complete", skipped, incomplete)
	}
}

// AT2: a clean file gains no blank line.
//
// The repair must only fire on a torn tail; a newline written before every
// record would be harmless to ReadRecords but is not what the file held
// before, and every other reader of a ledger would see it.
func TestAT2_AnAppendToACleanFileAddsNoBlankLine(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	const id = "sess"
	for i := 0; i < 2; i++ {
		if err := s.Append(Record{SessionID: id}); err != nil {
			t.Fatal(err)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, sessionFileName(id)+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || body[0] == '\n' {
		t.Fatalf("the first record does not start the file: %q", body)
	}
	for i := 1; i < len(body); i++ {
		if body[i] == '\n' && body[i-1] == '\n' {
			t.Fatalf("a blank line was written into a clean ledger: %q", body)
		}
	}
}
