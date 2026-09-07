//go:build darwin

package tui

import "syscall"

// The ioctl request numbers differ by platform, so they are named here rather
// than inlined at the call site.
const (
	reqGetTermios = syscall.TIOCGETA
	reqSetTermios = syscall.TIOCSETA
)
