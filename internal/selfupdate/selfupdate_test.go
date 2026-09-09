package selfupdate

import (
	"strings"
	"testing"
	"time"
)

// Ordering has to be numeric, not lexical. "0.10.0" sorts before "0.9.0" as a
// string, and a self-updater that gets this wrong tells someone on the newest
// release that they are behind, or worse, refuses the upgrade that matters.
//
// PASS: every pair orders by release precedence.
// FAIL: any lexical comparison leaking through.
func TestCompareOrdersNumerically(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.5.4", "v0.4.0", 1},
		{"v0.4.0", "v0.5.4", -1},
		{"v0.5.4", "v0.5.4", 0},
		{"0.5.4", "v0.5.4", 0},   // the leading v is cosmetic
		{"v0.10.0", "v0.9.0", 1}, // lexically the other way round
		{"v1.0.0", "v0.99.99", 1},
		{"v0.5.10", "v0.5.9", 1},
		{"v0.5", "v0.5.0", 0}, // a short tag is zero-filled
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// A prerelease is behind its own release: v0.6.0-rc.1 must not be offered to
// someone running v0.6.0.
//
// PASS: prereleases order below the release they lead to.
// FAIL: the suffix is ignored, making rc.1 equal to the final.
func TestComparePrereleaseIsBelowRelease(t *testing.T) {
	if Compare("v0.6.0-rc.1", "v0.6.0") != -1 {
		t.Errorf("prerelease should sort below its release")
	}
	if Compare("v0.6.0-rc.2", "v0.6.0-rc.1") != 1 {
		t.Errorf("rc.2 should sort above rc.1")
	}
}

// A source build reports "dev". It has no place on the version line, and
// claiming a dev build is "behind" v0.5.4 would push someone to overwrite the
// binary they are actively building.
//
// PASS: Newer is false whenever either side is unparseable.
// FAIL: "dev" is coerced to 0.0.0 and every release looks like an upgrade.
func TestNewerRefusesUnparseableVersions(t *testing.T) {
	for _, c := range []struct{ current, latest string }{
		{"dev", "v0.5.4"},
		{"v0.5.4", "dev"},
		{"", "v0.5.4"},
		{"v0.5.4", ""},
		{"not-a-version", "v0.5.4"},
	} {
		if Newer(c.current, c.latest) {
			t.Errorf("Newer(%q, %q) = true, want false", c.current, c.latest)
		}
	}
	if !Newer("v0.4.0", "v0.5.4") {
		t.Error("Newer(v0.4.0, v0.5.4) = false, want true")
	}
	if Newer("v0.5.4", "v0.4.0") {
		t.Error("a newer local build must not be reported as behind")
	}
}

// The digest must be bound to this exact filename. An unanchored match would
// accept the hash of any file listed in checksums.txt, which is the whole
// reason install.sh uses `awk '$2 == f'` and not grep.
//
// PASS: only the row whose second field is the filename is used.
// FAIL: a substring or prefix match returns a neighbouring hash.
func TestDigestForBindsToTheExactFilename(t *testing.T) {
	const body = `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  replay_0.5.4_darwin_amd64.tar.gz
bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb  replay_0.5.4_darwin_arm64.tar.gz
cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc  replay_0.5.4_linux_arm64.tar.gz
`
	got, err := DigestFor([]byte(body), "replay_0.5.4_darwin_arm64.tar.gz")
	if err != nil {
		t.Fatalf("DigestFor: %v", err)
	}
	if got != strings.Repeat("b", 64) {
		t.Errorf("DigestFor = %q, want the arm64 row", got)
	}

	// darwin_arm64 is a suffix of nothing here, but linux_arm64 shares the
	// "_arm64.tar.gz" tail: a sloppy HasSuffix would collide.
	if _, err := DigestFor([]byte(body), "replay_0.5.4_windows_amd64.tar.gz"); err == nil {
		t.Error("a filename absent from checksums.txt must be an error, not a silent pass")
	}
}

// goreleaser writes "hash *name" for binary mode on some platforms. Accepting
// only the two-space form would fail verification on a real release.
//
// PASS: the star-prefixed form resolves to the same digest.
// FAIL: the row is not matched and a valid download is rejected.
func TestDigestForAcceptsBinaryModeRows(t *testing.T) {
	body := strings.Repeat("d", 64) + " *replay_0.5.4_linux_amd64.tar.gz\n"
	got, err := DigestFor([]byte(body), "replay_0.5.4_linux_amd64.tar.gz")
	if err != nil {
		t.Fatalf("DigestFor: %v", err)
	}
	if got != strings.Repeat("d", 64) {
		t.Errorf("DigestFor = %q, want the star-mode row", got)
	}
}

// The staleness notice is computed from the date already compiled into the
// binary. It exists so the tool can say "you may be behind" without asking the
// network, because README and SURFACES.md promise no request the user did not
// type, and outbound_drift_test.go enforces that promise from the AST.
//
// PASS: a fresh build says nothing; an old build names its age.
// FAIL: any notice on a fresh build, or a notice that needs the network.
func TestStaleNoticeIsSilentOnAFreshBuild(t *testing.T) {
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	if got := StaleNotice("2026-09-06T18:30:09Z", now); got != "" {
		t.Errorf("a 3-day-old build should say nothing, got %q", got)
	}
	if got := StaleNotice("2026-05-01T00:00:00Z", now); got == "" {
		t.Error("a 131-day-old build should offer the upgrade command")
	} else if !strings.Contains(got, "replay upgrade") {
		t.Errorf("the notice must name the command to run, got %q", got)
	}
}

// "unknown" is what a source build carries. There is no age to report and
// nothing useful to say, so it must not produce "this build is 20336 days old".
//
// PASS: unparseable or empty dates produce no notice.
// FAIL: the zero time is treated as a real build date.
func TestStaleNoticeIgnoresUnknownBuildDates(t *testing.T) {
	now := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	for _, d := range []string{"unknown", "", "not-a-date"} {
		if got := StaleNotice(d, now); got != "" {
			t.Errorf("StaleNotice(%q) = %q, want empty", d, got)
		}
	}
	// A clock skewed backwards must not report a negative age either.
	if got := StaleNotice("2026-12-01T00:00:00Z", now); got != "" {
		t.Errorf("a build dated in the future should say nothing, got %q", got)
	}
}

// The release asset name is derived, not typed. Getting the arch token wrong
// downloads a 404 and reports it as "no build for your platform".
//
// PASS: the goreleaser name_template is reproduced exactly.
// FAIL: any drift from {{ProjectName}}_{{Version}}_{{Os}}_{{Arch}}.tar.gz.
func TestArchiveNameMatchesTheReleaseTemplate(t *testing.T) {
	got := ArchiveName("v0.5.4", "darwin", "arm64")
	if want := "replay_0.5.4_darwin_arm64.tar.gz"; got != want {
		t.Errorf("ArchiveName = %q, want %q", got, want)
	}
	// The tag keeps its v; the filename does not.
	if got := ArchiveName("0.5.4", "linux", "amd64"); got != "replay_0.5.4_linux_amd64.tar.gz" {
		t.Errorf("ArchiveName without a v prefix = %q", got)
	}
}

// The tag comes back on the Location header of the releases/latest redirect,
// which is not subject to the 60/hr unauthenticated API limit that install.sh
// documents as a routine failure behind shared NAT.
//
// PASS: the tag is taken from the final path segment.
// FAIL: a trailing slash or query string leaks into the tag.
func TestTagFromRedirectLocation(t *testing.T) {
	cases := map[string]string{
		"https://github.com/RedRobotKK/Replay/releases/tag/v0.5.4":  "v0.5.4",
		"https://github.com/RedRobotKK/Replay/releases/tag/v0.5.4/": "v0.5.4",
		"/RedRobotKK/Replay/releases/tag/v1.0.0":                    "v1.0.0",
	}
	for loc, want := range cases {
		got, err := TagFromLocation(loc)
		if err != nil {
			t.Errorf("TagFromLocation(%q): %v", loc, err)
			continue
		}
		if got != want {
			t.Errorf("TagFromLocation(%q) = %q, want %q", loc, got, want)
		}
	}
	if _, err := TagFromLocation("https://github.com/RedRobotKK/Replay/releases"); err == nil {
		t.Error("a location with no tag segment must be an error")
	}
}
