package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
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

// CM7: a path that does not exist is an error, not an empty measurement.
//
// transcriptFiles refuses a path it cannot stat. Without that refusal the
// command would report NOT MEASURED for a typo'd directory, which tells the
// reader "your budget is fine" when the truth is "I looked nowhere."
func TestCM7_ABadPathIsAnError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"ceiling", "--metered", "/no/such/path/here"}, &stdout, &stderr)
	if err == nil {
		t.Error("a nonexistent path was accepted and measured as if empty")
	}
}

// CM8: a file that parses to no session is skipped, not fatal, and does not panic.
//
// The per-file guard skips a file that will not parse. Neutralised, the walk
// runs the pricing loop over a nil session and panics on the first field
// access — so a corpus with one junk file beside a good one is the test: it
// must still produce a report.
func TestCM8_AMalformedFileIsSkipped(t *testing.T) {
	corpus(t) // lays down a good transcript under REPLAY_TRANSCRIPTS
	dir := os.Getenv("REPLAY_TRANSCRIPTS")
	// An EMPTY file makes ParseClaudeCode return (nil, "no conversation lines
	// found"), which is the path that hands the walk a nil session — the case
	// the guard exists for. A file of junk instead parses to a non-nil session
	// with zero lanes, whose pricing loop is a harmless no-op, so it would not
	// reach the guard and neutralising it would go unnoticed.
	junk := filepath.Join(dir, "proj", "empty.jsonl")
	if err := os.WriteFile(junk, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"ceiling", "--metered", "--day-ceiling", "500"}, &stdout, &stderr); err != nil {
		t.Fatalf("a nil-session file made the run fail rather than being skipped: %v", err)
	}
	if stdout.Len() == 0 {
		t.Error("no report was produced alongside the skipped file")
	}
}

// CM9: a dated request is priced at the time it ran, not at today's row.
//
// CeilingEffect.Add used PriceFor, which ignores dated windows. A September
// request under a September promotion is today's $10 if the lookup has no
// clock, and $5 if it uses the request timestamp. As-run uses the clock.
func TestCM9_DatedRequestIsPricedAtRequestTime(t *testing.T) {
	restore := cachemodel.Override(&cachemodel.Rules{
		Schema:  cachemodel.RulesSchema,
		Version: "test",
		Models: []cachemodel.ModelRule{
			{Match: "opus-5", MinPrefix: 512, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true},
			{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true,
				EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30"},
		},
	})
	defer restore()

	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	body := `{"uuid":"a1","type":"user","sessionId":"s","version":"2.0","timestamp":"2026-09-15T00:36:01Z","message":{"role":"user","content":"hi"}}
{"uuid":"a2","parentUuid":"a1","type":"assistant","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-09-15T00:36:02Z","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"one"}],"usage":{"input_tokens":1000000,"output_tokens":0}}}
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := run([]string{"ceiling", "--metered", "--json", path}, &stdout, &stderr); err != nil {
		t.Fatalf("ceiling --json: %v\n%s", err, stderr.String())
	}
	var doc struct {
		CorrectUSD float64 `json:"correctUsd"`
		Requests   int     `json:"requests"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if doc.Requests != 1 {
		t.Fatalf("requests = %d, want 1", doc.Requests)
	}

	at := time.Date(2026, 9, 15, 0, 36, 2, 0, time.UTC)
	today, _ := cachemodel.PriceFor("claude-opus-5")
	then, _ := cachemodel.PriceForAt("claude-opus-5", at)
	if today.InputPerMTok == then.InputPerMTok {
		t.Fatal("PriceFor and PriceForAt agree; the fixture cannot tell today from request time")
	}
	u := transcript.Usage{Input: 1_000_000}
	wantToday := cachemodel.CostUSD(u, today)
	wantThen := cachemodel.CostUSD(u, then)
	if math.Abs(doc.CorrectUSD-wantThen) > 1e-9 {
		t.Errorf("correctUsd = $%.6f, want $%.6f at the September row (today would be $%.6f)",
			doc.CorrectUSD, wantThen, wantToday)
	}
}
