//go:build unix

package tui

import (
	"os"
	"syscall"
)

// IsTerminal reports whether f is a terminal, by asking the terminal driver.
//
// It asks the same way rawMode does, with the termios ioctl, because that is
// the question. The cheap version tests os.ModeCharDevice, and /dev/null is a
// character device: that check calls a redirect to /dev/null a terminal and
// then colours output nobody is looking at. The tip line in this binary has
// exactly that defect on record, so this does not copy it.
//
// A file, a pipe and /dev/null all fail the ioctl with ENOTTY, which is the
// answer we want in all three cases.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	var t syscall.Termios
	return ioctlTermios(int(f.Fd()), reqGetTermios, &t) == nil
}
