package main

import (
	"bytes"
	"strings"
	"testing"
)

// Confirming a run that spends money.
//
// `replay probe --execute` is the only command here that creates billable
// traffic. Everything else reads files or forwards what an agent already sent.
// A command that spends should not do it because the plan scrolled past.
//
//	C1  only an unambiguous yes proceeds
//	C2  anything else refuses, including a bare y
//	C3  no answer at all refuses, and says why
//	C4  --yes skips the prompt without reading anything
//	C5  the prompt states what is about to be spent

func TestC1_AnUnambiguousYesProceeds(t *testing.T) {
	// PASS: "yes" in any case, with surrounding whitespace, proceeds.
	// FAIL: refusing a clear yes, which makes the command unusable.
	for _, in := range []string{"yes\n", "YES\n", "  yes  \n", "Yes\n"} {
		var out bytes.Buffer
		if !confirmSpend(strings.NewReader(in), &out, "10 requests", false) {
			t.Errorf("%q must proceed", in)
		}
	}
}

func TestC2_AnythingElseRefuses(t *testing.T) {
	// PASS: refused.
	// FAIL: proceeding on an ambiguous answer. "y" is excluded deliberately —
	// it is one keystroke from a reflex, and this spends the operator's money
	// at their provider. Typing the word is the point.
	for _, in := range []string{"y\n", "no\n", "n\n", "\n", "sure\n", "1\n", "yes please\n", "ye\n"} {
		var out bytes.Buffer
		if confirmSpend(strings.NewReader(in), &out, "10 requests", false) {
			t.Errorf("%q must not proceed", in)
		}
	}
}

func TestC3_NoAnswerRefusesAndSaysWhy(t *testing.T) {
	// A pipe, a cron job, a CI step: stdin closes immediately. Reading EOF as
	// consent would make every non-interactive invocation spend money.
	// PASS: refused, and the output names --yes as the deliberate way to run
	// unattended.
	// FAIL: proceeding, or refusing without saying how to proceed on purpose.
	var out bytes.Buffer
	if confirmSpend(strings.NewReader(""), &out, "10 requests", false) {
		t.Error("end of input must never be read as consent")
	}
	if !strings.Contains(out.String(), "--yes") {
		t.Errorf("the refusal must name the flag for running unattended; got:\n%s", out.String())
	}
	// The wording has to distinguish "you did not answer" from "you said no".
	// Both mention --yes, so asserting only that let the no-answer branch be
	// deleted with nothing failing. A script author seeing "Not confirmed"
	// looks for the answer their script gave; seeing "No answer" they look for
	// the stdin they never attached.
	if !strings.Contains(out.String(), "No answer") {
		t.Errorf("end of input must be reported as no answer, not as a refusal; got:\n%s", out.String())
	}
}

func TestC4_TheYesFlagSkipsThePromptEntirely(t *testing.T) {
	// PASS: proceeds without consuming input and without prompting.
	// FAIL: reading stdin anyway, which hangs a script that passed --yes
	// precisely so it would not be asked.
	var out bytes.Buffer
	r := &countingReader{}
	if !confirmSpend(r, &out, "10 requests", true) {
		t.Error("--yes must proceed")
	}
	if r.reads != 0 {
		t.Errorf("--yes read stdin %d time(s); it must not read at all", r.reads)
	}
	if out.Len() != 0 {
		t.Errorf("--yes must not prompt; got %q", out.String())
	}
}

func TestC5_ThePromptStatesWhatIsBeingSpent(t *testing.T) {
	// PASS: the budget appears in the prompt.
	// FAIL: asking "are you sure?" with nothing to be sure about. A
	// confirmation that does not say what it is confirming trains people to
	// type yes without reading, which is worse than no prompt at all.
	var out bytes.Buffer
	confirmSpend(strings.NewReader("no\n"), &out, "10 billable requests", false)
	s := out.String()
	if !strings.Contains(s, "10 billable requests") {
		t.Errorf("the prompt must say what is about to be spent; got:\n%s", s)
	}
	if !strings.Contains(strings.ToLower(s), "yes") {
		t.Errorf("the prompt must say what answer proceeds; got:\n%s", s)
	}
}

type countingReader struct{ reads int }

func (c *countingReader) Read(p []byte) (int, error) {
	c.reads++
	return 0, nil
}
