package transcript

import (
	"os"
	"strings"
	"testing"
)

// Phase 5 governance invariants for the TTL cache-behavior study's new
// provenance machinery (client-version provenance and repository identity).
// Each test here names the invariant directly, separate from the feature
// HARDEN tests in clientversionprovenance_test.go and repository_test.go,
// so a reader auditing the governance boundary finds them without having to
// infer intent from a feature test's assertions.

// (b) A client-version transition is recorded, never silently normalized
// away to the first version seen.
func TestGovernanceClientVersionTransitionIsNeverNormalizedAway(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(twoVersionLines("2.1.278", "2.1.291")))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ClientVersions) < 2 {
		t.Fatalf("a two-version session recorded %d versions, want both kept", len(s.ClientVersions))
	}
	if s.ClientVersionProvenance() != ClientVersionStraddling {
		t.Fatal("a two-version session must classify as straddling, never as single")
	}
}

// (d) Repository identity is recorded from the transcript's own file
// location, never inferred retrospectively from lanes, requests, or usage.
func TestGovernanceRepositoryIdentityIsNeverInferredFromSessionContent(t *testing.T) {
	root := t.TempDir()
	pathA := writeTranscript(t, root+"/repoA", "same.jsonl")
	pathB := writeTranscript(t, root+"/repoB", "same.jsonl")
	sa, err := ParseClaudeCodeFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := ParseClaudeCodeFile(pathB)
	if err != nil {
		t.Fatal(err)
	}
	// Byte-identical transcripts (so identical lanes, requests and usage)
	// under different directories must still diverge in RepositoryID: the
	// field tracks Path, never the parsed content.
	if sa.RepositoryID == sb.RepositoryID {
		t.Fatal("RepositoryID must come from the file's location, not from anything parsed out of it")
	}
}

// (e) Provenance is deterministic: the same input produces the same output
// on repeated computation, for client versions, their provenance
// classification, and repository identity alike.
func TestGovernanceProvenanceIsDeterministic(t *testing.T) {
	input := twoVersionLines("2.1.278", "2.1.291")
	s1, err := ParseClaudeCode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := ParseClaudeCode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(s1.ClientVersions) != len(s2.ClientVersions) {
		t.Fatalf("ClientVersions not reproducible: %v vs %v", s1.ClientVersions, s2.ClientVersions)
	}
	for i := range s1.ClientVersions {
		if s1.ClientVersions[i] != s2.ClientVersions[i] {
			t.Fatalf("ClientVersions not reproducible: %v vs %v", s1.ClientVersions, s2.ClientVersions)
		}
	}
	if s1.ClientVersionProvenance() != s2.ClientVersionProvenance() {
		t.Fatal("ClientVersionProvenance not reproducible")
	}
	path := writeTranscript(t, t.TempDir()+"/repo", "a.jsonl")
	r1, err := ParseClaudeCodeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := ParseClaudeCodeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if r1.RepositoryID != r2.RepositoryID {
		t.Fatalf("RepositoryID not reproducible: %q vs %q", r1.RepositoryID, r2.RepositoryID)
	}
}

// (f) None of the new provenance logic inspects any cost, token, or
// cache-behavior value. Written as a static check over the exact files that
// implement it, because the stronger guarantee here is structural: these
// files never import or reference the Usage type or its fields, so no
// runtime test could exercise a branch that does not exist.
func TestGovernanceProvenanceLogicNeverReadsUsageFields(t *testing.T) {
	forbidden := []string{
		"Usage", "CacheRead", "CacheCreation", "PromptTotal", "ephemeral_", "ThinkingTokens",
	}
	files := []string{"clientversion.go", "repository.go"}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, word := range forbidden {
			if strings.Contains(src, word) {
				t.Errorf("%s references %q: provenance logic must never read a cost/token/cache field", f, word)
			}
		}
	}
}
