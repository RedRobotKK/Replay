package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/advisor"
)

func costs(n int, f func(i int) float64) []float64 {
	xs := make([]float64, n)
	for i := range xs {
		xs[i] = f(i)
	}
	return xs
}

// The threshold has to arrive with its derivation, under a flag name that
// exists. A cap a user cannot check is one they take on faith, and every other
// number this tool prints can be checked.
//
// This test used to pin --spend-session-usd and --spend-session-tokens, which
// serve has never defined. The test agreed with the defect, so the suite was
// green while the tool recommended a command that fails. The names are now
// checked against the flag surface by
// internal/regression.TestEveryPrintedFlagIsDefined, which reads what the
// FlagSets define rather than what anyone wrote down twice.
func TestGuardAdvicePrintsItsDerivation(t *testing.T) {
	usd := costs(20, func(i int) float64 { return float64(i+1) * 0.5 })
	tok := costs(20, func(i int) float64 { return float64((i + 1) * 40_000) })
	got := strings.Join(guardAdviceLines(usd, tok), "\n")
	for _, want := range []string{"Q3", "IQR", "20 sessions", "--max-session-usd", "--max-session-tokens", "replay serve --max-session-usd"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

// The PRD's pass condition, at the command surface: an empty ledger and a
// three-session ledger both produce no threshold.
func TestGuardAdviceSaysNothingBelowTheFloor(t *testing.T) {
	for _, n := range []int{0, 3} {
		usd := costs(n, func(i int) float64 { return float64(i+1) * 0.5 })
		got := strings.Join(guardAdviceLines(usd, nil), "\n")
		if strings.Contains(got, "--max-session-usd") {
			t.Fatalf("%d sessions produced a cap:\n%s", n, got)
		}
		if !strings.Contains(got, "not enough") {
			t.Fatalf("%d sessions must say why there is no advice:\n%s", n, got)
		}
		if !strings.Contains(got, "10") {
			t.Fatalf("the refusal must name the floor so the user knows when to come back:\n%s", got)
		}
	}
}

// Print-only. This never becomes a written setting: a spend cap that a tool
// set for you is a refusal you did not choose.
func TestGuardAdviceIsNeverApplied(t *testing.T) {
	src := string(mustRead(t, "apply.go"))
	if strings.Contains(src, "spend-session-usd") || strings.Contains(src, "SpendSession") {
		t.Fatal("the apply path must not know how to write a spend cap")
	}
	if !strings.Contains(string(mustRead(t, "advise.go")), "guards") {
		t.Fatal("--guards must exist on advise")
	}
}

func TestGuardFloorMatchesTheAdvisor(t *testing.T) {
	if !strings.Contains(strings.Join(guardAdviceLines(nil, nil), "\n"), "10") {
		t.Fatalf("the printed floor must track advisor.MinGuardSessions (%d)", advisor.MinGuardSessions)
	}
}

// The assembled command carries only the caps the spread actually supports.
//
// Reported by guard-reachability as INERT: the `if okTok` arm ran and no test
// depended on whether it had. It is not dead code — it decides whether the
// printed invocation carries a token cap — so the fix is the assertion, not a
// deletion. A command that names a flag with no number behind it is one the
// reader pastes and the tool rejects.
func TestGuardAdviceCommandCarriesOnlyTheCapsItHas(t *testing.T) {
	usd := costs(20, func(i int) float64 { return float64(i+1) * 0.5 })
	tok := costs(20, func(i int) float64 { return float64((i + 1) * 40_000) })

	// Asserted on the assembled command LINE, not on the output as a whole.
	// Each cap also appears in the derivation block above the command, so a
	// whole-output check passes whether or not the command carries the flag —
	// which is exactly how this branch stayed inert while a test looked like
	// it covered it.
	cmdLine := func(usd, tok []float64) string {
		for _, l := range guardAdviceLines(usd, tok) {
			if strings.Contains(l, "replay serve") {
				return l
			}
		}
		return ""
	}

	both := cmdLine(usd, tok)
	if !strings.Contains(both, "--max-session-usd") || !strings.Contains(both, "--max-session-tokens") {
		t.Fatalf("the command does not carry both caps: %q", both)
	}

	// A spread with dollars and no token history prints one cap, not two.
	usdOnly := cmdLine(usd, nil)
	if !strings.Contains(usdOnly, "replay serve --max-session-usd") {
		t.Errorf("the dollar cap is missing from the command:\n%s", usdOnly)
	}
	if strings.Contains(usdOnly, "--max-session-tokens") {
		t.Errorf("a token cap was printed with no token spread behind it:\n%s", usdOnly)
	}

	// And the reverse, so the assertion cannot pass by the token arm never
	// running at all.
	tokOnly := cmdLine(nil, tok)
	if !strings.Contains(tokOnly, "--max-session-tokens") {
		t.Errorf("the token cap is missing from the command:\n%s", tokOnly)
	}
	if strings.Contains(tokOnly, "--max-session-usd") {
		t.Errorf("a dollar cap was printed with no dollar spread behind it:\n%s", tokOnly)
	}
}
