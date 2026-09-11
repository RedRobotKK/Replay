package main

import (
	"bytes"
	"strings"
	"testing"
)

// What `replay doctor` says about the corpus has to be true of it.
//
// Two sentences were not. The first promised more than the tool delivers:
//
//	1812 transcript files in all: a session writes one per agent lane, so
//	replay cost over this directory reports on every one of them
//
// `replay cost` reports on 1,797 of them. Nine fail to parse and six carry a
// model nothing prices, and the promise covered neither. TC-3 in
// doctor_counts_test.go asserts doctor's total equals len(transcriptFiles(root))
// — what the walker HANDS to cost, not what cost PRICES — so the test guarded
// the walk while the sentence promised the read.
//
// The second is a word. doctor's "lanes" excludes the session's own transcript
// (countNestedTranscripts skips the project's own children); `replay cost`
// prints "N sessions (M agent lanes)" where M counts every priced transcript,
// main lane included. On this corpus that is 1,688 against 1,797 — the same
// word, two denominators, and nothing on either surface saying so.

func doctorOutput(t *testing.T) string {
	t.Helper()
	// A corpus with a real fan-out, because the sentences under test only
	// print when one exists. The suite's TestMain points HOME at an empty
	// temporary directory, so a doctor run that borrowed the ambient HOME
	// would take the "none found" branch and assert nothing at all — which is
	// how the first version of this file passed against the defect it names.
	fannedOutCorpus(t)
	var out, errs bytes.Buffer
	if err := runDoctor(nil, &out, &errs); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	return out.String()
}

// DW1: doctor does not promise that cost reads everything it counted.
func TestDoctorDoesNotPromiseCostReadsEveryFile(t *testing.T) {
	got := doctorOutput(t)
	if strings.Contains(got, "reports on every one of them") {
		t.Errorf("doctor promises `replay cost` reads every file it counted. It "+
			"does not: files that fail to parse and files whose model is unpriced "+
			"are both excluded.\n%s", got)
	}
}

// DW2: and when it names a file count it says what a file is.
//
// The number is worth keeping — `replay cost` does walk every one of them —
// but the sentence has to stop where the evidence does.
func TestDoctorSaysWhatItCounted(t *testing.T) {
	got := doctorOutput(t)
	if !strings.Contains(got, "transcript files") {
		t.Fatalf("the fixture produced no file-count line, so this test is checking "+
			"nothing:\n%s", got)
	}
	for _, want := range []string{"sub-agent lane", "one per agent lane", "counts a session's own transcript as a lane too"} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor names a file total without %q, so a reader comparing it "+
				"to `replay cost` has two figures under one word and no way to "+
				"reconcile them:\n%s", want, got)
		}
	}
}
