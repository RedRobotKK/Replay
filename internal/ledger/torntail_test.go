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
	f, oerr := os.Open(p)
	if oerr != nil {
		t.Fatal(oerr)
	}
	clean, known := endsWithNewline(f)
	_ = f.Close()
	if !known {
		t.Error("an empty ledger file reads as unknown; without the size case the last " +
			"byte is read at offset -1 and every new ledger is unreadable until its " +
			"first record lands")
	}
	if !clean {
		t.Error("an empty ledger file reads as torn; it has no last line to tear")
	}

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

// TL6: the error paths in endsWithNewline are reachable, and reported.
//
// All three were UNREACHED — no test made them true — and an error branch
// nothing enters is a branch nobody has checked returns the right thing. Two
// inputs reach them: a file handle that is already closed fails at Stat, and a
// directory opens cleanly, stats cleanly, and fails at ReadAt with EISDIR.
//
// What matters is not that they error but that the error is RETURNED. A
// ledger that cannot be read must not come back as a ledger that is empty:
// that is absence reported as zero, and the whole point of the incomplete flag
// is to stop this file doing that.
func TestTL6_AnUnreadableLedgerErrorsRatherThanReadingEmpty(t *testing.T) {
	t.Run("closed handle fails at stat", func(t *testing.T) {
		p := writeLines(t, goodLine(t, "a"))
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if _, known := endsWithNewline(f); known {
			t.Error("a closed handle reported its last byte as known")
		}
	})

	// The directory case is asserted through ReadRecords rather than on the
	// helper directly, because the helper's answer for a directory is
	// platform-dependent and the caller's is not.
	//
	// On Unix a directory opens, stats with a non-zero size, and fails ReadAt:
	// unknown. On Windows a directory stats with Size() == 0, which is the
	// empty-file case, so the helper answers "clean, known" — truthfully, for
	// the question it was asked. Either way the scan that follows refuses the
	// directory, so ReadRecords errors on both, and that is the property
	// callers depend on.
	//
	// Asserting known == false here would have been a test that passes on the
	// machine it was written on and fails on a platform the project ships to,
	// which is the FD-11 shape this repository keeps finding.
	t.Run("write-only handle fails at readat", func(t *testing.T) {
		// Reaches the ReadAt branch on every platform.
		//
		// A directory reaches it on Unix and not on Windows, where a directory
		// stats with Size() == 0 and takes the empty-file exit instead. A
		// write-only handle stats fine, reports a real size, and refuses to be
		// read anywhere — so the branch is entered on the machine this was
		// written on and on the ones it ships to.
		p := writeLines(t, goodLine(t, "a"))
		f, err := os.OpenFile(p, os.O_WRONLY, 0)
		if err != nil {
			t.Skipf("cannot open write-only here: %v", err)
		}
		defer f.Close() //nolint:errcheck // test handle
		if _, known := endsWithNewline(f); known {
			t.Error("a handle that cannot be read reported its last byte as known")
		}
	})

	t.Run("ReadRecords surfaces it instead of returning no records", func(t *testing.T) {
		recs, skipped, incomplete, err := ReadRecords(t.TempDir())
		if err == nil {
			t.Fatalf("reading a directory as a ledger succeeded: %d records, skipped=%d, "+
				"incomplete=%v. An unreadable ledger must not read as an empty one",
				len(recs), skipped, incomplete)
		}
		if len(recs) != 0 {
			t.Errorf("records returned alongside the error: %d", len(recs))
		}
	})
}

// TL7: a line too long to scan is an error, not an empty ledger.
//
// The scanner is capped at 64MiB per line. Past that it stops and reports
// bufio.ErrTooLong, and `scanner.Err()` was UNREACHED: no test made it true,
// so nothing checked that the cap surfaces as an error rather than as a short
// read. A ledger truncated to "the records before the huge line" would be the
// same defect this file exists to prevent, arriving through the scanner
// instead of through the tail.
//
// The fixture is sparse: Truncate gives 65MiB of zero bytes, which contain no
// newline and cost no disk.
func TestTL7_ALineTooLongToScanIsReported(t *testing.T) {
	p := filepath.Join(t.TempDir(), "huge.jsonl")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	const past64MiB = 65 << 20
	if err := f.Truncate(past64MiB); err != nil {
		_ = f.Close()
		t.Skipf("cannot make a sparse file here: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	recs, _, _, err := ReadRecords(p)
	if err == nil {
		t.Fatalf("a %d-byte line with no newline read cleanly and returned %d records; "+
			"the scanner's cap is being reported as the end of the data", past64MiB, len(recs))
	}
	if len(recs) != 0 {
		t.Errorf("records returned alongside the error: %d", len(recs))
	}
}

// TL8: ReadFile surfaces a read failure instead of "no records".
//
// ReadFile returns `no records in <file>` when the slice is empty, and that
// message is indistinguishable from a ledger that is genuinely empty. The
// error branch above it was UNREACHED, so nothing established that an
// unreadable path takes the first exit rather than falling through to the
// second and telling the reader their ledger has nothing in it.
func TestTL8_ReadFileOnAnUnreadablePathSaysSoNotEmpty(t *testing.T) {
	_, err := ReadFile(t.TempDir())
	if err == nil {
		t.Fatal("reading a directory as a ledger file succeeded")
	}
	if strings.Contains(err.Error(), "no records in") {
		t.Errorf("an unreadable ledger is reported as an empty one: %v.\n"+
			"      Absence and unknown are different values, and the reader acts on "+
			"them differently", err)
	}
}

// TL9: an unreadable pin line is skipped, and the rest of the file still loads.
//
// loadPins says a pin it cannot read "is a decision it must make again rather
// than a reason to refuse to start". Nothing tested it. Both halves of the
// skip were unexercised: a line that is not JSON, and a line that parses but
// carries no session id — the second is the one a hand-edited or
// half-written pins file actually produces, and without the check it would
// install a pin under the empty key that every session with no id then shares.
func TestTL9_AnUnreadablePinIsSkippedNotFatal(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pins.jsonl")
	body := "" +
		"{\"session_id\":\"keep-me\"}\n" +
		"not json at all\n" +
		"{\"session_id\":\"\"}\n" + // parses, names no session
		"{\"session_id\":\"keep-me-too\"}\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	pins, err := loadPins(p)
	if err != nil {
		t.Fatalf("two bad lines made the whole pins file fatal: %v", err)
	}
	if len(pins) != 2 {
		t.Errorf("pins = %d, want 2; got %v", len(pins), pins)
	}
	if _, ok := pins[""]; ok {
		t.Error("a pin was installed under the empty session id. Every session that " +
			"cannot name itself would then share one pin")
	}
	for _, want := range []string{"keep-me", "keep-me-too"} {
		if _, ok := pins[want]; !ok {
			t.Errorf("pin %q was lost to a bad line elsewhere in the file", want)
		}
	}
}

// TL10: a pins file too long to scan is an error, not a silently empty map.
//
// The distinction loadPins draws is between a line it cannot read and a FILE
// it cannot read. The first is skipped; the second must not come back as "no
// pins", because no pins means every session re-decides, and a proxy that
// re-decides silently has lost state it was told to keep.
func TestTL10_AnUnscannablePinsFileIsAnError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pins.jsonl")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(65 << 20); err != nil {
		_ = f.Close()
		t.Skipf("cannot make a sparse file here: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	pins, err := loadPins(p)
	if err == nil {
		t.Fatalf("an unscannable pins file returned %d pins and no error; the proxy "+
			"would treat lost state as absent state", len(pins))
	}
}
