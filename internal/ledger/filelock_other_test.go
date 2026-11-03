//go:build !unix && !windows

package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

// FL6: on a platform with no supported lock, lockFile refuses and Append
// writes nothing.
//
// Only built where filelock_other.go is, so it runs only on such a GOOS
// (locally: GOOS=js GOARCH=wasm with Go's node exec wrapper). FL3 covers the
// same refusal on every CI platform through openLocked's lock parameter.
//
// PASS: lockFile returns an error, and Append returns an error with no record
// in the file.
// FAIL: a nil from lockFile, which is every writer proceeding unlocked.
func TestFL6_WithoutASupportedLockAppendRefuses(t *testing.T) {
	dir := t.TempDir()
	probe, err := os.CreateTemp(dir, "probe")
	if err != nil {
		t.Fatal(err)
	}
	defer probe.Close() //nolint:errcheck // probe only
	if err := lockFile(probe); err == nil {
		t.Fatal("lockFile returned nil on a platform with no file lock: writers would proceed unlocked")
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Append(Record{SessionID: "a", RequestID: "must-not-land"}); err == nil {
		t.Fatal("Append succeeded without a lock")
	}
	b, err := os.ReadFile(filepath.Join(dir, "a.jsonl"))
	if err == nil && len(b) != 0 {
		t.Fatalf("Append wrote %q without a lock", b)
	}
}
