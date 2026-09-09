package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
)

// The three refusals, callable directly.
//
// ADR-0014 practice 2: "Decisions live in pure functions; shells are
// addresses... This is not tidiness: the untestable shape WAS the
// vulnerability."
//
// budget.go proved that twice in one afternoon. R1 could only be reached
// through a corpus contrived to walk-but-yield-nothing, and until that fixture
// existed the test that looked like it covered R1 was exercising budget.go:112
// instead — a different refusal that opens with the same words. R3 could not be
// reached at all: it declines when analysis.Fit returns a non-positive ratio,
// and Fit cannot.
//
// Neither is a missing assertion. Both are reachability through a shell, and
// the shell is what this removes. budgetRefusal takes the four facts the
// decision actually depends on and returns the refusal or nil, so each branch
// is one call away and no branch can shadow another.
//
// R3 becomes testable here for the first time. Nothing in production can hand
// it a zero ratio; a test can, which is the point — the guard is defence in
// depth against Fit's contract changing, and defence nobody can exercise is
// decoration. FE7 in internal/analysis pins the contract; this pins the
// response to its violation.

// BR1: no requests and no sessions.
func TestBR1_NoCorpusAtAll(t *testing.T) {
	err := budgetRefusal(false, false, 0, analysis.TokenFit{TokensPerByte: 0.25}, []string{"/tmp/x"})
	if err == nil {
		t.Fatal("a corpus with no requests and no sessions was accepted")
	}
	if !strings.Contains(err.Error(), "no ledger found under") {
		t.Errorf("R1 did not answer: %v", err)
	}
}

// BR2: sessions were read, none from a ledger.
//
// Distinct from BR1 and the pair the shell could not tell apart. A transcript
// records what was called, never what was offered.
func TestBR2_SessionsButNoLedger(t *testing.T) {
	err := budgetRefusal(true, false, 7, analysis.TokenFit{TokensPerByte: 0.25}, []string{"/tmp/x"})
	if err == nil {
		t.Fatal("7 sessions with no ledger produced a budget")
	}
	if !strings.Contains(err.Error(), "carrying tool definitions") {
		t.Errorf("R2 did not answer: %v", err)
	}
	if strings.Contains(err.Error(), "no ledger found under") {
		t.Errorf("R1 answered for a corpus that has sessions: %v", err)
	}
}

// BR3: a ledger, and a ratio that cannot convert.
//
// The branch production cannot reach. It is exercised here because a guard
// nobody can exercise is decoration, and because the day Fit's postcondition
// changes this is what says the response is still right.
func TestBR3_ALedgerWithNoUsableFit(t *testing.T) {
	err := budgetRefusal(true, true, 3, analysis.TokenFit{TokensPerByte: 0}, []string{"/tmp/x"})
	if err == nil {
		t.Fatal("a zero tokens-per-byte ratio produced a budget; every standing figure in " +
			"it would be zero and presented as measured")
	}
	if !strings.Contains(err.Error(), "byte-to-token fit") {
		t.Errorf("R3 did not answer: %v", err)
	}
	// Negative too: a ratio below zero is not merely unusable, it would make
	// the standing cost negative.
	if err := budgetRefusal(true, true, 3, analysis.TokenFit{TokensPerByte: -1}, nil); err == nil {
		t.Error("a negative ratio produced a budget")
	}
}

// BR4: a real corpus is not refused.
//
// Three refusal tests alone are satisfied by a function that refuses
// everything, which is the failure mode a decision function invites.
func TestBR4_AUsableCorpusIsAccepted(t *testing.T) {
	if err := budgetRefusal(true, true, 3, analysis.TokenFit{TokensPerByte: 0.25}, nil); err != nil {
		t.Errorf("a ledger with a usable fit was refused: %v", err)
	}
}

// BR5: every refusal carries the NOT MEASURED tier.
//
// The tier is what a reader and a CI gate key on. A refusal that omitted it
// would read as an ordinary failure and be retried.
func TestBR5_EveryRefusalCarriesTheTier(t *testing.T) {
	for name, err := range map[string]error{
		"R1": budgetRefusal(false, false, 0, analysis.TokenFit{TokensPerByte: 0.25}, nil),
		"R2": budgetRefusal(true, false, 7, analysis.TokenFit{TokensPerByte: 0.25}, nil),
		"R3": budgetRefusal(true, true, 3, analysis.TokenFit{TokensPerByte: 0}, nil),
	} {
		if err == nil {
			t.Errorf("%s did not refuse", name)
			continue
		}
		if !strings.Contains(err.Error(), "NOT MEASURED") {
			t.Errorf("%s omits the NOT MEASURED tier: %v", name, err)
		}
		if !strings.Contains(err.Error(), "invalid usage") {
			t.Errorf("%s does not wrap errUsage, so the exit code will be wrong: %v", name, err)
		}
	}
}
