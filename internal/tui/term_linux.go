//go:build linux

package tui

// TCGETS and TCSETS. The syscall package does not export these as constants on
// every version, so they are written out.
const (
	reqGetTermios = 0x5401
	reqSetTermios = 0x5402
)
