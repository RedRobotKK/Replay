package selfupdate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// SIG6, SIG7: who is allowed to have signed checksums.txt.
//
// The check passed --certificate-identity-regexp "https://github.com/<repo>/.*",
// which cosign applies UNANCHORED to the certificate's subject alternative name
// (sigstore cosign pkg/cosign/verify.go: regex.MatchString(san)). That accepts
// a certificate from any workflow in the repository, on any branch or tag, and
// from any identity that merely contains the repository URL. "Built by CI from
// the tag", which is what `replay upgrade` prints, was not what was checked.
//
// cosignDecides stands in for cosign's identity decision so the rule can be
// tested without a real certificate, a network or a signing key. It follows
// cosign's own code for the two flags: --certificate-identity is compared with
// ==, and --certificate-identity-regexp is matched without anchors. Everything
// else cosign checks (the Fulcio chain, the transparency log, the signature
// bytes) is out of scope here and assumed to pass.
func cosignDecides(san string, calls *int) func(context.Context, string, ...string) error {
	return func(_ context.Context, _ string, args ...string) error {
		*calls++
		var exact, re string
		for i := 0; i+1 < len(args); i++ {
			switch args[i] {
			case "--certificate-identity":
				exact = args[i+1]
			case "--certificate-identity-regexp":
				re = args[i+1]
			}
		}
		switch {
		case exact != "":
			if san != exact {
				return errors.New("none of the expected identities matched what was in the certificate")
			}
			return nil
		case re != "":
			if !regexp.MustCompile(re).MatchString(san) {
				return errors.New("none of the expected identities matched what was in the certificate")
			}
			return nil
		}
		return errors.New("no identity constraint was passed")
	}
}

const sigTestTag = "v9.9.9" // the tag signedRelease publishes

func TestSIG6_OnlyTheReleaseWorkflowAtTheRequestedTagIsAccepted(t *testing.T) {
	const repo = "https://github.com/" + Repo
	cases := []struct {
		name   string
		san    string
		accept bool
	}{
		{"release workflow at the requested tag", repo + "/.github/workflows/release.yml@refs/tags/" + sigTestTag, true},
		{"another workflow in the repository, same tag", repo + "/.github/workflows/ci.yml@refs/tags/" + sigTestTag, false},
		{"release workflow on a branch", repo + "/.github/workflows/release.yml@refs/heads/main", false},
		{"release workflow at a different tag", repo + "/.github/workflows/release.yml@refs/tags/v0.8.0", false},
		{"release workflow in another repository", "https://github.com/Other/Replay/.github/workflows/release.yml@refs/tags/" + sigTestTag, false},
		{"an identity that only contains the repository URL", "https://attacker.example/?u=" + repo + "/.github/workflows/release.yml@refs/tags/" + sigTestTag, false},
		{"a person, not a workflow", "someone@example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreLook, restoreRun := LookCosign, runCosign
			t.Cleanup(func() { LookCosign, runCosign = restoreLook, restoreRun })
			LookCosign = func() (string, error) { return "/usr/bin/cosign", nil }
			calls := 0
			runCosign = cosignDecides(tc.san, &calls)

			srv, base := signedRelease(t, true)
			defer srv.Close()
			c := &Client{ReleasesBase: base}
			bin, sig, err := c.Fetch(context.Background(), sigTestTag, "linux", "amd64")

			// Negative control: the decision under test was actually taken.
			if calls != 1 {
				t.Fatalf("cosign was called %d times; the identity decision was not reached", calls)
			}
			if tc.accept {
				if err != nil {
					t.Fatalf("the genuine release identity was refused: %v", err)
				}
				if sig != SignatureVerified {
					t.Errorf("status %v, want verified", sig)
				}
				return
			}
			if err == nil {
				t.Fatalf("a certificate for %q was accepted as this project's release signer", tc.san)
			}
			if bin != nil {
				t.Error("a binary was returned alongside the refusal")
			}
			if sig == SignatureVerified {
				t.Error("the refusal still reports the signature as verified")
			}
			if !strings.Contains(err.Error(), "Nothing was installed") {
				t.Errorf("the refusal does not say nothing was installed: %v", err)
			}
		})
	}
}

// SIG7: the identity is built from the same tag string the archive was
// downloaded from, so the two cannot name different releases.
func TestSIG7_SignerIdentityIsTheReleaseWorkflowAtTheTag(t *testing.T) {
	for tag, want := range map[string]string{
		"v0.9.0":     "https://github.com/RedRobotKK/Replay/.github/workflows/release.yml@refs/tags/v0.9.0",
		"v1.0.0-rc1": "https://github.com/RedRobotKK/Replay/.github/workflows/release.yml@refs/tags/v1.0.0-rc1",
	} {
		if got := SignerIdentity(tag); got != want {
			t.Errorf("SignerIdentity(%q) = %q, want %q", tag, got, want)
		}
	}
}

// SIGR0: neither verifier skips the transparency log or the SCT.
//
// The real-cosign test (SIGR, mutation tag) passes both flags because its trust
// root is its own and cannot satisfy them. Production must never pass them:
// either flag accepts a signature nobody can audit. This runs in every build,
// unlike SIGR, so CI watches it.
func TestSIGR0_ProductionNeverSkipsTransparencyOrSCT(t *testing.T) {
	for _, rel := range []string{"verify.go", "fetch.go", filepath.Join("..", "..", "install.sh")} {
		body, err := os.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"--insecure-ignore-tlog", "--insecure-ignore-sct", "--offline"} {
			if strings.Contains(string(body), flag) {
				t.Errorf("%s passes %s to cosign", rel, flag)
			}
		}
	}
}
