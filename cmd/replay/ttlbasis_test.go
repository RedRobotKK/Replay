package main

// Guard-reachability contract tests for the TTL-basis helper functions
// (cost.go:246,249 and foldTTLBasis). These are pure functions whose three
// outcomes (empty / assumed_short / measured) are the user-visible wire value,
// so each branch must be behaviourally pinned.

import "testing"

func TestWriteTTLBasisString(t *testing.T) {
	cases := []struct {
		usd     float64
		assumed int
		want    string
	}{
		{0, 0, ""},
		{-1, 0, ""},
		{1.0, 1, "assumed_short"},
		{1.0, 0, "measured"},
	}
	for _, tc := range cases {
		if got := writeTTLBasisString(tc.usd, tc.assumed); got != tc.want {
			t.Fatalf("writeTTLBasisString(%v, %d) = %q, want %q", tc.usd, tc.assumed, got, tc.want)
		}
	}
}
