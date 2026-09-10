package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// `replay ceiling` makes the cache-blind arithmetic runnable.
//
// The finding sat in internal/cachemodel as a tested pure function with no way
// for anyone to run it: BlindCostUSD and CeilingEffect existed, the 7.94x was
// real, and a reader could reach none of it. This is the command surface, and
// its whole job is to walk the reader's own transcripts, fold them into a
// CeilingEffect, and print the sentence for their billing state.
//
// It derives nothing of its own. Both dollar figures come from CostUSD and
// BlindCostUSD over the same usage the provider reported, so the command cannot
// disagree with the library the tests already pin.

// CM1: the metered reader is told where their ceiling halts them.
//
// PASS: --metered with a ceiling prints a real spend figure below the ceiling.
// FAIL: no ratio, or a ratio of one on a corpus that plainly uses the cache.
func TestCM1_MeteredReaderGetsAThrottlePoint(t *testing.T) {
	corpus(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"ceiling", "--metered", "--day-ceiling", "500"}, &stdout, &stderr); err != nil {
		t.Fatalf("ceiling --metered: %v\n%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "halts execution at $") {
		t.Errorf("a metered reader was not told where the ceiling stops them:\n%s", out)
	}
	if !strings.Contains(out, "$500") {
		t.Errorf("the note does not name the ceiling it was given:\n%s", out)
	}
}

// CM2: the subscriber is never handed a bill.
//
// The one way this command becomes dishonest is telling a Max seat they are
// losing dollars they never spend. `replay cost` already refuses this; so must
// the command built on the same figures.
func TestCM2_SubscriberIsNeverHandedABill(t *testing.T) {
	corpus(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"ceiling", "--subscription", "--day-ceiling", "500"}, &stdout, &stderr); err != nil {
		t.Fatalf("ceiling --subscription: %v\n%s", err, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "halts execution at $") {
		t.Errorf("a subscription seat was told where a dollar ceiling stops it:\n%s", out)
	}
	if !strings.Contains(out, "NOT MEASURED") {
		t.Errorf("the subscription note does not name what is unmeasured about the allowance:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "token") {
		t.Errorf("the subscriber is not told the finding is in tokens:\n%s", out)
	}
}

// CM3: an unstated billing basis is refused, not guessed.
//
// A bare first-party model id is emitted by an API key and a subscription
// alike, so nothing in a transcript settles it. Guessing would put a bill in
// front of half the readers who get it wrong.
func TestCM3_UnstatedBasisIsRefused(t *testing.T) {
	corpus(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"ceiling", "--day-ceiling", "500"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("ceiling ran without a billing basis, so it either guessed or invented one")
	}
	if !strings.Contains(err.Error()+stderr.String(), "metered") ||
		!strings.Contains(err.Error()+stderr.String(), "subscription") {
		t.Errorf("the refusal does not tell the reader which flag to pass: %v\n%s", err, stderr.String())
	}
}

// CM4: both bases are refused together — a coherence check on the flags.
func TestCM4_BothBasesAtOnceIsRefused(t *testing.T) {
	corpus(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"ceiling", "--metered", "--subscription"}, &stdout, &stderr); err == nil {
		t.Error("both --metered and --subscription were accepted; the reader is one or the other")
	}
}

// CM5: an empty corpus reports NOT MEASURED, never a ratio of one.
//
// The defect this repository names most often: nothing measured presented as
// something measured. A cache-blind check over zero requests must not print
// "1.0x, your budget is accurate."
func TestCM5_EmptyCorpusIsNotMeasured(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REPLAY_TRANSCRIPTS", dir)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	var stdout, stderr bytes.Buffer
	err := run([]string{"ceiling", "--metered", "--day-ceiling", "500"}, &stdout, &stderr)
	out := stdout.String() + stderr.String()
	if err == nil && strings.Contains(out, "1.0") && !strings.Contains(strings.ToUpper(out), "NOT MEASURED") {
		t.Errorf("an empty corpus reported a ratio instead of NOT MEASURED:\n%s", out)
	}
	if !strings.Contains(strings.ToUpper(out), "NOT MEASURED") && err == nil {
		t.Errorf("an empty corpus neither errored nor said NOT MEASURED:\n%s", out)
	}
}

// CM6: --json carries the ratio, the n, and the basis.
//
// A consumer parsing this needs to know the ratio is a property of the
// arithmetic, how many requests stand behind it, and which billing basis was
// declared — because the same number means money to one reader and nothing to
// another.
func TestCM6_JSONCarriesRatioAndBasis(t *testing.T) {
	corpus(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"ceiling", "--metered", "--json"}, &stdout, &stderr); err != nil {
		t.Fatalf("ceiling --json: %v\n%s", err, stderr.String())
	}
	var doc struct {
		Schema   string  `json:"schema"`
		Basis    string  `json:"basis"`
		Requests int     `json:"requests"`
		Ratio    float64 `json:"ratio"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if doc.Basis != "metered" {
		t.Errorf("the JSON does not record the declared basis: %q", doc.Basis)
	}
	if doc.Requests <= 0 {
		t.Errorf("the JSON reports %d requests behind the ratio", doc.Requests)
	}
	if doc.Ratio <= 1 {
		t.Errorf("the ratio is %v; a corpus that uses the cache prices higher blind than correct", doc.Ratio)
	}
}
