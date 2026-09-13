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

// LookCosign and runCosign are the seam the signature check is tested through.
//
// LookCosign is EXPORTED, and that is the point of this change.
//
// It was unexported until 2026-09-13, so only this package could pin it. The
// v0.6.0 release then failed TWICE for one reason, in two different packages:
// tests whose result was decided by whether cosign happened to be on the PATH
// of the machine running them. Every CI runner has no cosign. The release job
// installs it, in order to sign. So the release runner is the only machine in
// this project's history that runs the suite WITH cosign present, and it is the
// machine that publishes.
//
// The first fix pinned this variable for this package only. cmd/replay reaches
// the same code through `replay upgrade` and could not pin an unexported
// variable, so its two upgrade tests kept reading the machine and the second
// release attempt died on them. That is the shape of fixing a defect where it
// was found rather than where it lives.
//
// NOT AN ENVIRONMENT VARIABLE, deliberately. A package variable is assigned at
// compile time by code in this module. An env var would let anyone who controls
// the environment during an upgrade switch the signature check off, which is
// the supply-chain hole this seam exists to protect.
//
// Production never assigns it.
//
// Production never assigns either. A test substitutes them to say whether
// cosign exists on this machine and what it decided, because the alternative is
// a guard that only fires on a machine with cosign installed and a real signed
// release to hand, which is a guard nothing in CI can ever watch fail.
var (
	LookCosign = func() (string, error) { return exec.LookPath("cosign") }
	runCosign  = func(ctx context.Context, bin string, args ...string) error {
		return exec.CommandContext(ctx, bin, args...).Run()
	}
	// makeVerifyDir and writeVerifyFile are the staging seam.
	//
	// Their error branches were reported UNREACHED by `guard reachability`, and
	// it was right: a fresh 0700 temp directory does not fail to be created or
	// written to on a healthy machine, so nothing could ever watch those
	// branches fire. They are the error paths of an UPGRADE'S SIGNATURE CHECK.
	// "It cannot fail here so nobody checked" is precisely the state that makes
	// a security branch wrong the first time it is finally taken, and a
	// full disk during an upgrade is not an exotic scenario.
	//
	// Production never assigns either.
	makeVerifyDir   = func() (string, error) { return os.MkdirTemp("", "replay-verify-") }
	writeVerifyFile = func(path string, body []byte) error { return os.WriteFile(path, body, 0o600) }
)

// SignatureStatus says which promise an upgrade actually kept.
//
// It exists because `replay upgrade` printed "Checksum verified" in all three
// outcomes, while install.sh prints three distinct lines. A user who ran the
// installer got signature verification and the same user running upgrade the
// following week did not, and nothing told them the guarantee had changed. The
// code half of that gap was closed on 2026-09-13; this is the half a person can
// see.
type SignatureStatus int

const (
	// SignatureUnchecked means cosign is not on this machine, so the download
	// was verified against checksums.txt and no further. Not a failure: it is
	// what install.sh does, and refusing to upgrade a machine without cosign
	// would strand everyone who installed the documented way.
	SignatureUnchecked SignatureStatus = iota
	// SignatureVerified means cosign confirmed checksums.txt was signed by this
	// project's CI, with the identity pinned to the repository and to GitHub's
	// OIDC issuer.
	SignatureVerified
)

func (s SignatureStatus) String() string {
	if s == SignatureVerified {
		return "signature verified (Sigstore, built by CI from the tag)"
	}
	return "cosign not installed, so the signature was not checked; checksums only"
}

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
func (c *Client) verifyChecksumSignature(ctx context.Context, base string, sums []byte) (SignatureStatus, error) {
	bin, err := LookCosign()
	if err != nil {
		// Same posture as install.sh: verify what can be verified, leave the
		// stronger check to machines that have the tool, and SAY WHICH.
		return SignatureUnchecked, nil
	}

	pem, pemErr := c.get(ctx, base+"/checksums.txt.pem")
	sig, sigErr := c.get(ctx, base+"/checksums.txt.sig")
	if pemErr != nil || sigErr != nil {
		return SignatureUnchecked, fmt.Errorf("no Sigstore signature was published for this release and cosign " +
			"is installed to check one. Every release this project's CI builds is signed, " +
			"so a missing signature means these assets are not the ones CI produced. " +
			"Nothing was installed")
	}

	dir, err := makeVerifyDir()
	if err != nil {
		// A DISTINCT message from the write failure below. Both said "could not
		// be staged" until 2026-09-13, and with one message for two branches a
		// test asserting on it passed whichever branch produced it: removing
		// this one entirely let execution fall through to the write, which
		// failed with the same words. Two failures, one sentence, one of them
		// unobservable.
		return SignatureUnchecked, fmt.Errorf("the staging directory for the signature could not be created, "+
			"so the signature was not checked and nothing was installed: %w", err)
	}
	// An empty path with no error would make every filepath.Join below relative
	// to the working directory, so the three staged files would be written into
	// whatever directory the user happened to run `replay upgrade` from, and
	// cosign would be handed those instead. Found while mutation-testing the
	// branch above: neutralising it left dir empty and three files appeared in
	// the package directory. That was a mutant rather than a defect, and the
	// hazard it exposed is real and one line to close.
	if dir == "" {
		return SignatureUnchecked, fmt.Errorf("the signature staging directory came back empty, so the " +
			"signature was not checked and nothing was installed")
	}
	defer func() { _ = os.RemoveAll(dir) }()

	for name, body := range map[string][]byte{
		"checksums.txt":     sums,
		"checksums.txt.pem": pem,
		"checksums.txt.sig": sig,
	} {
		if err := writeVerifyFile(filepath.Join(dir, name), body); err != nil {
			return SignatureUnchecked, fmt.Errorf("%s could not be staged for checking, so the signature was "+
				"not checked and nothing was installed: %w", name, err)
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
		return SignatureUnchecked, fmt.Errorf("signature verification FAILED for checksums.txt. Nothing was "+
			"installed. The checksums may be intact but they were not signed by this "+
			"project's CI: %w", err)
	}
	return SignatureVerified, nil
}
