//go:build !unix

package tui

import "os"

// IsTerminal is false on builds without the termios ioctl.
//
// Reporting false means colour is off by default there rather than wrong: the
// user can still ask for it with -color=always. Claiming true without being
// able to check would put escape sequences into files on the one platform this
// project cannot test.
func IsTerminal(f *os.File) bool { return false }
