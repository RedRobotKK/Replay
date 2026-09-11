//go:build windows

package ownerdir

// modeIsChecked reports whether the permission bits mean what they say. On
// Windows they do not: Go synthesises 0666 for any writable file and 0444 for
// a read-only one, so every ordinary directory would read as world-writable
// and every run would either chmod pointlessly or refuse to start.
//
// The same reasoning, and the same conclusion, as
// internal/consent/ownership_windows.go: the real analogue is the ACL, which
// the standard library does not expose, and approximating an access-control
// decision from a synthetic mode is worse than declining to make one.
// RELEASE-CRITERIA.md records Windows as unsupported for exactly this class
// of promise.
func modeIsChecked() bool { return false }
