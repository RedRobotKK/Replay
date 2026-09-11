package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/version"
)

// Nothing on this machine says the binary is old.
//
// `internal/selfupdate.StaleNotice` is pure arithmetic on the build date linked
// into the binary: it reaches no network, needs no account, and has been
// exported and tested since the package landed. It had no caller. The only way
// to learn a newer release existed was to type `replay upgrade --check`, which
// is a command an operator runs when they already suspect the answer -- and the
// case that motivated the package is the operator who did not suspect it, and
// ran 0.4.0 for three days while 0.5.4 was published (cmd/replay/upgrade.go:20).
//
// doctor.go:110 already told the reader that "`replay upgrade` tells an
// operator when the BINARY is stale". That sentence was false in this tree.
// This is the notice that makes it true, on the surface whose whole question is
// what is stale here.
func TestBuildNoticeSeparatesCurrentOldAndUnaged(t *testing.T) {
	now := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

	// Current: named, dated, and no warning. The line still has to appear,
	// because a report pasted into an issue that does not say which build
	// produced it is a report nobody can reproduce.
	fresh := buildNotice("v0.5.4", "abc1234", "2026-09-04T09:00:00Z", now)
	for _, want := range []string{"v0.5.4", "abc1234", "2026-09-04"} {
		if !strings.Contains(fresh, want) {
			t.Errorf("a current build must still name itself and its date; %q missing from:\n%s", want, fresh)
		}
	}
	if strings.Contains(strings.ToLower(fresh), "days old") {
		t.Errorf("a six-day-old build is not old:\n%s", fresh)
	}

	// Old: the age and the command that checks, in selfupdate's own words, so
	// the two surfaces cannot drift into two different thresholds.
	old := buildNotice("v0.4.0", "abc1234", "2026-06-06T00:00:00Z", now)
	for _, want := range []string{"96 days old", "replay upgrade"} {
		if !strings.Contains(old, want) {
			t.Errorf("a 96-day-old build must name %q, or the notice is a date with no\n"+
				"consequence and no next step. got:\n%s", want, old)
		}
	}

	// A source build is not old, it is unaged. `replay upgrade` refuses to
	// overwrite one (upgrade.go:78), so telling its operator to upgrade would
	// be advice the binary itself declines to take.
	//
	// Both shapes a source build arrives in are checked, and the second is the
	// one that matters. `go build ./cmd/replay` injects nothing, so the stamp
	// reads "unknown" and no arithmetic is possible anyway. `make build` sets
	// DATE unconditionally (Makefile:7) whatever VERSION says, so a source
	// build CAN carry a perfectly readable stamp -- and then the only thing
	// standing between its operator and a stale warning is that the version is
	// consulted before the date. A fixture with an unreadable date cannot tell
	// that ordering apart from its absence.
	for _, dev := range []string{
		buildNotice("dev", "abc1234", "unknown", now),
		buildNotice("dev", "abc1234", "2026-06-06T00:00:00Z", now),
	} {
		if strings.Contains(strings.ToLower(dev), "days old") {
			t.Errorf("a source build must never be reported as stale:\n%s", dev)
		}
		if !strings.Contains(dev, "source") {
			t.Errorf("a source build must say that is what it is, because no date and a\n"+
				"fresh date are different states (ADR-0018). got:\n%s", dev)
		}
	}

	// A build whose version string is empty is still a build, and the row still
	// has to say which one it is. version.Version is a link-time variable, so
	// "" is what an ldflags line that lost its value produces -- internal/tui's
	// versionTag() handles the same state for the same reason. Rendering
	// "build         , from source" would put a blank where the identity goes.
	nameless := buildNotice("", "abc1234", "unknown", now)
	if !strings.Contains(nameless, "unknown (abc1234)") {
		t.Errorf("a build with no version must still name itself rather than leave the\n"+
			"identity blank. got:\n%s", nameless)
	}

	// A release build whose stamp this binary cannot read is unknown, not
	// current. StaleNotice is silent on it by design, and silence here would
	// be read as "current" -- the reading that suppresses the warning.
	bad := buildNotice("v0.5.4", "abc1234", "the-third-of-never", now)
	if strings.Contains(strings.ToLower(bad), "days old") {
		t.Errorf("an unreadable build date must not be reported as stale either:\n%s", bad)
	}
	if !strings.Contains(bad, "unreadable") {
		t.Errorf("an unreadable build date must say so rather than be dropped, or the\n"+
			"absence of a warning means two different things. got:\n%s", bad)
	}

	// A build dated ahead of this clock is the same unknown wearing a date.
	// StaleNotice returns "" for it, deliberately, because a negative age means
	// a clock moved -- and rendering that as the current case would be the
	// third state collapsing into the first.
	ahead := buildNotice("v0.5.4", "abc1234", "2026-12-01T00:00:00Z", now)
	if strings.Contains(strings.ToLower(ahead), "days old") {
		t.Errorf("a future-dated build must not be reported as stale:\n%s", ahead)
	}
	if !strings.Contains(ahead, "ahead of this clock") {
		t.Errorf("a build dated in the future must say the clock is the suspect, not\n"+
			"pass silently as current. got:\n%s", ahead)
	}
}

// The join, not the function.
//
// Four tests calling buildNotice directly would all pass with the call site
// deleted, which is exactly how selfupdate.StaleNotice came to be built, tested
// and unreachable in the first place. This one runs the command.
func TestDoctorSaysWhichBuildTookTheReadings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	ver, commit, date := version.Version, version.Commit, version.Date
	version.Version, version.Commit, version.Date = "v0.4.0", "abc1234", "2026-06-06T00:00:00Z"
	clock := timeNow
	timeNow = func() time.Time { return time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() {
		version.Version, version.Commit, version.Date = ver, commit, date
		timeNow = clock
	})

	var out, errOut bytes.Buffer
	if err := run([]string{"doctor"}, &out, &errOut); err != nil {
		t.Fatalf("doctor: %v", err)
	}
	got := out.String()
	for _, want := range []string{"build         v0.4.0", "96 days old", "replay upgrade"} {
		if !strings.Contains(got, want) {
			t.Errorf("`replay doctor` on a 96-day-old build never said %q. The notice is\n"+
				"reachable only if the command prints it. got:\n%s", want, got)
		}
	}
}
