//go:build mutation

package blackbox

// IP: the installer's provenance check, run end to end.
//
// It lives here, behind the mutation tag, because it executes programs, and
// TestX402_ExecIsConfinedToTheMutationHarness keeps os/exec out of every
// ordinary build. CI runs this package with -tags mutation.
//
// Nothing in the tree executed install.sh. Every installer test read it as
// text, so a signature check that accepted any workflow in the repository, and
// a closing screen that never said whether provenance was checked at all, both
// passed. These tests run the real script against a fake release on a sealed
// PATH: a fake curl serves the release assets from a directory, and a fake
// cosign records its arguments and answers as told. Nothing reaches the network.
//
// The PATH is built from symlinks to the tools the script needs and nothing
// else, so whether cosign is "installed" is decided by the test and never by
// the machine running it. That is the FC-COSIGN lesson applied to the shell.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const ipVersion = "v9.9.9"

// The identity a genuine release certificate carries, as observed on the
// v0.9.0 certificate: the release workflow at the release tag.
const ipWantIdentity = "https://github.com/RedRobotKK/Replay/.github/workflows/release.yml@refs/tags/" + ipVersion

type ipRun struct {
	out        string // combined stdout and stderr
	err        error
	cosignArgs []string
	binDir     string
}

// installed reports whether a binary reached the destination.
func (r ipRun) installed() bool {
	_, err := os.Stat(filepath.Join(r.binDir, "replay"))
	return err == nil
}

// closing is the output after the install succeeded: the part of the screen a
// person reads last, and the part that must carry the provenance verdict.
func (r ipRun) closing(t *testing.T) string {
	t.Helper()
	i := strings.Index(r.out, "Installed replay "+ipVersion)
	if i < 0 {
		t.Fatalf("the install did not report success; output:\n%s", r.out)
	}
	return r.out[i:]
}

type ipCosign int

const (
	cosignAbsent ipCosign = iota
	cosignAccepts
	cosignRejects
)

func runInstaller(t *testing.T, cosign ipCosign, extraArgs ...string) ipRun {
	t.Helper()
	osName, arch := runtime.GOOS, runtime.GOARCH
	if osName != "darwin" && osName != "linux" {
		t.Skipf("install.sh supports darwin and linux only, not %s", osName)
	}
	if arch != "amd64" && arch != "arm64" {
		t.Skipf("install.sh supports amd64 and arm64 only, not %s", arch)
	}

	root := t.TempDir()
	assets := filepath.Join(root, "release")
	fakebin := filepath.Join(root, "bin")
	binDir := filepath.Join(root, "dest")
	home := filepath.Join(root, "home")
	for _, d := range []string{assets, fakebin, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// The release: an archive holding a replay that answers `version`, the
	// checksums file that lists it, and a certificate and signature for cosign.
	archive := "replay_" + strings.TrimPrefix(ipVersion, "v") + "_" + osName + "_" + arch + ".tar.gz"
	tgz := ipArchive(t, "#!/bin/sh\necho 'replay 9.9.9 (fake)'\n")
	ipWrite(t, filepath.Join(assets, archive), tgz, 0o644)
	sum := sha256.Sum256(tgz)
	ipWrite(t, filepath.Join(assets, "checksums.txt"), []byte(hex.EncodeToString(sum[:])+"  "+archive+"\n"), 0o644)
	ipWrite(t, filepath.Join(assets, "checksums.txt.pem"), []byte("fake certificate\n"), 0o644)
	ipWrite(t, filepath.Join(assets, "checksums.txt.sig"), []byte("fake signature\n"), 0o644)

	// A curl that serves the release directory and nothing else.
	ipWrite(t, filepath.Join(fakebin, "curl"), []byte(`#!/bin/sh
out=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    --proto|--proto-redir) shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
prefix="https://github.com/RedRobotKK/Replay/releases/download/`+ipVersion+`/"
case "$url" in "$prefix"*) ;; *) echo "fake curl: no route for $url" >&2; exit 6 ;; esac
f="$IP_ASSETS/${url#"$prefix"}"
[ -f "$f" ] || exit 22
if [ -n "$out" ]; then cp "$f" "$out"; else cat "$f"; fi
`), 0o755)

	cosignLog := filepath.Join(root, "cosign.args")
	switch cosign {
	case cosignAccepts, cosignRejects:
		code := "0"
		if cosign == cosignRejects {
			code = "1"
		}
		ipWrite(t, filepath.Join(fakebin, "cosign"), []byte(
			"#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$IP_COSIGN_LOG\"\nexit "+code+"\n"), 0o755)
	}

	// Every other tool the script uses, linked from this machine. cosign is
	// deliberately not on this list.
	for _, tool := range []string{
		"sh", "tr", "uname", "mktemp", "rm", "awk", "cut", "tar", "gzip", "install",
		"cp", "chmod", "mv", "head", "sed", "grep", "cat", "date", "sleep", "mkdir",
		"shasum", "sha256sum", "perl", "env", "ldd",
	} {
		p, err := exec.LookPath(tool)
		if err != nil {
			continue // shasum or sha256sum, ldd on linux only: the script probes for each
		}
		if err := os.Symlink(p, filepath.Join(fakebin, tool)); err != nil {
			t.Fatal(err)
		}
	}

	script := filepath.Join(repoRoot(t), "install.sh")
	args := append([]string{script, "--version", ipVersion, "--bin-dir", binDir, "--no-tui"}, extraArgs...)
	cmd := exec.Command(filepath.Join(fakebin, "sh"), args...)
	cmd.Env = []string{
		"PATH=" + fakebin,
		"HOME=" + home,
		"NO_COLOR=1",
		"CI=1",
		"REPLAY_NO_OPEN=1",
		"TERM=dumb",
		"IP_ASSETS=" + assets,
		"IP_COSIGN_LOG=" + cosignLog,
	}
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()

	run := ipRun{out: buf.String(), err: err, binDir: binDir}
	if b, rerr := os.ReadFile(cosignLog); rerr == nil {
		run.cosignArgs = strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	}
	return run
}

func ipArchive(t *testing.T, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "replay", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func ipWrite(t *testing.T, path string, b []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatal(err)
	}
}

// argAfter returns the value that follows flag in a recorded argument list.
func argAfter(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

// IP-1. The signature check must accept only the release workflow at the tag
// being installed. A regexp over the repository accepts a certificate from any
// workflow in it, on any ref, which is not what "built by CI from the tag" means.
func TestIP1_SignatureIdentityIsPinnedToTheReleaseWorkflowAtTheTag(t *testing.T) {
	r := runInstaller(t, cosignAccepts)
	if r.err != nil {
		t.Fatalf("install failed with cosign accepting: %v\n%s", r.err, r.out)
	}
	if len(r.cosignArgs) == 0 {
		t.Fatalf("cosign was never called; the harness is not reaching the signature check\n%s", r.out)
	}
	if v, ok := argAfter(r.cosignArgs, "--certificate-identity-regexp"); ok {
		t.Errorf("cosign is given --certificate-identity-regexp %q, which accepts any workflow in the repository", v)
	}
	got, ok := argAfter(r.cosignArgs, "--certificate-identity")
	if !ok {
		t.Fatalf("cosign is not given --certificate-identity; args: %q", r.cosignArgs)
	}
	if got != ipWantIdentity {
		t.Errorf("--certificate-identity = %q, want %q", got, ipWantIdentity)
	}
	if iss, _ := argAfter(r.cosignArgs, "--certificate-oidc-issuer"); iss != "https://token.actions.githubusercontent.com" {
		t.Errorf("--certificate-oidc-issuer = %q", iss)
	}
}

// IP-2. With no cosign, the closing screen must say plainly that provenance was
// not checked. One dim line in the middle of the download step is not a verdict.
func TestIP2_ClosingScreenSaysProvenanceWasNotCheckedWithoutCosign(t *testing.T) {
	r := runInstaller(t, cosignAbsent)
	if r.err != nil {
		t.Fatalf("install failed: %v\n%s", r.err, r.out)
	}
	c := r.closing(t)
	if !strings.Contains(c, "provenance  NOT CHECKED") {
		t.Errorf("closing screen does not carry the NOT CHECKED verdict:\n%s", c)
	}
	if !strings.Contains(c, "cosign") {
		t.Errorf("closing screen does not name cosign as the way to check it:\n%s", c)
	}
	if strings.Contains(c, "provenance  VERIFIED") {
		t.Errorf("closing screen claims VERIFIED with no cosign present:\n%s", c)
	}
}

// IP-3. When the signature verified, the closing screen says so, and names what
// it was verified against.
func TestIP3_ClosingScreenSaysProvenanceVerifiedWhenItWas(t *testing.T) {
	r := runInstaller(t, cosignAccepts)
	if r.err != nil {
		t.Fatalf("install failed: %v\n%s", r.err, r.out)
	}
	c := r.closing(t)
	if !strings.Contains(c, "provenance  VERIFIED") {
		t.Errorf("closing screen does not carry the VERIFIED verdict:\n%s", c)
	}
	if !strings.Contains(c, "release.yml") || !strings.Contains(c, ipVersion) {
		t.Errorf("VERIFIED verdict does not name the workflow and tag it was checked against:\n%s", c)
	}
}

// IP-4. --no-verify past a failed signature installs, and the verdict must not
// read as verified or as merely unchecked.
func TestIP4_NoVerifyPastAFailedSignatureSaysNotVerified(t *testing.T) {
	r := runInstaller(t, cosignRejects, "--no-verify")
	if r.err != nil {
		t.Fatalf("install failed: %v\n%s", r.err, r.out)
	}
	c := r.closing(t)
	if !strings.Contains(c, "provenance  NOT VERIFIED") {
		t.Errorf("closing screen does not carry the NOT VERIFIED verdict:\n%s", c)
	}
}

// IP-C1, control. A rejected signature stops the install and leaves nothing
// behind. This passes before and after the change; it is here so the tests
// above cannot pass on a harness that never reaches cosign.
func TestIPC1_RejectedSignatureInstallsNothing(t *testing.T) {
	r := runInstaller(t, cosignRejects)
	if r.err == nil {
		t.Fatalf("install succeeded with cosign rejecting the signature:\n%s", r.out)
	}
	if len(r.cosignArgs) == 0 {
		t.Fatalf("cosign was never called\n%s", r.out)
	}
	if !strings.Contains(r.out, "Signature verification FAILED") {
		t.Errorf("no failure message:\n%s", r.out)
	}
	if r.installed() {
		t.Errorf("a binary reached %s after a rejected signature", r.binDir)
	}
}
