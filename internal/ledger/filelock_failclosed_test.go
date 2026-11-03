package ledger

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FL3: a lock that cannot be taken is an error, and nothing is written.
//
// This is the path every platform without a supported lock takes
// (filelock_other.go): lockFile is lockUnsupported there. It is driven here
// through openLocked's lock parameter so it runs on the platforms CI has.
//
// PASS: openLocked returns an error that is errLockUnsupported and names the
// platform, returns no file, and the file's bytes are unchanged.
// FAIL: a nil error, which is a writer proceeding unlocked.
func TestFL3_ALockThatCannotBeTakenIsAnErrorNotAnUnlockedWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	const before = "existing\n"
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := openLocked(path, os.O_APPEND|os.O_RDWR, 0, lockUnsupported, unlockFile)
	if err == nil {
		_ = f.Close() // the test has already failed; this only tidies up
		t.Fatal("openLocked returned a file although its lock could not be taken")
	}
	if f != nil {
		t.Errorf("openLocked returned a file alongside its error")
	}
	if !errors.Is(err, errLockUnsupported) {
		t.Errorf("the error does not say the lock is unsupported: %v", err)
	}
	if !strings.Contains(err.Error(), "s.jsonl") {
		t.Errorf("the error does not name the file: %v", err)
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(b) != before {
		t.Errorf("the file changed: %q", b)
	}
}
