package main

import (
	"bytes"
	"strings"
	"testing"
)

// burn reports each surface on its own terms and refuses to add them up.
//
// The three metered surfaces do not measure the same quantity. A Claude Code
// token is billed or drawn against a subscription; a Codex token is drawn
// against a rate-limit window that the client will actually tell you about; an
// Ollama token is compute on this machine and is drawn against nothing. Their
// headline figures do not even agree on whether the cached prefix is inside
// them: Anthropic partitions it out, Codex nests it in, Ollama excludes it
// entirely and reports work rather than context.
//
// So a single grand total would be a number with no unit. This asserts the
// tool does not print one.
func TestBurnDoesNotSumSurfacesThatDoNotShareAUnit(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", "burndata"}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	got := out.String()
	for _, banned := range []string{"grand total", "combined total", "all surfaces total"} {
		if strings.Contains(strings.ToLower(got), banned) {
			t.Errorf("printed a %q across surfaces whose tokens are different quantities:\n%s",
				banned, got)
		}
	}
	// Each surface must be named with what it can answer, so a reader knows
	// which figure is a bill, which is a quota, and which is neither.
	for _, want := range []string{"claude-code", "codex", "ollama"} {
		if !strings.Contains(got, want) {
			t.Errorf("surface %q missing from the report:\n%s", want, got)
		}
	}
}

// The quota column tells the truth about which surface will answer.
//
// Only Codex reports a live quota reading. Anthropic's counter did not move
// under 3.09M tokens of titration, and Ollama has no quota at all. Printing a
// blank or a zero for the two that cannot answer would read as "nothing used".
func TestBurnSaysWhichSurfacesCannotReportQuota(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", "burndata"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "not reported") && !strings.Contains(got, "none") {
		t.Errorf("a surface with no quota signal must say so rather than show a blank:\n%s", got)
	}
}
