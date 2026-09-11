package guardcheck

import (
	"fmt"
	"go/build"
	"path/filepath"
	"runtime"
)

// BuildableHere reports whether this host compiles the file, and why not.
//
// A conditional inside a file the host does not compile is never neutralised
// and never scored. Worse, it is reported as neither survived nor unchecked —
// it does not reach the tool at all, and the pass still prints
// "0 survived, 0 unchecked". A guard added to a //go:build linux file lands on
// main carrying the same green line as one that was actually exercised.
//
// That is ADR-0014 one level up: for those files, the check that cannot fail
// is the guard pass itself.
//
// This repository has ten such files — internal/tui/term_linux.go, both
// *_windows.go pairs, the _other.go fallbacks — and internal/tui/term_unix.go
// is also the one place `unsafe` is used.
//
// go/build decides this, rather than a rule written here, because Go decides it
// two ways: a //go:build line and a filename suffix, and a check that reads only
// the first is blind to the second. build.Context.MatchFile reads both, against
// the real host, which is the same question `go build` asks.
func BuildableHere(path string) (bool, string) {
	ctx := build.Default
	// MatchFile reads "" as the working directory, the same as ".", so a bare
	// filename needs no special case. An earlier draft had one; the reviewer
	// reported it INERT, and checking rather than writing a test to keep it is
	// the difference between removing dead code and freezing it.
	dir, name := filepath.Split(path)
	ok, err := ctx.MatchFile(dir, name)
	if err != nil {
		// Unreadable or unparseable is not "another platform's". Saying it is
		// would file a broken file under a heading that implies it is fine
		// somewhere else.
		return false, fmt.Sprintf("could not be read: %v", err)
	}
	if ok {
		return true, ""
	}
	return false, fmt.Sprintf("not built on %s/%s", runtime.GOOS, runtime.GOARCH)
}

// BuiltOnSomePlatform reports whether any GOOS builds the file.
//
// The distinction this draws is the whole point of the report. A file carrying
// //go:build ignore is built nowhere and skipping it is correct — it is a
// standalone tool, and its conditionals are in no binary. A file carrying
// //go:build linux is built on Linux, so skipping it silently on darwin hides
// a guard that ships.
//
// ExcludedFromBuild cannot tell them apart: it evaluates with every tag false,
// under which both come back excluded. So this asks a different question —
// is there a GOOS that satisfies the constraint — and only a file where the
// answer is no gets filed as "excluded from every build".
func BuiltOnSomePlatform(path string) bool {
	dir, name := filepath.Split(path)
	for _, goos := range []string{
		"linux", "darwin", "windows", "freebsd", "openbsd", "netbsd",
		"dragonfly", "solaris", "android", "ios", "js", "plan9", "aix", "wasip1",
	} {
		ctx := build.Default
		ctx.GOOS = goos
		// GOARCH follows GOOS where they are not independent; amd64 exists for
		// every GOOS above except ios and js, and arm64 covers those.
		for _, arch := range []string{"amd64", "arm64", "wasm"} {
			ctx.GOARCH = arch
			if ok, err := ctx.MatchFile(dir, name); err == nil && ok {
				return true
			}
		}
	}
	return false
}
