//go:build !windows

package consent

import (
	"fmt"
	"io/fs"
)

// ownershipIsExclusive refuses a consent file that anyone other than its owner
// can write.
//
// A file any process on the box can write is not this user's decision, so the
// mode bits are load-bearing here rather than hygiene. checked is true because
// on these platforms the bits mean what they say.
func ownershipIsExclusive(info fs.FileInfo, path string) (checked bool, err error) {
	if info.Mode().Perm()&0o022 != 0 {
		return true, fmt.Errorf("%s is writable by group or other (%04o); refusing to treat it as this user's decision",
			path, info.Mode().Perm())
	}
	return true, nil
}
