//go:build unix

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

// Raw mode, with the standard library and nothing else.
//
// go.mod has no requires. For a binary that sits in front of your API traffic
// holding a token, dependency-free is a claim worth keeping, so this does the
// termios dance directly rather than adding golang.org/x/term for thirty lines.
// Shelling out to stty is not available either: the guard that keeps os/exec
// out of this binary is worth more than the convenience.
//
// unsafe is on the x402 allowlist for exactly this, with the reasoning recorded
// beside it. It buys one keypress at a time and grants no cryptography; the
// signing guard is untouched.
type termState struct {
	fd  int
	old syscall.Termios
}

// rawMode puts the terminal into per-key input and returns the previous state.
//
// A failure here is not fatal. The caller falls back to line mode, because a
// surface that refuses to start over per-key input is worse than one that asks
// for a return.
func rawMode() (*termState, error) {
	fd := int(os.Stdin.Fd())
	var old syscall.Termios
	if err := ioctlTermios(fd, reqGetTermios, &old); err != nil {
		return nil, err
	}
	raw := old
	// Per key, no echo, no carriage-return translation.
	//
	// ISIG stays on deliberately: ctrl-c must still kill the process. A
	// full-screen program you cannot interrupt is one people reach for the
	// window close button on, and then wonder why their terminal is broken.
	raw.Lflag &^= syscall.ICANON | syscall.ECHO
	raw.Iflag &^= syscall.ICRNL
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctlTermios(fd, reqSetTermios, &raw); err != nil {
		return nil, err
	}
	return &termState{fd: fd, old: old}, nil
}

// restore puts the terminal back.
//
// Called on every exit path including the signal handler. A program that exits
// leaving a terminal in raw mode with no cursor has broken the shell its user
// came from, and they will not know which program did it.
func (t *termState) restore() {
	if t == nil {
		return
	}
	_ = ioctlTermios(t.fd, reqSetTermios, &t.old)
}

func ioctlTermios(fd int, req uintptr, t *syscall.Termios) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), req,
		uintptr(unsafe.Pointer(t)), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
