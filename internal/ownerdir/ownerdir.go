// Package ownerdir keeps Replay's private directories private.
//
// Finding 7 of the 2026-09-04 adversarial security review:
//
//	Directory permissions are set at creation and never verified, so a
//	pre-existing world-writable directory stays world-writable
//
// The mechanism is os.MkdirAll, which applies its mode argument only when it
// creates a directory. On an existing one it is a successful no-op, so
// `mkdir -m 777 ~/.replay` before the first run — or a directory restored
// from an archive, or unpacked by a tool with a permissive umask — produced a
// ledger and a masking vault that every local account could read, with the
// code confidently passing 0700 on the line above. The same holds for
// os.WriteFile, which does not change the mode of a file that already exists:
// the vault key and the ledger label key are written with 0600 and keep
// whatever they had.
//
// The two secrets this protects are not hygiene. The vault key file decrypts
// the masking vault, which holds the plaintext of every secret the proxy
// replaced. The ledger label key un-anonymises the hashed path labels and the
// tool-call keys in the ledger. Either one, readable by another local
// account, makes the corresponding at-rest protection decorative.
//
// Repair rather than refusal, deliberately. These are directories Replay
// created for its own use, so tightening them is Replay keeping its own
// promise, not overruling an operator's choice about their files. What is NOT
// deliberate is repairing silently: a chmod that does not take — because the
// directory belongs to somebody else, or the filesystem does not carry the
// bits — is a refusal, because continuing would be writing secrets into a
// place this code has just discovered it does not control.
package ownerdir

import (
	"fmt"
	"os"
)

const (
	// DirPerm is the mode for a directory this package creates: owner-only.
	DirPerm os.FileMode = 0o700
	// FilePerm is the mode for a file inside one: owner-only.
	FilePerm os.FileMode = 0o600
	// looseBits are the group and other bits. Any of them set on a directory
	// holding keys or derived transcript data is the finding.
	looseBits os.FileMode = 0o077
)

// chmod and stat are indirected once so a test can reach the branch below
// that matters most and is otherwise unreachable: a chmod that reports
// success while the mode does not change.
//
// That is not a hypothetical. It is what an exFAT or FAT32 volume, a network
// mount with a fixed file mode, or a container bind mount does — the call
// returns nil and the bits stay exactly as they were. Without a seam the
// verification after the chmod is a branch no test can enter, which under
// ADR-0014 makes it indistinguishable from a branch that is not there. The
// same seam reaches the chmod-returned-an-error case, which on a real
// filesystem needs a directory owned by another account.
var (
	chmod = os.Chmod
	stat  = os.Stat
)

// EnsureDir creates dir if it is missing and verifies that it is owner-only,
// tightening it if it is not. It returns an error if the directory cannot be
// made owner-only, in which case the caller must not write secrets into it.
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, DirPerm); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	return tighten(dir, DirPerm, "directory")
}

// EnsureFile verifies that an existing file is owner-only, tightening it if it
// is not. A file that does not exist is not an error: the caller is about to
// create it, and os.WriteFile applies its mode to a new file correctly.
func EnsureFile(path string) error {
	if _, err := stat(path); os.IsNotExist(err) {
		return nil
	}
	return tighten(path, FilePerm, "file")
}

// tighten stats path, chmods it to want when group or other bits are set, and
// verifies the result.
//
// It carries no "is this really a directory" check of its own. It had one, and
// guard-reachability reported it as unreachable: os.MkdirAll already fails
// with ENOTDIR when a regular file sits at the path, so the second check could
// only ever run after the first had returned. Two checks for one condition
// also hid each other — removing either left the other passing the test, which
// is how a guard comes to look verified while nothing verifies it. The re-stat is the point: a chmod that returned nil on
// a filesystem that does not carry the bits, or on a path this process does
// not own, must not be reported as a repair.
func tighten(path string, want os.FileMode, kind string) error {
	info, err := stat(path)
	if err != nil {
		return fmt.Errorf("check %s %s: %w", kind, path, err)
	}
	if !modeIsChecked() || info.Mode().Perm()&looseBits == 0 {
		return nil
	}
	if err := chmod(path, want); err != nil {
		return fmt.Errorf("%s %s is readable or writable by group or other (%04o) and could not be tightened: %w",
			kind, path, info.Mode().Perm(), err)
	}
	after, err := stat(path)
	if err != nil {
		return fmt.Errorf("re-check %s %s: %w", kind, path, err)
	}
	if after.Mode().Perm()&looseBits != 0 {
		return fmt.Errorf("%s %s is readable or writable by group or other (%04o) and stayed that way after chmod; "+
			"Replay will not write keys or transcript data somewhere other local accounts can reach",
			kind, path, after.Mode().Perm())
	}
	return nil
}
