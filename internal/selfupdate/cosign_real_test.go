//go:build mutation

package selfupdate

// SIGR: the signer identity, decided by the real cosign binary.
//
// SIG8 models cosign's matching rules and SIG4 pins the arguments, so both
// pass whether or not cosign actually rejects an impostor. This runs the real
// executable. It is behind the mutation tag because it executes a program, and
// it skips unless REPLAY_REAL_COSIGN names one, because no CI runner has cosign.
// A skip is not a pass: the evidence is the run with the variable set.
//
// THE TRUST ROOT IS THE TEST'S OWN, AND THAT IS STATED RATHER THAN HIDDEN.
// Verifying against Sigstore's production root needs the network (TUF for the
// root, Rekor for the inclusion proof), and a Fulcio certificate expires ten
// minutes after issue, so a real release signature cannot be re-checked
// offline at all. So testdata/sigr holds a test CA and one Fulcio-shaped leaf
// per case, made by testdata/sigr/gen.sh with every private key deleted. The
// root reaches cosign through SIGSTORE_ROOT_FILE, and the transparency log and
// SCT checks, which only the production root can satisfy, are skipped. Those
// two flags are added here and never by production code; SIGR0 fails if
// verify.go or install.sh grows either.
//
// The fixtures are files rather than generated here because generating them
// needs crypto/ecdsa, which TestX402_NoSigningCapability keeps out of every
// file in this module, tests included.
//
// What this proves: given a certificate the chain accepts, real cosign accepts
// SignerIdentity(tag) and rejects every other subject and issuer below. What
// it does not prove: Fulcio chain, SCT or Rekor validation.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const ghIssuer = "https://token.actions.githubusercontent.com"

func realCosign(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("REPLAY_REAL_COSIGN")
	if bin == "" {
		t.Skip("REPLAY_REAL_COSIGN is unset, so the real-cosign check did not run. A skip is not a pass.")
	}
	out, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s version: %v\n%s", bin, err, out)
	}
	ver := ""
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "GitVersion:" {
			ver = f[1]
		}
	}
	if ver == "" {
		t.Fatalf("%s did not report a GitVersion:\n%s", bin, out)
	}
	if want := os.Getenv("REPLAY_REAL_COSIGN_VERSION"); want != "" && ver != want {
		t.Fatalf("cosign is %s, the run asked for %s", ver, want)
	}
	t.Logf("cosign %s at %s", ver, bin)
	return bin
}

// verifyReal runs real cosign over the fixture named leaf with identity flag
// flag and value want, and the issuer production passes. It returns cosign's
// output and its exit error.
func verifyReal(t *testing.T, bin, leaf, flag, want string) (string, error) {
	t.Helper()
	dir := filepath.Join("testdata", "sigr")
	cmd := exec.Command(bin, "verify-blob",
		"--certificate", filepath.Join(dir, leaf+".crt"),
		"--signature", filepath.Join(dir, leaf+".sig"),
		flag, want,
		"--certificate-oidc-issuer", ghIssuer,
		// Test trust root only; see the file comment.
		"--insecure-ignore-tlog", "--insecure-ignore-sct",
		filepath.Join(dir, "checksums.txt"),
	)
	cmd.Env = append(os.Environ(), "SIGSTORE_ROOT_FILE="+filepath.Join(dir, "root.crt"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestSIGR_RealCosignAcceptsOnlyTheReleaseWorkflowAtTheTag(t *testing.T) {
	bin := realCosign(t)
	// The fixtures were signed for v9.9.9. The control's subject is written
	// into its certificate by gen.sh, in the shape the real v0.9.0 certificate
	// carries (URI SAN, read with openssl on 2026-10-13):
	// https://github.com/RedRobotKK/Replay/.github/workflows/release.yml@refs/tags/v0.9.0
	// So a wrong SignerIdentity fails the control rather than signing it.
	want := SignerIdentity("v9.9.9")

	cases := []struct {
		leaf, what string
		accept     bool
	}{
		{"control", "release.yml at the tag", true},
		{"ci", "ci.yml at the tag", false},
		{"main", "release.yml on refs/heads/main", false},
		{"othertag", "release.yml at v0.8.0", false},
		{"fork", "release.yml in another owner's repository", false},
		{"contains", "an identity containing the real one", false},
		{"wrongissuer", "the right subject from another issuer", false},
	}
	for _, c := range cases {
		t.Run(c.leaf, func(t *testing.T) {
			out, err := verifyReal(t, bin, c.leaf, "--certificate-identity", want)
			t.Logf("%s: exit=%v\n%s", c.what, err, strings.TrimSpace(out))
			if c.accept && err != nil {
				t.Fatalf("real cosign rejected the identity production requires, so every verdict below is the harness, not cosign: %v\n%s", err, out)
			}
			if !c.accept && err == nil {
				t.Fatalf("real cosign ACCEPTED %s under --certificate-identity %q", c.what, want)
			}
			if !c.accept && !strings.Contains(out, "none of the expected identities matched") {
				t.Fatalf("rejected, but not for the identity. The verdict may be the harness:\n%s", out)
			}
		})
	}

	// The control that makes the rejections mean something: the regexp this
	// replaced, given the same ci.yml certificate, is accepted by the same
	// binary on the same root. If this ever stops being accepted, the cases
	// above may be rejecting for a reason other than the identity.
	t.Run("old regexp accepts ci.yml", func(t *testing.T) {
		out, err := verifyReal(t, bin, "ci", "--certificate-identity-regexp", "https://github.com/"+Repo+"/.*")
		t.Logf("exit=%v\n%s", err, strings.TrimSpace(out))
		if err != nil {
			t.Fatalf("the old regexp rejected ci.yml, so this harness cannot show the defect it guards: %v\n%s", err, out)
		}
	})
}
