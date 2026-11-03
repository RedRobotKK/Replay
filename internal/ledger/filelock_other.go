//go:build !unix && !windows

package ledger

import "os"

// lockFile fails where no file lock is implemented (js, wasip1, plan9, ...).
// It used to return nil, which let Append and purge write without the lock
// every ledger writer relies on (ADR-0030). Refusing is the safe default: a
// record not written is reported, and a record lost to an unlocked rewrite is
// not.
func lockFile(f *os.File) error { return lockUnsupported(f) }

// unlockFile is unreachable here, since lockFile never succeeds; it refuses
// for the same reason rather than claiming a release.
func unlockFile(f *os.File) error { return lockUnsupported(f) }
