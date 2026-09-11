//go:build !windows

package ownerdir

// modeIsChecked reports whether the permission bits mean what they say on
// this platform. They do here.
func modeIsChecked() bool { return true }
