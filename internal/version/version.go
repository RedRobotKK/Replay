// Package version exposes build metadata injected at link time.
package version

// These values are overwritten by -ldflags at build time. See the Makefile.
var (
	// Version is the semantic version of this build, or "dev" for local builds.
	Version = "dev"
	// Commit is the short git SHA the binary was built from.
	Commit = "unknown"
	// Date is the UTC build timestamp in RFC 3339 format.
	Date = "unknown"
)

// String renders the version line shown by `replay version`.
func String() string {
	return Version + " (" + Commit + ", built " + Date + ")"
}

// KnownCommit is the commit if one was stamped, and empty if it was not.
//
// Commit defaults to the sentinel "unknown" so `replay version` can say so in
// words. That sentinel must not leave the process inside a payload: `omitempty`
// drops an empty string and "unknown" is not empty, so it serialised into every
// corpus submission from an unstamped build and the receiver refused the lot
// with `field commit is not 7 to 40 lowercase hex characters`.
//
// A `go install pkg@version` build has no vcs.revision at all, so it has no
// commit to stamp and is exactly the build a new contributor runs. Absent is
// the honest answer for it, and absent is a fact a pool can act on; "unknown"
// is a word pretending to be a hash.
func KnownCommit(commit string) string {
	if commit == "unknown" {
		return ""
	}
	return commit
}
