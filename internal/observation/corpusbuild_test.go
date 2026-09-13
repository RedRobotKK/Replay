package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

// Defect #284: on 2026-09-12 two builds read the same transcript directory on
// the same machine and reported $4,088.49 and $11,969.37. Both submissions
// carried rulesVersion "anthropic-2026-09-01", because the provider's rule
// document genuinely had not changed. The code applying it had.
//
// A pool that added those two together would be summing figures produced by
// different arithmetic under one name, and replay.doctor/pool/ has been saying
// by hand which build produced each row because the file could not say it.
//
// These three fields are the file learning to say it.
//
// THE SHAPE OF THE FIX MATTERS AS MUCH AS THE FIX. They are additive and
// omitempty, with no schema bump, for the same reason CacheBreaks, ReReads and
// ErrorShare were: every submission written before today has to stay poolable.
// Digested marshals the struct, so a field that serialised when absent would
// change the digest of every historical file and orphan every roster entry
// naming one. The tests below are what hold that line.

// base returns a corpus that passes Validate, so each test moves one thing.
func base() Corpus {
	return Corpus{
		Schema:        CorpusSchema,
		TakenAt:       "2026-09-12T00:00:00Z",
		Tasks:         119,
		TotalUSD:      4088.49,
		RebilledUSD:   199.77,
		RebilledShare: 0.0489,
		MedianTaskUSD: 12.34,
		PricedAt:      "2026-09-07",
		RulesVersion:  "anthropic-2026-09-01",
		SourceTag:     "launch-2026-09",
		TagBasis:      "operator",
	}
}

// CB1: a submission that records no build identity serialises exactly as it did
// before these fields existed.
//
// This is the compatibility guarantee stated as bytes rather than as an
// intention. Every corpus file already written, and every roster entry naming
// one by digest, depends on it.
//
// PASS: the marshalled JSON contains none of the three keys.
// FAIL: every historical submission's digest changes, so every published roster
// row now names a file that cannot be reproduced from its own contents.
func TestCB1_AnOlderSubmissionSerialisesUnchanged(t *testing.T) {
	body, err := json.Marshal(base().Digested())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"binaryVersion", "commit", "pricingDigest"} {
		if strings.Contains(string(body), key) {
			t.Errorf("a corpus with no build identity serialised %q.\n"+
				"That changes the digest of every submission written before today, "+
				"and every roster entry that names one.\nGot: %s", key, body)
		}
	}
}

// CB2: and its digest is the one it always had.
//
// CB1 checks the keys are absent; this checks the consequence directly, by
// digesting a value built through the same struct and comparing against the
// digest of the same fields marshalled from a map that knows nothing about the
// new ones. If the two disagree, the format moved under its readers.
func TestCB2_TheDigestOfAnOlderSubmissionIsUnchanged(t *testing.T) {
	withStruct := base().Digested()

	// The same document, built without any knowledge of the new fields.
	var asMap map[string]any
	raw, _ := json.Marshal(base())
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"binaryVersion", "commit", "pricingDigest"} {
		if _, ok := asMap[key]; ok {
			t.Fatalf("%q leaked into the wire form of a corpus that never set it", key)
		}
	}

	if withStruct.Digest == "" {
		t.Fatal("digest did not compute")
	}
	// Re-digesting must be idempotent: the published file is the input.
	again := withStruct
	again.Digest = ""
	if got := again.Digested().Digest; got != withStruct.Digest {
		t.Errorf("digest is not reproducible from the file as published: %q then %q",
			withStruct.Digest, got)
	}
}

// CB3: the new fields are inside the digest, not beside it.
//
// The digest is what lets a roster entry stand for a submission. A build
// identity that sat outside it could be edited after publication without the
// name changing, which is exactly the property the digest exists to deny.
//
// PASS: changing any one of the three changes the digest.
// FAIL: the field is decoration. Someone could restate which build produced a
// row and every published digest would still check out.
func TestCB3_BuildIdentityIsCoveredByTheDigest(t *testing.T) {
	plain := base().Digested().Digest

	for _, tc := range []struct {
		name string
		set  func(*Corpus)
	}{
		{"binaryVersion", func(c *Corpus) { c.BinaryVersion = "v0.6.0" }},
		{"commit", func(c *Corpus) { c.Commit = "87a9b2a" }},
		{"pricingDigest", func(c *Corpus) { c.PricingDigest = "pab12cd34ef56" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.set(&c)
			if got := c.Digested().Digest; got == plain {
				t.Errorf("setting %s did not change the digest (%q both times), "+
					"so it could be restated after publication without the name moving",
					tc.name, plain)
			}
		})
	}
}

// CB4: two builds that priced differently produce different submissions even
// when every reported figure and the rules label agree.
//
// This is #284 as a test. Same tasks, same totals, same rulesVersion, same day.
// The only difference is which binary did the arithmetic. Before this change
// those two files were byte-identical and pooled as one.
func TestCB4_TwoBuildsAreDistinguishableUnderOneRulesVersion(t *testing.T) {
	older := base()
	older.BinaryVersion = "v0.5.4"
	older.Commit = "4dcfb9f"
	older.PricingDigest = "p0000000older"

	newer := base()
	newer.BinaryVersion = "v0.6.0"
	newer.Commit = "691450d"
	newer.PricingDigest = "p1111111newer"

	if older.RulesVersion != newer.RulesVersion {
		t.Fatal("this test is only meaningful while both carry the same rules label")
	}
	if older.Digested().Digest == newer.Digested().Digest {
		t.Error("two builds that priced the same corpus differently produced the same " +
			"submission digest. This is defect #284: $4,088.49 and $11,969.37 under " +
			"one name, and a pool with no way to tell them apart.")
	}
}

// CB5: a submission carrying build identity still validates, and one without it
// still validates too.
//
// Validate must not start requiring these. Every file written before today
// lacks them, and a pool that refused those would discard its own history to
// gain a field.
func TestCB5_ValidateRequiresNeitherAndAcceptsBoth(t *testing.T) {
	without := base().Digested()
	if err := without.Validate(); err != nil {
		t.Errorf("a pre-0.6.0 submission no longer validates: %v.\n"+
			"That retires every contribution made before today.", err)
	}

	with := base()
	with.BinaryVersion = "v0.6.0"
	with.Commit = "87a9b2a"
	with.PricingDigest = "pab12cd34ef56"
	if err := with.Digested().Validate(); err != nil {
		t.Errorf("a 0.6.0 submission does not validate: %v", err)
	}
}

// CB6: absence is a value, and it means something specific.
//
// ADR-0018: absence, zero and unknown are three different things. A submission
// with no binaryVersion is not a submission from an unknown build; it is a
// submission from a build that predates the field, which is a fact a pool can
// act on. Round-tripping has to preserve that rather than filling in a default.
func TestCB6_AbsentBuildIdentitySurvivesARoundTrip(t *testing.T) {
	body, _ := json.Marshal(base().Digested())

	var back Corpus
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatal(err)
	}
	if back.BinaryVersion != "" || back.Commit != "" || back.PricingDigest != "" {
		t.Errorf("decoding a pre-0.6.0 submission invented a build identity: %q %q %q",
			back.BinaryVersion, back.Commit, back.PricingDigest)
	}
	if back.Digest != base().Digested().Digest {
		t.Error("a round trip changed the digest of a pre-0.6.0 submission")
	}
}

// CB7: an ADDITIVE change does not move the schema string, and a rename does.
//
// This test asserted `CorpusSchema == "replay.corpus.v1"` until 2026-09-13, and
// its reasoning was right for the change it was written about. binaryVersion,
// commit and pricingDigest are additive and optional, which is precisely the
// change that does not need a new version: an older reader ignores them and a
// newer one finds them, and every submission written before they existed stays
// readable. CB6 above is what protects that, and it still passes.
//
// A RENAME IS NOT ADDITIVE. avoidableUsd became rebilledUsd, so a v1 reader
// handed a v2 document finds no figure where it expects one and reports zero.
// The old test's own warning is the argument for the bump rather than against
// it: "bumping retires every submission already written" is true, and a rename
// retires them whether the string moves or not. Moving it is what makes the
// retirement VISIBLE instead of silent.
//
// Daniel chose bump over re-derivation on 2026-09-13. There is one submission
// in existence, it is the maintainer's, and its digest still verifies against
// its own content: what it needs is a reader that knows which shape it is.
func TestCB7_AdditiveChangesDoNotMoveTheSchemaAndRenamesDo(t *testing.T) {
	if CorpusSchema != "replay.corpus.v2" {
		t.Errorf("CorpusSchema is %q, want replay.corpus.v2. The 2026-09-13 rename "+
			"changed field names, which is not an additive change.", CorpusSchema)
	}

	// The additive rule this test was originally written to protect. Adding an
	// optional field must not require a bump, or every future addition costs a
	// version and readers churn for nothing.
	withBuild := base()
	withBuild.BinaryVersion, withBuild.Commit, withBuild.PricingDigest =
		"v0.6.0", "0bcb2cb", "p02eb9163145c"
	if withBuild.Digested().Schema != base().Digested().Schema {
		t.Error("filling in the optional build-identity fields changed the schema " +
			"string. Those fields are additive and an addition must not cost a version.")
	}
}
