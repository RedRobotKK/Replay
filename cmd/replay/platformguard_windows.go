//go:build windows

package main

// platformRefusal states why this build declines to run.
//
// It names the mechanism rather than the policy, because a user who reads
// "unsupported" wants to know what specifically is not being done for them.
func platformRefusal() string {
	return "Replay does not run on Windows.\n\n" +
		"It writes a derived-data ledger and a masking vault encrypted under a key file\n" +
		"beside it, and internal/ownerdir verifies that both directories are private\n" +
		"before either is opened, refusing when it cannot. On Windows that check is a\n" +
		"no-op: the equivalent is an access-control list, and approximating one from a\n" +
		"Unix mode would be a security decision made out of a number that does not mean\n" +
		"what it looks like.\n\n" +
		"So this build refuses rather than writing your secrets into a directory it has\n" +
		"declined to verify. Run Replay under WSL2, Linux or macOS, where the check is\n" +
		"real. See the Platform section of README.md.\n"
}
