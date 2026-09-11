package guardcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// "SURVIVED" answered two different questions with one word, and they have
// different fixes.
//
// A guard can survive because no test ever makes its condition true — nothing
// exercises the branch, and the fix is a test. Or because the branch does run
// and nothing depends on the difference — the statement is redundant with the
// code below it, and the fix is to delete it. Writing a test to keep dead code
// is the wrong move, and telling the two apart by hand cost three separate
// diagnoses on one pull request.
//
// Coverage separates them mechanically: did any test enter the body?
//
// The profile in testdata is real output, not a hand-written imitation of one,
// because the whole claim being tested is that Go's block positions line up
// with the parser's body braces the way this code assumes. Regenerate it with:
//
//	go test -covermode=count -coverprofile=cover.out ./...
//
// run in a module holding testdata/tiny.go.txt as tiny.go, plus a test calling
// Taken(-1) and Untaken(1).

// tinySubject copies the fixture source somewhere the profile's file name will
// match, and returns its path.
func tinySubject(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "tiny.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tiny.go")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// GC5: a branch no test enters is told apart from one that runs and is ignored.
//
// The fixture's two functions are identical but for which one the test calls
// with a negative, so the only difference between the two verdicts is the
// coverage count — which is the thing under test.
func TestGC5_CoverageTellsUnreachedFromInert(t *testing.T) {
	cov, err := ParseCoverage(filepath.Join("testdata", "tiny.cover"))
	if err != nil {
		t.Fatalf("ParseCoverage: %v", err)
	}

	src := tinySubject(t)
	gs, err := Conditionals(src, map[int]bool{5: true, 13: true})
	if err != nil {
		t.Fatalf("Conditionals: %v", err)
	}
	if len(gs) != 2 {
		t.Fatalf("the fixture source no longer holds the two guards this profile was "+
			"measured from: found %d. The profile is stale; regenerate it.", len(gs))
	}

	for _, tc := range []struct {
		name      string
		guard     Guard
		wantTaken bool
	}{
		{"Taken", gs[0], true},
		{"Untaken", gs[1], false},
	} {
		taken, known := cov.BranchTaken(tc.guard)
		if !known {
			t.Errorf("%s: the profile carries no block inside the body at line %d, so "+
				"the block-matching assumption is wrong and every verdict built on it "+
				"is unfounded", tc.name, tc.guard.Line)
			continue
		}
		if taken != tc.wantTaken {
			t.Errorf("%s: branch taken = %v, want %v", tc.name, taken, tc.wantTaken)
		}
	}
}

// GC6: a guard coverage knows nothing about is unknown, not untaken.
//
// Absence, zero and unknown are three values (ADR-0018). Calling an unmatched
// block "never taken" would invent an unreached branch out of a parsing miss,
// and send someone to write a test for a branch their suite already covers.
func TestGC6_AnUnmatchedGuardIsUnknownNotUntaken(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	if err := os.WriteFile(profile, []byte("mode: count\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cov, err := ParseCoverage(profile)
	if err != nil {
		t.Fatalf("ParseCoverage: %v", err)
	}
	if taken, known := cov.BranchTaken(Guard{File: "nowhere.go", Line: 1}); known {
		t.Errorf("an unmatched guard was reported as known (taken=%v)", taken)
	}
}

// GC7: a nil Coverage answers unknown rather than panicking.
//
// The reviewer sets cov to nil when the profile will not parse, and goes on to
// report every survivor unclassified. A nil dereference there would turn a
// missing enrichment into a crash that loses the findings entirely.
func TestGC7_NilCoverageIsUnknownNotAPanic(t *testing.T) {
	var cov *Coverage
	if _, known := cov.BranchTaken(Guard{File: "x.go", Line: 1}); known {
		t.Error("a nil Coverage claimed to know whether a branch was taken")
	}
}

// GC24: the header is recognised as a header, not counted as damage.
//
// `mode: count` is the profile's first line and is not a block. Letting it
// fall through to the block parser would work — it fails to parse and is
// skipped — but it would land in the Unparsed count, and the count is there to
// mean "this parser no longer understands Go's format". A number that is one
// on every healthy profile cannot carry that meaning.
func TestGC24_TheHeaderIsNotCountedAsUnparsed(t *testing.T) {
	cov, err := ParseCoverage(filepath.Join("testdata", "tiny.cover"))
	if err != nil {
		t.Fatalf("ParseCoverage: %v", err)
	}
	if cov.Unparsed != 0 {
		t.Errorf("a healthy profile reported %d unparsed line(s), so the count cannot "+
			"distinguish a format change from ordinary parsing", cov.Unparsed)
	}
}

// GC25: a profile this parser cannot read is counted, not silently empty.
//
// Without the count, a format change reads as a suite whose branches are all
// unknown — the reviewer would say NOT MEASURED about every guard and give no
// hint that the instrument, not the suite, is what changed.
func TestGC25_AnUnreadableProfileIsCounted(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	body := "mode: count\nthis is not a cover line\nnor/is.go:this one\n"
	if err := os.WriteFile(profile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cov, err := ParseCoverage(profile)
	if err != nil {
		t.Fatalf("ParseCoverage: %v", err)
	}
	if cov.Unparsed != 2 {
		t.Errorf("Unparsed = %d, want 2; a profile this parser cannot read looks "+
			"identical to one with no coverage in it", cov.Unparsed)
	}
}

// GC26: a truncated profile line is rejected, not indexed into.
//
// A line ending at the colon leaves nothing after it, so the field split is
// empty. Reading fields[0] there panics, and a panic in the reviewer takes
// every finding of the run down with it — the tool would die on a malformed
// profile rather than report the guards it had already judged.
//
// The shape is not hypothetical: a profile truncated by a full disk or a
// killed test process ends mid-line.
func TestGC26_ATruncatedLineDoesNotPanic(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	body := "mode: count\n" +
		"pkg/a.go:\n" + // truncated at the colon: nothing follows
		"pkg/a.go:1.2,3.4\n" + // a span with no counts after it
		"pkg/a.go:1.2,3.4 1\n" // one field short
	if err := os.WriteFile(profile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cov, err := ParseCoverage(profile)
	if err != nil {
		t.Fatalf("ParseCoverage: %v", err)
	}
	if cov.Unparsed != 3 {
		t.Errorf("Unparsed = %d, want 3: every truncated line should be counted", cov.Unparsed)
	}
	if _, known := cov.BranchTaken(Guard{File: "pkg/a.go", Line: 1, BodyStart: pos(1, 2), BodyEnd: pos(3, 4)}); known {
		t.Error("a block was built out of a truncated line")
	}
}

// GC27: every malformed span is rejected, and each on its own terms.
//
// The profile is written by the toolchain, so these lines should not occur —
// which is exactly why they need covering. Code that only ever sees
// well-formed input has failure handling nobody has watched work, and the
// first time it matters is a truncated file on a full disk during a CI run
// nobody is watching.
//
// One case per branch, so a rejection that stops happening shows up as the
// specific line that started being accepted rather than as a count.
func TestGC27_EveryMalformedSpanIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		line string
	}{
		{"no comma in the span", "pkg/a.go:1.2 1 1"},
		{"three parts in the span", "pkg/a.go:1.2,3.4,5.6 1 1"},
		{"start has no dot", "pkg/a.go:12,3.4 1 1"},
		{"end has no dot", "pkg/a.go:1.2,34 1 1"},
		{"start line is not a number", "pkg/a.go:a.2,3.4 1 1"},
		{"start column is not a number", "pkg/a.go:1.a,3.4 1 1"},
		{"end line is not a number", "pkg/a.go:1.2,a.4 1 1"},
		{"end column is not a number", "pkg/a.go:1.2,3.a 1 1"},
		{"count is not a number", "pkg/a.go:1.2,3.4 1 x"},
		{"no colon at all", "just some text"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile := filepath.Join(t.TempDir(), "cover.out")
			if err := os.WriteFile(profile, []byte("mode: count\n"+tc.line+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			cov, err := ParseCoverage(profile)
			if err != nil {
				t.Fatalf("ParseCoverage: %v", err)
			}
			if cov.Unparsed != 1 {
				t.Errorf("Unparsed = %d, want 1: %q was accepted as a block",
					cov.Unparsed, tc.line)
			}
			if _, known := cov.BranchTaken(Guard{
				File: "pkg/a.go", Line: 1, BodyStart: pos(1, 2), BodyEnd: pos(3, 4),
			}); known {
				t.Errorf("a block was built from %q", tc.line)
			}
		})
	}
}

// GC28: a profile that cannot be opened is an error, not empty coverage.
//
// Empty coverage means "no block ran", which the reviewer reads as UNREACHED
// for every guard — it would tell a whole team to write tests for branches
// their suite already covers, on the strength of a missing file.
func TestGC28_AMissingProfileIsAnError(t *testing.T) {
	_, err := ParseCoverage(filepath.Join(t.TempDir(), "absent.out"))
	if err == nil {
		t.Fatal("a profile that does not exist parsed successfully, so every guard " +
			"would be classified against no evidence at all")
	}
	// The message is the observable part: scanning a nil file yields
	// ErrInvalid, so an error arrives either way — phrased as "invalid
	// argument", which describes a programming mistake rather than a missing
	// file.
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("a missing profile was diagnosed as something else: %v", err)
	}
}
