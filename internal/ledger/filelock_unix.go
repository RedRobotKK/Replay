//go:build unix

package ledger

import (
	"errors"
	"os"
	"syscall"
)

// lockFile takes an exclusive flock on f, waiting for it. flock rather than
// fcntl: an fcntl lock belongs to the process and is dropped when ANY
// descriptor it has on the file closes, so a reader elsewhere in the same
// process would silently release a writer's lock.
func lockFile(f *os.File) error {
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

// unlockFile does nothing on unix: closing the descriptor releases the flock,
// which is how the lock has always been released here, so LockedFile.Close
// behaves exactly as the bare close it replaced.
func unlockFile(*os.File) error { return nil }
