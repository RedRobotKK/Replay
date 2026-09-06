//go:build windows

package consent

import "io/fs"

// ownershipIsExclusive cannot run on Windows, and says so instead of guessing.
//
// Windows has no Unix permission bits. Go synthesises a mode for os.Stat:
// 0666 for any writable file, 0444 for a read-only one, regardless of who can
// actually reach it. Testing Perm()&0o022 against that therefore refuses every
// writable file on the platform, which is every consent file a user has just
// written. It refused them citing group and other permissions that do not
// exist on Windows.
//
// The real analogue is the file's ACL, which the standard library does not
// expose. Rather than approximate an access-control decision from a synthetic
// mode, the check reports that it did not run. Decision.OwnershipChecked
// carries that to the caller, so "verified as this user's" stays
// distinguishable from "not verifiable here" instead of collapsing into a
// silent yes.
func ownershipIsExclusive(info fs.FileInfo, path string) (checked bool, err error) {
	return false, nil
}
