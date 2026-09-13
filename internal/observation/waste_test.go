package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

// A submission written before the waste fields existed must still pool.
//
// Pool.Add recomputes the digest rather than trusting it, so any change to what
// Corpus serialises invalidates every submission already written. The waste
// fields are therefore optional and omitted when absent: an old payload
// unmarshals with them nil, marshals to the bytes it always did, and digests to
// the value already in the file.
//
// This is the test that makes the schema change safe rather than the reasoning
// that says it is.
func TestWasteFieldsDoNotChangeAnExistingDigest(t *testing.T) {
	before := Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-10T04:00:00Z",
		Tasks: 116, TotalUSD: 3424.33, RebilledUSD: 162.82,
		RebilledShare: 0.0475, MedianTaskUSD: 1.87,
		PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		SourceTag: "a3f19c02b7e4d581", TagBasis: "local",
	}.Digested()

	// Round-trip through JSON, the way a pool reads a file off disk.
	body, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	var read Corpus
	if err := json.Unmarshal(body, &read); err != nil {
		t.Fatal(err)
	}
	if got := read.Digested().Digest; got != before.Digest {
		t.Errorf("re-digesting a submission with no waste fields gave %s, want %s.\n"+
			"Adding a field that serialises when absent breaks every submission ever\n"+
			"written, because Add recomputes the digest and would refuse them all.", got, before.Digest)
	}
	if strings.Contains(string(body), "cacheBreaks") {
		t.Errorf("absent waste fields were serialised anyway:\n%s", body)
	}
}

// Not measured and measured-zero are different values.
//
// ADR-0018. A build that does not compute an error share and a corpus whose
// error share is genuinely 0.0 must not produce the same payload, or a pooled
// figure cannot tell "nobody measured this" from "everybody measured zero" —
// and the second is a finding while the first is an absence.
func TestWasteFieldsDistinguishAbsentFromZero(t *testing.T) {
	zero := 0.0
	measured := Corpus{Schema: CorpusSchema, ErrorShare: &zero}
	absent := Corpus{Schema: CorpusSchema}

	mb, _ := json.Marshal(measured)
	ab, _ := json.Marshal(absent)
	if !strings.Contains(string(mb), "errorShare") {
		t.Errorf("a measured zero was omitted, making it indistinguishable from absent:\n%s", mb)
	}
	if strings.Contains(string(ab), "errorShare") {
		t.Errorf("an unmeasured field was serialised:\n%s", ab)
	}
	if measured.Digested().Digest == absent.Digested().Digest {
		t.Error("a measured zero and an absent measurement digest identically, so a pool " +
			"cannot tell them apart even by content")
	}
}
