package version

import (
	"runtime/debug"
	"testing"
)

// The advertised install path cannot pass -ldflags.
//
// `go install github.com/RedRobotKK/Replay/cmd/replay@latest` builds from the
// module proxy, so the Makefile's `-X .../version.Version=` never runs and the
// binary reports the literal default. Measured 2026-09-17 against v0.6.0: the
// installed binary printed "replay dev (unknown, built unknown)" while its own
// BuildInfo carried `mod github.com/RedRobotKK/Replay v0.6.0`. The binary knew
// its version and said "dev".
//
// What it does NOT carry, same measurement: any `vcs.revision`, `vcs.time` or
// `vcs.modified` setting. Go stamps those from a working tree and a proxy build
// has none. So `Commit` cannot be recovered here, and these tests pin that it is
// not invented from the version string.

func TestAnLdflagsVersionStaysAuthoritative(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.6.0"}}
	if got := versionFrom("v0.5.4-3-gabc1234", info, true); got != "v0.5.4-3-gabc1234" {
		t.Errorf("a stamped version was overwritten by BuildInfo: got %q", got)
	}
}

func TestTheModuleVersionIsUsedWhenNothingWasStamped(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.6.0"}}
	if got := versionFrom("dev", info, true); got != "v0.6.0" {
		t.Errorf("the module version did not reach an unstamped build: got %q", got)
	}
}

func TestNoBuildInfoLeavesTheDefaultAlone(t *testing.T) {
	if got := versionFrom("dev", nil, false); got != "dev" {
		t.Errorf("a missing BuildInfo invented a version: got %q", got)
	}
}

// `go build ./cmd/replay` inside the module reports Main.Version as "(devel)".
// Substituting that for "dev" would trade one placeholder for a less familiar
// one and tell `buildNotice` a source build is a release.
func TestADevelPlaceholderIsNotAVersion(t *testing.T) {
	for _, v := range []string{"", "(devel)"} {
		info := &debug.BuildInfo{Main: debug.Module{Version: v}}
		if got := versionFrom("dev", info, true); got != "dev" {
			t.Errorf("Main.Version %q was treated as a release: got %q", v, got)
		}
	}
}

// Commit and Date have no source on this path. RFC: the endpoint refuses
// "unknown" and that refusal is correct, so this test exists to stop a later
// change deriving a commit from a tag, which would be provenance nobody
// verified.
func TestCommitAndDateAreNotSynthesised(t *testing.T) {
	if Commit != "unknown" || Date != "unknown" {
		t.Skip("a stamped build; this test is about the module-proxy path")
	}
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.6.0"}}
	_ = versionFrom("dev", info, true)
	if Commit != "unknown" {
		t.Errorf("Commit was populated on the proxy path: %q", Commit)
	}
	if Date != "unknown" {
		t.Errorf("Date was populated on the proxy path: %q", Date)
	}
}

// A source build is identified by VCS stamping, not by the shape of its
// version string.
//
// Go 1.27 computes a pseudo-version for a module build from a working tree, so
// `Main.Version` there is neither "" nor "(devel)": measured 2026-09-17 as
// "v0.6.1-0.20260916032843-28108676c247+dirty". Letting that through made
// `go build ./cmd/replay` report a release identity, and `buildNotice`
// (doctorbuild.go:68) gates the from-source branch on "dev", so a contributor's
// own build started rendering as an unreadable stamp instead.
//
// The distinguishing fact is not the string. A working-tree build carries
// `vcs.revision`, `vcs.time` and `vcs.modified`; a module-proxy install carries
// no vcs settings at all. Both measured against real binaries the same day.
// Matching on the pseudo-version's shape would pin this test to one toolchain's
// formatting; matching on vcs presence pins it to what the build actually was.
func TestAWorkingTreeBuildStaysASourceBuild(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.6.1-0.20260916032843-28108676c247+dirty"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "28108676c2478980b7d22892b584ea630b8c6c9e"},
			{Key: "vcs.time", Value: "2026-09-16T03:28:43Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	if got := versionFrom("dev", info, true); got != "dev" {
		t.Errorf("a working-tree build reported a release identity: got %q", got)
	}
}

// The clean-tree case, so the predicate is not accidentally about `+dirty`.
func TestACleanWorkingTreeBuildIsStillASourceBuild(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.6.1-0.20260916032843-28108676c247"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "28108676c2478980b7d22892b584ea630b8c6c9e"},
			{Key: "vcs.modified", Value: "false"},
		},
	}
	if got := versionFrom("dev", info, true); got != "dev" {
		t.Errorf("a clean working-tree build reported a release identity: got %q", got)
	}
}

// And the proxy install must still work: no vcs settings, real tag.
func TestAProxyInstallStillRecoversItsTag(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v0.6.0"}}
	if got := versionFrom("dev", info, true); got != "v0.6.0" {
		t.Errorf("the module-proxy tag was lost: got %q", got)
	}
}

// KnownCommit is the answer to a question the sentinel cannot give.
//
// `Commit` defaults to the literal "unknown", which is not empty, so the
// `omitempty` on observation.Corpus.Commit never elides it and the binary
// submits a provenance claim it knows nothing about. Production refuses it:
// functions/_lib/contribute.js validates `commit` against /^[0-9a-f]{7,40}$/
// and lists it in OPTIONAL, so an absent field is accepted and "unknown" is not.
// Read at source 2026-09-17.
//
// The transformation is one way and narrow. A value that is not the sentinel is
// returned byte for byte, including one this binary could not have produced:
// normalising here would invent provenance, and refusing here would move a
// validation the server already owns into a place no submission passes through.

func TestTheSentinelBecomesNothing(t *testing.T) {
	for _, in := range []string{"unknown", ""} {
		if got := KnownCommit(in); got != "" {
			t.Errorf("KnownCommit(%q) = %q, want empty so omitempty elides it", in, got)
		}
	}
}

func TestARealCommitSurvivesByteForByte(t *testing.T) {
	for _, in := range []string{
		"b3667e3",
		"28108676c2478980b7d22892b584ea630b8c6c9e",
	} {
		if got := KnownCommit(in); got != in {
			t.Errorf("KnownCommit(%q) = %q, want it unchanged", in, got)
		}
	}
}

// No normalisation, no truncation, no padding, no manufacture. Each of these
// is a value the server will refuse, and being refused for what you sent is
// better than being accepted for something you did not.
func TestNothingIsNormalisedOrManufactured(t *testing.T) {
	for _, in := range []string{
		"B3667E3", // uppercase
		"B3667e3", // mixed
		"b366",    // too short
		"28108676c2478980b7d22892b584ea630b8c6c9eff", // too long
		"not-hex",     // not hex
		"  b3667e3  ", // padded
	} {
		if got := KnownCommit(in); got != in {
			t.Errorf("KnownCommit(%q) = %q, want it returned unchanged", in, got)
		}
	}
}
