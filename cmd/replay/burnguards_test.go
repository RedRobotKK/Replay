package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Five conditionals in burn.go decided something a reader acts on, and no test
// could tell whether they ran.
//
// Each was confirmed by neutralisation before a line of this file was written:
// the condition forced false, `go test ./cmd/replay` still green. That is the
// verdict guard-reachability returns, and it says nothing about whether the
// branch deserves to exist — so each one below is argued for on what it does on
// a real machine, not on the fact that a tool named it.

// BG-1: a burn rate is reported only over a window that was actually observed.
//
// perHour divides tokens by the span between the first and last request seen.
// A surface that recorded neither end has no span, and one that recorded only
// the LAST end has a `first` of the zero time — January of year 1 — so the
// division succeeds and returns a rate averaged over two millennia. It comes
// out as a plausible-looking small number under a heading that says
// "tokens/hour over the window observed", which is the shape of wrong figure
// this report exists to stop printing.
//
// The h <= 0 check below it does not cover that case: a zero `first` with a
// real `last` is a large POSITIVE span. Deleting either check leaves a hole,
// which is why both are here.
//
// PASS: no rate unless both ends are real and apart.
// FAIL: a rate over a window nothing measured.
func TestBG1_ABurnRateNeedsBothEndsOfAWindow(t *testing.T) {
	seen := time.Date(2026, 3, 21, 0, 36, 1, 0, time.UTC)

	for _, c := range []struct {
		name string
		s    surfaceBurn
	}{
		{"neither end recorded", surfaceBurn{tokens: 1000}},
		{"only the last request recorded", surfaceBurn{tokens: 1000, last: seen}},
		{"only the first request recorded", surfaceBurn{tokens: 1000, first: seen}},
		{"one request, so the window has no width", surfaceBurn{tokens: 1000, first: seen, last: seen}},
	} {
		if r, ok := c.s.perHour(); ok {
			t.Errorf("%s: reported %g tokens/hour. There is no window here to divide by; "+
				"a zero timestamp is not a time, it is the absence of one.", c.name, r)
		}
	}

	// And where the window is real, the rate is the plain quotient. Asserted on
	// the value rather than on ok alone: a guard that returns true with a wrong
	// number is not fixed by a test that only checks it returned true.
	r, ok := surfaceBurn{tokens: 3000, first: seen, last: seen.Add(2 * time.Hour)}.perHour()
	if !ok {
		t.Fatalf("3000 tokens over two observed hours reports no rate")
	}
	if r != 1500 {
		t.Errorf("perHour = %g over a two-hour window holding 3000 tokens, want 1500", r)
	}
}

// BG-2: `replay burn` refuses an argument it does not understand.
//
// The flag is the one that redirects every surface reader away from this
// machine. A misspelling of it that parses as "no --dir given" does not fail,
// it succeeds against the operator's own corpus and prints a report of the
// wrong machine — and the report does not say which machine it read. So the
// parse error has to end the command rather than be noted and stepped over.
//
// PASS: an error back to the caller, and no report on stdout.
// FAIL: the table, computed from whatever the unparsed flags left behind.
func TestBG2_BurnRefusesAFlagItDoesNotKnow(t *testing.T) {
	// The failure mode under test is reading this machine's real corpus, so
	// the test must not be able to do that even when it fails.
	isolateHome(t, t.TempDir())
	t.Setenv(transcriptsEnv, "")

	var out, errOut bytes.Buffer
	err := run([]string{"burn", "--not-a-flag"}, &out, &errOut)
	if err == nil {
		t.Errorf("`replay burn --not-a-flag` returned no error")
	}
	if out.Len() != 0 {
		t.Errorf("a rejected command still printed a report:\n%s", out.String())
	}
}

// BG-3: the codex quota column reports what the rollout actually said.
//
// Codex is the only surface that reports a live quota reading, which is the
// entire reason the column exists on the other two rows. The reading is carried
// on a token_count event and is absent from rollouts that never received one,
// so the assignment is conditional — and with the condition removed the column
// falls back to the word "reported", which is a true statement about the
// surface and tells the operator nothing about their remaining window.
//
// Compared against the parser rather than against a literal, so a fixture that
// changes its numbers does not need this test edited to keep agreeing with it.
//
// PASS: the percentage and window the rollout carries.
// FAIL: the placeholder, or a reading invented where the rollout had none.
func TestBG3_TheCodexQuotaColumnReportsTheRolloutsReading(t *testing.T) {
	files, err := filepath.Glob("burndata/codex/*.jsonl")
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one codex rollout in burndata, got %v (%v)", files, err)
	}
	parsed, err := transcript.ParseCodexFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Quota == nil {
		t.Fatalf("the fixture carries no quota, so this test cannot tell a read " +
			"reading from an unread one")
	}
	want := fmt.Sprintf("%.0f%% of %s",
		parsed.Quota.PrimaryUsedPercent, minutes(parsed.Quota.PrimaryWindowMinutes))

	got := burnCodex("", "burndata")
	if got.quota != want {
		t.Errorf("codex quota = %q, want %q — the rollout reports a live window and "+
			"the column is not showing it", got.quota, want)
	}

	// The same corpus with the quota event removed must not acquire one, and
	// must not show a figure it does not have.
	bare := t.TempDir()
	stripCodexQuota(t, files[0], bare)
	none := burnCodex("", bare)
	if none.sessions != 1 {
		t.Fatalf("the stripped rollout was not read: %d session(s)", none.sessions)
	}
	if none.quota == want || strings.Contains(none.quota, "%") {
		t.Errorf("a rollout carrying no rate-limit event reports quota %q", none.quota)
	}
}

// stripCodexQuota copies a rollout into dir/codex with its rate-limit event
// dropped, giving a corpus identical in every respect except the reading.
func stripCodexQuota(t *testing.T, src, dir string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	dropped := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "rate_limits") {
			dropped++
			continue
		}
		kept = append(kept, line)
	}
	if dropped == 0 {
		t.Fatalf("no rate-limit event found in %s to strip", src)
	}
	out := filepath.Join(dir, "codex")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, filepath.Base(src)), []byte(strings.Join(kept, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
}

// BG-4: --dir reads the directory it was given and nothing else.
//
// burnClaudeCode returns early when a directory was named, and under the test
// suite's empty temp HOME removing that return changes no output at all — which
// is exactly why it went unobserved, and exactly why it is not redundant. On a
// machine with a real ~/.claude the early return is the only thing standing
// between `replay burn --dir <fixture>` and the operator's own transcripts:
// without it the claude-code row silently reports their private corpus under a
// heading that says it read the fixture. Somebody demonstrating the tool from a
// checkout would be reading their own sessions to a room.
//
// So the HOME here is not empty. It holds a parseable transcript, and the
// control below proves it is found when nothing redirects the reader.
//
// PASS: nothing read, and the row says so rather than showing a figure.
// FAIL: the HOME corpus, reported as though it came from --dir.
func TestBG4_ADirectoryFixtureDoesNotReadTheOperatorsOwnCorpus(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "session.jsonl"),
		[]byte(twoModelTranscript), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(transcriptsEnv, "")

	// Control. Without this, "read nothing" is what an empty HOME reports too,
	// and the assertion below would pass over a deleted guard.
	if live := burnClaudeCode(home, ""); live.requests == 0 {
		t.Fatalf("the HOME corpus is not being found at all, so this test cannot " +
			"tell a redirected reader from a missing one")
	}

	elsewhere := t.TempDir()
	s := burnClaudeCode(home, elsewhere)
	if s.requests != 0 || s.sessions != 0 {
		t.Errorf("--dir %s reported %d request(s) across %d session(s); the only "+
			"transcripts on this machine are in HOME, so that is where they came from",
			elsewhere, s.requests, s.sessions)
	}
	if got := costCell(s); got != "not read" {
		t.Errorf("the claude-code cost cell reads %q for a surface that was not "+
			"read at all", got)
	}
}

// BG-5: Claude Code spend is priced per request, and says how much of the
// surface the figure covers.
//
// A transcript is not one model. Sessions switch models mid-run and subagents
// run on cheaper ones, so pricing the surface as a whole would apply one row to
// requests it does not describe. Priced per request, a model no installed rules
// document names is counted as unpriced rather than as free — excluded and
// disclosed, which is the rule this report already keeps one column to the left
// for tokens.
//
// The corpus here is one request on a model the compiled table prices and one
// on a model nothing prices, so the total is real and incomplete at the same
// time, and the cell has to say both.
//
// PASS: the priced request's exact cost, and a coverage share below 100%.
// FAIL: nothing priced, or a partial total presented as the whole bill.
func TestBG5_ClaudeCodeSpendIsPricedPerRequestAndSaysWhatItMissed(t *testing.T) {
	if _, ok := cachemodel.PriceFor(pricedModel); !ok {
		t.Fatalf("%s is not priced by the compiled table; the fixture cannot show a "+
			"priced request beside an unpriced one", pricedModel)
	}
	if _, ok := cachemodel.PriceFor(unpricedModel); ok {
		t.Fatalf("%s is priced after all; with both models priced this test cannot "+
			"tell a per-request price from a per-surface one", unpricedModel)
	}

	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(proj, "session.jsonl")
	if err := os.WriteFile(path, []byte(twoModelTranscript), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(transcriptsEnv, "")

	// The expected figure comes from the parser's own record of the request, so
	// a fixture whose usage numbers change stays in agreement with it.
	sess, err := transcript.ParseClaudeCodeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want float64
	var found int
	for _, lane := range sess.Lanes {
		for _, r := range lane.Requests {
			if r.Model != pricedModel {
				continue
			}
			p, _ := cachemodel.PriceFor(r.Model)
			want += cachemodel.CostUSD(r.Usage, p)
			found++
		}
	}
	if found != 1 || want <= 0 {
		t.Fatalf("the fixture holds %d request(s) on %s costing $%.6f; it needs "+
			"exactly one, costing something", found, pricedModel, want)
	}

	s := burnClaudeCode(home, "")
	if s.pricedReqs != 1 || s.unpricedReqs != 1 {
		t.Fatalf("priced %d / unpriced %d request(s), want 1 and 1: one request ran "+
			"on %s and one on %s", s.pricedReqs, s.unpricedReqs, pricedModel, unpricedModel)
	}
	if math.Abs(s.costUSD-want) > 1e-9 {
		t.Errorf("costUSD = %.6f, want %.6f at the row in force for %s",
			s.costUSD, want, pricedModel)
	}
	got := costCell(s)
	if !strings.Contains(got, fmt.Sprintf("$%.2f", want)) {
		t.Errorf("the cost cell %q does not carry the priced request's cost of $%.2f", got, want)
	}
	if !strings.Contains(got, "(50%)") {
		t.Errorf("the cost cell reads %q; one request of two priced is 50%% of the "+
			"surface, and a partial total that does not say so is read as the whole bill", got)
	}
}

const (
	// pricedModel is in the compiled table; unpricedModel belongs to no family
	// it answers for, so it reaches the report as spend nothing can value.
	pricedModel   = "claude-opus-5"
	unpricedModel = "gpt-5-codex"
)

// BG-6: a dated request is priced at the time it ran, not at today's row.
//
// PriceFor ignores dated windows. A promotion that applied in September
// and has since ended is today's $10, and a report that uses PriceFor
// bills the September request at $10. As-run uses the row in force at
// the request timestamp.
func TestBG6_DatedRequestIsPricedAtRequestTime(t *testing.T) {
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

	today, ok := cachemodel.PriceFor(pricedModel)
	if !ok {
		t.Fatal("the override must price opus-5")
	}
	at := time.Date(2026, 9, 15, 0, 36, 2, 0, time.UTC)
	then, ok := cachemodel.PriceForAt(pricedModel, at)
	if !ok {
		t.Fatal("the dated window must price opus-5 in September")
	}
	if today.InputPerMTok == then.InputPerMTok {
		t.Fatal("PriceFor and PriceForAt agree; the fixture cannot tell today from request time")
	}

	home := t.TempDir()
	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(proj, "session.jsonl")
	if err := os.WriteFile(path, []byte(datedOpusTranscript), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(transcriptsEnv, "")

	sess, err := transcript.ParseClaudeCodeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var u transcript.Usage
	var found int
	for _, lane := range sess.Lanes {
		for _, r := range lane.Requests {
			if r.Model != pricedModel {
				continue
			}
			u = r.Usage
			found++
		}
	}
	if found != 1 {
		t.Fatalf("fixture holds %d priced request(s), want 1", found)
	}
	wantToday := cachemodel.CostUSD(u, today)
	wantThen := cachemodel.CostUSD(u, then)
	if wantToday == wantThen || wantThen <= 0 {
		t.Fatalf("the two clocks price the same ($%.6f); the assertion below proves nothing", wantThen)
	}

	s := burnClaudeCode(home, "")
	if math.Abs(s.costUSD-wantThen) > 1e-9 {
		t.Errorf("costUSD = $%.6f, want $%.6f at the September row (today would be $%.6f)",
			s.costUSD, wantThen, wantToday)
	}
}

// datedOpusTranscript is one request on opus-5, timestamped inside the
// September 2026 window, with 1M input tokens so the two rates differ by $5.
const datedOpusTranscript = `{"uuid":"a1","type":"user","sessionId":"s","version":"2.0","timestamp":"2026-09-15T00:36:01Z","message":{"role":"user","content":"hi"}}
{"uuid":"a2","parentUuid":"a1","type":"assistant","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-09-15T00:36:02Z","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"one"}],"usage":{"input_tokens":1000000,"output_tokens":0}}}
`

// twoModelTranscript is one session, one lane, two requests, on two models —
// one the compiled table prices and one it does not.
const twoModelTranscript = `{"uuid":"a1","type":"user","sessionId":"s","version":"2.0","timestamp":"2026-03-21T00:36:01Z","message":{"role":"user","content":"hi"}}
{"uuid":"a2","parentUuid":"a1","type":"assistant","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-03-21T00:36:02Z","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"one"}],"usage":{"input_tokens":1000,"output_tokens":200}}}
{"uuid":"a3","parentUuid":"a2","type":"user","timestamp":"2026-03-21T00:36:03Z","message":{"role":"user","content":"more"}}
{"uuid":"a4","parentUuid":"a3","type":"assistant","requestId":"r2","apiBlockIndex":0,"timestamp":"2026-03-21T00:36:04Z","message":{"role":"assistant","model":"gpt-5-codex","content":[{"type":"text","text":"two"}],"usage":{"input_tokens":2000,"output_tokens":300}}}
`
