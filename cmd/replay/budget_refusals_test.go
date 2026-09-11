package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Four refusals guard `replay budget`, and mutation says three of them were not
// doing what the suite implied.
//
//	R0  transcriptFiles err != nil    the argument holds no .jsonl, or is not there
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
// R0 was not in this list until 2026-09-10, and that is the finding worth
// keeping. It is not in budgetRefusal, so a note that enumerated budgetRefusal
// enumerated three of four and read as complete. It was found by
// scripts/refusal-reachability, which selects refusals by what they SAY rather
// than by which function holds them, and reported R0 SURVIVED. BG7 closes it.
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
// assigns DefaultTokensPerByte when sumBytes is zero, and otherwise divides two
// quantities that are both positive by construction, since a sample is only
// recorded when newTokens > 0. budget.go reaches R3 only when sawLedger, and
// fit then came from Fit. See TestFitAlwaysReturnsAPositiveRatio in
// internal/analysis, which pins the postcondition R3 depends on, so that if the
// contract ever changes somebody learns R3 has become live code.

// BG6: R1 is reached, and answers with its own reason.
//
// The fixture matters and the first version of this test got it wrong. An empty
// directory never reaches R1 at all: R0 refuses first, when the walk finds no
// .jsonl at all, and its message is nearly identical — both begin "NOT
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

	// R1, not R0, which appends the walk error in parentheses.
	if !strings.Contains(msg, "no ledger found under") {
		t.Errorf("R1 did not answer: %v", err)
	}
	if strings.Contains(msg, "no .jsonl transcripts found") {
		t.Fatalf("R0 answered instead, so this fixture never reaches R1 and the test "+
			"cannot see it: %v", err)
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

// BG7: the walk refusal answers for a directory with nothing in it, in its own
// words.
//
// A fourth refusal guards `replay budget` and the note above missed it, because
// it is not in budgetRefusal at all — it is the `if err != nil` on
// transcriptFiles in runBudget, which fires when the argument holds no .jsonl
// or does not exist. scripts/refusal-reachability reported it SURVIVED: no test
// reached it. BG6 cannot, by construction — its fixture writes a .jsonl
// precisely so the walk succeeds and R1 gets a turn.
//
// Neutralising it is silent rather than loud, which is why it needs its own
// test. transcriptFiles returns (nil, err); ignore the error and the nil file
// list walks zero sessions, so sawAnyReq is false and sessions is 0, and R1
// answers with "NOT MEASURED: no ledger found under <dir>." — the same opening
// words, the same tier, and no mention of what actually went wrong. A reader
// pointed at a typo'd path would be told their corpus was empty.
//
// The two are told apart by the wrapped cause. The walk refusal appends the
// underlying error in parentheses; R1 has nothing to append.
func TestBG7_TheWalkRefusalIsReachedAndDistinguishable(t *testing.T) {
	t.Run("a directory with no transcripts in it", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := runBudget([]string{t.TempDir(), "--json"}, &stdout, &stderr)
		if err == nil {
			t.Fatal("an empty directory produced a budget")
		}
		// The walk's own cause, wrapped. R1 does not carry it.
		if !strings.Contains(err.Error(), "no .jsonl transcripts found") {
			t.Errorf("the walk refusal did not answer, so it is unreached: %v", err)
		}
		if strings.Contains(err.Error(), "carrying tool definitions") {
			t.Errorf("a refusal about sessions answered for a corpus with no files: %v", err)
		}
		if out := stdout.String(); strings.Contains(out, "\"standing\"") {
			t.Errorf("standing costs were printed alongside the refusal:\n%s", out)
		}
	})

	t.Run("a path that is not there", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		missing := filepath.Join(t.TempDir(), "no-such-ledger")
		err := runBudget([]string{missing, "--json"}, &stdout, &stderr)
		if err == nil {
			t.Fatal("a path that does not exist produced a budget")
		}
		// The distinction that earns this guard its parenthetical: a reader
		// who typo'd a path is told the path is wrong, not that their corpus
		// is empty. Without the guard R1 answers and says the latter.
		//
		// Asserted on the tool's own words, not the operating system's. This
		// first read `no such file or directory`, which is what Unix says and
		// Windows does not — there it is "The system cannot find the file
		// specified", and the test failed on a refusal that had worked
		// perfectly. A test whose subject is "the path was not there" must
		// assert on the sentence this program wrote, or it is testing libc.
		// Both budget refusals open with "no ledger found under", so that
		// phrase alone would pass whichever one answered — the vacuous shape
		// this branch exists to remove. Only the walk refusal parenthesises
		// the underlying cause after the path, so that is what distinguishes
		// it, and it distinguishes without quoting any operating system.
		if !strings.Contains(err.Error(), missing+" (") {
			t.Errorf("a missing path was not reported as missing, so the reader is sent to "+
				"look at their corpus instead of their argument: %v", err)
		}
		if !strings.Contains(err.Error(), missing) {
			t.Errorf("the refusal does not name the path it could not read: %v", err)
		}
	})
}
