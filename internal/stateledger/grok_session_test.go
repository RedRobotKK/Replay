package stateledger

import (
	"os"
	"strings"
	"testing"
)

// The Grok usage-source investigation, as it actually happened.
//
// Every string below is an observation recorded in
// docs/evidence/grok-source-contract-2026-09-27.md. Nothing is illustrative.
// This is the canonical example the representation was designed against, and
// it is here to show the shape generalises, not to be special-cased anywhere
// in the package: the package has no knowledge of Grok.
//
// What makes it the right example is that it ends with a live open question.
// The investigation established that a belief was wrong and did NOT establish
// what is true instead, which is the ordinary outcome of real investigation
// and the one an ordinary trace cannot express.
func TestGrokUsageSourceInvestigation(t *testing.T) {
	l := New("grok usage-source investigation, 2026-09-27")

	c1 := l.Claim("updates.jsonl may be the authoritative record of Grok usage",
		"inherited: the historical reader at 6074075 sums updates.jsonl")

	// Evidence that raised the question, but could not answer it.
	l.Check(c1, Locates,
		"count sessions carrying each artifact",
		"48 sessions: 35 have both, 13 have updates.jsonl and no usage.json",
		Inconclusive)
	l.Check(c1, Executable,
		"sum inputTokens across updates.jsonl in the usage.json-less sessions",
		"581,144,282 in the largest session alone; 581,882,542 across the 13",
		Inconclusive)

	// The check that could settle it.
	k := l.Check(c1, Dispositive,
		"grok usage <session-with-no-usage.json>",
		"Error: No usage recorded",
		Contradicts)
	if err := l.Settle(c1, k); err != nil {
		t.Fatalf("the dispositive check was refused: %v", err)
	}
	repl := "usage.json is the record grok usage reads; grok usage succeeded on 35 of 35 sessions carrying one and failed on all 13 without"
	l.Revise(c1, k, Contradicted, &repl)

	// A second claim from the same investigation that is contradicted with NO
	// replacement, which is the case the ledger exists to represent honestly.
	c2 := l.Claim("where both artifacts exist they agree, so either may be read",
		"inferred from 34 of 35 sessions agreeing")
	k2 := l.Check(c2, Dispositive,
		"compare usage.json turns against updates.jsonl records in session 01a09207",
		"131 records on both sides, 18 turns differ, from turn 124 the updates value is the previous turn's usage value",
		Contradicts)
	l.Revise(c2, k2, Contradicted, nil)
	l.Open(c2, "what causes the one-turn lag is NOT_MEASURED: compaction, a partial write and a different quantity are all consistent with what is on disk")
	l.Open(c2, "whether usage.json can itself truncate or reset is not observed and not proven absent")

	got := l.Render()

	// The two properties a viewer must be able to read off the artifact.
	for _, want := range []string{
		"CONTRADICTED",             // why it changed its mind
		"UNRESOLVED",               // what it still does not know
		"grok usage",               // the check that did the work
		"Error: No usage recorded", // the evidence
		"NOT_MEASURED",             // the honest gap
		"DISPOSITIVE CHECK",        // and that the check could settle it
	} {
		if !strings.Contains(got, want) {
			t.Errorf("artifact omits %q", want)
		}
	}
	// A contradiction with no replacement must not acquire one.
	if l.ClaimByID(c2).Replacement != nil {
		t.Error("claim 2 gained a replacement it never established")
	}

	const golden = "testdata/grok-usage-source.txt"
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v (run with UPDATE_GOLDEN=1 to create)", err)
	}
	if string(want) != got {
		t.Errorf("rendering drifted from the golden artifact\n--- got ---\n%s", got)
	}
}
