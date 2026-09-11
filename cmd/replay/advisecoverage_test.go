package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// The report must say how much of itself the verifier cannot reach.
//
// Measured on this machine on 2026-09-11: ~/.replay/advice.json held 141
// suggestions — 118 "advice only", 23 "pending", and none ever applied,
// verified or dismissed in 28 days. The 118 is exactly KindHotFile plus
// KindCacheBreaks, which advisor.track returns AdviceOnly for unconditionally,
// before any other logic runs.
//
// So 83.7% of the tool's output sits outside its own verifier by construction,
// and nothing said so. AdviceOnly is an honest label — a hot file's share is
// not something a reader "applies a fix" to in a way the tool can observe —
// but a reader looking at 141 suggestions and a verifier has no way to learn
// that 118 of them will never be marked verified however long they wait.
//
// The footer listed the statuses and stopped: "Statuses: pending, applied,
// verified, not verified, advice only." A list of names is not a coverage
// figure.

// AC1: when advice-only suggestions are present, the report counts them.
func TestAC1_TheReportSaysHowMuchTheVerifierCannotReach(t *testing.T) {
	corpus(t)
	dir := os.Getenv("REPLAY_TRANSCRIPTS")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"advise", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("advise: %v\n%s", err, stderr.String())
	}
	out := stdout.String() + stderr.String()
	// Bracketed: the footer lists the status names, so a bare match
	// would find the word in a sentence about vocabulary and skip nothing.
	if !strings.Contains(out, "[advice only]") {
		t.Skip("this corpus produced no advice-only suggestion; AC1 has nothing to measure")
	}
	if !strings.Contains(out, "never be marked verified") {
		t.Errorf("the report names advice-only suggestions and does not say they can "+
			"never be verified, so a reader waits for a status that is not coming:\n%s", out)
	}
}

// AC2: the count is a count, not a word.
//
// "some of these cannot be verified" is the shape of a sentence that gets
// skipped. A reader deciding whether to trust the verifier needs to know it
// covers a sixth of the output, and only a number carries that.
func TestAC2_TheCoverageIsANumber(t *testing.T) {
	corpus(t)
	dir := os.Getenv("REPLAY_TRANSCRIPTS")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"advise", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("advise: %v\n%s", err, stderr.String())
	}
	out := stdout.String() + stderr.String()
	if !strings.Contains(out, "[advice only]") {
		t.Skip("no advice-only suggestion in this corpus")
	}
	line := coverageLine(out)
	if line == "" {
		t.Fatalf("no coverage line found in:\n%s", out)
	}
	if !strings.ContainsAny(line, "0123456789") {
		t.Errorf("the coverage sentence carries no figure, so it reads as a caveat "+
			"rather than a measurement: %q", line)
	}
}

// coverageLine returns the line that explains advice-only coverage.
func coverageLine(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "never be marked verified") {
			return l
		}
	}
	return ""
}
