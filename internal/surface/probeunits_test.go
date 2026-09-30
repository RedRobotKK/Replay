package surface

import (
	"os"
	"path/filepath"
	"testing"
)

// A counter name that appears twice in one record under two different UNITS
// must be counted once, and counted as tokens.
//
// Found by re-measuring OpenClaw for a bake-off on 2026-09-29. Its records
// carry `usage.cacheRead` in TOKENS and `usage.cost.cacheRead` in US DOLLARS,
// same spelling, two units, one nested inside the other. Probe walks every
// nested map and num() accepts any float, so both are counted: 451 records
// read as 902 observations and 116 non-zero reads as 232.
//
// That is not a rounding error, it is a unit error, and it reaches a verdict.
// ReadNonZero is what Classify uses to decide whether a counter survived to a
// boundary, and a fraction of a cent counted as a non-zero token reading is a
// reading of the wrong quantity.
//
// This is NOT the repeated-counter case Observables already documents for
// Grok. There the same quantity appears twice and double-counting inflates a
// count. Here a different quantity wears the same name, so the second reading
// is not the counter at all.
//
// PASS: one record with a nested cost object yields one read observation.
// FAIL: the dollar figure is counted as a token counter.
func TestPU1_ANestedCostObjectIsNotASecondTokenCounter(t *testing.T) {
	dir := t.TempDir()
	// Shaped after a real OpenClaw record. Values are the measured ones from
	// a single block; no message content is carried.
	line := `{"message":{"usage":{"input":17,"output":3,"cacheRead":10406,"cacheWrite":0,` +
		`"cost":{"input":0.0000136,"output":0.000012,"cacheRead":0.00083248,"cacheWrite":0}}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	// Exclude is deliberately opt-in. Skipping any sub-object named "cost" on
	// every surface would be a normalisation this package refuses elsewhere:
	// the spelling is the evidence, and a surface that genuinely reported a
	// token counter under that name would go silently unread.
	spec := FieldSpec{
		Write:    []string{"cacheWrite"},
		Read:     []string{"cacheRead"},
		Sequence: []string{"usage"},
		Exclude:  []string{"cost"},
	}
	got, err := Probe(dir, spec)
	if err != nil {
		t.Fatal(err)
	}

	// Without the exclusion the same record double counts. Pinned so the
	// hazard is visible at the call site rather than only in a comment, and so
	// a future default change has to break this line on purpose.
	bare := spec
	bare.Exclude = nil
	if loose, lerr := Probe(dir, bare); lerr == nil && loose.ReadObservations != 2 {
		t.Errorf("without Exclude the nested cost object should still be counted (%d), "+
			"otherwise this test is not measuring what it claims", loose.ReadObservations)
	}
	if got.ReadObservations != 1 {
		t.Errorf("ReadObservations = %d, want 1: the record carries one cacheRead counter, "+
			"and usage.cost.cacheRead is the same name in dollars rather than a second counter",
			got.ReadObservations)
	}
	if got.ReadNonZero != 1 {
		t.Errorf("ReadNonZero = %d, want 1: a fraction of a cent must not read as a non-zero token counter",
			got.ReadNonZero)
	}
	if got.WriteObservations != 1 {
		t.Errorf("WriteObservations = %d, want 1", got.WriteObservations)
	}
	if got.WriteNonZero != 0 {
		t.Errorf("WriteNonZero = %d, want 0", got.WriteNonZero)
	}
}
