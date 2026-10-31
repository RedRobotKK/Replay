package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// An erasure must not lose a record `replay serve` appends while it runs.
//
// purgeSession reads each ledger file, keeps every line that is not the
// erased session's, writes the kept lines to a sibling and renames it over the
// original. `replay serve` appends to the same file from another process, one
// record per open. A record appended after the read and before the rename was
// in the original and not in the replacement, so the rename deleted it: a
// record that was never matched, exported or reported, gone.
//
// The append is made from the rewrite seam, which runs between the read and
// the rename, by a ledger.Store exactly as serve makes it. It runs on its own
// goroutine because a correct purge holds the file while it rewrites and the
// append has to wait for it; appendSettle bounds how long the seam waits
// before letting the purge go on.

// appendSettle is how long the rewrite seam waits for the concurrent append to
// land before continuing. Unlocked, the append takes microseconds; locked, it
// waits for the purge and this lapses.
const appendSettle = 300 * time.Millisecond

// PG25: a record appended during an erasure survives it.
//
// PASS: the appended record is in the file afterwards, the erased session is
// not, and the other session's record is untouched.
// FAIL: the appended record is gone.
func TestPG25_ARecordAppendedDuringAnErasureSurvivesIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the ledger's file lock is a no-op on Windows, where the binary refuses to run")
	}
	dir := ledgerSessions(t, map[string][]string{"a.jsonl": {"wanted", "kept"}})
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	var appended sync.WaitGroup
	var appendErr error
	orig := purgeWriteFile
	purgeWriteFile = func(name string, data []byte, perm os.FileMode) error {
		appended.Add(1)
		done := make(chan struct{})
		go func() {
			defer appended.Done()
			defer close(done)
			// sessionFileName("a") is "a", so this lands in a.jsonl, as
			// serve's next record for session "a" would.
			appendErr = store.Append(ledger.Record{SessionID: "a", RequestID: "appended-during-purge"})
		}()
		select {
		case <-done:
		case <-time.After(appendSettle):
		}
		return orig(name, data, perm)
	}
	t.Cleanup(func() { purgeWriteFile = orig })

	var out, errb bytes.Buffer
	if err := runPurge([]string{dir, "--session", "wanted", "--yes"}, &out, &errb); err != nil {
		t.Fatalf("erasure failed: %v", err)
	}
	appended.Wait()
	if appendErr != nil {
		t.Fatalf("the concurrent append failed: %v", appendErr)
	}
	body, err := os.ReadFile(filepath.Join(dir, "a.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "appended-during-purge") {
		t.Fatalf("a record appended while the erasure ran was deleted by its rename:\n%s", s)
	}
	if strings.Contains(s, `"wanted"`) {
		t.Errorf("the erased session survived:\n%s", s)
	}
	if !strings.Contains(s, `"kept"`) {
		t.Errorf("the other session's record was lost:\n%s", s)
	}
	if !strings.HasSuffix(s, "\n") {
		t.Errorf("the ledger no longer ends in a newline: %q", s)
	}
}

// PG26: two erasures of one file at once lose nothing either asked to keep.
//
// Both used to write the same fixed sibling, a.jsonl.tmp, and rename it: one
// could rename the other's half-written file into place, or rename its own
// copy, made from a read taken before the other's rename, back over the
// other's result. Whichever finishes, every record that neither erasure
// matched must still be there, and an erasure that reports success must have
// actually removed its session.
func TestPG26_ConcurrentErasuresOfOneFileLoseNothingKept(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the ledger's file lock is a no-op on Windows, where the binary refuses to run")
	}
	const keepers = 50
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString(`{"session_id":"one"}` + "\n")
	b.WriteString(`{"session_id":"two"}` + "\n")
	for i := 0; i < keepers; i++ {
		b.WriteString(`{"session_id":"keep"}` + "\n")
	}
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := map[string]error{}
	var mu sync.Mutex
	for _, id := range []string{"one", "two"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out bytes.Buffer
			err := purgeSession(dir, id, "", true, &out)
			mu.Lock()
			errs[id] = err
			mu.Unlock()
		}()
	}
	wg.Wait()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if got := strings.Count(s, `"keep"`); got != keepers {
		t.Fatalf("records neither erasure matched = %d, want %d:\n%s", got, keepers, s)
	}
	for id, err := range errs {
		if err == nil && strings.Contains(s, `"`+id+`"`) {
			t.Errorf("erasure of %q reported success and its record is still there", id)
		}
	}
	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("a temporary file was left beside the ledger")
	}
}

// PG27: an erasure that read half of a record still being written refuses the
// file rather than splitting that record across two lines.
//
// The walk can land inside a write: the file then ends in half a record, and
// its rest arrives before the rewrite takes the lock. The kept lines end with
// a newline, so carrying the rest over after them would leave two unreadable
// halves where one complete record was. Driven through rewriteLocked, because
// the window is between the walk and the rewrite and no seam sits in it.
func TestPG27_AnErasureThatReadAHalfWrittenRecordRefusesTheFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the ledger's file lock is a no-op on Windows, where the binary refuses to run")
	}
	path := filepath.Join(t.TempDir(), "a.jsonl")
	const head = `{"session_id":"wanted"}` + "\n" + `{"session_id":"kept"}` + "\n" + `{"session_id":"kept","request_id":"half`
	const rest = `-written"}` + "\n"
	// The rest arrived after the walk read head.
	if err := os.WriteFile(path, []byte(head+rest), 0o600); err != nil {
		t.Fatal(err)
	}
	kept := []string{`{"session_id":"kept"}`, `{"session_id":"kept","request_id":"half`}

	err := rewriteLocked(path, []byte(head), kept)
	if err == nil || !strings.Contains(err.Error(), "being written") {
		t.Fatalf("err = %v, want a refusal naming the half-written read", err)
	}
	body, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(body) != head+rest {
		t.Fatalf("the refused file was changed:\n%q", body)
	}
}
