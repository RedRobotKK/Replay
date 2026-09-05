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

// The threshold has to arrive with its derivation. A cap a user cannot check
// is one they take on faith, and every other number this tool prints can be
// checked.
func TestGuardAdvicePrintsItsDerivation(t *testing.T) {
	usd := costs(20, func(i int) float64 { return float64(i+1) * 0.5 })
	tok := costs(20, func(i int) float64 { return float64((i + 1) * 40_000) })
	got := strings.Join(guardAdviceLines(usd, tok), "\n")
	for _, want := range []string{"Q3", "IQR", "20 sessions", "--spend-session-usd", "--spend-session-tokens"} {
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
		if strings.Contains(got, "--spend-session-usd") {
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
