package ledger

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// G207 / identity integrity: a corrupted .label-key must not be silently
// replaced. The key is what makes path labels and tool-call keys anonymous;
// overwriting it after corruption re-keys existing evidence under a new
// secret and makes prior labels unverifiable.
//
// Required behaviour:
//   - missing key  → create a new 32-byte key
//   - valid 32-byte key → preserve exactly
//   - existing key of any other length → refuse, preserve original bytes
//   - unreadable key → meaningful read error (covered by OS permission tests elsewhere)
//
// RED on main: loadOrCreateKey regenerates and overwrites short/long/empty keys.

func TestLabelKey_MissingCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, labelKeyFile)

	key, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatalf("missing key: %v", err)
	}
	if len(key) != labelKeyBytes {
		t.Fatalf("new key length = %d, want %d", len(key), labelKeyBytes)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, onDisk) {
		t.Fatalf("returned key does not match bytes on disk")
	}
}

func TestLabelKey_ValidPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, labelKeyFile)
	want := bytes.Repeat([]byte{0xab}, labelKeyBytes)
	if err := os.WriteFile(path, want, filePerm); err != nil {
		t.Fatal(err)
	}

	got, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatalf("valid key: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("valid key was altered")
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, want) {
		t.Fatalf("valid key on disk was altered")
	}
}

func TestLabelKey_ShortRefusedAndPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, labelKeyFile)
	corrupt := bytes.Repeat([]byte{0xcd}, 16) // half length
	if err := os.WriteFile(path, corrupt, filePerm); err != nil {
		t.Fatal(err)
	}

	_, err := loadOrCreateKey(path)
	if err == nil {
		t.Fatal("short key: expected error, got nil (silent re-key)")
	}
	onDisk, err2 := os.ReadFile(path)
	if err2 != nil {
		t.Fatal(err2)
	}
	if !bytes.Equal(onDisk, corrupt) {
		t.Fatalf("short key was overwritten; original corrupt bytes must be preserved\ngot  %x\nwant %x", onDisk, corrupt)
	}
}

func TestLabelKey_LongRefusedAndPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, labelKeyFile)
	corrupt := bytes.Repeat([]byte{0xef}, labelKeyBytes+8)
	if err := os.WriteFile(path, corrupt, filePerm); err != nil {
		t.Fatal(err)
	}

	_, err := loadOrCreateKey(path)
	if err == nil {
		t.Fatal("long key: expected error, got nil (silent re-key)")
	}
	onDisk, err2 := os.ReadFile(path)
	if err2 != nil {
		t.Fatal(err2)
	}
	if !bytes.Equal(onDisk, corrupt) {
		t.Fatalf("long key was overwritten; original corrupt bytes must be preserved")
	}
}

func TestLabelKey_EmptyRefusedAndPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, labelKeyFile)
	if err := os.WriteFile(path, []byte{}, filePerm); err != nil {
		t.Fatal(err)
	}

	_, err := loadOrCreateKey(path)
	if err == nil {
		t.Fatal("empty key: expected error, got nil (silent re-key)")
	}
	onDisk, err2 := os.ReadFile(path)
	if err2 != nil {
		t.Fatal(err2)
	}
	if len(onDisk) != 0 {
		t.Fatalf("empty key was overwritten; original empty file must be preserved, got %d bytes", len(onDisk))
	}
}

func TestLabelKey_OpenSurfacesRefusal(t *testing.T) {
	dir := t.TempDir()
	// Open calls ownerdir.EnsureDir / EnsureFile then loadOrCreateKey.
	// Plant a corrupt key after the directory exists so EnsureFile does not
	// replace it, then Open must refuse.
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, labelKeyFile)
	corrupt := []byte("not-32-bytes")
	if err := os.WriteFile(path, corrupt, filePerm); err != nil {
		t.Fatal(err)
	}

	_, err := Open(dir)
	if err == nil {
		t.Fatal("Open with corrupt label key: expected error, got nil")
	}
	// Original bytes must still be on disk after the failed Open.
	onDisk, err2 := os.ReadFile(path)
	if err2 != nil {
		t.Fatal(err2)
	}
	if !bytes.Equal(onDisk, corrupt) {
		t.Fatalf("Open overwrote corrupt label key")
	}
	// Error should be informative (not a generic open failure that hides the cause).
	if !errors.Is(err, errCorruptLabelKey) && !containsAll(err.Error(), "label key", "length") {
		// Accept either a sentinel or a message that names the problem.
		t.Logf("Open error (acceptable if it names the length problem): %v", err)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !bytes.Contains([]byte(s), []byte(p)) {
			return false
		}
	}
	return true
}
