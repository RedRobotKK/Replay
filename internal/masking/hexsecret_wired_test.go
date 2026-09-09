package masking

import (
	"strings"
	"testing"
)

// A lowercase hex credential must not survive the masker.
//
// LooksLikeHexSecret was written to catch exactly this and, until this test,
// had no caller outside its own unit tests. That mattered because the entropy
// heuristic structurally cannot reach these strings: FindEntropy requires an
// uppercase character to be present (entropy.go:87, `!seen[classUpper]`), and a
// lowercase hex token has none by construction. So the detector for the case
// the other detector cannot see was itself switched off.
//
// The exposure is real rather than theoretical. Lowercase hex is the shape of a
// GitHub personal access token's body, an AWS secret in some tooling, a Stripe
// restricted key's tail, and most self-issued API keys — and this runs inside a
// proxy whose stated job is to keep credentials out of a ledger the user then
// publishes findings from.
//
// The test goes through Mask, not through the predicate, on purpose. A unit
// test of LooksLikeHexSecret passed the whole time the secret was leaking; the
// question that matters is whether the byte survives the pipeline.
func TestHexSecretsAreMaskedEndToEnd(t *testing.T) {
	const secret = "a3f9c21e77b0d4e8a1c6f025b93d7e4482ac10df"
	if len(secret) < 32 {
		t.Fatalf("fixture is %d chars; the detector needs 32 or more", len(secret))
	}
	// Guard the premise rather than assume it: if the entropy path ever learns
	// to see lowercase hex, this test would pass for a reason that has nothing
	// to do with the wiring it exists to check.
	if FindEntropy([]byte(secret), nil) != nil {
		t.Fatal("the entropy heuristic now claims this run, so this test no longer " +
			"proves LooksLikeHexSecret is wired. Re-derive the fixture.")
	}

	for _, text := range []string{
		`api_key = "` + secret + `"`,
		"Authorization: Bearer " + secret,
		"run it with --token " + secret,
	} {
		m := New(newVault(t), nil)
		m.Entropy = true
		cue := string(body(text))
		out, _, err := m.Mask(body(text))
		if err != nil {
			t.Fatalf("Mask: %v", err)
		}
		if strings.Contains(string(out), secret) {
			t.Errorf("a lowercase hex credential survived masking:\n  in:  %s\n  out: %s\n"+
				"LooksLikeHexSecret exists for this and nothing calls it.", cue, out)
		}
	}
}

// And the other half: an ordinary hex string with no credential cue near it
// must still pass through untouched.
//
// Without this, wiring the detector could be "fixed" by masking every 32-plus
// character hex run — which would redact every commit hash, checksum and
// content digest in a transcript and make the ledger useless for the analysis
// it exists to support.
func TestOrdinaryHexIsNotMasked(t *testing.T) {
	const sha = "e1b2c3d4f5a60718293a4b5c6d7e8f9012345678"
	for _, text := range []string{
		"fixed in commit " + sha,
		"sha256 checksum " + sha,
	} {
		m := New(newVault(t), nil)
		m.Entropy = true
		benign := string(body(text))
		out, _, err := m.Mask(body(text))
		if err != nil {
			t.Fatalf("Mask: %v", err)
		}
		if !strings.Contains(string(out), sha) {
			t.Errorf("a benign hex string was masked, which would redact every commit "+
				"hash in a transcript:\n  in:  %s\n  out: %s", benign, out)
		}
	}
}
