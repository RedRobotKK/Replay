package ledger

import (
	"errors"
	"fmt"
	"syscall"
)

// How the Windows lock reads the results of its kernel32 calls
// (filelock_windows.go).
//
// It lives here, untagged and free of any Windows API, so the decisions it
// makes are tested (TestPC*) and neutralised by guard reachability on every
// host. A file that only Windows compiles cannot be scored on the ubuntu
// runner that does the scoring.

// procCall is one Win32 call made through a lazily loaded procedure, which
// reports failure as a zero first return with the reason in the error.
type procCall struct {
	name string
	// find resolves the procedure. LazyProc.Call panics on one that cannot
	// be found, so it is asked first.
	find func() error
	call func() (uintptr, error)
}

// do makes the call and says whether it failed.
func (c procCall) do() error {
	if err := c.find(); err != nil {
		return fmt.Errorf("%s unavailable: %w", c.name, err)
	}
	r1, e := c.call()
	if r1 != 0 {
		return nil
	}
	return callError(c.name, e)
}

// lockOrWait makes the lock call and, if it is still pending (an
// asynchronous handle), waits for the same request through wait.
func lockOrWait(lock, wait procCall, isPending func(error) bool) error {
	err := lock.do()
	if err == nil || !isPending(err) {
		return err
	}
	if werr := wait.do(); werr != nil {
		return fmt.Errorf("%s: waiting for the lock: %w", lock.name, werr)
	}
	return nil
}

// callError turns a failed call's error into one worth returning.
// LazyProc.Call always returns a non-nil error, an Errno of zero when
// GetLastError had nothing to say; a failure must still read as one.
func callError(name string, e error) error {
	var errno syscall.Errno
	if errors.As(e, &errno) && errno == 0 {
		return fmt.Errorf("%s failed without an error code", name)
	}
	return fmt.Errorf("%s: %w", name, e)
}
