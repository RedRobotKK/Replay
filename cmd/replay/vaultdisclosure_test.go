package main

import (
	"strings"
	"testing"
)

// VD. Finding 3, and the half of it the tool never said out loud.
//
// `--mask` writes the secrets it replaces into ~/.replay/vault, which turns a
// transient credential into one at rest. Entries expire after 24 hours, which
// bounds the window. What is not bounded is who can open it: **the vault key
// file sits in the same directory as the ciphertext it decrypts**, so anyone
// who can read that directory inside the window can read the secrets.
//
// That is the oldest open item in RELEASE-CRITERIA, and the file names two
// acceptable outcomes: move the key somewhere else, or say plainly that
// masking is a transit control and not storage. The key cannot move without
// the OS keychain, and reaching that needs os/exec, which
// TestX402_ExecIsConfinedToTheMutationHarness keeps out of every ordinary
// build because it can call anything.
//
// So the second outcome it is, and the README has carried that sentence since
// 2026-09-10. THE BINARY NEVER DID. A person who turns on masking because they
// want secrets protected is exactly the person who has not read the footprint
// section of a 600-line README, and the flag's own help text said nothing.
//
// A disclosure that lives only in documentation is a disclosure aimed at the
// reader who already agrees with it.

// VD1: the flag that creates the vault says what the vault does not protect.
//
// PASS: the help text names the key sitting beside the ciphertext.
// FAIL: the only warning is in a file the user has not opened, at the moment
// they are choosing to write credentials to disk.
func TestVD1_TheMaskFlagDisclosesTheVaultBoundary(t *testing.T) {
	help := maskFlagHelp()

	for _, want := range []string{"vault", "at rest"} {
		if !strings.Contains(strings.ToLower(help), want) {
			t.Errorf("the --mask help does not mention %q.\nIt is the flag that turns a "+
				"transient credential into one at rest, and it is the last moment the user "+
				"can decline.", want)
		}
	}
	if !strings.Contains(help, "not storage") {
		t.Error("the --mask help does not say masking is not storage. RELEASE-CRITERIA " +
			"names that sentence as one of the two acceptable closings of finding 3, and " +
			"a sentence that appears only in the README is aimed at the reader who has " +
			"already read the README.")
	}
}

// VD2: the running proxy says it too, beside the line that claims masking is on.
//
// "masking: on" is what an operator reads as the answer to "are my secrets
// safe". The qualification has to sit next to the claim, not in a file.
func TestVD2_TheRunningProxySaysItBesideTheClaim(t *testing.T) {
	line := vaultAtRestNotice()
	if line == "" {
		t.Fatal("the proxy prints nothing about the vault boundary while claiming masking is on")
	}
	for _, want := range []string{"vault", "key"} {
		if !strings.Contains(strings.ToLower(line), want) {
			t.Errorf("the notice does not mention %q: %q", want, line)
		}
	}
	if !strings.HasPrefix(line, "masking:") {
		t.Errorf("the notice does not sit under the masking banner, so a reader scanning "+
			"for the masking claim will not see its qualification: %q", line)
	}
}

// VD3: it does not fire when masking is off.
//
// A tool that warns about a vault it did not create is noise, and noise is how
// a real disclosure stops being read.
func TestVD3_NothingIsSaidWhenNoVaultExists(t *testing.T) {
	if got := vaultNoticeFor(false); got != "" {
		t.Errorf("masking is off and the tool warned about the vault anyway: %q", got)
	}
	if got := vaultNoticeFor(true); got == "" {
		t.Error("masking is on and the tool said nothing about the vault")
	}
}
