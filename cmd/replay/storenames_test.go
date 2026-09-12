package main

import (
	"slices"
	"testing"
)

// SN1: the disclosed name slice is homeStores(), name for name.
//
// A count is the wrong gate for a privacy disclosure. ST1 asserted
// len(known) < 8 — a floor — so the registry grew twice on 2026-09-10
// (contributor-secret, rules.json) and nothing went red. A later check that
// compares len(homeStores()) to a prose number fails the same way in reverse:
// add a thirteenth store, bump "twelve" to "thirteen", drop a sensitive name,
// and the counts still agree. The omitted name is then undisclosed, which is
// the original defect with the arithmetic kept consistent.
//
// The gate is the names. This slice is the disclosure; homeStores() is what
// the tool writes. They must be equal, in the registry's order. Adding a
// store without naming it here fails. Removing contributor-secret from either
// side fails even when the lengths still match.
//
// The two Sensitive stores are named in this slice on purpose: if both this
// list and the registry were emptied, slices.Equal would hold of two empty
// slices and the check would go vacuously true. Naming them here is the
// positive control that the slice is still a disclosure.
//
// PASS: disclosed and homeStores() name the same stores, in the same order.
// FAIL: a store the tool writes is not in this list, or this list names a
// store the tool does not write.
func TestSN1_DisclosedNamesEqualHomeStoreNames(t *testing.T) {
	disclosed := []string{
		"vault",
		"ledger",
		"archive",
		"advice.json",
		"policy.json",
		"cost-index.json",
		"measurements.jsonl",
		"seen.json",
		"tip.json",
		"serve.log",
		"contributor-secret",
		"rules.json",
	}

	for _, sensitive := range []string{"vault", "contributor-secret"} {
		if !slices.Contains(disclosed, sensitive) {
			t.Fatalf("the disclosed slice no longer names %q. If this list and the "+
				"registry were both emptied, slices.Equal would hold of two empty slices "+
				"and a store holding secrets would be undisclosed by a check that cannot fail",
				sensitive)
		}
	}

	var registered []string
	for _, s := range homeStores() {
		registered = append(registered, s.Name)
	}

	if !slices.Equal(registered, disclosed) {
		t.Errorf("homeStores() names %q; disclosed names %q. The gate is the names, "+
			"not the count: a store missing from the disclosed slice is undisclosed even "+
			"when the lengths agree",
			registered, disclosed)
	}
}
