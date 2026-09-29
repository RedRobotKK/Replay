package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/observation"
)

// The publication boundary, end to end.
//
// `replay pool` is where submitted files become a public document: it reads
// each corpus file a maintainer was handed, decodes it, and emits a roster
// anyone can download. That decode is the earliest point at which the complete
// submitted object exists, and it is therefore the only place the boundary can
// be enforced. Deleting a field after decoding would be too late in the way
// that matters: the field was in the payload, and the payload is what a
// contributor published.
//
// The invariant these tests hold is
//
//	forbidden input -> rejected -> absent from the published document
//
// and NOT
//
//	forbidden input -> decoded -> field quietly disappears
//
// The difference is observable: in the second, `replay pool` prints a roster
// that silently includes a submission that carried a prompt.

// poolCorpus writes a valid submission and returns its path.
func poolCorpus(t *testing.T, dir, tag string, extra map[string]any) string {
	t.Helper()
	c := observation.Corpus{
		Schema: observation.CorpusSchema, TakenAt: "2026-09-29T18:00:00Z",
		Tasks: 3, TotalUSD: 1.5, RebilledUSD: 0.1, RebilledShare: 6.67,
		MedianTaskUSD: 0.5, PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		SourceTag: tag, TagBasis: "local",
	}.Digested()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) > 0 {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		for k, v := range extra {
			m[k] = v
		}
		if b, err = json.Marshal(m); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "replay-corpus-"+tag+".json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// PB1: a submission carrying a forbidden key never reaches the published pool.
//
// Two files are pooled: one clean, one carrying a prompt. The clean one must
// appear; the tainted one must be refused BY NAME on stderr and must contribute
// nothing to the document, neither a roster row nor its money.
func TestPB1_ATaintedSubmissionIsRefusedAndIsNotPublished(t *testing.T) {
	dir := t.TempDir()
	poolCorpus(t, dir, "cleancleanclean00", nil)
	poolCorpus(t, dir, "taintedtainted01", map[string]any{
		"prompt": "REFACTOR THE PARSER, and here is our proprietary source",
	})

	var stdout, stderr bytes.Buffer
	err := runPool([]string{"--json", filepath.Join(dir, "replay-corpus-cleancleanclean00.json"),
		filepath.Join(dir, "replay-corpus-taintedtainted01.json")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("pooling the clean submission failed: %v\n%s", err, stderr.String())
	}

	// Refused, by name, where an operator will see it.
	if !strings.Contains(stderr.String(), "prompt") {
		t.Errorf("the refusal does not name the offending key, so an operator cannot "+
			"tell the contributor what to remove:\n%s", stderr.String())
	}

	published := stdout.String()
	// The payload must not survive anywhere in the published document.
	for _, forbidden := range []string{"REFACTOR THE PARSER", "proprietary", "prompt"} {
		if strings.Contains(published, forbidden) {
			t.Errorf("%q reached the published pool document:\n%s", forbidden, published)
		}
	}
	// And the tainted submission must not be counted, which is the half a
	// field-deleting fix would get wrong: the prompt would be gone and the
	// money would be pooled anyway.
	if strings.Contains(published, "taintedtainted01") {
		t.Errorf("the tainted submission has a roster row:\n%s", published)
	}
	if !strings.Contains(published, "cleancleanclean00") {
		t.Errorf("the clean submission was dropped along with the tainted one:\n%s", published)
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("the published document is not JSON: %v", err)
	}
	roster, _ := doc["roster"].([]any)
	if len(roster) != 1 {
		t.Errorf("the roster has %d entries, want 1: only the clean submission is "+
			"admitted, and the tainted one contributes nothing", len(roster))
	}
}

// PB2: the positive control. A submission carrying EVERY supported field pools.
//
// This is what proves the strictness is enforcing the schema rather than
// rejecting legitimate submissions. Without it, a guard that refused everything
// would pass PB1 and look like a success.
func TestPB2_ASubmissionCarryingEverySupportedFieldIsAdmitted(t *testing.T) {
	breaks, reReads := 4, 2
	errShare := 1.25
	c := observation.Corpus{
		Schema: observation.CorpusSchema, TakenAt: "2026-09-29T18:00:00Z",
		Tasks: 3, TotalUSD: 1.5, RebilledUSD: 0.1, RebilledShare: 6.67,
		MedianTaskUSD: 0.5, PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		Unpriced: 1, CacheBreaks: &breaks, ReReads: &reReads, ErrorShare: &errShare,
		SourceTag: "everyfieldset0001", TagBasis: "local",
		BinaryVersion: "v0.6.2", Commit: "abc1234", PricingDigest: "p98fe33932e65",
	}.Digested()

	// Every field of the struct is set, so this fixture cannot pass by omitting
	// the ones a stricter reader would object to.
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != len(corpusFieldNames(t)) {
		t.Fatalf("the fixture serialises %d keys and Corpus declares %d; a positive "+
			"control that omits a field does not control for it", len(m), len(corpusFieldNames(t)))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "replay-corpus-full.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := runPool([]string{"--json", path}, &stdout, &stderr); err != nil {
		t.Fatalf("a submission carrying every supported field was refused: %v\n%s",
			err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("a fully populated submission produced a complaint:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "everyfieldset0001") {
		t.Errorf("the submission is not in the published roster:\n%s", stdout.String())
	}
}

// corpusFieldNames returns every JSON key Corpus declares, so PB2 can assert
// its fixture is complete rather than merely large.
func corpusFieldNames(t *testing.T) []string {
	t.Helper()
	breaks, reReads := 0, 0
	errShare := 0.0
	b, err := json.Marshal(observation.Corpus{
		CacheBreaks: &breaks, ReReads: &reReads, ErrorShare: &errShare,
		BinaryVersion: "x", Commit: "x", PricingDigest: "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
