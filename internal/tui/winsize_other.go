//go:build !unix

package tui

// termCols has no portable answer off unix, so the caller falls back to
// COLUMNS and then to the design default. Windows has its own console API and
// this binary has never been tested there; claiming a width it cannot read
// would be worse than declining to.
func termCols() (int, bool) { return 0, false }
