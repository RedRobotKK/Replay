package analysis

import (
	"math"
	"testing"
)

// Conformance for the surfaces that are statistical rather than deterministic.
//
// The wire-parsing suites assert values: 9630 inclusive of 9600 cached leaves
// 30 fresh, and anything else is a defect. Nothing here can be asserted that
// way. Sigma, the byte-to-token fit and the spend fence are estimates over
// samples, and their correct output changes with the sample.
//
// So the pass/fail conditions are about the DECISION PROCEDURE, not the number:
// when it must refuse, what it must never do regardless of the draw, and how it
// behaves as evidence accumulates. Those are as falsifiable as an equality and
// they are what actually protects a user, because the failure mode of a
// statistical surface is not a wrong digit — it is a confident answer built on
// nothing.
//
//	P1  refusal is monotone in evidence: below the gate it refuses, at and
//	    above it answers, never the reverse
//	P2  a refusal says what was missing, in numbers
//	P3  degenerate input refuses rather than emitting a confident point
//	P4  the same input gives the same answer, every time
//	P5  the estimate tracks a known ground truth on synthetic data
//	P6  uncertainty grows as the sample shrinks; it never shrinks to zero
//	P7  no output is silently zero — refusal and "measured as none" are
//	    distinguishable by the caller

// ---------------------------------------------------------------- dilation

// P1 and P2 for sigma. It is a ratio of two fitted ratios, so it is only an
// observation when both sides are, and the refusal has to say which side was
// short and by how much.
func TestDilationRefusesAndSaysWhy(t *testing.T) {
	good := TokenFit{TokensPerByte: 0.25, RelativeError: 0.05, Turns: MinDilationTurns}

	cases := []struct {
		name string
		fits map[string]TokenFit
		want string // a substring the refusal must contain
	}{
		{
			name: "the target model was never seen",
			fits: map[string]TokenFit{"from": good},
			want: "no turns on the wire",
		},
		{
			name: "one turn short of the gate",
			fits: map[string]TokenFit{
				"from": good,
				"to":   {TokensPerByte: 0.3, Turns: MinDilationTurns - 1},
			},
			want: "of 10 turns",
		},
		{
			name: "turns but no fitted ratio",
			fits: map[string]TokenFit{
				"from": good,
				"to":   {TokensPerByte: 0, Turns: 40},
			},
			want: "no fitted tokens-per-byte",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := MeasureDilation("from", "to", c.fits)
			if d.Measured {
				t.Fatalf("P1 measured sigma %.4f from insufficient evidence", d.Sigma)
			}
			if d.Sigma != 0 {
				t.Errorf("P7 a refusal carried a sigma of %.4f; a caller reading the "+
					"number without checking Measured would price against it", d.Sigma)
			}
			if !contains(d.Why, c.want) {
				t.Errorf("P2 the refusal does not say what was missing.\n  got:  %q\n  want substring: %q",
					d.Why, c.want)
			}
		})
	}
}

// P1. Exactly at the gate it must answer, or the gate is off by one and every
// borderline comparison is silently declined.
func TestDilationAnswersExactlyAtTheGate(t *testing.T) {
	fits := map[string]TokenFit{
		"a": {TokensPerByte: 0.20, RelativeError: 0.04, Turns: MinDilationTurns},
		"b": {TokensPerByte: 0.25, RelativeError: 0.03, Turns: MinDilationTurns},
	}
	d := MeasureDilation("a", "b", fits)
	if !d.Measured {
		t.Fatalf("P1 refused at exactly the gate (%d turns): %s", MinDilationTurns, d.Why)
	}
	if math.Abs(d.Sigma-1.25) > 1e-9 {
		t.Errorf("P5 sigma: got %.6f, want 1.25 (0.25 / 0.20)", d.Sigma)
	}
	// P6. Uncertainty compounds across the two sides; it cannot come out
	// smaller than either input.
	if d.RelativeError < 0.04 {
		t.Errorf("P6 combined relative error %.4f is below the larger input (0.04). "+
			"Combining two uncertain numbers cannot produce a more certain one",
			d.RelativeError)
	}
}

// P4. Same inputs, same sigma, every time.
func TestDilationIsDeterministic(t *testing.T) {
	fits := map[string]TokenFit{
		"a": {TokensPerByte: 0.211, RelativeError: 0.071, Turns: 44},
		"b": {TokensPerByte: 0.263, RelativeError: 0.052, Turns: 51},
	}
	first := MeasureDilation("a", "b", fits)
	for i := 0; i < 40; i++ {
		got := MeasureDilation("a", "b", fits)
		if got != first {
			t.Fatalf("P4 run %d differed: %+v then %+v", i, first, got)
		}
	}
}

func contains(hay, needle string) bool {
	return len(needle) > 0 && len(hay) >= len(needle) && indexOf(hay, needle) >= 0
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
