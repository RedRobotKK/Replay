package observation

import (
	"encoding/json"
	"testing"
)

// The contribution payload's shape around an unknown commit.
//
// These assert on the parsed object, not on the marshalled string, so key
// order and formatting cannot make a broken payload look right.
//
// The production contract, read at source 2026-09-17 in
// functions/_lib/contribute.js: `commit` is in OPTIONAL, not REQUIRED, and its
// shape is /^[0-9a-f]{7,40}$/. An absent field is never examined. A present one
// that is "" or "unknown" fails that pattern. So omission is the only truthful
// representation of a commit this binary does not have, and the empty string is
// not a wire value, only the Go value that omitempty elides.

func wireKeys(t *testing.T, c Corpus) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func commitBase() Corpus {
	return Corpus{Schema: CorpusSchema, TakenAt: "2026-09-17T00:00:00Z", Tasks: 1,
		BinaryVersion: "v0.6.0", PricingDigest: "p0123456789ab"}
}

func TestAnEmptyCommitLeavesNoCommitKey(t *testing.T) {
	c := commitBase()
	c.Commit = ""
	if _, present := wireKeys(t, c)["commit"]; present {
		t.Error(`"commit" is on the wire when it should be absent`)
	}
}

// Absence, not null and not "". Each is a different thing to a reader, and two
// of them are refused by the validator.
func TestOmissionIsNotNullAndNotEmptyString(t *testing.T) {
	c := commitBase()
	c.Commit = ""
	m := wireKeys(t, c)
	if raw, present := m["commit"]; present {
		t.Errorf(`"commit" present as %s; the representation of an unknown commit is absence`, raw)
	}
	for _, bad := range []string{"commitSha", "gitCommit", "commit_hash", "commitHash"} {
		if _, present := m[bad]; present {
			t.Errorf("an alternate provenance key %q appeared; the fix is omission, not renaming", bad)
		}
	}
}

func TestAKnownCommitIsCarriedExactly(t *testing.T) {
	for _, sha := range []string{"b3667e3", "28108676c2478980b7d22892b584ea630b8c6c9e"} {
		c := commitBase()
		c.Commit = sha
		m := wireKeys(t, c)
		raw, present := m["commit"]
		if !present {
			t.Fatalf("a known commit %q was dropped", sha)
		}
		var got string
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("commit is not a JSON string: %s", raw)
		}
		if got != sha {
			t.Errorf("commit = %q, want %q; no normalisation or truncation", got, sha)
		}
	}
}

// Version availability and commit availability are separate facts. A proxy
// install has one and not the other.
func TestVersionSurvivesWithoutACommit(t *testing.T) {
	c := commitBase()
	c.Commit = ""
	m := wireKeys(t, c)
	if _, present := m["commit"]; present {
		t.Error("commit present")
	}
	var bv string
	if err := json.Unmarshal(m["binaryVersion"], &bv); err != nil || bv != "v0.6.0" {
		t.Errorf("binaryVersion = %v (err %v), want v0.6.0 unchanged", bv, err)
	}
	var pd string
	if err := json.Unmarshal(m["pricingDigest"], &pd); err != nil || pd != "p0123456789ab" {
		t.Errorf("pricingDigest = %v (err %v), want it unaffected", pd, err)
	}
}

// Digested marshals the struct, so omitempty governs the digest bytes and the
// wire bytes identically. The failure this guards is a payload that omits
// commit while its digest was computed over one that carried "unknown".
func TestTheDigestIsOverThePayloadActuallySubmitted(t *testing.T) {
	c := commitBase()
	c.Commit = ""
	d := c.Digested()
	if _, present := wireKeys(t, d)["commit"]; present {
		t.Fatal("the digested payload carries a commit key")
	}
	sentinel := commitBase()
	sentinel.Commit = "unknown"
	if d.Digest == sentinel.Digested().Digest {
		t.Error("a commitless payload digests the same as one carrying the sentinel")
	}
	again := commitBase()
	again.Commit = ""
	if again.Digested().Digest != d.Digest {
		t.Error("the same commitless payload digested differently twice")
	}
}
