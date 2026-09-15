package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// burn answered a question it was not asked.
//
// `replay cost <dir>` takes a positional path. burn takes `-dir` and nothing
// positional, and its flag set discarded extra arguments without a word. So
// `replay burn ~/.codex/sessions/2026/09` parsed cleanly, scanned the
// machine's own surfaces instead, and printed a machine-wide total under a
// heading the reader believes is scoped to the path they typed.
//
// Measured on this machine 2026-09-15: that invocation and `replay burn` with
// no argument and `replay burn` against an EMPTY directory all printed the
// identical 6,751 codex requests and 610,551,532 tokens. Three different
// questions, one answer, no indication that two of them were ignored.
//
// That is worse than an error. Every figure this tool prints is supposed to
// name the population it was read from, and this one named a directory it
// never opened.
func TestBurnRefusesAPositionalPathRatherThanIgnoringIt(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"burn", "burndata"}, &out, &errOut)

	if err == nil {
		t.Fatalf("burn accepted a positional path and reported anyway; "+
			"the figures below describe the machine, not the path asked for:\n%s", out.String())
	}
	if !errors.Is(err, errUsage) {
		t.Errorf("burn refused with %v, which does not wrap errUsage, so the exit code will be wrong", err)
	}
	if strings.Contains(out.String(), "per surface") {
		t.Errorf("burn printed its report while refusing; a refusal that still prints a total "+
			"is read as a total:\n%s", out.String())
	}
}

// The refusal has to name the spelling that works, or the reader is left
// guessing which of the two conventions this command uses. Naming it is the
// difference between a refusal and a wall.
func TestBurnRefusalNamesTheDirFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"burn", "burndata"}, &out, &errOut)

	// The refusal travels as the returned error, which main prints. Reading
	// only the buffers here would have passed a command that refused in
	// silence.
	said := errOut.String() + out.String()
	if err != nil {
		said += err.Error()
	}
	if !strings.Contains(said, "-dir") {
		t.Errorf("refusal never names -dir, so it says what is wrong without saying what is right:\n%s", said)
	}
}

// The guard must not cost the flag its job.
func TestBurnStillReadsTheDirFlag(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"burn", "--dir", "burndata"}, &out, &errOut); err != nil {
		t.Fatalf("-dir stopped working: %v\n%s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "codex") {
		t.Errorf("-dir parsed but read nothing:\n%s", out.String())
	}
}
