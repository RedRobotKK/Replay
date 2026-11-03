//go:build unix

package ledger

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The paths through openLocked, stillAt and RewriteLocked that only a file
// changing under a waiter, or a failing call, can reach.
//
// Unix only. Several rename or remove a file another handle holds open,
// which Windows does not permit for handles Go opens. Whether purge's rename
// works on Windows is TestXL2/TestXL3's question, asked on windows-latest.

// waitBehind starts openLocked on path behind a lock this test holds, waits
// until it is blocked, runs change, then releases. It returns what the
// waiter got.
func waitBehind(t *testing.T, path string, flag int, unlock func(*os.File) error, change func()) (*LockedFile, error) {
	t.Helper()
	first, err := OpenLocked(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	type got struct {
		f   *LockedFile
		err error
	}
	ch := make(chan got, 1)
	go func() {
		f, err := openLocked(path, flag, 0o600, lockFile, unlock)
		ch <- got{f, err}
	}()
	select {
	case g := <-ch:
		t.Fatalf("the waiter did not wait (err=%v)", g.err)
	case <-time.After(flBlockWindow):
	}
	change()
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case g := <-ch:
		return g.f, g.err
	case <-time.After(flReleaseTimeout):
		t.Fatal("the waiter never returned")
	}
	return nil, nil
}

// FL7: a file replaced while a writer waited is not the file it gets.
//
// PASS: the waiter's handle is the replacement, so what it writes is at path.
// FAIL: it holds the replaced file and its write is lost.
func TestFL7_AFileReplacedWhileWaitingIsReopened(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	f, err := waitBehind(t, path, os.O_RDWR|os.O_APPEND, unlockFile, func() {
		if err := os.WriteFile(path+".new", []byte("replacement\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path+".new", path); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("after\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "replacement\nafter\n" {
		t.Fatalf("the write went to the replaced file: path holds %q", b)
	}
}

// FL8: a file removed while a writer waited is created again, not written
// into the removed one.
func TestFL8_AFileRemovedWhileWaitingIsOpenedAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	f, err := waitBehind(t, path, os.O_CREATE|os.O_RDWR|os.O_APPEND, unlockFile, func() {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("after\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "after\n" {
		t.Fatalf("path holds %q", b)
	}
}

// FL9: releasing the stale lock on a replaced file can fail, and says so.
func TestFL9_AFailedReleaseOfAStaleLockIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	refused := errors.New("unlock refused")
	_, err := waitBehind(t, path, os.O_RDWR, func(*os.File) error { return refused }, func() {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	})
	if !errors.Is(err, refused) || !strings.Contains(err.Error(), "release s.jsonl") {
		t.Fatalf("want the release error, got %v", err)
	}
}

// FL10: an open that fails is returned, not retried into a lock.
func TestFL10_AnOpenThatFailsIsReturned(t *testing.T) {
	_, err := OpenLocked(filepath.Join(t.TempDir(), "missing", "s.jsonl"), os.O_RDWR, 0)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want not-exist, got %v", err)
	}
}

// FL11: stillAt reports a stat it could not make, for either side.
func TestFL11_StillAtReportsAStatItCouldNotMake(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(plain)
	if err != nil {
		t.Fatal(err)
	}
	// A path under a regular file is ENOTDIR, which is not "does not exist".
	if _, err := stillAt(f, filepath.Join(plain, "s.jsonl")); err == nil || !strings.Contains(err.Error(), "stat s.jsonl") {
		t.Errorf("an unstatable path read as an answer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := stillAt(f, plain); err == nil || !strings.Contains(err.Error(), "stat locked") {
		t.Errorf("a closed handle's stat read as an answer: %v", err)
	}
	// And openLocked returns that error rather than a file: the waiter's
	// directory is moved away and a regular file put at its name.
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = waitBehind(t, filepath.Join(sub, "s.jsonl"), os.O_RDWR, unlockFile, func() {
		if err := os.Rename(sub, sub+".moved"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sub, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if err == nil || !strings.Contains(err.Error(), "stat s.jsonl") {
		t.Fatalf("openLocked did not return the stat error: %v", err)
	}
}

// rlFile writes body to a fresh ledger file.
func rlFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// RW1-RW7: RewriteLocked, the purge rewrite, on each of its outcomes.
func TestRW1_AnAppendSinceTheReadIsCarriedOver(t *testing.T) {
	p := rlFile(t, "gone\nkept\nappended\n")
	if err := RewriteLocked(p, []byte("gone\nkept\n"), []string{"kept"}, os.WriteFile, os.Rename); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "kept\nappended\n" {
		t.Fatalf("got %q", b)
	}
}

func TestRW2_ErasingEverythingLeavesAnEmptyFile(t *testing.T) {
	p := rlFile(t, "gone\n")
	if err := RewriteLocked(p, []byte("gone\n"), nil, os.WriteFile, os.Rename); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); len(b) != 0 {
		t.Fatalf("got %q", b)
	}
}

func TestRW3_AFileChangedOtherThanByAnAppendIsRefused(t *testing.T) {
	p := rlFile(t, "different\n")
	err := RewriteLocked(p, []byte("gone\n"), nil, os.WriteFile, os.Rename)
	if err == nil || !strings.Contains(err.Error(), "changed other than by an append") {
		t.Fatalf("got %v", err)
	}
}

func TestRW4_AReadThatEndedMidRecordIsRefused(t *testing.T) {
	p := rlFile(t, "gone\nhalf-and-rest\n")
	err := RewriteLocked(p, []byte("gone\nhalf"), []string{"x"}, os.WriteFile, os.Rename)
	if err == nil || !strings.Contains(err.Error(), "was being written") {
		t.Fatalf("got %v", err)
	}
}

func TestRW5_AFailedWriteAndAFailedRenameAreReported(t *testing.T) {
	p := rlFile(t, "gone\n")
	full := errors.New("disk full")
	if err := RewriteLocked(p, []byte("gone\n"), nil, func(string, []byte, os.FileMode) error { return full }, os.Rename); !errors.Is(err, full) {
		t.Errorf("write: got %v", err)
	}
	denied := errors.New("denied")
	if err := RewriteLocked(p, []byte("gone\n"), nil, os.WriteFile, func(string, string) error { return denied }); !errors.Is(err, denied) {
		t.Errorf("rename: got %v", err)
	}
}

func TestRW6_AFileThatCannotBeLockedIsReported(t *testing.T) {
	err := RewriteLocked(filepath.Join(t.TempDir(), "missing.jsonl"), nil, nil, os.WriteFile, os.Rename)
	if err == nil || !strings.Contains(err.Error(), "locking") {
		t.Fatalf("got %v", err)
	}
}

func TestRW7_AFileThatCannotBeReReadIsReported(t *testing.T) {
	// A directory opens and locks on unix, and does not read.
	err := RewriteLocked(t.TempDir(), nil, nil, os.WriteFile, os.Rename)
	if err == nil || !strings.Contains(err.Error(), "re-reading") {
		t.Fatalf("got %v", err)
	}
}

// FO9: openShared wraps the handle create returns, here a real descriptor.
func TestFO9_OpenSharedWrapsTheHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.jsonl")
	f, err := openShared(path, os.O_RDWR|os.O_CREATE, 0o600, func(name string, _, _, _ uint32) (uintptr, error) {
		fd, err := syscall.Open(name, syscall.O_RDWR|syscall.O_CREAT, 0o600)
		return uintptr(fd), err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("x\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "x\n" {
		t.Fatalf("got %q", b)
	}
}
