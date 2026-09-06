package consent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A consent file the user wrote must be readable on every platform this
// project ships to.
//
// The permission gate was written in Unix terms: refuse the file if
// Perm()&0o022 is set, because a file any process can write is not this
// user's decision. That is correct on Unix and meaningless on Windows, where
// there are no mode bits and Go synthesises 0666 for any writable file. The
// gate therefore refused every consent decision on Windows, including one the
// user had just written through the installer, and it refused with a message
// about group and other permissions that do not exist there.
//
// The consequence was not a failing test. It was a Windows user who could not
// opt into the corpus and could not grant update consent, because the answer
// they gave was thrown away every time it was read.
func TestConsent_AUserWrittenFileIsReadableOnThisPlatform(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "replay")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg, CorpusFileName)
	if err := os.WriteFile(path, []byte("corpus_opt_in = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	d, err := ReadCorpusConsent(dir)
	if err != nil {
		t.Fatalf("a 0600 file the user wrote must be readable on %s, not refused: %v",
			runtime.GOOS, err)
	}
	if !d.Allowed() {
		t.Errorf("the file grants consent; got state %q", d.State)
	}
}

// Where the check cannot run, the decision must say so rather than imply a
// guarantee it did not make.
//
// Silently skipping the gate on Windows would leave every caller believing
// the file was verified as this user's, which is the failure mode this whole
// package exists to avoid. OwnershipChecked reports whether the check
// actually happened, so a caller can tell "verified" from "not verifiable
// here".
func TestConsent_TheDecisionReportsWhetherOwnershipWasVerified(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "replay")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, CorpusFileName), []byte("corpus_opt_in = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := ReadCorpusConsent(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := runtime.GOOS != "windows"
	if d.OwnershipChecked != want {
		t.Errorf("OwnershipChecked = %v on %s, want %v. On Unix the mode bits are "+
			"meaningful and the check runs; on Windows they are not and the decision "+
			"must not claim it verified anything", d.OwnershipChecked, runtime.GOOS, want)
	}
}

// The Unix gate itself must keep working. Loosening it to fix Windows would
// trade a broken platform for a broken guarantee.
func TestConsent_AWorldWritableFileIsStillRefusedOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no Unix mode bits on Windows; the sibling test covers what applies there")
	}
	dir := t.TempDir()
	cfg := filepath.Join(dir, "replay")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg, CorpusFileName)
	if err := os.WriteFile(path, []byte("corpus_opt_in = true\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o666); err != nil {
		t.Fatal(err)
	}
	d, err := ReadCorpusConsent(dir)
	if err == nil {
		t.Fatalf("a 0666 consent file must be refused on Unix; got state %q", d.State)
	}
	if !strings.Contains(err.Error(), "writable by group or other") {
		t.Errorf("the refusal must say why: %v", err)
	}
	if d.Allowed() {
		t.Error("a refused decision must never be Allowed")
	}
}
