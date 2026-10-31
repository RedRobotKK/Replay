package ledger

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// maxLockedOpenAttempts bounds how often OpenLocked reopens a path whose file
// was replaced while it waited for the lock. Each retry means a rewrite ran to
// completion in between, so reaching the bound means something is rewriting
// this one file continuously, which is a fault to report rather than a queue
// to wait in.
const maxLockedOpenAttempts = 16

// OpenLocked opens a ledger file and returns it holding the exclusive advisory
// lock that every writer of a ledger file takes (ADR-0030): Store.Append, and
// `replay purge --session`, which rewrites a file by renaming a replacement
// over it.
//
// The lock is on the open file, not on the name, so after it is granted the
// file is checked to still be the one at path. A rewrite that renamed a
// replacement into place while this waited has left the lock on a file nobody
// will read again, and anything written to it would be lost; the replacement
// is opened and locked instead. Closing the returned file releases the lock.
//
// On a platform with no advisory lock (Windows, where the binary refuses to
// run) the lock is not taken and only the replaced-file check remains.
func OpenLocked(path string, flag int, perm os.FileMode) (*os.File, error) {
	for attempt := 0; attempt < maxLockedOpenAttempts; attempt++ {
		f, err := os.OpenFile(path, flag, perm)
		if err != nil {
			return nil, err
		}
		if err := lockFile(f); err != nil {
			_ = f.Close() // the lock error is the one worth reporting
			return nil, fmt.Errorf("lock %s: %w", filepath.Base(path), err)
		}
		same, err := stillAt(f, path)
		if err != nil {
			_ = f.Close() // the stat error is the one worth reporting
			return nil, err
		}
		if same {
			return f, nil
		}
		// Replaced or removed while this waited: release the stale lock and
		// take the file that is there now.
		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("release %s: %w", filepath.Base(path), err)
		}
	}
	return nil, fmt.Errorf("%s was replaced %d times while waiting to lock it", filepath.Base(path), maxLockedOpenAttempts)
}

// stillAt reports whether f is the file at path now. A path that no longer
// exists is not f, and is not an error: the caller opens it again.
func stillAt(f *os.File, path string) (bool, error) {
	held, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("stat locked %s: %w", filepath.Base(path), err)
	}
	now, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", filepath.Base(path), err)
	}
	return os.SameFile(held, now), nil
}
