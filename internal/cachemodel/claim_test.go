package cachemodel

import "testing"

func ptr(n int) *int { return &n }

// Every published provider number is a claim, and this is the field no
// provider dashboard will ever show you: whether real traffic agrees with it.
//
// The bounds come from replaying sessions. UpperBound is the smallest prompt
// ever seen served from cache, so the true minimum is at most that. LowerBound
// is the largest prompt ever seen uncached, so the true minimum is above it.
// The truth lies in (LowerBound, UpperBound].
func TestStatusIsDerivedFromTheBounds(t *testing.T) {
	for _, tc := range []struct {
		name       string
		documented int
		obs        *Observation
		want       ClaimStatus
	}{
		{"no observation at all", 512, nil, StatusUntested},
		{"observation with no bounds", 512, &Observation{Sessions: 4}, StatusUntested},
		{
			"documented above everything ever cached is refuted",
			8192, &Observation{UpperBound: ptr(4096), Sessions: 11, Machines: 1},
			StatusContradicted,
		},
		{
			"documented at or below something seen uncached is refuted",
			512, &Observation{LowerBound: ptr(512), Sessions: 11, Machines: 1},
			StatusContradicted,
		},
		{
			"one-sided agreement is not confirmation",
			512, &Observation{UpperBound: ptr(40563), Sessions: 11, Machines: 1},
			StatusUnverified,
		},
		{
			"documented inside a closed interval agrees",
			2048, &Observation{LowerBound: ptr(1024), UpperBound: ptr(4096), Sessions: 11, Machines: 3},
			StatusConsistent,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Claim{Documented: tc.documented, Observed: tc.obs}
			if got := c.Status(); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// Falsification is asymmetric and the model has to encode that. One machine
// seeing a prompt cached below the published minimum refutes it outright.
// One machine agreeing with it proves nothing, because the sampling, not the
// sample size, is what is in question.
func TestOneMachineCanRefuteButNotConfirm(t *testing.T) {
	refute := Claim{Documented: 4096, Observed: &Observation{UpperBound: ptr(1000), Sessions: 1, Machines: 1}}
	if got := refute.Status(); got != StatusContradicted {
		t.Fatalf("a single counterexample must refute: %q", got)
	}
	confirm := Claim{Documented: 2048, Observed: &Observation{LowerBound: ptr(1024), UpperBound: ptr(4096), Sessions: 1, Machines: 1}}
	if got := confirm.Status(); got == StatusContradicted {
		t.Fatalf("agreement is not a contradiction: %q", got)
	}
}

// The interval must be reported, because "consistent" over a range of 39,000
// tokens and "consistent" over a range of 8 are different statements and the
// word alone cannot tell them apart.
func TestTheIntervalTravelsWithTheVerdict(t *testing.T) {
	c := Claim{Documented: 2048, Observed: &Observation{LowerBound: ptr(1024), UpperBound: ptr(40563), Sessions: 11}}
	lo, hi, ok := c.Interval()
	if !ok || lo != 1024 || hi != 40563 {
		t.Fatalf("interval %d..%d ok=%v", lo, hi, ok)
	}
	if w, ok := c.IntervalWidth(); !ok || w != 39539 {
		t.Fatalf("width %d ok=%v", w, ok)
	}
}

// A status written into the file by hand is worth nothing: it is another claim
// wearing a verdict's clothes. The document may carry observations; the verdict
// is computed here or not at all.
func TestADeclaredStatusIsRefused(t *testing.T) {
	err := (&Rules{
		Schema: RulesSchema, Version: "test-2026-09-05",
		Models: []ModelRule{{
			Match: "m", MinPrefix: 512,
			MinPrefixClaim: &Claim{Documented: 512, DeclaredStatus: "consistent"},
		}},
	}).validate()
	if err == nil {
		t.Fatal("a hand-written status was accepted; the verdict must be derived from evidence")
	}
}

// An impossible interval is a bug in whatever produced the file, not a finding.
func TestAnInvertedIntervalIsRefused(t *testing.T) {
	err := (&Rules{
		Schema: RulesSchema, Version: "test-2026-09-05",
		Models: []ModelRule{{
			Match: "m", MinPrefix: 512,
			MinPrefixClaim: &Claim{Documented: 512, Observed: &Observation{LowerBound: ptr(9000), UpperBound: ptr(1000)}},
		}},
	}).validate()
	if err == nil {
		t.Fatal("an upper bound below the lower bound was accepted")
	}
}
