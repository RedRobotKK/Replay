package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-WK. The bearer key must not reach the binary.
//
// A specialist panel found this on 2026-09-12. The Watch plugin's SessionEnd
// hook called:
//
//	replay watch emit --repo "$repo" --key "$key" --out "$out" "$transcript"
//
// The binary CANNOT SEND. `TestO7_ThisPackageCannotSend` enforces that with an
// import allowlist, and it is the project's central claim. So the key had no
// use there at all: it was a live credential placed in the process table on
// every session, readable by any other process on the machine, to be received
// by code that could not have used it.
//
// The key is used once, by the curl that actually makes the request, which is
// the only place it has a job.
//
// PASS: no invocation of the binary in the plugin passes a key.
// FAIL: a credential is back in `ps` output for no reason.
func TestFCWK_TheHookDoesNotHandTheBinaryACredential(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "plugins", "replay", "hooks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no plugin hooks directory yet: %v", err)
	}
	var checked int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		checked++
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for i, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			// Any invocation of the binary. curl is the one caller allowed a key.
			if !strings.Contains(line, "replay ") || strings.Contains(line, "curl") {
				continue
			}
			if strings.Contains(line, "--key") || strings.Contains(line, "Bearer") {
				t.Errorf("%s:%d hands a credential to a binary that cannot send, which "+
					"puts it in `ps` output on every session for no purpose:\n  %s",
					e.Name(), i+1, trimmed)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no hook scripts were read, so this test proves nothing")
	}
}

// FC-WK2: the README's no-network-call sentence is scoped to the binary.
//
// The same panel's reading. The README said every command works "with no
// account, no key and no network call" while a shell script in this same
// repository posts to replay.doctor with a bearer key. Both halves were written
// honestly and the pair is not defensible: a reader who finds the curl line
// after reading that sentence has caught the project doing the thing it says it
// does not do, and the distinction between "the binary" and "a script we ship"
// is one drawn for the writer's benefit rather than the reader's.
//
// The curl line is not the defect. The sentence beside it was.
func TestFCWK2_TheNoNetworkClaimIsScopedToTheBinary(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	readme := string(raw)

	ships := false
	if hooks, err := os.ReadDir(filepath.Join(root, "plugins", "replay", "hooks")); err == nil {
		for _, e := range hooks {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, "plugins", "replay", "hooks", e.Name()))
			if err != nil {
				continue
			}
			// An EXECUTED request, not the word. This hook prints an install
			// line containing `curl` and its header comment discusses the send
			// path that was removed; neither opens a socket, and a test that
			// counted them would demand the README apologise for prose.
			for _, line := range strings.Split(string(b), "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}
				if strings.Contains(trimmed, "printf") || strings.Contains(trimmed, "echo ") {
					continue
				}
				for _, sender := range []string{"curl ", "wget ", "nc ", "/dev/tcp"} {
					if strings.Contains(trimmed, sender) {
						ships = true
					}
				}
			}
		}
	}
	if !ships {
		return // Nothing in the repo sends; the unqualified sentence is true.
	}

	// Whitespace-normalised, because the README wraps. The first version of
	// this test searched for the literal sentence, the sentence spans a line
	// break, the search found nothing, and the test returned early reporting
	// success. It passed against a README that had never been fixed. That is
	// the exact defect this package is for, committed by the test written to
	// prevent it.
	flat := strings.Join(strings.Fields(readme), " ")
	claim := "no account, no key and no network call"
	idx := strings.Index(flat, claim)
	if idx == -1 {
		return // The sentence is gone; nothing to scope.
	}
	readme = flat
	// Within a short window after the claim, the README must say what it covers
	// and admit what sends.
	window := readme[idx:min(idx+900, len(readme))]
	if !strings.Contains(window, "binary") {
		t.Error("the README claims no network call and does not say the claim is about " +
			"the binary, while a shell script in this repository posts with a bearer key.")
	}
	if !strings.Contains(strings.ToLower(window), "plugin") {
		t.Error("the README claims no network call and never mentions the plugin that " +
			"does send. A reader who finds the curl line afterwards has caught the " +
			"project doing the thing it says it does not do.")
	}
}

// FC-NET. The README must not claim the binary makes no network call.
//
// It said "no account, no key and no network call" until 2026-09-13, in two
// places, and it was false. `internal/observation/observation.go` says so in
// the source, unprompted: "the binary as a whole is a proxy and also originates
// billable requests under `probe --execute`, so a claim that the binary never
// reaches the network would be false."
//
// THIS IS THE SECOND TIME. `cmd/replay/outbound_drift_test.go` exists because
// the same sentence went false once before: the README said the binary made no
// request except the proxy and one you type, then `replay probe` shipped and
// nothing edited the claim. That test catches a new outbound PACKAGE. Nothing
// caught the SENTENCE, so the sentence broke again in the same way.
//
// The true promise is the one cmd/replay/upgrade.go states: replay originates
// no request you did not type. That is narrower and defensible, and it is what
// the README says now.
func TestFCNET_TheReadmeDoesNotClaimNoNetworkCall(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	flat := strings.Join(strings.Fields(string(raw)), " ")

	// The claim, in the shapes it has actually taken. Each is checked outside
	// quotation marks, so the README can still describe the retired sentence.
	for _, claim := range []string{
		"no key and no network call",
		"no key, no network call",
		"makes no network call",
		"never reaches the network",
	} {
		idx := 0
		for {
			i := strings.Index(flat[idx:], claim)
			if i < 0 {
				break
			}
			at := idx + i
			// Quoted, so it is the README talking ABOUT the claim.
			if at > 0 && flat[at-1] == '"' {
				idx = at + len(claim)
				continue
			}
			t.Errorf("README asserts %q. The binary reaches the network from five "+
				"packages, every one listed in docs/SURFACES.md and derived from the "+
				"code by TestOutboundSurfacesAreAllDocumented. The true promise is "+
				"upgrade.go's: replay originates no request you did not type.", claim)
			break
		}
	}

	// And the narrower promise must actually be there, or this test only
	// forbids a sentence without requiring the honest one.
	if !strings.Contains(flat, "originates no request you did not type") {
		t.Error("README no longer carries the promise it can keep. Forbidding the false " +
			"claim without stating the true one leaves a reader with nothing.")
	}
}
