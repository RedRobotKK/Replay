package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Three refusals guard `replay budget`, and mutation says two of them were not
// doing what the suite implied.
//
//	R1  !sawAnyReq && sessions == 0   no ledger anywhere
//	R2  !sawLedger                    sessions read, none from a ledger
//	R3  fit.TokensPerByte <= 0        definitions exist, cannot be converted
//
// Measured by disabling each in turn and running the whole suite:
//
//	R1 -> SURVIVED
//	R2 -> caught
//	R3 -> SURVIVED
//
// R1 surviving is the interesting one, because TestBG2 already drives an empty
// directory and asserts a NOT MEASURED refusal comes back. It passes with R1
// disabled — R2 fires instead and also says NOT MEASURED. The test cannot tell
// the two apart, so it was covering for the guard it looked like it tested.
// ADR-0014 lists this shape: a guard shadowed by another, where disabling it
// changes nothing observable.
//
// BG6 fixes that by asserting on R1's own reason rather than on the tier it
// shares with R2.
//
// R3 is a different finding and is NOT closed by a test here: it is
// UNREACHABLE. analysis.Fit cannot return a non-positive TokensPerByte — it
// assigns defaultTokensPerByte when sumBytes is zero, and otherwise divides two
// quantities that are both positive by construction, since a sample is only
// recorded when newTokens > 0. budget.go reaches R3 only when sawLedger, and
// fit then came from Fit. See TestFitAlwaysReturnsAPositiveRatio in
// internal/analysis, which pins the postcondition R3 depends on, so that if the
// contract ever changes somebody learns R3 has become live code.

// BG6: R1 is reached, and answers with its own reason.
//
// The fixture matters and the first version of this test got it wrong. An empty
// directory never reaches R1 at all: budget.go:112 refuses first, when the walk
// finds no .jsonl at all, and its message is nearly identical — both begin "NOT
// MEASURED: no ledger found under". So the first BG6 passed with R1 disabled,
// which is the very failure it was written to catch, and I had to watch it pass
// under mutation to discover that.
//
// Reaching R1 needs a corpus the walker opens and that yields no requests and
// no sessions. A .jsonl carrying a line that is not a session record does it.
// The two messages are then told apart by :112's parenthetical.
func TestBG6_TheNoLedgerRefusalIsReachedAndDistinguishable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "empty.jsonl"),
		[]byte("{\"type\":\"not-a-session\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := runBudget([]string{dir, "--json"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a corpus with no requests produced a budget")
	}
	msg := err.Error()

	// R1, not the walk refusal at :112, which appends the walk error in
	// parentheses.
	if !strings.Contains(msg, "no ledger found under") {
		t.Errorf("R1 did not answer: %v", err)
	}
	if strings.Contains(msg, "no .jsonl transcripts found") {
		t.Fatalf("the walk refusal at budget.go:112 answered instead, so this fixture "+
			"never reaches R1 and the test cannot see it: %v", err)
	}
	// R2's words. If R1 is disabled this is what comes back, and asserting only
	// on the shared prefix accepts it.
	if strings.Contains(msg, "carrying tool definitions") {
		t.Errorf("the sessions-without-a-ledger guard answered for a corpus with no "+
			"sessions, so R1 is not firing: %v", err)
	}
	if out := stdout.String(); strings.Contains(out, "\"standing\"") {
		t.Errorf("standing costs were printed alongside the refusal:\n%s", out)
	}
}
