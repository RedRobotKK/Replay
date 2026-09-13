package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// BP. The budget artefact did not say which binary computed it.
//
// A budget file is committed to a repository and read back months later by a
// gate that will fail somebody's build. Between those two moments the binary
// changes. `standing.tokens_per_request` is a count this code produced from a
// corpus, and what counts as a tool token, how the system prompt is measured
// and which requests are admissible are all decisions in this tree rather than
// facts about the world.
//
// So two builds can read one corpus and write different standing costs, and
// the committed file had no way to say which one wrote it. That is defect #284
// in a different unit: two builds priced one corpus at $4,088.49 and
// $11,969.37 under a single rules label, and v0.6.0 fixed it for corpus
// submissions by recording the build.
//
// WHY THERE IS NO PRICING DIGEST HERE, WHICH IS NOT AN OVERSIGHT. A review
// recommended adding `pricingDigest` alongside, by analogy with the corpus
// submission. The analogy does not hold: budget.go contains no dollar, no
// price and no USD, and the artefact is token counts throughout. A pricing
// digest on a file that prices nothing would be provenance for a computation
// that never happened, which is the kind of figure this repository exists to
// refuse. If a gate ever compares dollars, the digest becomes necessary in the
// same commit and not before.

// BP1: a written budget names the build that produced it.
//
// PASS: binary_version and commit are present and non-empty.
// FAIL: a committed file that a gate will act on cannot say which code wrote
// it, and two builds disagreeing look identical to the reader.
func TestBP1_TheArtefactNamesTheBuildThatWroteIt(t *testing.T) {
	b, err := json.Marshal(newBudgetFile(standing{TokensPerRequest: 100}, nil, measured{Sessions: 1, Requests: 10, Source: "x"}))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"binary_version", "commit"} {
		v, ok := m[key]
		if !ok {
			t.Errorf("the budget artefact has no %q.\nA gate reading this months from now "+
				"cannot tell whether the build that wrote it counted tool tokens the same "+
				"way the build reading it does.", key)
			continue
		}
		if s, _ := v.(string); strings.TrimSpace(s) == "" {
			t.Errorf("%q is empty, which is the same as absent to a reader", key)
		}
	}
}

// BP2: the schema moved, because the shape did.
//
// Unlike the corpus submission, nothing reads a budget file yet: the gate does
// not exist. So there is no installed base to keep poolable and no reason to
// make the new fields optional, and a reader that one day meets schema 1 should
// be able to say precisely what it is missing rather than guess.
func TestBP2_TheSchemaMovedWithTheShape(t *testing.T) {
	if BudgetSchema < 2 {
		t.Errorf("BudgetSchema is %d. The artefact gained required provenance fields, so a "+
			"file written before them is a different shape, and a gate must be able to tell.",
			BudgetSchema)
	}
}

// BP3: provenance travels with the figure it describes.
//
// The existing `measured` block records the corpus. This records the code. Both
// are needed and neither substitutes: the same corpus read by two builds is the
// case that motivated this, and the same build over two corpora is the case
// `measured` already covered.
func TestBP3_BothProvenanceBlocksSurvive(t *testing.T) {
	b, _ := json.Marshal(newBudgetFile(standing{TokensPerRequest: 1}, nil, measured{Sessions: 2, Requests: 3, Source: "s"}))
	var m map[string]any
	_ = json.Unmarshal(b, &m)

	if _, ok := m["measured"]; !ok {
		t.Error("the corpus provenance block is gone. Recording the code is not a substitute " +
			"for recording what it read.")
	}
	if _, ok := m["binary_version"]; !ok {
		t.Error("the build provenance is gone")
	}
}
