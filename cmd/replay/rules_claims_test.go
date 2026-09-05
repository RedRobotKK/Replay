package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

func p(n int) *int { return &n }

// A claim nobody prints is a claim nobody checks. The whole point of carrying
// documented and observed is that a reader can see where they disagree.
func TestRulesPrintsTheVerdictOnEveryClaim(t *testing.T) {
	got := strings.Join(claimLines([]cachemodel.ModelRule{
		{Match: "opus-5", MinPrefix: 512, MinPrefixClaim: &cachemodel.Claim{
			Documented: 512,
			Observed:   &cachemodel.Observation{UpperBound: p(40563), Sessions: 11, Machines: 1},
		}},
		{Match: "haiku-4-5", MinPrefix: 4096, MinPrefixClaim: &cachemodel.Claim{
			Documented: 4096,
			Observed:   &cachemodel.Observation{UpperBound: p(1000), Sessions: 3, Machines: 2},
		}},
		{Match: "unmeasured", MinPrefix: 1024},
	}), "\n")

	if !strings.Contains(got, "unverified") {
		t.Fatalf("one-sided evidence must read unverified:\n%s", got)
	}
	if !strings.Contains(got, "contradicted") {
		t.Fatalf("a documented minimum above everything ever cached is contradicted:\n%s", got)
	}
	if !strings.Contains(got, "40563") || !strings.Contains(got, "11 sessions") {
		t.Fatalf("the evidence behind a verdict must be printed with it:\n%s", got)
	}
	if !strings.Contains(got, "untested") {
		t.Fatalf("a row with no claim must say untested, not go silent:\n%s", got)
	}
}

// The contradiction is the finding. It has to be impossible to miss in a wall
// of otherwise unremarkable rows.
func TestAContradictionIsCalledOut(t *testing.T) {
	got := strings.Join(claimLines([]cachemodel.ModelRule{
		{Match: "m", MinPrefix: 4096, MinPrefixClaim: &cachemodel.Claim{
			Documented: 4096,
			Observed:   &cachemodel.Observation{UpperBound: p(1000), Sessions: 3},
		}},
	}), "\n")
	if !strings.Contains(got, "CONTRADICTED") {
		t.Fatalf("a refuted provider figure must be shouted, not listed:\n%s", got)
	}
}
