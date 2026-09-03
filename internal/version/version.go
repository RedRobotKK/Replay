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
