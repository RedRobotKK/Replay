//go:build !windows

package ledger

import "os"

// openFile is os.OpenFile: unix needs no sharing flags to rename over an open
// file, and every other platform refuses the lock before it would matter.
func openFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// renameFile is os.Rename.
func renameFile(from, to string) error { return os.Rename(from, to) }
