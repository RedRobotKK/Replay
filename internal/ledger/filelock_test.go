//go:build unix || windows

package ledger

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The ledger's file lock, exercised through separate open handles.
//
// Every test here opens the file more than once. flock (unix) and LockFileEx
// (Windows) both lock per open file, not per process, so two handles in one
// process contend exactly as two processes do. A same-process mutex would not
// make any of these pass: Store's mutex is per Store, and these use either no
// Store or one Store per writer. The cross-process versions, with real child
// processes, are in internal/blackbox (TestXL*).
//
// These run on every CI runner, windows-latest included: they are built
// wherever a lock is implemented and start no process. FL3, the refusal where
// none is, is in filelock_failclosed_test.go and runs everywhere.

const (
	// flBlockWindow is how long a second locker must stay waiting for the
	// test to call it blocked. Taking a free lock takes microseconds.
	flBlockWindow = 300 * time.Millisecond
	// flReleaseTimeout bounds how long a released lock may take to reach
	// the waiter before the test calls it never released.
	flReleaseTimeout = 10 * time.Second
	// flWorkers and flRounds size the contention tests.
	flWorkers = 4
	flRounds  = 50
	// flHoldFor is how long each contention round holds the lock, so an
	// overlap has time to be seen.
	flHoldFor = 100 * time.Microsecond
)

// FL1: a second handle waits while the first holds the lock, and gets it once
// the first is closed.
//
// PASS: the second OpenLocked does not return within flBlockWindow, then
// returns without error after the first is closed.
// FAIL: it returns while the first is held (no exclusion), or never returns
// (closing did not release).
func TestFL1_ASecondHandleWaitsUntilTheFirstIsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	first, err := OpenLocked(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan error, 1)
	go func() {
		second, err := OpenLocked(path, os.O_RDWR, 0)
		if err == nil {
			err = second.Close()
		}
		got <- err
	}()
	select {
	case err := <-got:
		_ = first.Close() // the test has already failed; this only tidies up
		t.Fatalf("a second handle took the lock while the first held it (err=%v)", err)
	case <-time.After(flBlockWindow):
	}
	if err := first.Close(); err != nil {
		t.Fatalf("releasing the first lock: %v", err)
	}
	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("the waiter failed after the lock was released: %v", err)
		}
	case <-time.After(flReleaseTimeout):
		t.Fatal("closing the first handle did not release the lock to the waiter")
	}
}

// FL2: repeated lock and unlock under contention never lets two holders in.
//
// PASS: across flWorkers x flRounds acquisitions, the holder count inside the
// critical section is never above one.
// FAIL: any overlap, or any acquisition or release error.
func TestFL2_RepeatedLockUnlockUnderContentionIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var holders, overlaps atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, flWorkers*flRounds)
	for w := 0; w < flWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < flRounds; r++ {
				f, err := OpenLocked(path, os.O_RDWR, 0)
				if err != nil {
					errs <- err
					return
				}
				if holders.Add(1) != 1 {
					overlaps.Add(1)
				}
				time.Sleep(flHoldFor)
				holders.Add(-1)
				if err := f.Close(); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("lock or unlock failed: %v", err)
	}
	if n := overlaps.Load(); n != 0 {
		t.Fatalf("%d acquisitions overlapped another holder: the lock did not exclude", n)
	}
}

// FL4: the lock does not stop a plain reader.
//
// On Windows a LockFileEx range is mandatory: a handle that did not take the
// lock cannot read or write bytes inside it. ReadRecords, IsLedgerFile and
// purge's walk read ledger files without the lock, so the locked range must
// not cover the data. On unix flock is advisory and this holds trivially.
//
// PASS: os.ReadFile sees the file's bytes while another handle holds the lock.
// FAIL: the read errors (a lock over the data on Windows) or differs.
func TestFL4_TheLockDoesNotBlockAPlainReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	const body = "{\"a\":1}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := OpenLocked(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // the assertion below is the test; release is FL1's
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("a plain read failed while the lock was held: %v", err)
	}
	if string(b) != body {
		t.Fatalf("a plain read saw %q, want %q", b, body)
	}
}

// FL5: writers with their own Store each lose nothing when they contend.
//
// One Store per writer, so Store's own mutex serialises nothing and the file
// lock is the only coordination, as between two `replay serve` processes.
//
// PASS: every record is in the file exactly once and every line decodes.
// FAIL: a missing, duplicated or unreadable line.
func TestFL5_WritersWithTheirOwnStoreLoseNothing(t *testing.T) {
	dir := t.TempDir()
	stores := make([]*Store, flWorkers)
	for i := range stores {
		s, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		stores[i] = s
	}
	var wg sync.WaitGroup
	errs := make(chan error, flWorkers)
	for w, s := range stores {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < flRounds; r++ {
				if err := s.Append(Record{SessionID: "a", RequestID: fmt.Sprintf("w%d-%d", w, r)}); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("append failed: %v", err)
	}
	seen := readRequestIDs(t, filepath.Join(dir, "a.jsonl"))
	for w := 0; w < flWorkers; w++ {
		for r := 0; r < flRounds; r++ {
			id := fmt.Sprintf("w%d-%d", w, r)
			if seen[id] != 1 {
				t.Errorf("record %s appears %d times, want 1", id, seen[id])
			}
		}
	}
	if len(seen) != flWorkers*flRounds {
		t.Errorf("%d distinct records, want %d", len(seen), flWorkers*flRounds)
	}
}

// readRequestIDs counts each request id in a ledger file and fails on any line
// that is not a record.
func readRequestIDs(t *testing.T, path string) map[string]int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // read-only
	seen := map[string]int{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec Record
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("unreadable line %q: %v", sc.Text(), err)
		}
		seen[rec.RequestID]++
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return seen
}
