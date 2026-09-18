package cachemodel

import (
	"strings"
	"testing"
)

// The allowance note must not hardcode a multiplier that is not universal.
//
// It read "the same 0.10x discount", which is right for most rows and wrong by
// four times for claude-fable-5-1 and claude-mythos-5-1, whose cache reads bill
// at 0.025x under a documented Anthropic exception (readMultiplierNewest,
// anthropic.go:106). A Fable operator reading `replay cost` was told their
// reads cost four times what they do.
//
// The note's actual subject is that nobody has measured whether cache reads
// weigh against a subscription allowance at the SAME rate they weigh against a
// bill. That question does not need the number, and stating one it cannot
// support is the failure this note otherwise exists to describe.
func TestTheAllowanceNoteDoesNotHardcodeAMultiplier(t *testing.T) {
	got := CeilingEffect{Requests: 3, Tokens: Tokens{Input: 1000, CacheWrite: 2000, CacheRead: 300000}}.AllowanceNote()
	// The defect is an UNATTRIBUTED rate, not a digit. Naming 0.10x alone tells
	// a Fable operator something false. Naming both, each against its models,
	// is the truth and is allowed. A first draft of this test banned every
	// numeral and would have failed the correct fix as well as the defect.
	if strings.Contains(got, "0.10x") && !strings.Contains(got, "0.025x") {
		t.Error("the allowance note states 0.10x without naming the 0.025x rows; " +
			"a Fable 5.1 or Mythos 5.1 operator is told their reads cost four " +
			"times what they do")
	}
	if strings.Contains(got, "same 0.10x discount") {
		t.Error(`the note calls 0.10x "the same discount" as though it were universal`)
	}
	if !strings.Contains(got, "NOT MEASURED") {
		t.Error("the note must keep saying the allowance question is not measured")
	}
}

// Fable's exception is real and must stay confined to the 5.1 rows.
func TestFableFiveOneReadsAtTheDocumentedExceptionRate(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  float64
	}{
		{"claude-fable-5-1", 0.025},
		{"claude-mythos-5-1", 0.025},
		{"claude-fable-5", 0.10},
		{"claude-opus-5", 0.10},
	} {
		p, ok := PriceFor(tc.model)
		if !ok {
			t.Fatalf("%s is not priced", tc.model)
		}
		if p.ReadMult != tc.want {
			t.Errorf("%s read multiplier = %v, want %v", tc.model, p.ReadMult, tc.want)
		}
	}
}
