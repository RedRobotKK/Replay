package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
	"github.com/RedRobotKK/Replay/internal/usage"
)

// `replay cost --usage`: what token counts alone can and cannot say.
//
// The happy path here is one line of arithmetic. Everything worth testing is
// on the other side: which figures this input CANNOT support, and whether the
// report says so instead of printing a zero. ADR-0018 rule 2 — absence, zero
// and unknown are three values — is the whole specification of this command.

// The fixture, chosen so every branch is exercised by one file.
//
// r1 opens the session and writes 8000 tokens of cache.
// r2 reads exactly what r1 wrote: reproduced, no break.
// r3 reads nothing at all: broken, deficit 9200, and the cause IS decidable
//
//	from usage — nothing was read, so the divergence is before the first
//	message.
//
// r4 reads 4000 of the 9000 r3 wrote: broken, deficit 5000, and the cause is
//
//	NOT decidable from usage — same model, inside the TTL, a partial read.
//	Only the message history could say what moved, and there is none.
//
// The expected figures below are literals. They are what somebody worked out
// from the four records by hand, so they cannot agree with the code by
// construction (ADR-0018 rule 3).
const (
	usageFixtureBreaks    = 2
	usageFixtureDeficit   = 9200 + 5000
	usageFixtureRequests  = 4
	usageFixtureModel     = "claude-sonnet-4-5"
	usageFixtureInputRate = 3.0 // dollars per million input tokens, price table
)

type fixtureRecord struct {
	minute                          int
	prompt, fresh, read, write, out int
	model                           string
	session                         string
}

func usageFixture() []fixtureRecord {
	return []fixtureRecord{
		{0, 9000, 1000, 0, 8000, 100, usageFixtureModel, "s1"},
		{1, 9700, 500, 8000, 1200, 100, usageFixtureModel, "s1"},
		{2, 10000, 1000, 0, 9000, 100, usageFixtureModel, "s1"},
		{3, 10000, 500, 4000, 5500, 100, usageFixtureModel, "s1"},
	}
}

func exportDoc(complete bool, dated bool, recs []fixtureRecord) string {
	var b strings.Builder
	b.WriteString(`{"schema":"replay.usage.v1","complete":`)
	if complete {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString(`,"records":[`)
	for i, r := range recs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"session":"` + r.session + `","model":"` + r.model + `"`)
		if dated {
			t := time.Date(2026, 9, 1, 10, r.minute, 0, 0, time.UTC)
			b.WriteString(`,"at":"` + t.Format(time.RFC3339) + `"`)
		}
		b.WriteString(`,"prompt":` + itoa(r.prompt) +
			`,"fresh":` + itoa(r.fresh) +
			`,"cached_read":` + itoa(r.read) +
			`,"cached_write":` + itoa(r.write) +
			`,"output":` + itoa(r.out) + `}`)
	}
	b.WriteString(`]}`)
	return b.String()
}

func itoa(n int) string { return strings.TrimSpace(strings.Join([]string{jsonInt(n)}, "")) }

func jsonInt(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func writeExport(t *testing.T, doc string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "usage.json")
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func parsedFixture(t *testing.T, complete, dated bool) *usage.Export {
	t.Helper()
	e, err := usage.ParseExport([]byte(exportDoc(complete, dated, usageFixture())))
	if err != nil {
		t.Fatalf("fixture is not a valid export: %v", err)
	}
	return e
}

// UO9: cost from usage alone is the same cost the transcript path measures.
//
// This is the test that crosses the join. The usage-only path reaches the
// price table through usage.Record.ToAnthropic(); the established path reaches
// it through analysis.Tally.Add. A field transposed in that map-back —
// cached_read landing in Fresh, say — is arithmetically silent and moves every
// figure in the report, and no assertion inside either half can see it.
//
// The oracle is the OTHER implementation, fed the identical counts. It is not
// derived from the thing it checks: neither path can be edited into agreement
// with the other without editing both.
//
// PASS: identical totals to the cent.
// FAIL: any mis-mapping. Reintroduce by swapping two fields in ToAnthropic.
func TestUO9_CostFromUsageAloneMatchesTheTranscriptPath(t *testing.T) {
	rep, _ := priceUsage(parsedFixture(t, true, true))

	lane := &transcript.Lane{ID: "s1"}
	for _, r := range usageFixture() {
		lane.Requests = append(lane.Requests, &transcript.Request{
			Model:     r.model,
			Timestamp: time.Date(2026, 9, 1, 10, r.minute, 0, 0, time.UTC),
			Usage: transcript.Usage{
				Input: r.fresh, CacheRead: r.read, CacheCreation: r.write, Output: r.out,
			},
		})
	}
	want := analysis.AsRun(lane).CostUSD

	if rep.TotalUSD == 0 {
		t.Fatal("the usage-only path priced nothing at all")
	}
	if diff := rep.TotalUSD - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("usage-only total $%.6f, transcript path $%.6f over the identical counts", rep.TotalUSD, want)
	}
	if rep.Requests != usageFixtureRequests {
		t.Errorf("counted %d requests, want %d", rep.Requests, usageFixtureRequests)
	}
}

// UO10: with a complete, timestamped export the break deficit is MEASURED.
//
// This is the claim worth being careful about, because it is the one a reader
// would assume needs the transcript. It does not: ExpectedRead is
// prompt-minus-uncached-tail of the previous request, and both numbers are
// provider-reported. The deficit is arithmetic on measured values, so it is a
// measurement, and the usage-only report is entitled to print it without a
// tier caveat.
//
// PASS: 2 breaks, 14200 tokens, priced at the table's input rate.
// FAIL: a different count. Reintroduce by comparing each record against the
// first rather than against its predecessor.
func TestUO10_ACompleteDatedExportMeasuresTheBreakDeficit(t *testing.T) {
	rep, rows := priceUsage(parsedFixture(t, true, true))

	if rep.Breaks == nil {
		t.Fatalf("breaks came back NOT MEASURED for an export that declared itself complete and dated every record: %s", rep.NotMeasuredWhy)
	}
	if *rep.Breaks != usageFixtureBreaks {
		t.Errorf("counted %d breaks, want %d", *rep.Breaks, usageFixtureBreaks)
	}
	if rep.AvoidableTokens == nil || *rep.AvoidableTokens != usageFixtureDeficit {
		t.Errorf("re-billed tokens %v, want %d", rep.AvoidableTokens, usageFixtureDeficit)
	}
	wantUSD := float64(usageFixtureDeficit) / 1_000_000 * usageFixtureInputRate
	if rep.AvoidableUSD == nil {
		t.Fatal("avoidable dollars came back NOT MEASURED beside a measured token deficit")
	}
	if diff := *rep.AvoidableUSD - wantUSD; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("avoidable $%.6f, want $%.6f", *rep.AvoidableUSD, wantUSD)
	}
	if len(rows) != 1 || rows[0].ID != "s1" {
		t.Fatalf("expected one session row for s1, got %d rows", len(rows))
	}
	// The share is a measurement in its own right whenever there is a total to
	// be a share of, and the summary block leads with it.
	if rep.AvoidableShare == nil {
		t.Fatal("the avoidable share was withheld over a corpus with a real total; it is measured here")
	}
	if diff := *rep.AvoidableShare - wantUSD/rep.TotalUSD; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("share %.6f, want %.6f", *rep.AvoidableShare, wantUSD/rep.TotalUSD)
	}
	if out := renderUsageCost(rep, rows, false); !strings.Contains(out, "% of the total") {
		t.Errorf("the measured share never reaches the screen:\n%s", out)
	}
}

// UO11: an export that does not declare completeness reports breaks NOT
// MEASURED, and prices the cost anyway.
//
// This is ADR-0002's two tiers doing real work in one report. The cost is
// Measured — the counts are the provider's and the price table is dated. The
// break figure is NOT MEASURED, because a request missing from the export is
// indistinguishable from a cache break and nothing in the file settles it.
//
// The important half is that the two are independent. A tool that downgraded
// the whole report because one figure was unavailable would be useless to the
// finance owner who only ever wanted the total.
//
// PASS: cost identical to the complete export's; breaks nil, not zero.
// FAIL: breaks reported as 0, or the cost withheld. Reintroduce by having
// priceUsage return 0 instead of nil.
func TestUO11_AnUndeclaredExportWithholdsBreaksAndKeepsTheCost(t *testing.T) {
	complete, _ := priceUsage(parsedFixture(t, true, true))
	partial, _ := priceUsage(parsedFixture(t, false, true))

	if partial.Breaks != nil {
		t.Errorf("breaks reported as %d over an export that cannot support the figure", *partial.Breaks)
	}
	if partial.AvoidableUSD != nil || partial.AvoidableTokens != nil {
		t.Errorf("avoidable reported (%v / %v) over an export that cannot support it", partial.AvoidableUSD, partial.AvoidableTokens)
	}
	if partial.NotMeasuredWhy == "" {
		t.Error("no reason given for withholding the break figure; a refusal a reader cannot act on is a blank screen")
	}
	if diff := partial.TotalUSD - complete.TotalUSD; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("the cost moved when the completeness declaration did: $%.6f against $%.6f", partial.TotalUSD, complete.TotalUSD)
	}
}

// UO12: an export missing a timestamp cannot be ordered, and says so
// differently.
//
// Two distinct facts are missing in UO11 and here, and the fix a reader has to
// make is different in each. If both produced the same sentence the message
// would be decoration.
//
// PASS: breaks nil, and the reason mentions the ordering problem rather than
// the completeness one.
// FAIL: the same reason as UO11, or a break figure. Reintroduce by collapsing
// SequenceEvidence's two branches into one.
func TestUO12_AnUndatedExportCannotBeSequencedAndSaysWhy(t *testing.T) {
	undated, _ := priceUsage(parsedFixture(t, true, false))
	undeclared, _ := priceUsage(parsedFixture(t, false, true))

	if undated.Breaks != nil {
		t.Errorf("breaks reported as %d over records that cannot be put in order", *undated.Breaks)
	}
	if undated.TotalUSD == 0 {
		t.Error("the cost was withheld along with the breaks; a token count does not need a clock to be priced")
	}
	if undated.NotMeasuredWhy == undeclared.NotMeasuredWhy {
		t.Errorf("both defects give the same reason (%q); the reader cannot tell which to fix", undated.NotMeasuredWhy)
	}
	if !strings.Contains(undated.NotMeasuredWhy, "timestamp") {
		t.Errorf("the reason does not name the missing field: %q", undated.NotMeasuredWhy)
	}
}

// UO13: a break whose cause usage cannot settle is counted as NOT MEASURED,
// never as a named cause.
//
// This is the tier split the brief asks for, at its sharpest: the SAME break
// is Measured for its deficit and NOT MEASURED for its cause. cachemodel
// settles three causes from usage and timing alone — TTL expired, model
// changed, nothing read. Everything else needs the message history, and the
// transcript path's answer for those ("prefix diverged inside the message
// history at an unknown block", located by the byte-to-token fit) is not
// available here and must not be borrowed: there is no fit, because there are
// no bytes.
//
// PASS: r3's cause named, r4's counted as unmeasured, and the two never mix.
// FAIL: r4 given a cause. Reintroduce by defaulting to CauseUnknown when
// ClassifyBreak declines.
func TestUO13_ACauseUsageCannotSettleIsNotMeasuredNotGuessed(t *testing.T) {
	rep, _ := priceUsage(parsedFixture(t, true, true))

	if rep.CauseNotMeasured == nil {
		t.Fatal("the count of breaks whose cause is unknown is itself absent; it is the figure this path exists to be honest about")
	}
	if *rep.CauseNotMeasured != 1 {
		t.Errorf("%d breaks with an unmeasured cause, want 1", *rep.CauseNotMeasured)
	}
	named := 0
	for _, n := range rep.Causes {
		named += n
	}
	if named != 1 {
		t.Errorf("%d breaks given a named cause, want 1 (only the one usage settles)", named)
	}
	if named+*rep.CauseNotMeasured != *rep.Breaks {
		t.Errorf("causes account for %d of %d breaks; a break is either explained or it is not", named+*rep.CauseNotMeasured, *rep.Breaks)
	}
	for cause := range rep.Causes {
		if strings.Contains(cause, "unknown block") || strings.Contains(cause, "re-rendered") {
			t.Errorf("a cause that needs the message history was named from usage alone: %q", cause)
		}
	}
	// And it has to reach the screen. A count kept in a struct nobody prints
	// is the built-but-unwired shape ADR-0018 names, one layer down.
	out := renderUsageCost(rep, fixtureRows(t), false)
	if !strings.Contains(out, "NOT MEASURED. The prefix diverged") {
		t.Errorf("the break whose cause was never measured is not on screen:\n%s", out)
	}
}

// fixtureRows is the fixture's session rows, for the render assertions above.
func fixtureRows(t *testing.T) []usageRow {
	t.Helper()
	_, r := priceUsage(parsedFixture(t, true, true))
	return r
}

// UO14: the report names what usage alone can never report.
//
// ADR-0018 rule 4: saying "not measured" has to be cheap on every surface. A
// reader coming to this path from `replay cost` will notice figures missing
// and has to be told they are structurally absent rather than zero — otherwise
// the honest fallback reads as a broken build.
//
// PASS: the report names the missing surfaces and marks them NOT MEASURED.
// FAIL: they are silently absent. Reintroduce by deleting the block.
func TestUO14_TheReportNamesWhatUsageAloneCannotReport(t *testing.T) {
	rep, rows := priceUsage(parsedFixture(t, true, true))
	out := renderUsageCost(rep, rows, false)

	if !strings.Contains(strings.ToUpper(out), "NOT MEASURED") {
		t.Fatalf("no NOT MEASURED anywhere in a report built from token counts alone:\n%s", out)
	}
	// Each of these is a figure `replay cost` prints from a transcript and
	// this path structurally cannot: repeated tool results and tool errors
	// need the blocks, blame needs the bytes, alternative layouts need the
	// context.
	for _, subject := range []string{"repeated", "error", "blame", "layout"} {
		if !strings.Contains(strings.ToLower(out), subject) {
			t.Errorf("the report never mentions %q, so a reader cannot tell it is absent by design:\n%s", subject, out)
		}
	}
}

// UO15: --max-avoidable-usd refuses to pass when avoidable is NOT MEASURED.
//
// The gate exists to fail a build on spend nobody chose. Over an export whose
// break figure could not be measured there is no such spend to compare, and a
// gate that went green there would report a clean bill of health nobody
// earned — on exactly the input a regulated operator is most likely to hand
// it. This mirrors checkAvoidableCeiling's refusal over an unpriced corpus.
//
// PASS: an error, and the exit is non-zero.
// FAIL: green. Reintroduce by treating a nil avoidable as 0 in the gate.
func TestUO15_TheGateRefusesAnUnmeasuredAvoidableFigure(t *testing.T) {
	path := writeExport(t, exportDoc(false, true, usageFixture()))
	var out, errb bytes.Buffer
	err := run([]string{"cost", "--usage", path, "--max-avoidable-usd", "1.00"}, &out, &errb)
	if err == nil {
		t.Fatalf("the gate passed over an avoidable figure that was never measured:\n%s", out.String())
	}
	if !strings.Contains(strings.ToUpper(err.Error()+out.String()), "NOT MEASURED") {
		t.Errorf("the refusal does not carry the tier: %v\n%s", err, out.String())
	}

	// And it still gates normally when the figure IS measured, or the refusal
	// above is just the flag being broken.
	good := writeExport(t, exportDoc(true, true, usageFixture()))
	var out2, errb2 bytes.Buffer
	if err := run([]string{"cost", "--usage", good, "--max-avoidable-usd", "1.00"}, &out2, &errb2); err != nil {
		t.Errorf("a measured avoidable figure of $0.04 was refused against a $1.00 ceiling: %v\n%s", err, out2.String())
	}
}

// UO16: a record whose model is not in the price table is excluded and
// counted, never priced as free.
//
// The transcript path already refuses to count an unpriced session as zero.
// The same rule has to survive here, and it is easier to get wrong: these are
// records rather than files, so an unpriced one sits inside a session that is
// otherwise priced and contributes a silent nothing to the total.
//
// PASS: the request count says 2 priced, the report names the 2 it could not
// price, and the total is the priced pair alone.
// FAIL: four requests priced, two of them at $0. Reintroduce by dropping the
// priced flag from PriceFor's result.
func TestUO16_UnpricedRecordsAreExcludedAndNamedNotCountedAsFree(t *testing.T) {
	recs := usageFixture()
	recs[2].model = "claude-haiku-3" // in the table, deliberately unpriced
	recs[3].model = "claude-haiku-3"
	e, err := usage.ParseExport([]byte(exportDoc(true, true, recs)))
	if err != nil {
		t.Fatal(err)
	}
	rep, rows := priceUsage(e)

	if rep.UnpricedRequests != 2 {
		t.Errorf("%d unpriced requests, want 2", rep.UnpricedRequests)
	}
	if rep.Requests != 2 {
		t.Errorf("%d priced requests counted, want 2; an unpriced request must not be counted as a priced one", rep.Requests)
	}
	out := renderUsageCost(rep, rows, false)
	if !strings.Contains(out, "not in") && !strings.Contains(out, "price table") {
		t.Errorf("the report does not say why 2 requests were left out:\n%s", out)
	}
}

// UO17: the JSON carries nulls where the figure was not measured, never zeros.
//
// The renderer is not the only consumer. A dashboard reading `avoidableUsd: 0`
// plots a corpus with no waste; reading `null` it has to decide what to do,
// which is the decision ADR-0018 says must not be made for it. This is the
// same distinction `errorShare` already draws with a *float64 in cost.go.
//
// PASS: the keys are present and null.
// FAIL: present and 0, or absent. Absent is nearly as bad — a consumer that
// defaults a missing key to zero has been handed the same lie one layer up.
func TestUO17_TheJSONSaysNullNotZeroForWhatItDidNotMeasure(t *testing.T) {
	path := writeExport(t, exportDoc(false, true, usageFixture()))
	var out, errb bytes.Buffer
	if err := run([]string{"cost", "--usage", path, "--json"}, &out, &errb); err != nil {
		t.Fatalf("%v\n%s", err, errb.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("the --json output is not JSON: %v\n%s", err, out.String())
	}
	summary, ok := doc["summary"].(map[string]any)
	if !ok {
		t.Fatalf("no summary object in the output:\n%s", out.String())
	}
	for _, key := range []string{"avoidableUsd", "avoidableTokens", "breaks"} {
		v, present := summary[key]
		if !present {
			t.Errorf("%q is absent rather than null; a consumer defaulting a missing key to zero has been handed the same lie one layer up", key)
			continue
		}
		if v != nil {
			t.Errorf("%q is %v, want null: the figure was not measured", key, v)
		}
	}
	if summary["totalUsd"] == nil {
		t.Error("totalUsd is null; the cost WAS measured and withholding it is the opposite error")
	}
	if doc["schema"] != "replay.cost.usage.v1" {
		t.Errorf("schema is %v; a usage-only report must not arrive under the transcript report's schema, because the two answer different questions", doc["schema"])
	}
}

// UO18: --usage refuses the surfaces it cannot honestly produce.
//
// --share and --png publish a card built on the avoidable rate; --compare
// splits a corpus by date and reports the change; --contribute pools figures
// into the calibration corpus. Each is either unavailable or would carry a
// figure this path did not measure into somewhere it cannot be retracted from
// — a public post, or somebody else's corpus.
//
// Refusing is cheap. Emitting a card with a blank where the headline goes is
// not.
//
// PASS: each combination errors, and the message names the flag.
// FAIL: any of them produces output. Reintroduce by deleting the guard.
func TestUO18_UsageOnlyRefusesTheSurfacesItCannotProduce(t *testing.T) {
	path := writeExport(t, exportDoc(true, true, usageFixture()))
	for _, extra := range [][]string{
		{"--share"},
		{"--share", "--png", "/tmp/replay-uo18.png"},
		{"--compare", "2026-09-01"},
		{"--contribute", "launch"},
	} {
		args := append([]string{"cost", "--usage", path}, extra...)
		var out, errb bytes.Buffer
		err := run(args, &out, &errb)
		if err == nil {
			t.Errorf("%v was accepted on the usage-only path and printed:\n%s", extra, out.String())
			continue
		}
		if !strings.Contains(err.Error(), strings.TrimPrefix(extra[0], "--")) {
			t.Errorf("%v: the refusal does not name the flag: %v", extra, err)
		}
	}
}

// UO18b: a transcript path handed to --usage is refused rather than ignored.
//
// The two are different evidence. Silently dropping the positional argument
// would produce a report over one of them under a command line that named
// both, and nothing on screen would say which figure came from where.
//
// PASS: refused, and the message quotes the path.
// FAIL: the path is ignored. Reintroduce by deleting the last case in
// usageOnlyRefusals.
func TestUO18b_ATranscriptPathBesideUsageIsRefused(t *testing.T) {
	path := writeExport(t, exportDoc(true, true, usageFixture()))
	var out, errb bytes.Buffer
	err := run([]string{"cost", "--usage", path, "/tmp/some/transcripts"}, &out, &errb)
	if err == nil {
		t.Fatalf("a transcript path was silently ignored beside --usage:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "/tmp/some/transcripts") {
		t.Errorf("the refusal does not name what it refused: %v", err)
	}
}

// unpricedFixture is the same traffic on a model the table carries but does
// not price.
func unpricedFixture() []fixtureRecord {
	recs := usageFixture()
	for i := range recs {
		recs[i].model = "claude-haiku-3"
	}
	return recs
}

// UO19: a session's model column names the model that carried the money.
//
// A session is not one model. Naming the first one seen would label a row by
// whichever request happened to open it, which on a session that escalated to
// a larger model is the opposite of the routing decision the column exists to
// inform.
//
// PASS: the dearer model, which is second in the file.
// FAIL: the first. Reintroduce by assigning row.Model unconditionally.
func TestUO19_TheModelColumnNamesWhereTheMoneyWent(t *testing.T) {
	recs := []fixtureRecord{
		{0, 1000, 1000, 0, 0, 10, "claude-sonnet-4-5", "s1"},
		{1, 900000, 900000, 0, 0, 10, "claude-opus-4-8", "s1"},
	}
	e, err := usage.ParseExport([]byte(exportDoc(true, true, recs)))
	if err != nil {
		t.Fatal(err)
	}
	_, rows := priceUsage(e)
	if len(rows) != 1 {
		t.Fatalf("want one row, got %d", len(rows))
	}
	if rows[0].Model != "claude-opus-4-8" {
		t.Errorf("row labelled %q; the money went to claude-opus-4-8 and the column is what a routing decision reads", rows[0].Model)
	}
}

// UO20: a session began when its earliest record began.
//
// Records arrive in whatever order the query returned them, so first-seen is
// not first. A row timestamped by whichever record the file happened to list
// first would put sessions in the wrong order in any consumer that sorts on
// it — including a reconciliation joining this against an invoice period.
//
// PASS: 10:00, from a file that lists 10:02 first.
// FAIL: 10:02. Reintroduce by taking the first record's timestamp.
func TestUO20_ASessionRowIsTimestampedByItsEarliestRecord(t *testing.T) {
	recs := []fixtureRecord{
		{2, 1000, 1000, 0, 0, 10, usageFixtureModel, "s1"},
		{0, 1000, 1000, 0, 0, 10, usageFixtureModel, "s1"},
	}
	e, err := usage.ParseExport([]byte(exportDoc(true, true, recs)))
	if err != nil {
		t.Fatal(err)
	}
	_, rows := priceUsage(e)
	want := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	if !rows[0].At.Equal(want) {
		t.Errorf("row timestamped %v, want %v", rows[0].At, want)
	}
}

// UO21: a session nothing could be priced in is not a row, and the report says
// so rather than showing an empty table.
//
// cost.go excludes an unpriced transcript rather than counting it as free.
// The same rule has to hold here, and the empty case has to say what happened:
// a report that printed "0 sessions, $0.00" over four real requests is the
// clean bill of health nobody earned.
//
// PASS: no rows, and the text names the price table as the reason.
// FAIL: a $0.00 row, or silence. Reintroduce by dropping the row.Requests
// check.
func TestUO21_ASessionThatCouldNotBePricedIsNotARow(t *testing.T) {
	e, err := usage.ParseExport([]byte(exportDoc(true, true, unpricedFixture())))
	if err != nil {
		t.Fatal(err)
	}
	rep, rows := priceUsage(e)
	if len(rows) != 0 || rep.Sessions != 0 {
		t.Fatalf("%d rows over a corpus where nothing had a price", len(rows))
	}
	if rep.UnpricedRequests != 4 {
		t.Errorf("%d unpriced requests counted, want 4; excluded is not the same as never seen", rep.UnpricedRequests)
	}
	out := renderUsageCost(rep, rows, false)
	if !strings.Contains(out, "price table") {
		t.Errorf("the empty report does not say why it is empty:\n%s", out)
	}
	// And it must not print the spend block at all. "median task $0.00" over a
	// corpus where nothing had a price is the clean bill of health nobody
	// earned, and it reads as a measurement.
	for _, forbidden := range []string{"median task", "p90 task", "avoidable"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("the empty report prints %q over zero priced sessions:\n%s", forbidden, out)
		}
	}

	// And the gate must not go green over it.
	path := writeExport(t, exportDoc(true, true, unpricedFixture()))
	var so, se bytes.Buffer
	if err := run([]string{"cost", "--usage", path, "--max-avoidable-usd", "1.00"}, &so, &se); err == nil {
		t.Errorf("the gate passed over a corpus that priced nothing:\n%s", so.String())
	}
}

// UO22: a share of a zero total is NOT MEASURED, not 0%.
//
// This is the same figure split three ways in one row. The avoidable dollars
// were measured, and measured at nothing. The share of them was not measured
// at all, because there is no denominator. Printing "(0% of the total)" would
// collapse the second into the first — ADR-0018's rule 2, inside a single
// line of output.
//
// PASS: the dollars print, the share says NOT MEASURED.
// FAIL: "0%". Reintroduce by defaulting the share to 0 in renderUsageCost.
func TestUO22_AShareOfNothingIsNotMeasuredNotZeroPercent(t *testing.T) {
	recs := []fixtureRecord{
		{0, 0, 0, 0, 0, 0, usageFixtureModel, "s1"},
		{1, 0, 0, 0, 0, 0, usageFixtureModel, "s1"},
	}
	e, err := usage.ParseExport([]byte(exportDoc(true, true, recs)))
	if err != nil {
		t.Fatal(err)
	}
	rep, rows := priceUsage(e)
	if rep.Sessions != 1 {
		t.Fatalf("want the session counted, got %d", rep.Sessions)
	}
	if rep.AvoidableShare != nil {
		t.Errorf("a share of %v was computed over a total of $0.00", *rep.AvoidableShare)
	}
	if rep.AvoidableUSD == nil {
		t.Fatal("the avoidable dollars were withheld; they were measured, and measured at nothing")
	}
	out := renderUsageCost(rep, rows, false)
	if strings.Contains(out, "0% of the total") {
		t.Errorf("a ratio nobody took is printed as 0%%:\n%s", out)
	}
	if !strings.Contains(out, "share NOT MEASURED") {
		t.Errorf("the absent share is not marked:\n%s", out)
	}
}

// UO23: the per-session figures on the rows and the totals in the summary come
// out of one walk.
//
// The version this replaced re-derived each row's breaks in a second function.
// Two implementations of one piece of arithmetic disagree eventually, and the
// disagreement is invisible: the summary and the table are read at different
// moments by the same reader.
//
// PASS: the rows sum to the summary, across two sessions.
// FAIL: any drift. Reintroduce by resetting the per-session accumulator in the
// wrong place.
func TestUO23_TheRowsSumToTheSummary(t *testing.T) {
	recs := usageFixture()
	second := usageFixture()
	for i := range second {
		second[i].session = "s2"
		second[i].minute += 10
	}
	e, err := usage.ParseExport([]byte(exportDoc(true, true, append(recs, second...))))
	if err != nil {
		t.Fatal(err)
	}
	rep, rows := priceUsage(e)
	if len(rows) != 2 {
		t.Fatalf("want two session rows, got %d", len(rows))
	}
	breaks, deficit, cost := 0, 0, 0.0
	for _, r := range rows {
		if r.Breaks == nil || r.AvoidableTokens == nil {
			t.Fatalf("row %s carries no break figures under a summary that has them", r.ID)
		}
		breaks += *r.Breaks
		deficit += *r.AvoidableTokens
		cost += r.CostUSD
	}
	if breaks != *rep.Breaks || deficit != *rep.AvoidableTokens {
		t.Errorf("rows sum to %d breaks / %d tokens; the summary says %d / %d",
			breaks, deficit, *rep.Breaks, *rep.AvoidableTokens)
	}
	if diff := cost - rep.TotalUSD; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("rows sum to $%.6f, summary says $%.6f", cost, rep.TotalUSD)
	}
	if breaks != 2*usageFixtureBreaks {
		t.Errorf("%d breaks over two copies of a fixture with %d, want %d", breaks, usageFixtureBreaks, 2*usageFixtureBreaks)
	}
}

// UO24: --per-task prints a row per session, and marks the absent figures on
// each row rather than printing a dash a reader will read as zero.
//
// PASS: the measured table carries a dollar figure; the unmeasured one says so
// on the row, and --json under --per-task carries the rows.
// FAIL: rows missing, or an absent figure rendered as $0.00. Reintroduce by
// dropping the per-row nil check.
func TestUO24_PerTaskRowsCarryTheirOwnTier(t *testing.T) {
	measured, mrows := priceUsage(parsedFixture(t, true, true))
	mout := renderUsageCost(measured, mrows, true)
	if !strings.Contains(mout, "s1") || !strings.Contains(mout, "breaks") {
		t.Errorf("--per-task printed no table:\n%s", mout)
	}
	// The row's own figure, not the summary's. "$0.04" also appears in the
	// block above the table, so the assertion that catches a row rendered as
	// NOT MEAS. under a measured summary is the absence of the marker.
	if strings.Contains(mout, "NOT MEAS.") {
		t.Errorf("a row says NOT MEAS. under a summary that measured it:\n%s", mout)
	}
	if !strings.Contains(mout, "$0.04") {
		t.Errorf("the measured row carries no avoidable figure:\n%s", mout)
	}
	unmeasured, urows := priceUsage(parsedFixture(t, false, true))
	out := renderUsageCost(unmeasured, urows, true)
	if !strings.Contains(out, "NOT MEAS.") {
		t.Errorf("a row whose break figure was never measured does not say so:\n%s", out)
	}

	path := writeExport(t, exportDoc(true, true, usageFixture()))
	var so, se bytes.Buffer
	if err := run([]string{"cost", "--usage", path, "--per-task", "--json"}, &so, &se); err != nil {
		t.Fatalf("%v\n%s", err, se.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(so.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	tasks, ok := doc["tasks"].([]any)
	if !ok || len(tasks) != 1 {
		t.Errorf("--per-task --json carried no task rows: %v", doc["tasks"])
	}
}

// UO25: the gate fires when the measured figure is over the ceiling, and a
// negative ceiling is refused.
//
// The refusal in UO15 only proves the gate can say no. Without this it might
// say no to everything, which is a gate that cannot pass and is as useless as
// one that cannot fail (ADR-0014).
//
// PASS: over the ceiling errors; a negative ceiling errors differently.
// FAIL: either is accepted. Reintroduce by dropping the comparison.
func TestUO25_TheGateFiresAndRefusesANegativeCeiling(t *testing.T) {
	path := writeExport(t, exportDoc(true, true, usageFixture()))

	var so, se bytes.Buffer
	err := run([]string{"cost", "--usage", path, "--max-avoidable-usd", "0.01"}, &so, &se)
	if err == nil {
		t.Errorf("$0.04 of measured avoidable spend passed a $0.01 ceiling:\n%s", so.String())
	}

	var so2, se2 bytes.Buffer
	err = run([]string{"cost", "--usage", path, "--max-avoidable-usd", "-1"}, &so2, &se2)
	if err == nil {
		t.Error("a negative ceiling was accepted; every figure is under it and the gate can never fire")
	} else if !strings.Contains(err.Error(), "positive") {
		t.Errorf("the refusal does not say what is wrong with -1: %v", err)
	}
}

// UO26: an unreadable file and an unreadable export both refuse, and both name
// the path.
//
// A regulated operator running this in a pipeline gets one line of stderr. It
// has to say which file, because the whole point of the path is that the file
// came from somewhere else.
//
// PASS: both error, both quote the path.
// FAIL: either succeeds, or the message is generic. Reintroduce by dropping
// the %s wrapper.
func TestUO26_AnUnreadableExportRefusesAndNamesTheFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-here.json")
	var so, se bytes.Buffer
	err := run([]string{"cost", "--usage", missing}, &so, &se)
	if err == nil {
		t.Fatal("a missing usage file produced a report")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("the error does not name the file: %v", err)
	}
	// It has to be the filesystem's own error, not the parser's. Reading past
	// a failed open hands ParseExport an empty buffer, which refuses too — for
	// the wrong reason, and tells the operator their export is malformed when
	// the path is simply wrong.
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing file was reported as something other than a missing file: %v", err)
	}

	garbage := writeExport(t, `{"schema":"vendor.v9","records":[]}`)
	var so2, se2 bytes.Buffer
	err = run([]string{"cost", "--usage", garbage}, &so2, &se2)
	if err == nil {
		t.Fatal("a foreign export produced a report")
	}
	if !strings.Contains(err.Error(), garbage) {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// UO27: the cause block appears only when there is a break to explain, and the
// NOT MEASURED line only when there is one it cannot.
//
// A standing "NOT MEASURED: 0" line is the caveat becoming furniture. It stops
// being read, and the next time it carries a real number nobody notices.
//
// PASS: a clean session prints no cause block; a session whose every break is
// usage-decidable prints causes and no unmeasured line.
// FAIL: either block prints unconditionally. Reintroduce by dropping the
// count checks.
func TestUO27_TheCauseBlockAppearsOnlyWhenThereIsSomethingToExplain(t *testing.T) {
	clean := []fixtureRecord{
		{0, 9000, 1000, 0, 8000, 100, usageFixtureModel, "s1"},
		{1, 9700, 500, 8000, 1200, 100, usageFixtureModel, "s1"},
	}
	e, err := usage.ParseExport([]byte(exportDoc(true, true, clean)))
	if err != nil {
		t.Fatal(err)
	}
	rep, rows := priceUsage(e)
	if *rep.Breaks != 0 {
		t.Fatalf("the clean fixture broke %d times; it is supposed to reproduce", *rep.Breaks)
	}
	if out := renderUsageCost(rep, rows, false); strings.Contains(out, "Why the cache broke") {
		t.Errorf("a cause block printed over zero breaks:\n%s", out)
	}

	// Three records: the third reads nothing, which usage settles on its own.
	//
	// Copied rather than appended in place. `clean` is read again below, and
	// appending to it while assigning elsewhere writes through its backing
	// array whenever it has spare capacity — the two fixtures would then be
	// the same fixture, and the second assertion would be checking the first.
	decidable := make([]fixtureRecord, len(clean), len(clean)+1)
	copy(decidable, clean)
	decidable = append(decidable, fixtureRecord{2, 10000, 1000, 0, 9000, 100, usageFixtureModel, "s1"})
	e2, err := usage.ParseExport([]byte(exportDoc(true, true, decidable)))
	if err != nil {
		t.Fatal(err)
	}
	rep2, rows2 := priceUsage(e2)
	if *rep2.CauseNotMeasured != 0 {
		t.Fatalf("%d unexplained causes over a break usage settles on its own", *rep2.CauseNotMeasured)
	}
	out := renderUsageCost(rep2, rows2, false)
	if !strings.Contains(out, "Why the cache broke") {
		t.Errorf("no cause block over a break with a known cause:\n%s", out)
	}
	if strings.Contains(out, "NOT MEASURED. The prefix diverged") {
		t.Errorf("the unmeasured-cause line printed with a count of zero:\n%s", out)
	}
}

// errWriteRefused is what a failing writer returns, so a test can prove the
// error travelled rather than matching on prose.
var errWriteRefused = errors.New("the pipe closed")

// refusingWriter fails every write. A closed pipe, a full disk, and a reader
// that walked away all look like this.
type refusingWriter struct{}

func (refusingWriter) Write([]byte) (int, error) { return 0, errWriteRefused }

// UO29: a failed write of the JSON report is returned, never swallowed.
//
// `replay cost --usage ... --json` is read by a pipeline. A broken pipe or a
// full disk that produced a truncated document and exit 0 is the worst
// available outcome: the consumer parses what arrived, or worse, parses a
// prefix that happens to be valid, and nothing anywhere says the report is
// incomplete. Half a measurement presented as a whole one is the defect
// ADR-0018 is about, arriving through the io layer.
//
// PASS: an error, naming the write, wrapping the writer's own.
// FAIL: nil. Reintroduce by dropping the writeJSON error check in
// runCostUsage.
func TestUO29_AFailedJSONWriteIsReturned(t *testing.T) {
	path := writeExport(t, exportDoc(true, true, usageFixture()))
	err := runCostUsage(path, true, false, 0, refusingWriter{})
	if err == nil {
		t.Fatal("the JSON report failed to write and the command reported success; a pipeline reads a truncated document and nothing says so")
	}
	if !errors.Is(err, errWriteRefused) {
		t.Errorf("the writer's own error did not survive: %v", err)
	}
	if !strings.Contains(err.Error(), "writ") {
		t.Errorf("the error does not say a write is what failed, so it reads as a problem with the export: %v", err)
	}
}

// UO30: a failed write of the printed report is returned, never swallowed.
//
// Same defect, the other branch. Here the reader is a person and the failure
// is quieter still: a report that stops halfway looks like a report that had
// less to say, and the block it stops before is the NOT MEASURED block.
//
// PASS: an error, naming the write, wrapping the writer's own.
// FAIL: nil. Reintroduce by dropping the io.WriteString error check.
func TestUO30_AFailedReportWriteIsReturned(t *testing.T) {
	path := writeExport(t, exportDoc(true, true, usageFixture()))
	err := runCostUsage(path, false, false, 0, refusingWriter{})
	if err == nil {
		t.Fatal("the report failed to write and the command reported success; a report truncated before the NOT MEASURED block reads as a report with nothing to declare")
	}
	if !errors.Is(err, errWriteRefused) {
		t.Errorf("the writer's own error did not survive: %v", err)
	}
	if !strings.Contains(err.Error(), "writ") {
		t.Errorf("the error does not say a write is what failed: %v", err)
	}
}

// UO31: a reader outside the dollar zone gets the conversion note.
//
// The local figure is an indication of size and is not the reader's bill: a
// card issuer converts at its own rate on its settlement date and adds a
// foreign transaction fee. The note is what says so, and a converted figure
// printed without it is a number claiming to be a bill it is not — the same
// family of defect as every other figure on this screen, arriving through the
// currency column instead of the token counts.
//
// PASS: under ja_JP with an explicit rate, the yen column appears and the note
// under the block explains what it is.
// FAIL: the column prints and the note does not. Reintroduce by deleting the
// fx.Note() branch in renderUsageCost.
func TestUO31_AConvertedFigureCarriesItsNote(t *testing.T) {
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	t.Setenv("REPLAY_FX_JPY", "150")

	rep, rows := priceUsage(parsedFixture(t, true, true))
	out := renderUsageCost(rep, rows, false)

	if !strings.Contains(out, "JPY") {
		t.Fatalf("a ja_JP reader sees no local figure at all, so this test cannot say anything about its note:\n%s", out)
	}
	if !strings.Contains(out, "card issuer") {
		t.Errorf("the yen figure is printed with nothing saying it is not the reader's bill:\n%s", out)
	}

	// And a dollar-zone reader gets neither, or the note is furniture.
	t.Setenv("LC_ALL", "en_US.UTF-8")
	if plain := renderUsageCost(rep, rows, false); strings.Contains(plain, "card issuer") {
		t.Errorf("a conversion note printed with no conversion:\n%s", plain)
	}
}
