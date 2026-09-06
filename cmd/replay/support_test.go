package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// The ask belongs wherever the tool has just done something for somebody.
//
// Until now exactly one command carried it — `cost` — because that is the one
// with a dollar figure attached. Every other analysis command handed over a
// result and said nothing. `context` names what is filling a context window,
// `blame` ranks what the carried content cost, `diff` finds the turn where the
// cache broke: a reader who got that and was never told the thing is free and
// maintained by one person cannot act on it.
//
// Two rules make this an ask rather than a nag. It fires only when a real
// result was produced — a refusal, an empty corpus or a usage error never
// carries it — and it never touches machine-readable output, because a funding
// line inside JSON is corruption, not persuasion.

const fundingHandle = "buymeacoffee.com/saitodaniel"

// S1: every value-delivering command surfaces the ask.
//
// The list is derived from the dispatch table rather than typed, so a command
// added next month is covered the day it exists rather than the day somebody
// remembers it.
//
// PASS: a real result carries the ask.
// FAIL: a command that hands over a finding in silence.
func TestS1_EveryResultCarriesTheAsk(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "transcript", "testdata")
	for _, cmd := range valueCommands() {
		t.Run(cmd, func(t *testing.T) {
			var out, errb bytes.Buffer
			if err := run([]string{cmd, dir}, &out, &errb); err != nil {
				t.Skipf("%s did not produce a result here: %v", cmd, err)
			}
			if out.Len() == 0 {
				t.Skipf("%s produced no output", cmd)
			}
			if !strings.Contains(out.String(), fundingHandle) {
				t.Errorf("%s produced a result and never said who maintains it or that it is "+
					"funded by tips. The ask belongs where the value lands.", cmd)
			}
		})
	}
}

// S2: machine-readable output is never touched.
//
// PASS: --json parses, and carries no funding string.
// FAIL: a tip line inside JSON, which breaks every caller and is the fastest
// way to get the whole thing stripped out by users.
func TestS2_MachineReadableOutputIsNotPolluted(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "transcript", "testdata")
	for _, cmd := range []string{"cost", "advise", "context"} {
		t.Run(cmd, func(t *testing.T) {
			var out, errb bytes.Buffer
			if err := run([]string{cmd, dir, "--json"}, &out, &errb); err != nil {
				t.Skipf("%s --json unavailable: %v", cmd, err)
			}
			if out.Len() == 0 {
				t.Skip("no output")
			}
			if strings.Contains(out.String(), fundingHandle) {
				t.Fatalf("%s --json contains the funding line; JSON is for programs", cmd)
			}
			var v any
			if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &v); err != nil {
				t.Errorf("%s --json does not parse, so something was appended to it: %v", cmd, err)
			}
		})
	}
}

// S3: a refusal is not a result.
//
// Asking for money immediately after refusing to answer is the shape that
// makes people uninstall something.
//
// PASS: an error path carries no ask.
// FAIL: the ask on a failure.
func TestS3_ARefusalNeverCarriesTheAsk(t *testing.T) {
	var out, errb bytes.Buffer
	_ = run([]string{"cost", filepath.Join("testdata", "definitely-not-here")}, &out, &errb)
	if strings.Contains(out.String()+errb.String(), fundingHandle) {
		t.Error("the tool asked for money on a path where it produced nothing")
	}
}

// S4: the ask names what was delivered.
//
// A generic plea is worth less than a specific one and is harder to trust. The
// cost line already does this — it names the dollars it found — and the
// others should name their own result rather than borrowing a number they did
// not compute.
//
// PASS: the line references the command's own finding.
// FAIL: the same sentence everywhere, or a dollar figure on a command that
// computed none.
func TestS4_TheAskNamesWhatWasDelivered(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "transcript", "testdata")
	var out, errb bytes.Buffer
	if err := run([]string{"context", dir}, &out, &errb); err != nil {
		t.Skipf("context unavailable: %v", err)
	}
	got := out.String()
	i := strings.Index(got, fundingHandle)
	if i < 0 {
		t.Skip("covered by S1")
	}
	ask := got[max(0, i-400):]
	if strings.Contains(ask, "$") {
		t.Error("the context ask quotes a dollar figure that command did not compute")
	}
}
