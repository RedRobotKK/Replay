package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A ledger being written is not a ledger with a corrupt record in it.
//
// Store.Append writes one record per os.File.Write under a mutex, with
// O_APPEND. Go loops on a short write, so a reader that opens the file
// between two iterations of that loop sees a line with no terminating
// newline — the record is not damaged, it has not finished arriving.
//
// ReadRecords counted that line in `skipped`, alongside a genuinely corrupt
// line and an older-schema record. Three different facts in one integer, and
// the integer reaches the reader: it is carried into Session.Skipped and
// reported. A user running `replay cost` against a ledger that `replay serve`
// is still writing was told records had been skipped, which reads as data
// loss and was not.
//
// It also broke CI. TestWhatIfMatchesOfflineReplayAndStaysOffTheWire polls the
// ledger every 10ms while the proxy writes growing bodies, and treats any
// skipped line as fatal. On a slower ubuntu runner the poll landed inside a
// large write and main went red three merges running, while the same test
// passed 30/30 locally and under -race.
//
// The three cases are now three values.

func writeLines(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "")), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// goodLine is a record the current schema accepts.
func goodLine(t *testing.T, id string) string {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Append(Record{SessionID: id}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, sessionFileName(id)+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TL1: a half-written last line is incomplete, not skipped.
func TestTL1_ATrailingPartialLineIsNotASkippedRecord(t *testing.T) {
	whole := goodLine(t, "a")
	half := whole[:len(whole)/2] // no newline: the write is still in flight
	p := writeLines(t, whole, half)

	recs, skipped, incomplete, err := ReadRecords(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Errorf("records = %d, want 1: the complete record before the torn one", len(recs))
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0. A record still being written has not been "+
			"skipped, and reporting it as skipped tells the reader they lost data "+
			"they did not lose", skipped)
	}
	if !incomplete {
		t.Error("incomplete = false: the file does not end in a newline, so the last " +
			"record had not finished arriving and the reader is not being told")
	}
}

// TL2: a corrupt line in the MIDDLE is still skipped, and still reported.
//
// This is the case the counter exists for, and the one a lenient fix would
// lose. The line is complete — it ends in a newline — and it is not a record.
func TestTL2_ACorruptLineMidFileIsStillSkipped(t *testing.T) {
	a, b := goodLine(t, "a"), goodLine(t, "b")
	p := writeLines(t, a, "{\"schema\":1,\"broken\":\n", b)

	recs, skipped, incomplete, err := ReadRecords(p)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1: a complete line that is not a record is data "+
			"loss and must still be counted", skipped)
	}
	if incomplete {
		t.Error("incomplete = true on a file that ends in a newline; nothing was in flight")
	}
	if len(recs) != 2 {
		t.Errorf("records = %d, want 2: the readable records on either side", len(recs))
	}
}

// TL3: a clean file reports neither.
func TestTL3_ACompleteFileIsNeitherSkippedNorIncomplete(t *testing.T) {
	p := writeLines(t, goodLine(t, "a"), goodLine(t, "b"))
	recs, skipped, incomplete, err := ReadRecords(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 || skipped != 0 || incomplete {
		t.Errorf("records=%d skipped=%d incomplete=%v; want 2, 0, false",
			len(recs), skipped, incomplete)
	}
}

// TL4: a complete last record missing only its newline is not incomplete.
//
// Store.Append puts the record and its terminator in one buffer, so this
// state means the write was torn on its final byte. The record itself parsed
// and matched the schema — nothing about it is unknown, and flagging the file
// incomplete would report a doubt that does not exist.
//
// Without this, `lastWasSkip && !endsClean` can be weakened to
// `lastWasSkip || !endsClean` and every test still passes.
func TestTL4_AValidLastRecordWithoutItsNewlineIsComplete(t *testing.T) {
	whole := goodLine(t, "a")
	p := writeLines(t, strings.TrimSuffix(whole, "\n"))

	recs, skipped, incomplete, err := ReadRecords(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || skipped != 0 {
		t.Errorf("records=%d skipped=%d; want 1, 0", len(recs), skipped)
	}
	if incomplete {
		t.Error("incomplete = true for a record that parsed and matched the schema. " +
			"The missing byte is the terminator, not any part of the record")
	}
}

// TL5: a ledger file that exists with nothing in it yet is not torn.
//
// ReadRecords already says in its loop that "a file exists before its first
// record is flushed", and this is the read path for that window.
//
// What the early return in endsWithNewline actually prevents is not a wrong
// flag but an ERROR: without it the last byte is read at offset -1 and the
// whole file fails with "negative offset", so an empty ledger becomes
// unreadable rather than empty. Changing the value it returns changes nothing,
// because a file with no lines never sets lastWasSkip — that mutant is
// equivalent, and this test is aimed at the error instead.
func TestTL5_AnEmptyLedgerFileIsNotTorn(t *testing.T) {
	p := writeLines(t)
	recs, skipped, incomplete, err := ReadRecords(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 0 || skipped != 0 {
		t.Errorf("records=%d skipped=%d; want 0, 0", len(recs), skipped)
	}
	if incomplete {
		t.Error("an empty ledger file is reported as a torn write; every ledger would " +
			"be torn between creation and its first record")
	}
}
