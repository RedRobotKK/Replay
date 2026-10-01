package surface

import "testing"

// RPL-C008 and RPL-C005. The three-state distinction and the refusal.
//
// ADR-0018's rule is that absence, zero and unknown are three values. The
// cheapest way for that rule to rot is for a missing counter to be read as a
// zero counter somewhere downstream, because both arrive as the integer 0 in
// a struct. These tests hold the two apart at the point where the verdict is
// decided.

// Absent and zero must not produce the same verdict. This is the positive
// and negative control pair for the three-state rule.
func TestC008_AbsentFieldAndZeroFieldDiffer(t *testing.T) {
	// A corpus where the provider's write counter is genuinely not in the
	// payload at all.
	absent := Observables{
		Boundary:             "test-absent",
		Records:              500,
		WriteFieldPresent:    false,
		RecordsWithWrite:     0,
		WriteObservations:    0,
		WriteNonZero:         0,
		RecordsWithRead:      500,
		ReadObservations:     500,
		ReadNonZero:          480,
		PerRequestSequencing: true,
	}
	// A corpus where the counter IS in the payload and reads zero on every
	// record. Same integer, entirely different fact.
	zero := absent
	zero.WriteFieldPresent = true
	zero.RecordsWithWrite = 500
	zero.WriteObservations = 500
	zero.WriteNonZero = 0

	unknownContract := ContractFact{Write: ContractUnknown}

	va := Classify(absent, unknownContract)
	vz := Classify(zero, unknownContract)

	if va.Class == vz.Class {
		t.Errorf("a missing write counter and a present-but-zero write counter both "+
			"classified as %s. Absence and zero have collapsed into one value, which "+
			"is the defect ADR-0018 exists to prevent.", va.Class)
	}
	t.Logf("absent -> %s; present-and-zero -> %s", va.Class, vz.Class)
}

// RPL-C005. A refusal must be reachable and must be a verdict, not a crash
// and not a silent default. ClassUndetermined is the refusal; these are the
// inputs that must reach it.
func TestC005_RefusalIsDistinguishableFromZero(t *testing.T) {
	cases := []struct {
		name string
		o    Observables
		c    ContractFact
		why  string
	}{
		{
			name: "no records at all",
			o:    Observables{Boundary: "empty", Records: 0},
			c:    ContractFact{Write: ContractPricedDistinctly, Source: "provider pricing page"},
			why:  "an empty corpus cannot establish anything about a counter",
		},
		{
			name: "zero write observations and an unknown provider contract",
			o: Observables{
				Boundary: "ambiguous", Records: 400,
				WriteFieldPresent: true, RecordsWithWrite: 400,
				WriteObservations: 0, WriteNonZero: 0,
				RecordsWithRead: 400, ReadObservations: 400, ReadNonZero: 390,
				PerRequestSequencing: true,
			},
			c:   ContractFact{Write: ContractUnknown},
			why: "the zeros are uninterpreted: no contract says whether a write should have appeared",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := Classify(tc.o, tc.c)
			if v.Class != ClassUndetermined {
				t.Errorf("expected the refusal %s because %s, got %s (%s)",
					ClassUndetermined, tc.why, v.Class, v.Why)
			}
			if v.Why == "" {
				t.Error("the refusal carries no reason. A refusal that does not say " +
					"why is indistinguishable from a failure to run.")
			}
		})
	}

	// NEGATIVE CONTROL. If everything refused, the refusal would carry no
	// information. A well-formed corpus with a sourced contract must NOT
	// refuse.
	good := Observables{
		Boundary: "clean", Records: 1000,
		WriteFieldPresent: true, RecordsWithWrite: 1000,
		WriteObservations: 1000, WriteNonZero: 900,
		RecordsWithRead: 1000, ReadObservations: 1000, ReadNonZero: 950,
		PerRequestSequencing: true, OracleClasses: 4,
	}
	v := Classify(good, ContractFact{Write: ContractPricedDistinctly, Source: "provider pricing page"})
	if v.Class == ClassUndetermined {
		t.Errorf("a clean corpus with a sourced contract still refused (%s). The "+
			"refusal fires on everything and therefore means nothing.", v.Why)
	}
}

// An unsourced contract fact is the one input a corpus cannot check, so it
// must be rejected rather than trusted. This is the verifier-cannot-create-
// evidence invariant at its sharpest point.
func TestC005_AnUnsourcedContractIsRefused(t *testing.T) {
	if err := (ContractFact{Write: ContractPricedDistinctly}).Validate(); err == nil {
		t.Error("an unsourced contract claim validated. The classifier would then " +
			"treat an assertion with no provenance as a provider fact.")
	}
	if err := (ContractFact{Write: ContractPricedDistinctly, Source: "pricing page"}).Validate(); err != nil {
		t.Errorf("a sourced contract was rejected: %v. The check is too strict to be useful.", err)
	}
	// ContractUnknown needs no source: it asserts nothing.
	if err := (ContractFact{Write: ContractUnknown}).Validate(); err != nil {
		t.Errorf("the unknown contract was rejected: %v. Refusing to assert must "+
			"not itself require evidence.", err)
	}
}
