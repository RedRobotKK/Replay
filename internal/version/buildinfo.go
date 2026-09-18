package version

import "runtime/debug"

// init recovers the version the binary already knows when nothing stamped it.
//
// `-ldflags` is the authoritative supply route and stays that way: the Makefile
// and .goreleaser.yaml both set these three symbols, and a stamped value is
// never second-guessed below. What this covers is the one advertised path that
// cannot stamp. `go install github.com/RedRobotKK/Replay/cmd/replay@latest`
// builds from the module proxy, where the user has no place to pass -ldflags, so
// the binary shipped by the command in README.md reported "dev" while its own
// BuildInfo carried the module version. Measured 2026-09-17 against v0.6.0.
//
// Only Version is recovered. `Commit` and `Date` are left at "unknown" because
// a proxy build genuinely carries neither: Go stamps `vcs.revision` and
// `vcs.time` from a working tree, and there is none. Deriving a commit from
// `v0.6.0` would mean resolving a tag over the network or shipping a tag-to-SHA
// table, and either would hand the contribution endpoint a provenance claim
// nobody verified. The endpoint's refusal of "unknown" is correct and this
// change does not try to satisfy it.
func init() {
	info, ok := debug.ReadBuildInfo()
	Version = versionFrom(Version, info, ok)
}

// versionFrom picks between the stamped value and the module's own version.
//
// Split out from init so the decision can be tested without a build. Every
// refusal below returns `stamped` unchanged rather than a second placeholder,
// because `buildNotice` already distinguishes a source build from a release by
// reading "dev", and a value it does not recognise would be rendered as a
// release date it cannot age.
func versionFrom(stamped string, info *debug.BuildInfo, ok bool) string {
	// A stamped build has already answered this question.
	if stamped != "dev" {
		return stamped
	}
	if !ok || info == nil {
		return stamped
	}
	// A working-tree build is a source build, whatever its version string says.
	//
	// Go stamps vcs.revision only when it compiled from a checkout, and a
	// module-proxy install carries no vcs settings at all. That is the fact
	// being read here. Go 1.27 gives a working-tree build a pseudo-version, so
	// Main.Version alone no longer separates the two, and matching on the
	// pseudo-version's shape would pin this to one toolchain's formatting.
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return stamped
		}
	}
	// "(devel)" is what a module build with no VCS reports, and "" is what a
	// binary with no module reports. Neither names a release.
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return stamped
}

// KnownCommit returns the commit, or nothing when there is no commit to give.
//
// `Commit` defaults to the literal "unknown", which is not empty, so the
// `omitempty` on observation.Corpus.Commit cannot elide it and the binary
// submits a provenance claim it has never checked. Production refuses that
// value and is right to: functions/_lib/contribute.js lists `commit` in
// OPTIONAL and shapes it /^[0-9a-f]{7,40}$/, so a field that is absent is
// accepted and one reading "unknown" is not.
//
// Only the sentinel and the empty string are translated. Anything else is
// returned byte for byte, including a value this binary could not have
// produced. Lower-casing, truncating or padding here would repair a claim into
// one nobody verified, and refusing here would move a validation the server
// already owns to a place no submission passes through.
func KnownCommit(commit string) string {
	if commit == "unknown" {
		return ""
	}
	return commit
}
