//go:build unix

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

// termSize asks the terminal how wide and how tall it is.
//
// COLUMNS alone was not enough, and shipping only that would have been a fix
// that mostly does not fire: bash maintains COLUMNS for its own line editing
// and does not export it, so a child process usually cannot see it. A reader on
// an 80-column-designed layout in a 60-column window would have kept the ragged
// output and the fix would have looked done.
//
// This asks the driver instead, the same way IsTerminal does, so it works
// whether or not a shell chose to export anything.
// The height was already coming back from this ioctl and being dropped on the
// floor: the struct has always had a Row field and the function returned only
// ws.Col. One field from useful, and it is why the advise screen rendered 704
// lines into a 24-row terminal — nothing in the package could ask.
func termSize() (cols, rows int, ok bool) {
	var ws struct{ Row, Col, Xpixel, Ypixel uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	// A pipe, a file and /dev/null all fail here with ENOTTY, which is the
	// answer wanted in all three: a size read from a pipe is not a size, and
	// the committed screen images are generated into exactly that.
	if errno != 0 || ws.Col == 0 {
		return 0, 0, false
	}
	// Row can be zero on a terminal that reports a width, so it is reported
	// separately rather than failing the whole read: a caller that knows the
	// width and not the height is better off than one told neither.
	return int(ws.Col), int(ws.Row), true
}
