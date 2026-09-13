package main

import (
	"bytes"
	"strings"
	"testing"
)

// A build that fails on re-billed spend.
//
// The gate reads the figure `replay cost` already computes and compares it to a
// ceiling the caller types. It derives nothing of its own: a governance check
// whose numbers come from anywhere but the measurement is a check that can be
// right about a corpus nobody ran.
//
// Three refusals shape it, and all three are the same refusal in different
// clothes: a threshold is only meaningful against a number that was measured.
//
//   - A corpus that priced nothing must not pass. `replay cost` exits 0 on an
//     empty corpus and says so in prose, so a gate that only watched the exit
//     code would go green on a CI runner with no transcripts and report a clean
//     bill of health nobody earned.
//   - Unpriced models are excluded from the total rather than counted as free,
//     so a ceiling compared against a total with exclusions has to say how many.
//   - Zero means off, the same convention `serve --max-day-usd` uses.

// CG1: spend over the ceiling fails the build.
func TestCG1_OverTheCeilingFails(t *testing.T) {
	corpus(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"cost", "--max-rebilled-usd", "0.01"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("re-billed spend above the ceiling exited 0, so a CI gate built on it " +
			"would pass the one build it exists to stop")
	}
	out := stdout.String() + stderr.String() + err.Error()
	if !strings.Contains(out, "0.01") {
		t.Errorf("the failure does not name the ceiling it breached:\n%s", out)
	}
}

// CG2: spend under the ceiling passes, and still prints the report.
//
// A gate that swallows the report to show a verdict has taken away the thing
// the reader needed to act on it.
func TestCG2_UnderTheCeilingPasses(t *testing.T) {
	corpus(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"cost", "--max-rebilled-usd", "10000"}, &stdout, &stderr); err != nil {
		t.Fatalf("spend well under the ceiling failed the build: %v", err)
	}
	if !strings.Contains(stdout.String(), "re-billed") {
		t.Error("the cost report was suppressed by the gate")
	}
}

// CG3: a corpus that priced nothing does not pass.
//
// The defect this whole repository keeps finding. `replay cost` exits 0 with no
// transcripts, so a ceiling check that trusted the exit code would go green on
// a runner that measured nothing, and a green check that measured nothing is
// worse than a red one.
func TestCG3_NothingMeasuredIsNotAPass(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REPLAY_TRANSCRIPTS", dir)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	var stdout, stderr bytes.Buffer
	err := run([]string{"cost", "--max-rebilled-usd", "5"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("a gate over zero priced tasks passed; nothing was measured, so there is " +
			"no spend to be under a ceiling")
	}
	out := stdout.String() + stderr.String() + err.Error()
	if !strings.Contains(strings.ToUpper(out), "NOT MEASURED") &&
		!strings.Contains(strings.ToLower(out), "priced nothing") {
		t.Errorf("the refusal does not say that nothing was measured:\n%s", out)
	}
}

// CG4: a negative ceiling is refused; zero means off.
//
// The first version of this test demanded that zero be refused too, on the
// theory that an unset variable would expand to it. That was wrong twice: an
// unset variable expands to empty and fails at the flag parser, not to zero,
// and `0 = off` is this repository's own convention — `serve --max-day-usd`
// documents exactly that. A gate with its own private meaning for zero would be
// the surprise, not the safeguard.
func TestCG4_ANegativeCeilingIsRefused(t *testing.T) {
	corpus(t)
	for _, v := range []string{"-1", "-0.5"} {
		var stdout, stderr bytes.Buffer
		err := run([]string{"cost", "--max-rebilled-usd", v}, &stdout, &stderr)
		if err == nil {
			t.Errorf("--max-rebilled-usd %s was accepted; an unset variable expands to "+
				"empty and would silently fail every build", v)
		}
	}
}

// CG5: without the flag, nothing changes.
//
// The gate is opt-in. A cost report that started failing builds because someone
// upgraded would be a breaking change delivered as a patch.
func TestCG5_AbsentFlagChangesNothing(t *testing.T) {
	corpus(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"cost"}, &stdout, &stderr); err != nil {
		t.Fatalf("plain cost failed with no gate configured: %v", err)
	}
}
