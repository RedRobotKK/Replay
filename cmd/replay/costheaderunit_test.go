package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The sentence this branch exists to stop being printed was entered by no test.
//
// guard-reachability reported `if s.Unit == unitLane` in costHeaderLine as
// UNREACHED. The comment inside it says "Cost per task over a lane count is the
// sentence this whole change exists to stop being printed" — so the headline
// behaviour of the change had no test making its condition true, and deleting
// the branch would have left the suite green while reinstating the defect.
//
// The two rows are the point. A session-unit report counts sessions and says so
// while also printing the lane count, because the gap between 114 and 1,614 is
// what made the published figure wrong. A lane-unit report may not borrow the
// word "task" for a number that counts lanes.

func TestCH1_ALaneUnitReportDoesNotCallItsRowsTasks(t *testing.T) {
	got := costHeaderLine(costSummary{Unit: unitLane, Tasks: 1614, Lanes: 1614})
	if strings.Contains(got, "Cost per task") {
		t.Errorf("a report whose rows are agent lanes is headed \"Cost per task\", which "+
			"is the sentence this change exists to stop printing:\n  %s", got)
	}
	if !strings.Contains(got, "Cost per agent lane") {
		t.Errorf("the unit is not named as a lane:\n  %s", got)
	}
	if !strings.Contains(got, "1614 agent lanes") {
		t.Errorf("the subject does not say what the 1614 counts:\n  %s", got)
	}
}

func TestCH2_ASessionUnitReportPrintsBothCounts(t *testing.T) {
	got := costHeaderLine(costSummary{Unit: unitSession, Tasks: 114, Lanes: 1614})
	if !strings.Contains(got, "Cost per task") {
		t.Errorf("a session-unit report lost its own heading:\n  %s", got)
	}
	// Both numbers, because either alone is the figure that was retracted.
	if !strings.Contains(got, "114 sessions (1614 agent lanes)") {
		t.Errorf("the session count is printed without the lane count behind it. The gap "+
			"between the two is the fact that made the original figure wrong:\n  %s", got)
	}
}

// TestCH3 pins the third arm: equal counts print one number, not "(114 agent
// lanes)" after "114 sessions". Without it, `s.Lanes > s.Tasks` can be
// weakened to `!=` or dropped and nothing notices.
func TestCH3_EqualCountsAreNotPrintedTwice(t *testing.T) {
	got := costHeaderLine(costSummary{Unit: unitSession, Tasks: 114, Lanes: 114})
	if strings.Contains(got, "agent lanes)") {
		t.Errorf("one lane per session, and the report says it twice:\n  %s", got)
	}
	if !strings.Contains(got, "114 sessions") {
		t.Errorf("the session count is missing:\n  %s", got)
	}
}

// TestCH4 covers sessionTime's refusal.
//
// UNREACHED, and it is the ADR-0018 distinction in one function: a report with
// no requests has an UNKNOWN time, and the zero Time is how that is spelled.
// Nothing tested that a nil report did not panic, let alone what it returned.
func TestCH4_SessionTimeOfAnEmptyReportIsZeroNotAGuess(t *testing.T) {
	for _, tc := range []struct {
		name string
		rep  *analysis.LaneReport
	}{
		{"nil report", nil},
		{"nil lane", &analysis.LaneReport{}},
		{"no requests", &analysis.LaneReport{Lane: &transcript.Lane{}}},
	} {
		if got := sessionTime(tc.rep); !got.IsZero() {
			t.Errorf("%s: sessionTime = %v, want the zero time — there is no first "+
				"request to take it from", tc.name, got)
		}
	}
	// And the measured case, so the zero above is a refusal rather than the
	// only thing this function can return.
	want := time.Date(2026, 9, 11, 4, 5, 6, 0, time.UTC)
	rep := &analysis.LaneReport{Lane: &transcript.Lane{
		Requests: []*transcript.Request{{Timestamp: want}},
	}}
	if got := sessionTime(rep); !got.Equal(want) {
		t.Errorf("sessionTime = %v, want %v: the first request's stamp is the session's", got, want)
	}
}

// TestCH5_AFailedWriteIsReturnedRatherThanSwallowed covers the two write
// guards in runCost that guard-reachability reported UNREACHED and INERT.
//
// They are one-line `if err != nil { return err }` pairs, which is the shape
// that gets deleted as noise or rewritten as `_, _ =` by someone tidying. The
// consequence is not a lost diagnostic: it is `replay cost | head` exiting 0
// on a report that stopped halfway through — a money figure with its caveats
// missing, and nothing for a reader or a script to tell that from a complete
// one.
//
// The corpus is supplied explicitly. Written first against the default corpus,
// this test passed for a reason that had nothing to do with the guards: the
// test HOME holds no transcripts, so cost took the empty-corpus path, wrote
// its "No transcripts found" notice to STDERR, and returned nil having sent
// stdout nothing at all. A failing stdout writer cannot fail if nothing is
// written to it.
func TestCH5_AFailedWriteIsReturnedRatherThanSwallowed(t *testing.T) {
	src := filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("no transcript fixture in this tree: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), b, 0o600); err != nil {
		t.Fatal(err)
	}

	// Sanity first: the corpus produces a report at all. Without this the
	// loop below can pass on an empty document.
	var probe, stderr bytes.Buffer
	if err := run([]string{"cost", dir}, &probe, &stderr); err != nil {
		t.Fatalf("cost on the fixture failed: %v (stderr %q)", err, stderr.String())
	}
	full := probe.Len()
	if full == 0 {
		t.Fatalf("the fixture produced no report; this test asserts nothing (stderr %q)", stderr.String())
	}

	// Cut the report at several points, including inside the notes that follow
	// the main block, which is where the two guards live.
	for _, after := range []int{0, 1, full / 2, full - 1} {
		stderr.Reset()
		w := &failAfter{n: after}
		if err := run([]string{"cost", dir}, w, &stderr); err == nil {
			t.Errorf("the writer failed after %d of %d bytes and cost reported success; a "+
				"reader piping this into head cannot tell a complete total from a "+
				"truncated one", after, full)
		}
	}
}

// failOnCall accepts every write but the nth, counting from zero.
//
// A byte cutoff cannot aim. The two guards under test sit on different writes
// in the same report, and choosing a byte offset that lands on one of them
// means recomputing that offset whenever a line above it changes. Counting
// calls instead lets the test sweep every write in the path, which is both
// what the reviewer asks for and stable under edits to the report.
type failOnCall struct {
	n     int
	calls int
}

func (f *failOnCall) Write(p []byte) (int, error) {
	f.calls++
	if f.calls-1 == f.n {
		return 0, errNoSpace
	}
	return len(p), nil
}

// TestCH6_EveryWriteInTheReportHasItsErrorReturned sweeps the writes.
//
// TestCH5 proved the report as a whole does not swallow a write failure. It
// could not prove it of a PARTICULAR write: cutting at a byte offset fails
// whichever write happens to straddle it, and a later guard returning the same
// error hides a swallow in an earlier one. Neutralising the guard at cost.go:880
// changed nothing TestCH5 could see, for exactly that reason.
//
// This fails each write in turn and requires an error out of every one. A
// swallowed write is then visible wherever it is, and a write added later is
// covered the day it lands rather than the day someone remembers.
func TestCH6_EveryWriteInTheReportHasItsErrorReturned(t *testing.T) {
	src := filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("no transcript fixture in this tree: %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	// A home carrying a second agent, so the note naming what this total leaves
	// out is written rather than skipped. Its write is the one the reviewer
	// reported UNREACHED.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	codex := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(codex, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codex, "r.jsonl"), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// How many writes a clean run makes, so the sweep covers all of them.
	counter := &failOnCall{n: -1}
	var stderr bytes.Buffer
	if err := run([]string{"cost", dir}, counter, &stderr); err != nil {
		t.Fatalf("cost on the fixture failed: %v (stderr %q)", err, stderr.String())
	}
	total := counter.calls
	t.Logf("the report makes %d writes", total)
	if total < 2 {
		t.Fatalf("the report made %d write(s); this test asserts nothing", total)
	}

	for i := 0; i < total; i++ {
		stderr.Reset()
		w := &failOnCall{n: i}
		if err := run([]string{"cost", dir}, w, &stderr); err == nil {
			t.Errorf("write %d of %d failed and cost reported success: that write's error "+
				"is discarded, so the report can stop there and still exit 0", i, total)
		}
	}
}
