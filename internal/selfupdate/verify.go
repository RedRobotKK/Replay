package selfupdate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// The Sigstore check lives in its own file, and that is a decision rather than
// tidiness.
//
// fetch.go holds an os/exec exemption whose whole defensibility is that it runs
// ONE thing: a binary this process just downloaded and checksum-verified, by
// absolute path, with the single literal argument "version".
// TestX402_SelfUpdateExecIsNotArbitrary pins that shape so the exemption cannot
// quietly grow. Adding a second, differently shaped exec call to that file
// would have been exactly the growth the test was written to catch, and the
// test caught it.
//
// So this gets its own file and its own pinned shape. The two calls make
// different promises and they are reviewed separately.
//
// THE HONEST EXPOSURE, stated rather than buried: this resolves "cosign" from
// PATH. Somebody who controls PATH during an upgrade controls what runs. That
// is the same exposure install.sh accepts with `command -v cosign`, and it is
// the price of not reimplementing Sigstore verification inside a module with
// zero third-party dependencies. It is not a new exposure introduced here, but
// it is a real one and it should not be discovered by a reader.

// lookCosign and runCosign are the seam the signature check is tested through.
//
// Production never assigns either. A test substitutes them to say whether
// cosign exists on this machine and what it decided, because the alternative is
// a guard that only fires on a machine with cosign installed and a real signed
// release to hand, which is a guard nothing in CI can ever watch fail.
var (
	lookCosign = func() (string, error) { return exec.LookPath("cosign") }
	runCosign  = func(ctx context.Context, bin string, args ...string) error {
		return exec.CommandContext(ctx, bin, args...).Run()
	}
)

// verifyChecksumSignature checks the Sigstore signature on checksums.txt.
//
// WHY THIS EXISTS. Until 2026-09-13 this package verified the hash and never
// the signature, while install.sh refused when cosign was present and no
// signature was published. Two install routes, two different promises, and
// nothing told the user which one they were getting. The releases were signed
// the whole time: release.yml publishes checksums.txt.pem and checksums.txt.sig
// and they went unread.
//
// The hash alone covers a corrupted download and an archive swapped against an
// unmodified checksums.txt. It cannot cover anything able to modify
// checksums.txt itself, because the file that is trusted is the file that is
// fetched. That gap is what the signature closes.
//
// COSIGN IS SHELLED OUT TO RATHER THAN REIMPLEMENTED. A correct Sigstore check
// needs a Fulcio certificate chain, a Rekor inclusion proof and an identity
// check against the signing workflow. This module has zero third-party
// dependencies on purpose, and a partial check written here would read as
// verification in the code and in the docs while proving only that somebody
// signed something.
//
// NO COSIGN MEANS CHECKSUMS ONLY, and that is deliberate rather than lax: it is
// exactly what install.sh does, and refusing to upgrade a machine that has no
// cosign would strand every user who installed the documented way.
func (c *Client) verifyChecksumSignature(ctx context.Context, base string, sums []byte) error {
	bin, err := lookCosign()
	if err != nil {
		// Same posture as install.sh: say nothing here, verify what can be
		// verified, and leave the stronger check to machines that have the tool.
		return nil
	}

	pem, pemErr := c.get(ctx, base+"/checksums.txt.pem")
	sig, sigErr := c.get(ctx, base+"/checksums.txt.sig")
	if pemErr != nil || sigErr != nil {
		return fmt.Errorf("no Sigstore signature was published for this release and cosign " +
			"is installed to check one. Every release this project's CI builds is signed, " +
			"so a missing signature means these assets are not the ones CI produced. " +
			"Nothing was installed")
	}

	dir, err := os.MkdirTemp("", "replay-verify-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	for name, body := range map[string][]byte{
		"checksums.txt":     sums,
		"checksums.txt.pem": pem,
		"checksums.txt.sig": sig,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
			return err
		}
	}

	// The two identity flags are not optional. Without them cosign confirms that
	// the blob was signed by somebody, which is not the question being asked.
	// These pin the signer to this repository's workflows and to GitHub's OIDC
	// issuer, and they are the same two install.sh passes.
	if err := runCosign(ctx, bin,
		"verify-blob",
		"--certificate", filepath.Join(dir, "checksums.txt.pem"),
		"--signature", filepath.Join(dir, "checksums.txt.sig"),
		"--certificate-identity-regexp", "https://github.com/"+Repo+"/.*",
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		filepath.Join(dir, "checksums.txt"),
	); err != nil {
		return fmt.Errorf("signature verification FAILED for checksums.txt. Nothing was "+
			"installed. The checksums may be intact but they were not signed by this "+
			"project's CI: %w", err)
	}
	return nil
}
