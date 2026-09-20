package surfaces

import "testing"

// Measured on one machine 2026-09-15 across five agent stores. Four distinct
// ways a surface can fail to tell you what it wrote, and cachemodel's
// ClassifyCounters cannot separate two of them: it sees read>0 write=0 and
// answers CountersWriteMissing for both Codex, which has no write field at
// all, and Grok, which has the field and reports zero on every record.
//
// That difference decides what a reader may conclude. An absent field claims
// nothing. A zero looks like a measurement and will be summed by every tool
// downstream.

func TestClassify_BothReported(t *testing.T) {
	// Claude Code: 400 turns, 199,240,300 read, 686,189 written.
	got := Classify(Reading{Records: 400, Reads: 199_240_300, Writes: 686_189, WriteFieldPresent: true})
	if got != StateInstrumented {
		t.Fatalf("read and write both reported classified %q, want %q", got, StateInstrumented)
	}
}

func TestClassify_WriteFieldAbsentIsNotTheSameAsZero(t *testing.T) {
	// Codex: 400 turns, 33,439,104 read, no write key exists in the shape.
	got := Classify(Reading{Records: 400, Reads: 33_439_104, Writes: 0, WriteFieldPresent: false})
	if got == StateWriteReportedZero {
		t.Fatal("absent write field classified as a reported zero; an absent field claims nothing " +
			"and a zero claims something false, and collapsing them is the defect this package exists to name")
	}
	if got != StateWriteFieldAbsent {
		t.Fatalf("absent write field classified %q, want %q", got, StateWriteFieldAbsent)
	}
}

func TestClassify_PresentAndAlwaysZeroIsAbsenceWearingAValue(t *testing.T) {
	// Grok: 264 records, 343,358,720 read, cacheCreationTokens present and 0 every time.
	got := Classify(Reading{Records: 264, Reads: 343_358_720, Writes: 0, WriteFieldPresent: true})
	if got != StateWriteReportedZero {
		t.Fatalf("a present write field reporting zero beside 343M of reads classified %q, want %q: "+
			"a read serves a prefix some earlier request wrote", got, StateWriteReportedZero)
	}
}

func TestClassify_NoRecordsIsSilentNotUncached(t *testing.T) {
	// Cursor: 142 transcripts, no token or cost field anywhere.
	got := Classify(Reading{Records: 0})
	if got != StateSilent {
		t.Fatalf("a surface reporting nothing classified %q, want %q: no instrumentation and no "+
			"caching are different states and neither is observable here", got, StateSilent)
	}
}

func TestClassify_RecordsButNoReadsIsStillSilent(t *testing.T) {
	// Records exist and every counter is zero: nothing can be concluded.
	if got := Classify(Reading{Records: 50, Reads: 0, Writes: 0, WriteFieldPresent: true}); got != StateSilent {
		t.Fatalf("records with no reads and no writes classified %q, want %q", got, StateSilent)
	}
}

func TestClassify_NegativeCountsAreRefusedRatherThanClassified(t *testing.T) {
	// A broken reader is not a cache story, and must not get this package's name on it.
	for _, r := range []Reading{{Records: 10, Reads: -1}, {Records: 10, Writes: -5, WriteFieldPresent: true}} {
		if got := Classify(r); got != StateUnreadable {
			t.Errorf("Reading%+v classified %q, want %q", r, got, StateUnreadable)
		}
	}
}

// A mutation sweep on 2026-09-15 showed the Records guard could be changed to
// `Records < 0` without reddening anything: every test that reached it also had
// zero counters, so it arrived at Silent by the next branch instead. A branch
// that cannot change an outcome is not evidence.
//
// The case that separates them is incoherent rather than merely empty: no
// records found, yet tokens counted. That is a broken scan, not a quiet
// surface, and it must not be reported as one.
func TestClassify_NoRecordsButCountedTokensIsABrokenScan(t *testing.T) {
	got := Classify(Reading{Records: 0, Reads: 9_984, WriteFieldPresent: true})
	if got == StateSilent {
		t.Fatal("a scan that found no records but counted 9,984 read tokens was reported as Silent; " +
			"the counts had to come from somewhere, so the scan is what is broken")
	}
	if got != StateUnreadable {
		t.Fatalf("incoherent reading classified %q, want %q", got, StateUnreadable)
	}
}
