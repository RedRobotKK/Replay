// Package selfupdate resolves, verifies and installs a newer replay release.
//
// It exists because there was no way to learn that a newer release had shipped.
// The only upgrade path was re-pasting the install line, so a machine could sit
// on a build for as long as nobody happened to re-read the README.
//
// Two rules shape the whole package.
//
// First, nothing here runs unless the user typed `replay upgrade`. README and
// docs/SURFACES.md promise that replay originates no network request you did
// not type, and cmd/replay/outbound_drift_test.go derives that promise from the
// AST rather than trusting the sentence. A background version ping is the
// ordinary way to build this and it is not available here: it would make the
// promise false by addition, which is precisely the drift that test was written
// to catch. StaleNotice is the answer instead — it reads the build date already
// compiled into the binary and reaches nothing.
//
// Second, the verification install.sh performs is the floor, not the ceiling.
// An upgrade path that checked less than the installer would be a way to
// downgrade your own guarantees by using the tool as intended.
package selfupdate

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Binary is the installed command name, and the ProjectName goreleaser builds
// its archive names from.
const Binary = "replay"

// Repo is the GitHub owner/name that publishes releases.
const Repo = "RedRobotKK/Replay"

// staleAfter is how old a build gets before StaleNotice mentions it.
//
// Thirty days is chosen against the release cadence, not picked round: this
// project shipped v0.4.0 and v0.5.4 three days apart, so a week would fire
// constantly and a quarter would never fire while it mattered. The notice is
// one line on stderr and it names a command; it is not a nag, and there is
// deliberately no counter, no cache file and no escalation.
const staleAfter = 30 * 24 * time.Hour

// release is a parsed semantic version. Only what ordering needs is kept.
type release struct {
	nums []int  // major, minor, patch, zero-filled
	pre  string // prerelease suffix, empty for a final release
	ok   bool
}

// parse reads a tag like "v0.5.4" or "0.6.0-rc.1". A source build carries
// "dev" and an unbuilt one carries "", and both must fail rather than
// degrade to 0.0.0 — see Newer.
func parse(tag string) release {
	s := strings.TrimSpace(tag)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return release{}
	}
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		// Build metadata after + is not part of precedence (semver §10), and
		// is dropped rather than compared.
		if s[i] == '+' {
			s = s[:i]
		} else {
			return finish(s[:i], s[i+1:])
		}
	}
	return finish(s, "")
}

func finish(core, pre string) release {
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return release{}
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return release{}
		}
		nums[i] = n
	}
	return release{nums: nums, pre: pre, ok: true}
}

// Compare orders two release tags by precedence: -1 if a is older than b, +1 if
// newer, 0 if equal. Unparseable tags compare equal to each other and older
// than anything parseable, but callers should use Newer, which refuses them
// outright.
func Compare(a, b string) int {
	ra, rb := parse(a), parse(b)
	switch {
	case !ra.ok && !rb.ok:
		return 0
	case !ra.ok:
		return -1
	case !rb.ok:
		return 1
	}
	for i := range ra.nums {
		if ra.nums[i] != rb.nums[i] {
			if ra.nums[i] < rb.nums[i] {
				return -1
			}
			return 1
		}
	}
	// A release outranks any prerelease of the same numbers (semver §11.3), and
	// two prereleases fall back to string order, which is right for the rc.N
	// and beta.N shapes this project uses.
	switch {
	case ra.pre == rb.pre:
		return 0
	case ra.pre == "":
		return 1
	case rb.pre == "":
		return -1
	case ra.pre < rb.pre:
		return -1
	default:
		return 1
	}
}

// Newer reports whether latest is strictly newer than current.
//
// It is false whenever either version is unparseable. That is the load-bearing
// case: a source build reports "dev", and treating "dev" as 0.0.0 would tell
// someone actively building from source that every release is an upgrade and
// invite them to overwrite the binary they are testing.
func Newer(current, latest string) bool {
	rc, rl := parse(current), parse(latest)
	if !rc.ok || !rl.ok {
		return false
	}
	return Compare(latest, current) > 0
}

// ArchiveName reproduces the goreleaser name_template
// {{.ProjectName}}_{{.Version}}_{{.Os}}_{{.Arch}}.tar.gz. Version there is the
// tag without its leading v, which is why this is derived in one place rather
// than formatted at each call site.
func ArchiveName(tag, goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", Binary, strings.TrimPrefix(tag, "v"), goos, goarch)
}

// ErrNoDigest reports that checksums.txt did not list the file asked about.
// install.sh treats this as fatal — "checksums.txt does not list X. Nothing was
// installed." — and so does this package.
var ErrNoDigest = errors.New("checksums.txt does not list this archive")

// DigestFor returns the sha256 recorded for name in a checksums.txt body.
//
// The match is on the whole second field, never a substring. An unanchored
// match would accept the hash of any file in the list, which turns a verified
// download into a verified-something-else: every release lists archives for
// four platforms, and they all end in _<arch>.tar.gz.
func DigestFor(checksums []byte, name string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// GNU coreutils writes "hash  name" for text mode and "hash *name" for
		// binary mode. Both appear in the wild; the star is not part of the name.
		if fields[1] == name || fields[1] == "*"+name {
			d := fields[0]
			if len(d) != 64 {
				return "", fmt.Errorf("checksums.txt lists %s with a %d-character digest, want 64", name, len(d))
			}
			return strings.ToLower(d), nil
		}
	}
	return "", ErrNoDigest
}

// TagFromLocation extracts the release tag from the Location header of
// https://github.com/<repo>/releases/latest.
//
// That redirect is served by github.com, not api.github.com, so it is not
// subject to the 60 requests/hour unauthenticated limit that install.sh
// documents as routinely hit on CI and behind shared NAT. One request, no
// token, no JSON.
func TagFromLocation(loc string) (string, error) {
	u, err := url.Parse(loc)
	if err != nil {
		return "", fmt.Errorf("unreadable redirect location %q: %w", loc, err)
	}
	path := strings.TrimSuffix(u.Path, "/")
	const marker = "/releases/tag/"
	i := strings.LastIndex(path, marker)
	if i < 0 {
		return "", fmt.Errorf("redirect to %q names no release tag", loc)
	}
	tag := path[i+len(marker):]
	if tag == "" || strings.Contains(tag, "/") {
		return "", fmt.Errorf("redirect to %q names no release tag", loc)
	}
	return tag, nil
}

// StaleNotice returns one line telling the reader their build is old and naming
// the command that checks, or "" when there is nothing to say.
//
// It asks the network nothing. buildDate is the RFC 3339 stamp linked into the
// binary at release time, so the age is arithmetic on a value already present.
// This is the whole reason the notice can exist at all: a version check would
// be an outbound request the user did not type.
//
// Silent on: a fresh build, a source build ("unknown"), an unparseable stamp,
// and a build dated in the future, which means the clock moved rather than that
// the binary is -40 days old.
func StaleNotice(buildDate string, now time.Time) string {
	built, err := time.Parse(time.RFC3339, strings.TrimSpace(buildDate))
	if err != nil {
		return ""
	}
	age := now.Sub(built)
	if age < staleAfter {
		return ""
	}
	return fmt.Sprintf("This build is %d days old. %s upgrade checks for a newer release.",
		int(age.Hours()/24), Binary)
}
