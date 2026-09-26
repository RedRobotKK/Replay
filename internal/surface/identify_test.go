package surface

import "testing"

// THE INVARIANT. The same corpus bytes classify differently, and only the
// independently sourced contract moves the answer.
//
// This is the whole package in one test. Codex's real corpus shape is a
// populated read counter beside a write counter that is zero on every record.
// That shape is observationally identical under two incompatible worlds: a
// provider that performs no billable writes, and a provider that does whose
// client drops the counter. If a single set of observations ever produced the
// same class under a known and an unknown contract, the classifier would be
// reading the zeros as evidence, which is the defect this package exists to
// prevent.
func TestZeroWritesClassifyOnlyWithAContract(t *testing.T) {
	// One corpus. Not re-derived per case, so the test cannot pass by using
	// different evidence for different contracts.
	observed := Observables{
		Boundary: "codex-shaped", Records: 100,
		WriteFieldPresent: true, WriteObservations: 100, WriteNonZero: 0,
		ReadObservations: 100, ReadNonZero: 99,
		PerRequestSequencing: true,
	}

	unknown := Classify(observed, ContractFact{})
	if unknown.Class != ClassUndetermined {
		t.Errorf("with no contract: class = %q, want %q. Zeros are not a finding: "+
			"a provider that never writes and a client that drops the counter "+
			"produce this corpus identically", unknown.Class, ClassUndetermined)
	}

	priced := Classify(observed, OpenAIModernContract)
	if priced.Class != ClassIIArtifactLoss {
		t.Errorf("with a distinctly-priced contract: class = %q, want %q. The "+
			"provider bills for writes, so a counter that is zero everywhere "+
			"did not survive to this boundary", priced.Class, ClassIIArtifactLoss)
	}

	notPriced := Classify(observed, DeepSeekContract)
	if notPriced.Class != ClassIVIncommensurable {
		t.Errorf("with a not-distinctly-priced contract: class = %q, want %q",
			notPriced.Class, ClassIVIncommensurable)
	}

	if unknown.Class == priced.Class {
		t.Error("identical observations produced the same class under a known " +
			"and an unknown contract: the classifier is deciding from corpus " +
			"bytes alone, which is the one thing it must never do")
	}
}

// An unsourced contract claim is refused.
//
// The contract is the input no corpus can check, so the only thing standing
// behind it is the document it was read from. A ContractFact asserting a
// pricing class with no source would let an assertion typed into a struct
// literal upgrade a corpus of zeros into a confident class.
func TestUnsourcedContractIsRefused(t *testing.T) {
	observed := Observables{
		Boundary: "x", Records: 10,
		WriteFieldPresent: true, WriteObservations: 10, WriteNonZero: 0,
		ReadObservations: 10, ReadNonZero: 10, PerRequestSequencing: true,
	}
	v := Classify(observed, ContractFact{Write: ContractPricedDistinctly})
	if v.Class != ClassUndetermined {
		t.Errorf("class = %q, want %q: a pricing claim with no provenance is "+
			"not an input, it is an assertion", v.Class, ClassUndetermined)
	}
	if v.Passed("N1a") {
		t.Error("N1a passed on an unsourced contract fact")
	}
}

// Every class the framework names is reachable, and reachable for its own
// reason. A classifier that could never return a class would make that class
// a comment rather than a verdict.
func TestEveryClassIsReachable(t *testing.T) {
	full := Observables{
		Boundary: "anthropic-shaped", Records: 50,
		WriteFieldPresent: true, WriteObservations: 50, WriteNonZero: 49,
		ReadObservations: 50, ReadNonZero: 50,
		PerRequestSequencing: true, OracleClasses: 4,
	}
	noOracle := full
	noOracle.OracleClasses = 0

	noField := Observables{Boundary: "cursor-shaped", Records: 20}

	zeroWrite := full
	zeroWrite.WriteNonZero = 0
	zeroWrite.OracleClasses = 0

	cases := []struct {
		name string
		o    Observables
		c    ContractFact
		want Class
	}{
		{"claude code shape", full, AnthropicContract, Class0Identified},
		{"openai seam shape, no oracle", noOracle, OpenAIModernContract, ClassIMarginalOnly},
		{"codex transcript shape", zeroWrite, OpenAIModernContract, ClassIIArtifactLoss},
		{"cursor shape, no field", noField, AnthropicContract, ClassIIINoObservable},
		{"deepseek shape", full, DeepSeekContract, ClassIVIncommensurable},
		{"grok shape, contract unknown", zeroWrite, XAIContract, ClassUndetermined},
		{"empty corpus", Observables{Boundary: "empty"}, AnthropicContract, ClassUndetermined},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.o, tc.c).Class; got != tc.want {
				t.Errorf("class = %q, want %q", got, tc.want)
			}
		})
	}
}

// Observed writes are not enough on their own. Without the contract, whether
// the quantity is a priced one is open, so the economic class must stay open
// too. This is the mirror of the zero-write invariant: neither a zero nor a
// non-zero licenses a class by itself.
func TestObservedWritesWithoutAContractStayUndetermined(t *testing.T) {
	o := Observables{
		Boundary: "writes but unknown pricing", Records: 30,
		WriteFieldPresent: true, WriteObservations: 30, WriteNonZero: 28,
		ReadObservations: 30, ReadNonZero: 30,
		PerRequestSequencing: true, OracleClasses: 3,
	}
	v := Classify(o, XAIContract)
	if v.Class != ClassUndetermined {
		t.Errorf("class = %q, want %q: writes were observed, but whether they "+
			"are distinctly priced was not established, so no economic class "+
			"follows", v.Class, ClassUndetermined)
	}
}

// Sequencing is required, and its absence must not be silently tolerated.
func TestCumulativeOnlyCannotBeDifferenced(t *testing.T) {
	o := Observables{
		Boundary: "cumulative only", Records: 40,
		WriteFieldPresent: true, WriteObservations: 40, WriteNonZero: 40,
		ReadObservations: 40, ReadNonZero: 40,
		PerRequestSequencing: false, OracleClasses: 4,
	}
	if got := Classify(o, AnthropicContract).Class; got != ClassIIINoObservable {
		t.Errorf("class = %q, want %q: a cumulative counter cannot answer a "+
			"question about a boundary", got, ClassIIINoObservable)
	}
}

// A verdict has to say which half of the argument each condition came from,
// or a reviewer cannot tell a measurement from a citation.
func TestVerdictSeparatesCorpusFromContract(t *testing.T) {
	o := Observables{
		Boundary: "b", Records: 10,
		WriteFieldPresent: true, WriteObservations: 10, WriteNonZero: 9,
		ReadObservations: 10, ReadNonZero: 10,
		PerRequestSequencing: true, OracleClasses: 4,
	}
	v := Classify(o, AnthropicContract)
	basis := map[string]Basis{}
	for _, c := range v.Conditions {
		basis[c.ID] = c.Basis
	}
	if basis["N1a"] != FromContract {
		t.Errorf("N1a basis = %q, want %q: no corpus can establish a provider's "+
			"pricing", basis["N1a"], FromContract)
	}
	for _, id := range []string{"N1b", "N2", "N3"} {
		if basis[id] != FromCorpus {
			t.Errorf("%s basis = %q, want %q", id, basis[id], FromCorpus)
		}
	}
	if v.Contract.Source == "" {
		t.Error("verdict carries no contract provenance, so the half of the " +
			"argument a reviewer cannot check is also the half they cannot find")
	}
}

// Condition (d): a null is a finding only when the instrument was recording.
func TestNullsNeedAPopulatedReadCounter(t *testing.T) {
	recording := Observables{ReadObservations: 10, ReadNonZero: 9}
	silent := Observables{ReadObservations: 10, ReadNonZero: 0}
	if !NullsInterpretable(recording) {
		t.Error("a zero write beside a live read is an observed zero")
	}
	if NullsInterpretable(silent) {
		t.Error("with every counter at zero, an idle surface and a broken one " +
			"are indistinguishable, so a null means nothing")
	}
}
