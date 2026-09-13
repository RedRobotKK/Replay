package main

import (
	"github.com/RedRobotKK/Replay/internal/proxy"
)

// The masking disclosures, in one place because they are one argument.
//
// FINDING 3, AND WHY IT CLOSES HERE RATHER THAN IN THE CODE IT IS ABOUT.
//
// `--mask` writes the secrets it replaces into ~/.replay/vault, which turns a
// transient credential into one at rest. Entries expire after 24 hours, so the
// window is bounded. Who can open it is not: the vault key file sits in the
// same directory as the ciphertext it decrypts, so anyone who can read that
// directory inside the window can read the secrets.
//
// RELEASE-CRITERIA names two acceptable closings: move the key, or say plainly
// that masking is a transit control and not storage. The key cannot move. The
// OS keychain needs os/exec, and TestX402_ExecIsConfinedToTheMutationHarness
// keeps os/exec out of every ordinary build on the grounds that it can call
// anything, which is a stronger guarantee than this one finding is worth
// trading for.
//
// So the second closing, and the README has carried that sentence since
// 2026-09-10. What was missing is that the BINARY never said it. A person who
// turns on masking because they want secrets protected is precisely the person
// who has not read the footprint section of a long README, and a disclosure
// that lives only in documentation is aimed at the reader who already agrees
// with it.
//
// It is said twice on purpose: once on the flag, which is the last moment the
// user can decline, and once under the running banner, because "masking: on"
// is the line an operator reads as the answer to "are my secrets safe" and a
// qualification has to sit next to the claim it qualifies.

// maskFlagHelp is the -mask help text.
//
// Both limits, in the order they bite. The OpenAI path is unmasked entirely,
// which is the larger hole; the vault boundary applies even on the path that
// does work.
func maskFlagHelp() string {
	return "EXPERIMENTAL: replace secrets matching the named pattern set with vault " +
		"placeholders before requests leave the machine, and restore them in responses " +
		"within -rehydrate-scope (see README). It reads " + proxy.MessagesPath +
		" and nothing else: " + proxy.ChatCompletionsPath + " is EXPERIMENTAL, UNMASKED, " +
		"so secrets in OpenAI-compatible traffic are forwarded to the provider in clear " +
		"even with this on, and the proxy says so on stderr once per path. " +
		"It also writes what it masks to a vault under ~/.replay, which puts a credential " +
		"at rest that was not at rest before, and the vault key file sits in that same " +
		"directory: this is a control on what leaves the machine, not storage, and anyone " +
		"who can read the directory within the retention window can read the secrets"
}

// vaultAtRestNotice is the line printed under "masking: on".
//
// Prefixed "masking:" so it sits with the claim when an operator scans the
// startup output for that word, which is what they scan for.
func vaultAtRestNotice() string {
	// The path is written without a trailing conjunction on purpose.
	// TestEveryPrintedCommandDispatches scans printed text for command shapes
	// and read "~/.replay and" as a command named "replay and", which is a
	// false positive and also a fair one: if a scanner can read it that way so
	// can a person skimming output.
	return "masking: secrets are written to a vault under ~/.replay, whose key file " +
		"sits beside them, so this bounds what leaves the machine rather than who can read " +
		"the vault; it is a transit control, not storage"
}

// vaultNoticeFor returns the notice only when a vault will exist.
//
// Warning about a vault the run did not create is noise, and noise is how a
// disclosure stops being read.
func vaultNoticeFor(masking bool) string {
	if !masking {
		return ""
	}
	return vaultAtRestNotice()
}
