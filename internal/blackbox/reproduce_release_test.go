//go:build mutation

package blackbox

// RR: scripts/reproduce-release.sh, run as release.yml runs it, against a
// release built here.
//
// The script decides whether a published tag rebuilds to the published bytes.
// Until 2026-10-13 it took the toolchain and the commit from the published
// binary's own stamps, so an artifact built from the wrong commit was rebuilt
// from that same wrong commit, and the question it answered was "does this
// binary reproduce itself". The tag is the claim; the artifact is what is being
// checked against it. So every input now comes from the tag, and the artifact's
// stamps must agree with it before anything is built.
//
// The fixture is a tiny module at the real import path, committed, tagged and
// "published" by this test with goreleaser's flags, then served through a fake
// gh and a git insteadOf redirect. No network: GOPROXY=off, and the toolchain
// the fixture names is the one running this test, so GOTOOLCHAIN never fetches.
//
// Every failure case asserts that "REPRODUCES:" was NOT printed, because the
// defect that matters is a check that reports success without comparing.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	rrTag      = "v9.9.9"
	rrVersion  = "9.9.9"
	rrOS       = "linux"
	rrArch     = "amd64"
	rrModule   = "github.com/RedRobotKK/Replay"
	rrUpstream = "https://github.com/RedRobotKK/Replay"
)

func rrRun(t *testing.T, dir string, env []string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v in %s: %v\n%s", name, args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

var rrGitEnv = []string{
	"GIT_AUTHOR_NAME=rr", "GIT_AUTHOR_EMAIL=rr@test.invalid",
	"GIT_COMMITTER_NAME=rr", "GIT_COMMITTER_EMAIL=rr@test.invalid",
	"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
}

// rrFixture is one source repository and one set of published assets.
type rrFixture struct {
	src, assets, bin, stage string
	// alt is a second Go binary built from the same commit with the same
	// stamps but a different -X Version: a plant that passes every stamp
	// check and differs only in its bytes.
	alt string
	// rec holds one environment dump per git, gh and tar invocation, written
	// by the wrappers below, so a test can see what each command inherited.
	rec string
}

type rrOpts struct {
	toolchain   string // go.mod toolchain line; "" omits it
	brokenBuild bool   // a file the published build excludes and the rebuild cannot compile
	tagParent   bool   // publish from the tagged commit's parent
	dirty       bool   // publish from a modified tree
	versionX    string // -X Version value used for the published build
	variant     string // commits a VARIANT file with this content: a different tree, so a different commit
}

func rrNewFixture(t *testing.T, o rrOpts) rrFixture {
	t.Helper()
	root := t.TempDir()
	f := rrFixture{src: filepath.Join(root, "src"), assets: filepath.Join(root, "assets"), bin: filepath.Join(root, "bin")}
	for _, d := range []string{f.src, f.assets, f.bin, filepath.Join(f.src, "cmd", "replay"), filepath.Join(f.src, "internal", "version")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		if err := os.WriteFile(filepath.Join(f.src, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gomod := "module " + rrModule + "\n\ngo 1.24\n"
	if o.toolchain != "" {
		gomod += "\ntoolchain " + o.toolchain + "\n"
	}
	write("go.mod", gomod)
	write("internal/version/version.go", "package version\n\nvar (\n\tVersion = \"dev\"\n\tCommit  = \"none\"\n\tDate    = \"unknown\"\n)\n")
	write("cmd/replay/main.go", "package main\n\nimport (\n\t\"fmt\"\n\n\t\""+rrModule+"/internal/version\"\n)\n\nfunc main() { fmt.Println(version.Version, version.Commit, version.Date) }\n")
	if o.brokenBuild {
		write("cmd/replay/broken.go", "//go:build !published\n\npackage main\n\nfunc init() { undefinedOnPurpose() }\n")
	}
	write("LICENSE", "fixture\n")
	if o.variant != "" {
		write("VARIANT", o.variant+"\n")
	}
	rrRun(t, f.src, rrGitEnv, "git", "init", "-q", "-b", "main")
	rrRun(t, f.src, rrGitEnv, "git", "add", "-A")
	rrRun(t, f.src, append(rrGitEnv, "GIT_AUTHOR_DATE=2026-01-02T03:04:05Z", "GIT_COMMITTER_DATE=2026-01-02T03:04:05Z"), "git", "commit", "-q", "-m", "one")
	if o.tagParent {
		write("NOTES", "second\n")
		rrRun(t, f.src, rrGitEnv, "git", "add", "-A")
		rrRun(t, f.src, append(rrGitEnv, "GIT_AUTHOR_DATE=2026-01-03T03:04:05Z", "GIT_COMMITTER_DATE=2026-01-03T03:04:05Z"), "git", "commit", "-q", "-m", "two")
	}
	rrRun(t, f.src, rrGitEnv, "git", "tag", rrTag)

	// Publish from a real clone, as goreleaser does on the runner.
	pub := filepath.Join(root, "pub")
	rrRun(t, root, rrGitEnv, "git", "clone", "-q", f.src, pub)
	ref := rrTag
	if o.tagParent {
		ref = rrTag + "~1"
	}
	rrRun(t, pub, rrGitEnv, "git", "checkout", "-q", ref)
	if o.dirty {
		if err := os.WriteFile(filepath.Join(pub, "LICENSE"), []byte("edited\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	short := rrRun(t, pub, rrGitEnv, "git", "rev-parse", "--short", "HEAD")
	date := rrRun(t, pub, append(rrGitEnv, "TZ=UTC"), "git", "log", "-1", "--format=%cd", "--date=format-local:%Y-%m-%dT%H:%M:%SZ")
	ver := rrVersion
	if o.versionX != "" {
		ver = o.versionX
	}
	args := []string{"build", "-trimpath"}
	if o.brokenBuild {
		args = append(args, "-tags", "published")
	}
	args = append(args, "-ldflags", "-s -w -X "+rrModule+"/internal/version.Version="+ver+
		" -X "+rrModule+"/internal/version.Commit="+short+
		" -X "+rrModule+"/internal/version.Date="+date,
		"-o", filepath.Join(root, "stage", "replay"), "./cmd/replay")
	rrRun(t, pub, []string{"GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=0", "GOOS=" + rrOS, "GOARCH=" + rrArch}, "go", args...)
	f.alt = filepath.Join(root, "alt", "replay")
	altArgs := append([]string(nil), args...)
	for i, a := range altArgs {
		if strings.Contains(a, "internal/version.Version=") {
			altArgs[i] = strings.Replace(a, "internal/version.Version="+ver, "internal/version.Version="+ver+"-plant", 1)
		}
		if a == filepath.Join(root, "stage", "replay") {
			altArgs[i] = f.alt
		}
	}
	rrRun(t, pub, []string{"GOTOOLCHAIN=local", "GOPROXY=off", "CGO_ENABLED=0", "GOOS=" + rrOS, "GOARCH=" + rrArch}, "go", altArgs...)
	f.stage = filepath.Join(root, "stage")
	if err := os.WriteFile(filepath.Join(f.stage, "LICENSE"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.pack(t)

	// A fake gh that serves only `release download` from the assets directory
	// and fails, as the real one does, when a requested asset does not exist.
	gh := `#!/bin/sh
[ "$1 $2" = "release download" ] || { echo "fake gh: unexpected $*" >&2; exit 64; }
shift 2; tag="$1"; shift; dest=.; pats=""
while [ $# -gt 0 ]; do case "$1" in
  --repo) shift ;; -p) shift; pats="$pats $1" ;; -D) shift; dest="$1" ;; esac; shift; done
[ "$tag" = "` + rrTag + `" ] || { echo "fake gh: release $tag not found" >&2; exit 1; }
for p in $pats; do [ -f "` + f.assets + `/$p" ] || { echo "no assets match the file pattern $p" >&2; exit 1; }; cp "` + f.assets + `/$p" "$dest/"; done
`
	f.rec = filepath.Join(root, "rec")
	if err := os.MkdirAll(f.rec, 0o755); err != nil {
		t.Fatal(err)
	}
	gh = strings.Replace(gh, "#!/bin/sh\n", "#!/bin/sh\n/usr/bin/env > \"$(mktemp '"+f.rec+"/gh.XXXXXX')\"\nprintf '%s\\n' \"$@\" > \"$(mktemp '"+f.rec+"/gh-args.XXXXXX')\"\n", 1)
	if err := os.WriteFile(filepath.Join(f.bin, "gh"), []byte(gh), 0o755); err != nil {
		t.Fatal(err)
	}
	// The script clones the upstream URL. This fake git, first on PATH, sends
	// that one URL to the fixture and passes everything else to the real git.
	// It replaced a GIT_CONFIG_* insteadOf redirect in the environment, which
	// was the very bypass a hostile caller could use.
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	realTar, err := exec.LookPath("tar")
	if err != nil {
		t.Fatal(err)
	}
	git := "#!/bin/sh\n/usr/bin/env > \"$(mktemp '" + f.rec + "/git.XXXXXX')\"\n" +
		"for a do shift; [ \"$a\" = '" + rrUpstream + "' ] && a='" + f.src + "'; set -- \"$@\" \"$a\"; done\n" +
		"exec '" + realGit + "' \"$@\"\n"
	tar := "#!/bin/sh\n/usr/bin/env > \"$(mktemp '" + f.rec + "/tar.XXXXXX')\"\nexec '" + realTar + "' \"$@\"\n"
	for name, body := range map[string]string{"git": git, "tar": tar} {
		if err := os.WriteFile(filepath.Join(f.bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// pack archives the stage directory and writes checksums.txt for it, as
// goreleaser does. Tests that tamper with the published binary edit the stage
// and pack again, so checksums.txt always vouches for what is served: the
// script has to catch the tampering some other way.
func (f rrFixture) pack(t *testing.T) {
	t.Helper()
	// Written with archive/tar, as goreleaser writes release archives, not
	// with the system tar: bsdtar's headers are not the plain ustar a release
	// carries, and GNU tar's differ again, so a fixture made by `tar czf`
	// would be a different archive on each machine (2026-10-14).
	archive := fmt.Sprintf("replay_%s_%s_%s.tar.gz", rrVersion, rrOS, rrArch)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"replay", "LICENSE"} {
		fi, err := os.Lstat(filepath.Join(f.stage, name))
		if err != nil {
			t.Fatal(err)
		}
		hdr := &tar.Header{Name: name, Mode: int64(fi.Mode().Perm()), Typeflag: tar.TypeReg, Format: tar.FormatUSTAR}
		var data []byte
		if fi.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(filepath.Join(f.stage, name))
			if err != nil {
				t.Fatal(err)
			}
			hdr.Typeflag, hdr.Linkname = tar.TypeSymlink, link
		} else if data, err = os.ReadFile(filepath.Join(f.stage, name)); err != nil {
			t.Fatal(err)
		}
		hdr.Size = int64(len(data))
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.assets, archive), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	body := buf.Bytes()
	sum := sha256.Sum256(body)
	sums := hex.EncodeToString(sum[:]) + "  " + archive + "\n" +
		strings.Repeat("0", 64) + "  replay_" + rrVersion + "_darwin_arm64.tar.gz\n"
	if err := os.WriteFile(filepath.Join(f.assets, "checksums.txt"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f rrFixture) run(t *testing.T, extraEnv ...string) (string, error) {
	t.Helper()
	return f.runArgs(t, []string{rrTag, rrOS, rrArch}, extraEnv...)
}

// runArgs runs the script. extraEnv comes last, so it wins over the harness's
// own settings: that is how a test plants a hostile caller environment.
func (f rrFixture) runArgs(t *testing.T, args []string, extraEnv ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{filepath.Join(repoRoot(t), "scripts", "reproduce-release.sh")}, args...)...)
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), rrGitEnv...)
	cmd.Env = append(cmd.Env,
		"PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GOPROXY=off", "GOFLAGS=",
	)
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// rrMustFail requires the documented exit code, not merely a failure: 2 means
// nothing was compared, 1 means the bytes differ, and a test that accepted any
// non-zero status would pass on a crash for the wrong reason.
func rrMustFail(t *testing.T, out string, err error, wantCode int, want string) {
	t.Helper()
	t.Logf("output:\n%s", out)
	if err == nil {
		t.Fatalf("the script exited 0 on a release that must not pass")
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != wantCode {
		t.Fatalf("exit status %v, want %d", err, wantCode)
	}
	if strings.Contains(out, "REPRODUCES:") {
		t.Fatalf("the script printed REPRODUCES: on a release that must not pass")
	}
	if want != "" && !strings.Contains(out, want) {
		t.Fatalf("failed, but not for the reason under test: want %q in the output", want)
	}
}

func TestRR_AMatchingReleaseReproduces(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	out, err := f.run(t)
	t.Logf("output:\n%s", out)
	if err != nil {
		t.Fatalf("the control did not reproduce, so every failure below may be the harness: %v", err)
	}
	if !strings.Contains(out, "REPRODUCES: "+rrTag+" "+rrOS+"/"+rrArch) {
		t.Fatal("exit 0 without the REPRODUCES line")
	}
	var pub, reb string
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "published sha256:"); ok {
			pub = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "rebuilt sha256:"); ok {
			reb = strings.TrimSpace(v)
		}
	}
	if len(pub) != 64 || pub != reb {
		t.Fatalf("REPRODUCES printed, but the hashes it compared were %q and %q", pub, reb)
	}
}

func TestRR_DifferentBytesDoNotReproduce(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version(), versionX: "9.9.9-tampered"})
	out, err := f.run(t)
	rrMustFail(t, out, err, 1, "DOES NOT REPRODUCE")
}

func TestRR_AMissingArchiveFails(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	if err := os.Remove(filepath.Join(f.assets, fmt.Sprintf("replay_%s_%s_%s.tar.gz", rrVersion, rrOS, rrArch))); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "could not download")
}

// Exit 1 means the bytes were compared and differ, and release.yml's step
// reports a release as public on exit 1 even when the release job failed.
// mktemp ran under set -e without a refusal, so a full or unwritable /tmp
// exited 1 before anything was downloaded, and the step said "already public"
// about a tag nothing had checked (2026-10-14).
func TestRR_AWorkingDirectoryThatCannotBeMadeComparesNothing(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	fake := "#!/bin/sh\necho 'mktemp: No space left on device' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(f.bin, "mktemp"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "nothing was compared")
	if strings.Contains(out, "Downloading") {
		t.Fatalf("the script went on to the download without a working directory:\n%s", out)
	}
}

func TestRR_AMissingChecksumsFileFails(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	if err := os.Remove(filepath.Join(f.assets, "checksums.txt")); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "could not download")
}

func TestRR_AnArchiveThatDisagreesWithChecksumsFails(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	p := filepath.Join(f.assets, "checksums.txt")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Repeat("1", 64)+string(body[64:])), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "but checksums.txt says")
}

func TestRR_AnArchiveAbsentFromChecksumsFails(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	p := filepath.Join(f.assets, "checksums.txt")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitN(string(body), "\n", 2)
	if err := os.WriteFile(p, []byte(lines[1]), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "checksums.txt has no entry for")
}

func TestRR_AFailedRebuildFails(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version(), brokenBuild: true})
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "the rebuild of "+rrTag+" failed")
	if strings.Contains(out, "DOES NOT REPRODUCE") {
		t.Fatal("the rebuild failed to compile, and the script went on to compare anyway")
	}
}

func TestRR_ABinaryFromAnotherCommitIsRejectedBeforeBuilding(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version(), tagParent: true})
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "was built from")
	if strings.Contains(out, "Rebuilding") {
		t.Fatal("rejected only after rebuilding; the tag/commit disagreement must stop it first")
	}
}

func TestRR_ABinaryFromAModifiedTreeIsRejected(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version(), dirty: true})
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "from a modified tree")
	if strings.Contains(out, "Rebuilding") {
		t.Fatal("rejected only after rebuilding")
	}
}

func TestRR_ATagWithNoToolchainLineIsAmbiguous(t *testing.T) {
	f := rrNewFixture(t, rrOpts{})
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "toolchain")
}

func TestRR_AToolchainDisagreementIsRejectedBeforeBuilding(t *testing.T) {
	// go.mod at the tag names a different toolchain from the one that built the
	// published binary. Built with GOTOOLCHAIN=local, so the stamp is the
	// running toolchain and the line is a lie.
	other := "go1.26.0"
	if runtime.Version() == other {
		other = "go1.26.1"
	}
	f := rrNewFixture(t, rrOpts{toolchain: other})
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "toolchain")
	if strings.Contains(out, "Rebuilding") {
		t.Fatal("rejected only after rebuilding")
	}
}

// The caller's Go environment must not reach the rebuild. GOFLAGS and GOAMD64
// both change the bytes of an identical source (measured 2026-10-13), and a
// go env file, a go.work above the clone or GOEXPERIMENT can do the same. The
// rebuild states its build environment instead of inheriting one.
func TestRR_TheCallersGoEnvironmentDoesNotReachTheRebuild(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	out, err := f.run(t, "GOFLAGS=-tags=rrnoise", "GOAMD64=v3")
	t.Logf("output:\n%s", out)
	if err != nil || !strings.Contains(out, "REPRODUCES: ") {
		t.Fatalf("a release that reproduces did not, because the caller's environment reached the rebuild: %v", err)
	}
}

func TestRR_ASymlinkWhereTheBinaryShouldBeIsRefused(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	if err := os.Remove(filepath.Join(f.stage, "replay")); err != nil {
		t.Fatal(err)
	}
	// An absolute target that exists on the machine running the check. A
	// dangling link fails the regular-file test on its own; this one passes it,
	// so only the symlink test stands between it and a hash of /bin/sh.
	if err := os.Symlink("/bin/sh", filepath.Join(f.stage, "replay")); err != nil {
		t.Fatal(err)
	}
	f.pack(t)
	out, err := f.run(t)
	// Refused by the upgrade reader first: selfupdate.Unpack rejects a
	// symlink where the binary should be, before tar extracts anything.
	rrMustFail(t, out, err, 2, "replay upgrade would not install a binary")
}

func TestRR_AnArchiveListedTwiceInChecksumsIsRefused(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	p := filepath.Join(f.assets, "checksums.txt")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(string(body), "\n", 2)[0] + "\n"
	if err := os.WriteFile(p, append(body, []byte(first)...), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "more than once")
}

// The stamps are a consistency check ahead of the byte comparison, so they are
// tested against a binary whose build info was edited after the build: the
// commit is right, the commit time is not.
func TestRR_AForgedCommitTimeStampIsRefusedBeforeBuilding(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	p := filepath.Join(f.stage, "replay")
	body, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	from, to := []byte("vcs.time=2026-01-02T03:04:05Z"), []byte("vcs.time=2026-01-02T03:04:06Z")
	// Go embeds the build info more than once; a forgery edits every copy.
	if bytes.Count(body, from) == 0 {
		t.Fatal("the fixture binary carries no vcs.time stamp to forge")
	}
	if err := os.WriteFile(p, bytes.ReplaceAll(body, from, to), 0o755); err != nil {
		t.Fatal(err)
	}
	f.pack(t)
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "commit time")
	if strings.Contains(out, "Rebuilding") {
		t.Fatal("rejected only after rebuilding")
	}
}

func TestRR_AMemberThatIsNotAGoBinaryIsRefused(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	if err := os.WriteFile(filepath.Join(f.stage, "replay"), []byte("#!/bin/sh\necho not go\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.pack(t)
	out, err := f.run(t)
	rrMustFail(t, out, err, 2, "no Go build info")
}

// Arguments are refused before anything is downloaded or cloned: a tag that
// looks like an option, and a platform that looks like a path.
func TestRR_MalformedArgumentsAreRefusedBeforeAnyWork(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	for _, args := range [][]string{
		{"--upload-pack=touch", rrOS, rrArch},
		{"main", rrOS, rrArch},
		{rrTag, "../linux", rrArch},
		{rrTag, rrOS, "amd64 x"},
	} {
		out, err := f.runArgs(t, args)
		rrMustFail(t, out, err, 2, "")
		if strings.Contains(out, "Downloading") {
			t.Fatalf("%q was refused only after downloading", args)
		}
	}
}

// rrEntry is one archive member, written in order by rrPackEntries.
type rrEntry struct {
	name string
	body []byte
}

// rrPackEntries replaces the published archive with exactly these members, in
// this order, and rewrites checksums.txt to vouch for it, as an attacker able
// to replace both would.
func (f rrFixture) rrPackEntries(t *testing.T, entries []rrEntry) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o755, Size: int64(len(e.body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archive := fmt.Sprintf("replay_%s_%s_%s.tar.gz", rrVersion, rrOS, rrArch)
	if err := os.WriteFile(filepath.Join(f.assets, archive), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	if err := os.WriteFile(filepath.Join(f.assets, "checksums.txt"), []byte(hex.EncodeToString(sum[:])+"  "+archive+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The gate must judge the member a user actually gets. `replay upgrade`
// (selfupdate.unpack) installs the FIRST regular member whose base name is
// replay; install.sh extracts the member named exactly replay, and with two of
// them the later one lands. Until 2026-10-13 the gate extracted `replay` and
// compared that, so an archive carrying a planted binary under another path,
// or under the same name ahead of the genuine one, passed while upgrade
// installed the plant. One member, named exactly replay, or nothing is compared.
func TestRR_AnArchiveWithMoreThanOneReplayMemberIsRefused(t *testing.T) {
	for _, plant := range []string{"bin/replay", "replay", "./replay"} {
		t.Run(strings.ReplaceAll(plant, "/", "_"), func(t *testing.T) {
			f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
			good, err := os.ReadFile(filepath.Join(f.stage, "replay"))
			if err != nil {
				t.Fatal(err)
			}
			f.rrPackEntries(t, []rrEntry{
				{plant, []byte("#!/bin/sh\necho planted\n")},
				{"replay", good},
				{"LICENSE", []byte("fixture\n")},
			})
			out, err := f.run(t)
			rrMustFail(t, out, err, 2, "exactly one")
		})
	}
}

// The control for the test above: the same writer, one genuine member, passes.
// Without it a refusal could be the writer, not the layout.
func TestRR_ThePackEntriesWriterProducesAReproducibleArchive(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	good, err := os.ReadFile(filepath.Join(f.stage, "replay"))
	if err != nil {
		t.Fatal(err)
	}
	f.rrPackEntries(t, []rrEntry{{"replay", good}, {"LICENSE", []byte("fixture\n")}})
	out, err := f.run(t)
	t.Logf("output:\n%s", out)
	if err != nil || !strings.Contains(out, "REPRODUCES: ") {
		t.Fatalf("the control archive did not reproduce: %v", err)
	}
}

// The rebuild's environment, observed at the real go build call rather than
// inferred from its bytes. A wrapper named go, first on PATH, records the
// environment of the build invocation and hands off to the real toolchain. The
// caller's environment is hostile in every variable that can change the
// output or the toolchain's provenance; the build must see none of it.
func TestRR_TheRebuildSeesAStatedEnvironmentNotTheCallers(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	rec := filepath.Join(t.TempDir(), "build.env")
	// Paths are written into the wrapper, not passed in the environment: the
	// script builds under an allow-list that drops anything it does not name.
	wrapper := "#!/bin/sh\nif [ \"$1\" = build ]; then /usr/bin/env > '" + rec + "'; fi\nexec '" + realGo + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(f.bin, "go"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	hostileEnv := filepath.Join(t.TempDir(), "goenv")
	if err := os.WriteFile(hostileEnv, []byte("GOFLAGS=-tags=fromgoenv\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	poison := filepath.Join(t.TempDir(), "poisoned-cache")
	out, err := f.run(t,
		"GOFLAGS=-tags=rrnoise", "GOAMD64=v3", "GOARM64=v9.0", "GOENV="+hostileEnv,
		"GOWORK="+filepath.Join(t.TempDir(), "go.work"), "GOCACHE="+poison,
		"GOSUMDB=off", "GONOSUMDB=*", "GONOSUMCHECK=1", "GOPRIVATE=*", "GONOPROXY=*", "GOINSECURE=*",
		// Found reaching the build on 2026-10-13, each able to change the
		// outcome: an external cache program, module mode, another GOROOT, a
		// temp dir. ZZ_SENTINEL is no Go variable at all: its absence shows the
		// environment is an allow-list, so a variable Go adds later is dropped
		// too.
		"GOCACHEPROG="+filepath.Join(t.TempDir(), "cacheprog"), "GO111MODULE=off",
		"GOROOT="+t.TempDir(), "GOTMPDIR="+filepath.Join(t.TempDir(), "missing"), "ZZ_SENTINEL=reached")
	t.Logf("output:\n%s", out)
	if err != nil || !strings.Contains(out, "REPRODUCES: ") {
		t.Fatalf("did not reproduce under a hostile caller environment: %v", err)
	}
	raw, err := os.ReadFile(rec)
	if err != nil {
		t.Fatalf("the wrapper never saw a go build call, so this test observed nothing: %v", err)
	}
	got := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			got[k] = v
		}
	}
	want := map[string]string{
		"GOFLAGS": "", "GOENV": "off", "GOWORK": "off", "GOEXPERIMENT": "", "GOAMD64": "v1",
		"GOARM64": "v8.0", "CGO_ENABLED": "0", "GOTOOLCHAIN": runtime.Version(),
		"GOSUMDB": "sum.golang.org", "GONOSUMDB": "", "GONOSUMCHECK": "", "GOPRIVATE": "",
		"GONOPROXY": "", "GOINSECURE": "", "GOOS": rrOS, "GOARCH": rrArch,
	}
	for k, v := range want {
		if g, ok := got[k]; !ok || g != v {
			t.Errorf("the build saw %s=%q (set=%v), want %q", k, g, ok, v)
		}
	}
	for _, k := range []string{"GOCACHEPROG", "GOROOT", "GOTMPDIR", "ZZ_SENTINEL", "GOPROXY"} {
		if v, ok := got[k]; ok {
			t.Errorf("the build inherited %s=%q from the caller", k, v)
		}
	}
	if got["GO111MODULE"] != "on" {
		t.Errorf("the build saw GO111MODULE=%q, want on", got["GO111MODULE"])
	}
	if c := got["GOCACHE"]; c == "" || c == poison || !strings.HasSuffix(c, "/gocache") {
		t.Errorf("the build used GOCACHE=%q; it must be a fresh cache inside the run's own directory, "+
			"not the caller's, which a poisoned entry could answer for", c)
	}
}

// One member with the right base name but the wrong path. Whether
// `tar xzf archive replay` finds "./replay" differs between GNU tar and bsdtar,
// so the gate does not ask tar: the one member must be named exactly replay.
func TestRR_ALoneReplayMemberUnderAnotherPathIsRefused(t *testing.T) {
	for _, name := range []string{"./replay", "bin/replay"} {
		t.Run(strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
			good, err := os.ReadFile(filepath.Join(f.stage, "replay"))
			if err != nil {
				t.Fatal(err)
			}
			f.rrPackEntries(t, []rrEntry{{name, good}, {"LICENSE", []byte("fixture\n")}})
			out, err := f.run(t)
			rrMustFail(t, out, err, 2, "named exactly replay")
		})
	}
}

// Members whose names merely contain replay are not the binary and are not
// counted. A man page or a shell completion is the likely next thing goreleaser
// packs, and a gate that counted it would refuse every honest release after.
func TestRR_AMemberWhoseNameOnlyContainsReplayIsNotCounted(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	good, err := os.ReadFile(filepath.Join(f.stage, "replay"))
	if err != nil {
		t.Fatal(err)
	}
	f.rrPackEntries(t, []rrEntry{
		{"replay", good},
		{"replay.1", []byte(".TH REPLAY 1\n")},
		{"completions/replay.bash", []byte("complete -F _replay replay\n")},
		{"LICENSE", []byte("fixture\n")},
	})
	out, err := f.run(t)
	t.Logf("output:\n%s", out)
	if err != nil || !strings.Contains(out, "REPRODUCES: ") {
		t.Fatalf("an archive with a man page and a completion beside the binary did not reproduce: %v", err)
	}
}

// rrRecorded returns every environment the named tool was started with.
func (f rrFixture) rrRecorded(t *testing.T, tool string) []map[string]string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(f.rec, tool+".*"))
	var out []map[string]string
	for _, p := range files {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]string{}
		for _, line := range strings.Split(string(b), "\n") {
			if k, v, ok := strings.Cut(line, "="); ok {
				m[k] = v
			}
		}
		out = append(out, m)
	}
	return out
}

// HE1: a caller cannot point the gate at another repository.
//
// The published binary is built from a DECOY repository with the same tag
// name. A hostile caller adds a git insteadOf rule that sends the gate's clone
// to the decoy. If git honours it, the gate rebuilds the decoy, the bytes
// match, and it prints REPRODUCES about a binary the tagged upstream source
// never produced.
func TestHE1_AHostileGitRedirectCannotChangeTheSourceVerified(t *testing.T) {
	upstream := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	decoy := rrNewFixture(t, rrOpts{toolchain: runtime.Version(), variant: "decoy"})
	for _, name := range []string{fmt.Sprintf("replay_%s_%s_%s.tar.gz", rrVersion, rrOS, rrArch), "checksums.txt"} {
		b, err := os.ReadFile(filepath.Join(decoy.assets, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(upstream.assets, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The decoy's tree differs, so its v9.9.9 is a different commit, and its
	// published binary reproduces from the decoy and from nothing else.
	out, err := upstream.run(t,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=url."+decoy.src+".insteadOf",
		"GIT_CONFIG_VALUE_0="+upstream.src)
	t.Logf("output:\n%s", out)
	if err == nil || strings.Contains(out, "REPRODUCES:") {
		t.Fatal("a caller's GIT_CONFIG insteadOf rule sent the clone to another repository and the gate passed")
	}
	if !strings.Contains(out, "was built from") {
		t.Fatalf("refused, but not because the binary's commit disagrees with upstream's tag")
	}
}

// HE2: git, gh and tar each start with the stated environment and nothing
// else, under every hostile variable at once.
func TestHE2_ExternalCommandsSeeOnlyTheAllowList(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	hostile := []string{
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.fsmonitor", "GIT_CONFIG_VALUE_0=true",
		"GIT_DIR=" + t.TempDir(), "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + t.TempDir(), "GIT_TEMPLATE_DIR=" + t.TempDir(),
		"GH_HOST=attacker.example", "GH_REPO=attacker/replay", "GH_CONFIG_DIR=" + t.TempDir(), "GH_ENTERPRISE_TOKEN=x",
		"TAR_OPTIONS=--strip-components=1", "GREP_OPTIONS=-v", "LC_ALL=tr_TR.UTF-8", "ZZ_SENTINEL=reached",
		"GH_TOKEN=token-the-ci-job-passes",
	}
	out, err := f.run(t, hostile...)
	t.Logf("output:\n%s", out)
	if err != nil || !strings.Contains(out, "REPRODUCES: ") {
		t.Fatalf("an honest release did not reproduce under a hostile caller environment: %v", err)
	}
	allowed := map[string]bool{"PATH": true, "HOME": true, "GH_TOKEN": true, "LC_ALL": true,
		"PWD": true, "OLDPWD": true, "SHLVL": true, "_": true, "REPRO_SANITIZED": true,
		"GIT_CONFIG_NOSYSTEM": true, "GIT_CONFIG_GLOBAL": true, "GIT_TERMINAL_PROMPT": true}
	// go build stamps VCS info by running git itself, under pinned_go's own
	// allow-list, which adds Go's stated variables.
	goStated := map[string]bool{"GOENV": true, "GOFLAGS": true, "GOWORK": true, "GOEXPERIMENT": true, "GOAMD64": true,
		"GOARM64": true, "GO111MODULE": true, "GOSUMDB": true, "GONOSUMDB": true, "GONOSUMCHECK": true, "GOPRIVATE": true,
		"GONOPROXY": true, "GOINSECURE": true, "GOCACHE": true, "GOPATH": true, "GOTOOLCHAIN": true, "CGO_ENABLED": true,
		"GOOS": true, "GOARCH": true, "GOROOT": true}
	hostileKeys := map[string]bool{}
	for _, kv := range hostile {
		k, _, _ := strings.Cut(kv, "=")
		hostileKeys[k] = true
	}
	for _, tool := range []string{"git", "gh", "tar"} {
		envs := f.rrRecorded(t, tool)
		if len(envs) == 0 {
			t.Fatalf("%s was never started, so this test observed nothing about it", tool)
		}
		for _, env := range envs {
			// Every invocation: nothing the caller planted, except the token
			// CI legitimately passes.
			for k := range hostileKeys {
				if _, ok := env[k]; ok && k != "GH_TOKEN" && (k != "LC_ALL" || env[k] != "C") {
					t.Errorf("%s inherited the caller's %s=%q", tool, k, env[k])
				}
			}
			// git runs started by go build to stamp VCS info carry go's own
			// computed environment (GOVERSION, GOMODCACHE under the run's
			// directory, CC...), built from pinned_go's allow-list, not from
			// the caller. Invocations the script starts itself must hold the
			// allow-list and nothing more.
			if _, fromGo := env["GOVERSION"]; fromGo {
				continue
			}
			if env["LC_ALL"] != "C" {
				t.Errorf("%s, started by the script, ran with LC_ALL=%q, want C", tool, env["LC_ALL"])
			}
			if tool == "git" && (env["GIT_CONFIG_GLOBAL"] != "/dev/null" || env["GIT_CONFIG_NOSYSTEM"] != "1") {
				t.Errorf("git ran with GIT_CONFIG_GLOBAL=%q GIT_CONFIG_NOSYSTEM=%q; it must read no user or system config",
					env["GIT_CONFIG_GLOBAL"], env["GIT_CONFIG_NOSYSTEM"])
			}
			for k, v := range env {
				// TZ=UTC is the script's own, on the git log that reads the
				// commit time goreleaser stamps.
				if k == "TZ" && v == "UTC" {
					continue
				}
				if !allowed[k] && !goStated[k] {
					t.Errorf("%s, started by the script, carries %s=%q", tool, k, v)
				}
			}
			if env["LC_ALL"] != "" && env["LC_ALL"] != "C" {
				t.Errorf("%s ran with LC_ALL=%q, want C", tool, env["LC_ALL"])
			}
		}
	}
	for _, env := range f.rrRecorded(t, "gh") {
		if env["GH_TOKEN"] != "token-the-ci-job-passes" {
			t.Errorf("gh did not receive the GH_TOKEN the CI job passes; legitimate authentication would break")
		}
	}
	for _, env := range f.rrRecorded(t, "git") {
		if _, fromScript := env["GIT_CONFIG_GLOBAL"]; fromScript && (env["GIT_CONFIG_GLOBAL"] != "/dev/null" || env["GIT_CONFIG_NOSYSTEM"] != "1") {
			t.Errorf("git ran with GIT_CONFIG_GLOBAL=%q GIT_CONFIG_NOSYSTEM=%q", env["GIT_CONFIG_GLOBAL"], env["GIT_CONFIG_NOSYSTEM"])
		}
	}
}

// HE3: each hostile variable alone, so one cannot hide behind another.
func TestHE3_EachHostileVariableAloneLeavesTheVerdictUnchanged(t *testing.T) {
	for _, v := range []string{"GIT_CONFIG_COUNT=1", "GIT_DIR=/nonexistent", "GIT_ALTERNATE_OBJECT_DIRECTORIES=/nonexistent",
		"GH_HOST=attacker.example", "GH_REPO=attacker/replay", "TAR_OPTIONS=--strip-components=1", "GREP_OPTIONS=-v", "LC_ALL=tr_TR.UTF-8"} {
		t.Run(strings.SplitN(v, "=", 2)[0], func(t *testing.T) {
			f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
			out, err := f.run(t, v)
			if err != nil || !strings.Contains(out, "REPRODUCES: ") {
				t.Fatalf("%s alone changed the verdict on an honest release: %v\n%s", v, err, out)
			}
		})
	}
}

// HE4: HOME is kept, for gh's stored login, so git must not read the
// configuration that lives there. A caller who controls HOME can write a
// .gitconfig with an insteadOf rule; that redirect must change nothing.
func TestHE4_AGitconfigUnderHOMECannotRedirectTheClone(t *testing.T) {
	upstream := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	decoy := rrNewFixture(t, rrOpts{toolchain: runtime.Version(), variant: "decoy"})
	for _, name := range []string{fmt.Sprintf("replay_%s_%s_%s.tar.gz", rrVersion, rrOS, rrArch), "checksums.txt"} {
		b, err := os.ReadFile(filepath.Join(decoy.assets, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(upstream.assets, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	home := t.TempDir()
	cfg := "[url \"" + decoy.src + "\"]\n\tinsteadOf = " + upstream.src + "\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := upstream.run(t, "HOME="+home)
	t.Logf("output:\n%s", out)
	if err == nil || strings.Contains(out, "REPRODUCES:") {
		t.Fatal("a .gitconfig under the caller's HOME redirected the clone and the gate passed")
	}
	if !strings.Contains(out, "was built from") {
		t.Fatal("refused, but not because the binary's commit disagrees with upstream's tag")
	}
}

// HE5: a caller who sets REPRO_SANITIZED=1 themselves, beside a hostile
// variable, does not skip the allow-list. The marker only stops a loop; with
// the environment still dirty the script refuses.
func TestHE5_AForgedSanitizedMarkerDoesNotSkipTheAllowList(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	out, err := f.run(t, "REPRO_SANITIZED=1", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=url./nonexistent.insteadOf", "GIT_CONFIG_VALUE_0="+f.src)
	rrMustFail(t, out, err, 2, "still not the allow-list")
}

// HE6: gh is told the host explicitly, so GH_HOST, which the allow-list also
// drops, could not choose it even if it reached gh.
func TestHE6_TheDownloadNamesItsHost(t *testing.T) {
	f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
	out, err := f.run(t, "GH_HOST=attacker.example")
	if err != nil {
		t.Fatalf("control did not reproduce: %v\n%s", err, out)
	}
	files, _ := filepath.Glob(filepath.Join(f.rec, "gh-args.*"))
	if len(files) == 0 {
		t.Fatal("gh was never called")
	}
	for _, p := range files {
		b, _ := os.ReadFile(p)
		args := strings.Split(strings.TrimSpace(string(b)), "\n")
		found := false
		for i, a := range args {
			if a == "--repo" && i+1 < len(args) {
				found = true
				if args[i+1] != "github.com/RedRobotKK/Replay" {
					t.Errorf("gh was given --repo %q; it must name the host, github.com/RedRobotKK/Replay", args[i+1])
				}
			}
		}
		if !found {
			t.Error("gh was called without --repo")
		}
	}
}

// HE7: a shell function the caller exports cannot stand in for the commands
// that enforce the allow-list (F2 of the v3.5 review, 2026-10-14). Where sh is
// bash (macOS /bin/sh is bash 3.2), bash imports exported functions from the
// environment, and a function shadows every command and regular builtin of the
// same name: a function named env answered the script's own listing of the
// environment with a clean one, the re-run was skipped, and a hostile
// variable reached gh and git. read and [ were shadowable the same way. Before
// the re-run the script uses only reserved words, special builtins and
// /usr/bin/env by path; the re-run under env -i starts a shell with no
// functions at all.
func TestHE7_AnExportedShellFunctionCannotReplaceTheAllowList(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is needed to run the script as macOS runs it: %v", err)
	}
	for name, fn := range map[string]string{
		"env": `() { printf 'PATH=%s\nHOME=%s\n' "$PATH" "$HOME"; }`,
		// A read that ends the listing early and claims PATH was seen, so a
		// script that counts on reaching PATH= is not saved by that alone.
		"read": `() { path_seen=1; return 1; }`,
		"[":    `() { return 1; }`,
	} {
		t.Run(name, func(t *testing.T) {
			f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
			// sh is bash for the whole run, including the script's re-run,
			// which finds sh on PATH.
			if err := os.Symlink(bash, filepath.Join(f.bin, "sh")); err != nil {
				t.Fatal(err)
			}
			hostile := []string{"BASH_FUNC_" + name + "%%=" + fn, "ZZ_SENTINEL=reached",
				"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.fsmonitor", "GIT_CONFIG_VALUE_0=true"}
			// The precondition: this bash, run as sh, imports the function,
			// so the attack is live on this machine and not vacuous.
			probe := exec.Command(filepath.Join(f.bin, "sh"), "-c", "command -V '"+name+"'")
			probe.Env = append(os.Environ(), hostile...)
			if pout, _ := probe.CombinedOutput(); !strings.Contains(string(pout), "function") {
				t.Fatalf("precondition: bash as sh did not import the exported %s function: %s", name, pout)
			}
			cmd := exec.Command(filepath.Join(f.bin, "sh"), filepath.Join(repoRoot(t), "scripts", "reproduce-release.sh"), rrTag, rrOS, rrArch)
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), rrGitEnv...)
			cmd.Env = append(cmd.Env, "PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GOPROXY=off", "GOFLAGS=")
			cmd.Env = append(cmd.Env, hostile...)
			o, err := cmd.CombinedOutput()
			out := string(o)
			t.Logf("output:\n%s", out)
			// What reached the tools first: that is the property under test,
			// and a run that failed for another reason must still show it.
			for _, tool := range []string{"gh", "git"} {
				envs := f.rrRecorded(t, tool)
				if len(envs) == 0 {
					t.Fatalf("%s was never started, so this test observed nothing about it", tool)
				}
				for _, env := range envs {
					for _, k := range []string{"ZZ_SENTINEL", "GIT_CONFIG_COUNT", "BASH_FUNC_" + name + "%%"} {
						if _, ok := env[k]; ok {
							t.Errorf("%s inherited the caller's %s: the exported %s function skipped the allow-list", tool, k, name)
						}
					}
				}
			}
			if err != nil || !strings.Contains(out, "REPRODUCES: ") {
				t.Fatalf("an honest release did not reproduce: %v", err)
			}
		})
	}
}

// HE8: a forged REPRO_SANITIZED marker beside an exported function still
// refuses, with exit 2. The refusal used to be printed with echo, and a
// function named echo that exits 0 turned it into a silent exit 0 under bash
// as sh. It is printed with /usr/bin/printf by path, and exit is a special
// builtin no function can replace.
func TestHE8_AForgedMarkerBesideAnExportedFunctionStillRefuses(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is needed to run the script as macOS runs it: %v", err)
	}
	for _, name := range []string{"echo", "printf", "["} {
		t.Run(name, func(t *testing.T) {
			f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
			if err := os.Symlink(bash, filepath.Join(f.bin, "sh")); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(filepath.Join(f.bin, "sh"), filepath.Join(repoRoot(t), "scripts", "reproduce-release.sh"), rrTag, rrOS, rrArch)
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), rrGitEnv...)
			cmd.Env = append(cmd.Env, "PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GOPROXY=off", "GOFLAGS=",
				"REPRO_SANITIZED=1", "BASH_FUNC_"+name+"%%=() { exit 0; }")
			o, err := cmd.CombinedOutput()
			rrMustFail(t, string(o), err, 2, "still not the allow-list")
		})
	}
}

// HE9: a caller's SHELLOPTS is refused, under bash as sh and under dash
// (R1 of the v3.6 review, 2026-10-14). bash as sh enables the options it lists
// before the script's first line, and with xtrace it expands the caller's PS4
// before every command: PS4='$((clean=1,path_seen=1))' rewrote the allow-list
// verdict, the re-run was skipped, hostile variables reached gh and git, and
// the run printed REPRODUCES. The first line now turns tracing off with set, a
// special builtin, and a SHELLOPTS or BASHOPTS in the environment is refused
// rather than re-run, because one PS4 expansion still happens before that line
// can act. The trace also printed GH_TOKEN; it must not appear.
func TestHE9_ACallersShellOptionsAreRefused(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash is needed to run the script as macOS runs it: %v", err)
	}
	shells := map[string]string{"bash-as-sh": bash}
	if dash, err := exec.LookPath("dash"); err == nil {
		shells["dash"] = dash
	}
	for shName, shPath := range shells {
		for name, env := range map[string][]string{
			"xtrace rewriting the verdict": {"SHELLOPTS=xtrace", "PS4=$((opts=0,clean=1,path_seen=1))"},
			"xtrace and a token":           {"SHELLOPTS=xtrace", "PS4=+ "},
			"BASHOPTS":                     {"BASHOPTS=nullglob"},
		} {
			t.Run(shName+"/"+strings.ReplaceAll(name, " ", "_"), func(t *testing.T) {
				f := rrNewFixture(t, rrOpts{toolchain: runtime.Version()})
				if err := os.Symlink(shPath, filepath.Join(f.bin, "sh")); err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(filepath.Join(f.bin, "sh"), filepath.Join(repoRoot(t), "scripts", "reproduce-release.sh"), rrTag, rrOS, rrArch)
				cmd.Dir = t.TempDir()
				cmd.Env = append(os.Environ(), rrGitEnv...)
				cmd.Env = append(cmd.Env, "PATH="+f.bin+string(os.PathListSeparator)+os.Getenv("PATH"), "GOPROXY=off", "GOFLAGS=",
					"ZZ_SENTINEL=reached", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.fsmonitor", "GIT_CONFIG_VALUE_0=true",
					"GH_TOKEN=tok-must-not-print")
				cmd.Env = append(cmd.Env, env...)
				o, err := cmd.CombinedOutput()
				out := string(o)
				for _, tool := range []string{"gh", "git"} {
					if n := len(f.rrRecorded(t, tool)); n != 0 {
						t.Errorf("%s was started %d times; a refused run starts nothing", tool, n)
					}
				}
				if strings.Contains(out, "tok-must-not-print") {
					t.Errorf("the output carries GH_TOKEN")
				}
				rrMustFail(t, out, err, 2, "SHELLOPTS or BASHOPTS")
			})
		}
	}
}
