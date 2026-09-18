package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

// A pooled row has to be able to say which vendor it came from.
//
// Until now it could not. The payload carried spend, cache counts and build
// provenance, and nothing that named the surface read or the models run. So a
// hundred contributed corpora would have pooled into one undifferentiated
// figure, and the question the launch is about, what Astra costs against what
// Fable costs, could not be asked of them at all.
//
// Surfaces and Models are additive and optional, exactly as CacheBreaks,
// ReReads, ErrorShare and the three #284 provenance fields were, and for the
// same reason: Digested marshals this struct, so a field that serialised when
// absent would change the digest of every submission written before today and
// orphan every roster entry naming one. The schema string does not move.
//
// Two properties, and the first matters more than the second:
//
//  1. ABSENT CHANGES NOTHING. A corpus without them marshals to the bytes it
//     marshalled to yesterday, so everything already pooled stays poolable.
//  2. PRESENT IS DETERMINISTIC. encoding/json sorts map keys, so the same
//     histogram always produces the same bytes and therefore the same digest,
//     whatever order the caller built it in.

func baseCorpus() Corpus {
	return Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-17", Tasks: 5,
		TotalUSD: 1.5, RebilledUSD: 0.25, RebilledShare: 0.16, MedianTaskUSD: 0.3,
		PricedAt: "2026-09-01", RulesVersion: "openai-2026-09-15", Unpriced: 0,
		SourceTag: "launch-2026-09", TagBasis: "flag",
	}
}

func TestSurfacesAndModelsAreAbsentUnlessSet(t *testing.T) {
	b, err := json.Marshal(baseCorpus())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"surfaces"`, `"models"`} {
		if strings.Contains(string(b), key) {
			t.Errorf("%s serialised while unset, which changes the digest of every submission written before it:\n%s", key, b)
		}
	}
}

func TestSurfacesAndModelsSerialiseAfterProvenanceAndBeforeDigest(t *testing.T) {
	c := baseCorpus()
	c.Surfaces = []string{"codex"}
	c.Models = map[string]int{"gpt-6-astra": 22, "gpt-5.4": 298}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	iSurfaces := strings.Index(s, `"surfaces"`)
	iModels := strings.Index(s, `"models"`)
	iDigest := strings.Index(s, `"digest"`)
	if iSurfaces < 0 || iModels < 0 {
		t.Fatalf("surfaces or models did not serialise when set:\n%s", s)
	}
	// Field order is the digest. The receiver rebuilds these bytes from a fixed
	// list, so a field in a different place than that list expects hashes
	// differently and every submission carrying it is refused.
	if iSurfaces >= iModels || iModels >= iDigest {
		t.Errorf("order must be surfaces, then models, then digest; got %d, %d, %d:\n%s",
			iSurfaces, iModels, iDigest, s)
	}
}

func TestModelHistogramIsDeterministicWhateverOrderItWasBuiltIn(t *testing.T) {
	one := baseCorpus()
	one.Models = map[string]int{"gpt-6-astra": 22, "gpt-5.4": 298, "gpt-5.1-codex-mini": 594}
	two := baseCorpus()
	two.Models = map[string]int{"gpt-5.1-codex-mini": 594, "gpt-5.4": 298, "gpt-6-astra": 22}

	a, err := json.Marshal(one)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(two)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("the same histogram built in two orders produced two documents:\n%s\n%s", a, b)
	}
	// And sorted, because the receiver hashes the keys in sorted order rather
	// than trusting the order they arrived in.
	if i, j := strings.Index(string(a), "gpt-5.1"), strings.Index(string(a), "gpt-6"); i > j {
		t.Errorf("map keys are not sorted, so the receiver cannot reproduce the bytes:\n%s", a)
	}
}

func TestDigestMovesWhenTheSurfaceDoes(t *testing.T) {
	// Two corpora identical but for the surface must not share a digest: a pool
	// that cannot tell them apart is the thing these fields exist to prevent.
	claude := baseCorpus()
	claude.Surfaces = []string{"claude-code"}
	codex := baseCorpus()
	codex.Surfaces = []string{"codex"}
	if claude.Digested().Digest == codex.Digested().Digest {
		t.Error("a claude-code corpus and a codex corpus digested identically")
	}
	if baseCorpus().Digested().Digest == claude.Digested().Digest {
		t.Error("naming the surface did not change the digest")
	}
}
