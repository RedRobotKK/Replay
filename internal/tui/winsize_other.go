//go:build !unix

package tui

// termSize has no portable answer off unix, so the caller falls back to
// COLUMNS and LINES and then to the design defaults. Windows has its own
// console API and this binary has never been tested there; claiming a size it
// cannot read would be worse than declining to.
func termSize() (cols, rows int, ok bool) { return 0, 0, false }
