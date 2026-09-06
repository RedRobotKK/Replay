package main

import "testing"

// Transcripts overlap, and the total says so.
//
// A sub-agent lane re-renders its parent's requests, so the same requestId
// appears in more than one file. Across the measured corpus 54.9% of raw usage
// records are duplicates on that account, which is why any analysis that sums
// usage across files without deduplicating is wrong.
//
// `replay cost` mostly escapes it, and not by accident: MainLane takes the
// largest NON-sidechain lane, so a re-render inside a sidechain is skipped.
// Measured with the real parser over 1,484 files: 30,716 requests counted,
// 30,286 distinct, so 430 are counted more than once - 1.4%, not 54.9%.
//
// 1.4% does not justify restructuring the cost pipeline, which would mean
// recomputing lane costs and putting the reconciliation invariants at risk for
// a rounding correction. It does justify saying so, because a total presented
// without its known overlap is a total presented as exact.

// OV-1: the overlap is reported when there is one.
//
// PASS: a figure the reader can discount by.
// FAIL: silence, which is the same total wearing a claim of exactness.
func TestOV1_OverlapIsReported(t *testing.T) {
	got := overlapNote(430, 30716)
	if got == "" {
		t.Fatal("an overlap of 430 in 30,716 must be disclosed")
	}
	for _, want := range []string{"430", "1.4"} {
		if !contains(got, want) {
			t.Errorf("the note omits %q: %q", want, got)
		}
	}
}

// OV-2: no overlap, no note.
//
// PASS: empty, so a clean corpus reads clean.
// FAIL: "0 requests overlapped", which trains the reader to skip the line on
// the runs where it matters.
func TestOV2_NoOverlapNoNote(t *testing.T) {
	if got := overlapNote(0, 30716); got != "" {
		t.Errorf("a corpus with no overlap must print nothing, got %q", got)
	}
	if got := overlapNote(5, 0); got != "" {
		t.Errorf("no requests means no percentage to state, got %q", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
