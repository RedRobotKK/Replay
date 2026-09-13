package observation

import (
	"encoding/json"
	"testing"
)

// The website and the docs quote this payload's size and field count in a
// dozen places: "502 bytes", "16 fields", "sixteen counts and ratios, no
// text". Those sentences were true when they were written and stopped being
// true the moment three fields were added, and nothing would have noticed.
//
// A published figure with nothing holding it to the code is a figure that
// drifts. This is what holds it. If these numbers change, this test fails and
// names the documents that have to change with them.

// corpusDocumentedFields is the number of keys a submission can carry.
// Quoted as "nineteen fields" in the documents listed in the failure below.
const corpusDocumentedFields = 19

// corpusDocumentedMaxBytes bounds the wire form. Quoted as "under 600 bytes".
//
// It is a bound rather than a figure because the size moves with the data: the
// floats serialise to their shortest round-tripping form, so a tidy number is
// shorter than an untidy one. The published claim is the bound, which is the
// only part that is true for every contributor.
const corpusDocumentedMaxBytes = 600

// full is a submission with every optional field set and every float at full
// round-trip width. Nothing a real contributor sends can be larger: the counts
// are already implausible and the strings are already the longest shapes the
// binary emits.
func full() Corpus {
	breaks, rereads := 999999, 999999
	errShare := 0.123456789012345
	return Corpus{
		Schema:         CorpusSchema,
		TakenAt:        "2026-09-12T00:00:00Z",
		Tasks:          1234567,
		TotalUSD:       1234567.8901234567,
		AvoidableUSD:   123456.78901234567,
		AvoidableShare: 0.123456789012345,
		MedianTaskUSD:  1234.5678901234567,
		PricedAt:       "2026-09-07",
		RulesVersion:   "anthropic-2026-09-01",
		Unpriced:       999999,
		CacheBreaks:    &breaks,
		ReReads:        &rereads,
		ErrorShare:     &errShare,
		SourceTag:      "c98cd7ce9c61ba2e",
		TagBasis:       "operator",
		BinaryVersion:  "v0.6.0-rc.1",
		Commit:         "87a9b2ad",
		PricingDigest:  "p02eb9163145c",
	}
}

// CS1: the payload carries the number of fields the documents say it does.
//
// PASS: a fully-populated submission has exactly corpusDocumentedFields keys.
// FAIL: every place that quotes a field count is now wrong. The list is in the
// failure message because the person who adds the eighteenth field is not
// necessarily the person who knows where the count is published.
func TestCS1_TheFieldCountIsTheOneTheDocumentsQuote(t *testing.T) {
	body, err := json.Marshal(full().Digested())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != corpusDocumentedFields {
		t.Errorf("a full submission carries %d fields, the documents say %d.\n"+
			"If that is intended, change the constant here and then these, which quote it:\n"+
			"  docs/SURFACES.md\n"+
			"  docs/evidence/contribution-payload-2026-09-10.md\n"+
			"  docs/evidence/README.md\n"+
			"  replay.doctor  src/pages/index.astro\n"+
			"  replay.doctor  src/content/docs/faq.md\n"+
			"  replay.doctor  src/content/docs/contribute.md\n"+
			"  replay.doctor  functions/_lib/contribute.js   (the live allowlist, which REFUSES an unknown key)\n"+
			"Fields seen: %v",
			len(m), corpusDocumentedFields, keysOf(m))
	}
}

// CS2: the wire form stays under the bound the documents quote.
//
// The claim this protects is not vanity. "Smaller than one line of one
// transcript" is the sentence that makes the contribution easy to agree to,
// and it is checkable only while it is true.
func TestCS2_TheWireFormStaysUnderTheDocumentedBound(t *testing.T) {
	body, err := json.Marshal(full().Digested())
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > corpusDocumentedMaxBytes {
		t.Errorf("the largest submission this binary can write is %d bytes; the documents "+
			"say under %d.\nThe payload is: %s", len(body), corpusDocumentedMaxBytes, body)
	}
}

// CS3: the smallest submission is still a submission.
//
// A build from before the optional fields existed writes the minimum. It has
// to stay valid and stay poolable, which is the promise CB1 and CB5 make and
// this one measures.
func TestCS3_TheSmallestSubmissionIsStillValid(t *testing.T) {
	min := Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-12T00:00:00Z",
		Tasks: 1, TotalUSD: 1, MedianTaskUSD: 1,
		PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		SourceTag: "a", TagBasis: "operator",
	}.Digested()
	if err := min.Validate(); err != nil {
		t.Fatalf("the minimum submission does not validate: %v", err)
	}
	body, _ := json.Marshal(min)
	if len(body) > corpusDocumentedMaxBytes {
		t.Errorf("the minimum submission is %d bytes", len(body))
	}
}

// CS4: no field name carries anything identifying.
//
// corpus_test.go already reflects over the struct for a banned list. This
// checks the wire form instead, because that is what a contributor actually
// sends and reads, and because a json tag can differ from its field name.
func TestCS4_NoWireFieldNameIsIdentifying(t *testing.T) {
	body, _ := json.Marshal(full().Digested())
	var m map[string]any
	_ = json.Unmarshal(body, &m)

	banned := []string{"path", "project", "session", "dir", "home", "user", "host", "file", "model", "prompt"}
	for k := range m {
		for _, b := range banned {
			if k == b {
				t.Errorf("the wire form carries a field named %q", k)
			}
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
