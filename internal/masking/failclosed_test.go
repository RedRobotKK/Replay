package masking

import (
	"errors"
	"strings"
	"testing"
)

// Failing open on a metric is fine. Failing open on a secret is an incident.
//
// Mask returned the ORIGINAL body when the vault could not store a match, and
// the proxy logged "the request was forwarded UNMASKED, including any secret
// already matched" and forwarded it. The comment above that call says it
// "fails open like every other feature" - correct for a policy, wrong for a
// redaction, and the difference is that the secret leaves the machine.
//
// The rule is availability WITHOUT leakage: the stream still goes through, the
// credential does not. When the mapping cannot be stored, the region is
// blind-scrubbed with a terminal placeholder that carries no vault entry and
// cannot be rehydrated.

// MK-1: a vault failure never returns the secret.
//
// PASS: the body that comes back does not contain the credential, whatever
// else it contains.
// FAIL: the original body, which is what shipped.
func TestMK1_VaultFailureNeverReturnsTheSecret(t *testing.T) {
	const secret = "sk-ant-api03-AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHH"
	body := []byte(`{"messages":[{"role":"user","content":"key is ` + secret + `"}]}`)

	m := NewWithVault(nil, failingVault{})
	out, _, err := m.Mask(body)
	if strings.Contains(string(out), secret) {
		t.Fatalf("the secret survived a vault failure:\n%s", out)
	}
	if err == nil {
		t.Error("the caller must still learn the mapping could not be stored")
	}
}

// MK-2: the stream still goes through.
//
// Blind-scrubbing preserves uptime; refusing the request would trade one
// failure mode for another. The body must remain usable.
func TestMK2_TheStreamStillGoesThrough(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"key is sk-ant-api03-AAAABBBBCCCCDDDDEEEEFFFFGGGGHHHH and please continue"}]}`)
	m := NewWithVault(nil, failingVault{})
	out, _, _ := m.Mask(body)
	if len(out) == 0 {
		t.Fatal("blind-scrubbing must return a body, not nothing")
	}
	if !strings.Contains(string(out), "please continue") {
		t.Errorf("the rest of the request must survive:\n%s", out)
	}
	if !strings.Contains(string(out), BlindPlaceholder) {
		t.Errorf("the scrubbed region must be marked so the reader knows what happened:\n%s", out)
	}
}

// MK-3: the blind placeholder is not a vault placeholder.
//
// Rehydration looks up REPLAY_SECRET_ tokens. A blind scrub has no entry, so
// it must not wear that prefix or a later restore silently finds nothing and
// reports a denial for a secret that was never stored.
func TestMK3_BlindPlaceholderIsDistinctFromAVaultOne(t *testing.T) {
	if strings.HasPrefix(BlindPlaceholder, PlaceholderPrefix) {
		t.Errorf("%q wears the vault prefix; rehydration would look it up and find nothing",
			BlindPlaceholder)
	}
}

type failingVault struct{}

func (failingVault) Placeholder(string, string) (string, error) {
	return "", errors.New("vault directory is read-only")
}

// MK-4: lowercase hex near a credential cue is a secret; bare hex is a hash.
//
// The entropy heuristic required lowercase AND uppercase AND digits, so a
// lowercase hex credential was invisible to it. Closing that with a length
// rule was measured first and rejected: 1,928 lowercase-hex runs of 32+
// characters in the corpus, of which 848 are 40 characters (git SHAs), 490 are
// 32 (MD5 or a dashless UUID) and 234 are 64 (SHA-256). A length rule redacts
// commit hashes out of prompts and ruins the tool it is protecting.
//
// A prefix anchor does not help either: a prefixed key like sk-ant- is already
// caught by the pattern matcher, so the blind spot is precisely the hex with
// NO recognisable envelope.
//
// Proximity is what actually separates them. Measured on the same corpus: 43
// of 1,928 hex runs carry a credential cue within 40 characters before them,
// and 1,885 are bare.
//
// PASS: cued hex is detected, bare hex of every canonical hash length is not.
// FAIL: either blindness to the cued case, or a masker that eats git history.
func TestMK4_HexIsJudgedByProximityNotLength(t *testing.T) {
	const key = "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
	if !LooksLikeHexSecret("api_key = \""+key+"\"", len("api_key = \""), len(key)) {
		t.Error("a hex run introduced by api_key must be treated as a credential")
	}
	// A SHORT hex run WITH a cue. Without this the length floor is never
	// exercised: every benign case below is rejected by the missing cue, so
	// removing the floor changes nothing and the mutant survives. That is the
	// fifth time today a test in this project has refused for the wrong
	// reason.
	short := `api_key = "abc123"`
	if LooksLikeHexSecret(short, strings.Index(short, "abc123"), 6) {
		t.Error("a six-character hex value is not a credential, cue or not; the length floor " +
			"is what stops every short hex literal being masked")
	}
	for _, benign := range []string{
		"d67dd6c4293f06b9b2a7522d7131b2696014e0bc",                         // 40, git SHA
		"9e107d9d372bb6826bd81d3542a419d6",                                 // 32, MD5
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // 64
	} {
		line := "see commit " + benign + " for the change"
		if LooksLikeHexSecret(line, strings.Index(line, benign), len(benign)) {
			t.Errorf("masker over-reached and would corrupt a bare %d-character hash: %s",
				len(benign), benign)
		}
	}
}
