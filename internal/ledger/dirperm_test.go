package ledger

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Finding 7 of the 2026-09-04 adversarial security review, at the ledger.
//
// Verbatim:
//
//	Directory permissions are set at creation and never verified, so a
//	pre-existing world-writable directory stays world-writable
//	(`ledger/store.go:52`, `masking/vault.go:62`)
//
// What is at stake here specifically: the ledger directory holds the label
// key, and the label key is what makes the hashed path labels and the tool
// call keys anonymous. Another local account that can read `.label-key` can
// un-anonymise every path and confirm every tool call in every session file
// beside it — which is the same disclosure findings 3 and 4 are about,
// arriving through the directory instead.
//
// The repair lives in internal/ownerdir, which has its own boundary tests.
// These two pin that the ledger actually calls it.

// LP1: opening a ledger over a pre-existing 0777 directory leaves it private.
//
// PASS: 0700 after Open.
// FAIL: still 0777, which is os.MkdirAll succeeding as a no-op — the exact
// mechanism the finding names.
func TestLP1_OpenTightensAPreExistingWorldWritableLedgerDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	dir := filepath.Join(t.TempDir(), "ledger")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err != nil {
		t.Fatalf("Open: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got&0o077 != 0 {
		t.Errorf("ledger directory left at %04o after Open", got)
	}
}

// LP2: a pre-existing label key that others can read is tightened.
//
// os.WriteFile leaves an existing file's mode alone, and the key is only
// written when it is absent or the wrong length — so a 32-byte key at 0644
// is loaded and used, at 0644, forever.
func TestLP2_OpenTightensAWorldReadableLabelKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, labelKeyFile)
	if err := os.WriteFile(key, make([]byte, labelKeyBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(key, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err != nil {
		t.Fatalf("Open: %v", err)
	}
	info, err := os.Stat(key)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got&0o077 != 0 {
		t.Errorf("label key left at %04o after Open; anyone who can read it can un-anonymise the ledger beside it", got)
	}
}

// LP3: a ledger directory that cannot be made private stops Open.
//
// The check is only worth having if its failure is fatal. A version that
// logged and carried on would write the label key and every session file into
// a directory it had just failed to make private, which is worse than not
// checking: it produces a reassuring line in the log.
//
// The path is a real one — a regular file where the directory should be, which
// is what a botched `~/.replay` restore looks like — rather than a seam.
func TestLP3_ALedgerDirectoryThatCannotBeCreatedStopsOpen(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(filepath.Join(file, "ledger")); err == nil {
		t.Fatal("Open succeeded with a regular file in the path of its directory")
	} else if !strings.Contains(err.Error(), "ledger directory") {
		t.Errorf("the error must say which directory failed; got %q", err)
	}
}

// LP4: a label key that cannot be read is a refusal, not a new key.
//
// A symlink loop at `.label-key` is the shape this takes on a corrupted or
// hostile `~/.replay`. It matters more than it looks: loadOrCreateKey treats
// any read failure as "no key yet" and generates a fresh one, so without the
// check ahead of it an unreadable key silently becomes a NEW key — and every
// hashed path label and tool call key in the existing ledger beside it stops
// matching, with nothing said.
func TestLP4_AnUnreadableLabelKeyStopsOpen(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink loops")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, labelKeyFile)
	if err := os.Symlink(key, key); err != nil {
		t.Skipf("cannot create a symlink loop here: %v", err)
	}
	// The prefix, not a substring. Without it this test passes either way:
	// neutralise the EnsureFile check and loadOrCreateKey reaches the same
	// loop, fails its own write, and returns "write label key: ..." — which
	// also contains "label key". guard-reachability called that INERT, and it
	// was right: two error paths over one condition, each making the other
	// look tested.
	if _, err := Open(dir); err == nil {
		t.Fatal("Open succeeded with an unreadable label key, and would have generated a replacement")
	} else if !strings.HasPrefix(err.Error(), "ledger label key:") {
		t.Errorf("the key must be refused before loadOrCreateKey decides to generate a new one; got %q", err)
	}
}
