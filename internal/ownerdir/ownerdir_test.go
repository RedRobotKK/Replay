package ownerdir

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func requireUnixModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
}

// D1: a pre-existing world-writable directory is not left as it was.
//
// This is finding 7 in one assertion. `mkdir -m 777 ~/.replay` before the
// first run, and os.MkdirAll(dir, 0700) succeeds without changing anything.
//
// PASS: 0700 afterwards.
// FAIL: any group or other bit, which is the ledger and the vault key sitting
// somewhere every local account can reach.
func TestD1_AWorldWritableDirectoryIsTightened(t *testing.T) {
	requireUnixModes(t)
	dir := filepath.Join(t.TempDir(), "replay")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	// os.Mkdir applies the umask, so set the bits explicitly: the hostile
	// case is a directory that really is 0777, not one the test's umask
	// quietly fixed.
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != DirPerm {
		t.Errorf("directory mode = %04o, want %04o", got, DirPerm)
	}
}

// D2: every loose bit is caught, not just the world-writable spelling.
//
// A group-readable directory is enough to lose the vault key, and it is the
// mode a shared-account machine most plausibly has. A check written against
// 0o002 alone would pass D1 and miss this.
func TestD2_EveryGroupAndOtherBitIsCaught(t *testing.T) {
	requireUnixModes(t)
	for _, mode := range []os.FileMode{0o750, 0o705, 0o770, 0o707, 0o701, 0o710, 0o777} {
		dir := filepath.Join(t.TempDir(), "replay")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		if err := EnsureDir(dir); err != nil {
			t.Fatalf("EnsureDir on %04o: %v", mode, err)
		}
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got&0o077 != 0 {
			t.Errorf("directory left at %04o after starting at %04o", got, mode)
		}
	}
}

// D3: an owner-only directory is left exactly as it is.
//
// The repair must not be a blanket chmod. An operator who has chosen 0500 for
// a read-only ledger directory has made a decision inside the boundary this
// check defends, and widening it back to 0700 would be this code overruling
// them in the name of security.
func TestD3_AnAlreadyPrivateDirectoryIsUntouched(t *testing.T) {
	requireUnixModes(t)
	dir := filepath.Join(t.TempDir(), "replay")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o500 {
		t.Errorf("mode = %04o, want it left at 0500", got)
	}
}

// D4: a pre-existing key file that anyone can read is tightened.
//
// os.WriteFile does not change the mode of a file that already exists, so the
// vault key and the ledger label key keep whatever mode they arrived with —
// from an archive, a backup restore, or a version of Replay that predates the
// mode being set at all.
func TestD4_AWorldReadableKeyFileIsTightened(t *testing.T) {
	requireUnixModes(t)
	path := filepath.Join(t.TempDir(), ".vault-key")
	if err := os.WriteFile(path, []byte("not-a-real-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureFile(path); err != nil {
		t.Fatalf("EnsureFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got&0o077 != 0 {
		t.Errorf("key file mode = %04o, want no group or other bits", got)
	}
}

// D5: a missing file is not an error, and a directory in a file's place is.
func TestD5_MissingFileIsFineAndAFileInThePlaceOfADirectoryIsNot(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureFile(filepath.Join(dir, "not-there")); err != nil {
		t.Errorf("EnsureFile on a missing path = %v, want nil", err)
	}
	path := filepath.Join(dir, "occupied")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureDir(path); err == nil {
		t.Error("EnsureDir over an existing regular file must refuse")
	}
}

// D6: a chmod that reports success while the mode does not change is a
// refusal, not a silent pass.
//
// This is the branch that matters and the one a real filesystem here will not
// produce: chmod needs ownership, not a writable parent, so the obvious
// "directory owned by somebody else" setup succeeds on macOS and the
// verification after it is never entered. An exFAT volume, a network mount
// with a fixed file mode, or a container bind mount does exactly this —
// returns nil and leaves 0777 in place.
//
// Neutralising the verification and watching this test stay green was how the
// seam came to exist: the first version of this file had a test for this case
// that skipped on the developer's machine, which is a check that cannot fail.
//
// PASS: an error, and one that names the mode.
// FAIL: nil, which is Replay writing the vault key into a directory it has
// just failed to make private and reporting success.
func TestD6_AChmodThatDoesNotTakeIsRefused(t *testing.T) {
	requireUnixModes(t)
	dir := filepath.Join(t.TempDir(), "replay")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	restore := chmod
	chmod = func(string, os.FileMode) error { return nil } // the lying filesystem
	defer func() { chmod = restore }()

	err := EnsureDir(dir)
	if err == nil {
		t.Fatal("EnsureDir returned nil after a chmod that did not take")
	}
	if !strings.Contains(err.Error(), "0777") {
		t.Errorf("the error must name the mode it found; got %q", err)
	}
}

// D7: a chmod that returns an error is a refusal too.
//
// The sibling of D6. On a real filesystem this is a directory belonging to
// another account, which a test cannot create without being root.
func TestD7_AChmodThatFailsIsRefused(t *testing.T) {
	requireUnixModes(t)
	dir := filepath.Join(t.TempDir(), "replay")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	restore := chmod
	chmod = func(string, os.FileMode) error { return errors.New("operation not permitted") }
	defer func() { chmod = restore }()

	err := EnsureDir(dir)
	if err == nil {
		t.Fatal("EnsureDir returned nil after a chmod that failed")
	}
	if !strings.Contains(err.Error(), "operation not permitted") {
		t.Errorf("the underlying failure must survive into the error; got %q", err)
	}
}

// D8: the seam is not load-bearing in production.
//
// A seam that a test replaces is a seam a mistake can leave replaced. This
// pins that the package-level values are the standard library's.
func TestD8_TheSeamDefaultsToTheRealSyscalls(t *testing.T) {
	if reflect.ValueOf(chmod).Pointer() != reflect.ValueOf(os.Chmod).Pointer() {
		t.Error("chmod is not os.Chmod")
	}
	if reflect.ValueOf(stat).Pointer() != reflect.ValueOf(os.Stat).Pointer() {
		t.Error("stat is not os.Stat")
	}
}

// D9: a stat that fails is reported, not treated as "nothing to do".
//
// The realistic shape is a symlink loop or a permission error at
// `~/.replay/vault/.vault-key`. Swallowing it would mean this package
// declaring a path private on the strength of a reading it never got — which
// is the failure mode the whole file exists to avoid.
//
// Driven through the seam because the branch is otherwise unreachable:
// guard-reachability reported it as UNREACHED, and a branch no test can enter
// is indistinguishable from one that is not there.
func TestD9_AStatThatFailsIsReported(t *testing.T) {
	restore := stat
	stat = func(string) (os.FileInfo, error) { return nil, errors.New("input/output error") }
	defer func() { stat = restore }()

	err := EnsureDir(t.TempDir())
	if err == nil {
		t.Fatal("EnsureDir returned nil when it could not stat the directory")
	}
	if !strings.Contains(err.Error(), "input/output error") {
		t.Errorf("the underlying failure must survive into the error; got %q", err)
	}
}

// D10: a re-stat that fails after the chmod is reported too.
//
// The chmod said it worked. Without a successful reading afterwards there is
// no evidence it did, and "the chmod returned nil" is exactly the evidence D6
// already showed to be worthless.
func TestD10_ARestatThatFailsAfterTheChmodIsReported(t *testing.T) {
	requireUnixModes(t)
	dir := filepath.Join(t.TempDir(), "replay")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	calls := 0
	restore := stat
	stat = func(p string) (os.FileInfo, error) {
		calls++
		if calls > 1 {
			return nil, errors.New("stale NFS file handle")
		}
		return os.Stat(p)
	}
	defer func() { stat = restore }()

	err := EnsureDir(dir)
	if err == nil {
		t.Fatal("EnsureDir returned nil when it could not confirm the chmod took")
	}
	if !strings.Contains(err.Error(), "stale NFS file handle") {
		t.Errorf("the underlying failure must survive into the error; got %q", err)
	}
}
