package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RPL-C005, at the printed surface.
//
// "Anything this tool cannot measure, it declines to print, and says why, in
// the place the number would have gone."
//
// The internal half was already established: surface.Classify reaches
// ClassUndetermined with a reason. The recorded gap was that the PRINTED
// surface after a refusal had never been inspected. This is that gap.
//
// The fixture isolates exactly one unmeasurable quantity. Usage is present
// and provider-reported on every record; only the PRICE of half of them is
// unavailable, because their model is not in the table. Nothing is malformed
// and nothing is missing, so any silence is a choice rather than a failure.

// mixedPricedCorpus writes one transcript holding two priced requests and two
// unpriced ones, and returns its directory.
func mixedPricedCorpus(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	var lines []byte
	for i, model := range []string{"claude-opus-5", "claude-opus-5", c005UnpricedModel, c005UnpricedModel} {
		u := transcript.Usage{Input: 2000, CacheRead: 9000, CacheCreation: 800, Output: 400}
		rec := ledger.Record{
			Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(i) * time.Minute),
			SessionID: "mix", RequestID: "req-" + string(rune('A'+i)),
			Path: "/v1/messages", Status: 200, LatencyMS: 800,
			RequestSummary: ledger.RequestSummary{Model: model,
				Prompt: ledger.Prompt{SystemBytes: 50, Messages: []ledger.Message{{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 150}}}}}},
			Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 100}}},
		}
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, append(b, '\n')...)
	}
	if err := os.WriteFile(filepath.Join(dir, "mix.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

const c005UnpricedModel = "totally-unknown-model-x9"

// CR0. The fixture's assumption, checked before anything is concluded from it.
// If the unknown model turns out to be priced by a substring match, every
// assertion below is measuring something else.
func TestC005_TheFixtureIsolatesExactlyOneUnmeasurableQuantity(t *testing.T) {
	if p, ok := cachemodel.PriceFor(c005UnpricedModel); ok {
		t.Fatalf("%q IS priced (%v). The fixture does not isolate an unpriceable "+
			"request and the rest of this file measures something else.", c005UnpricedModel, p)
	}
	if _, ok := cachemodel.PriceFor("claude-opus-5"); !ok {
		t.Fatal("claude-opus-5 is NOT priced, so the fixture has no priced half and " +
			"the comparison below is meaningless")
	}
}

// CR1. DEFECT, OBSERVED.
//
// The `unpriced` disclosure has TRANSCRIPT granularity. renderCost's own doc
// comment says so: "unpriced is transcripts that were READ and priced to
// nothing". Pricing has RECORD granularity.
//
// So a transcript holding a mix is counted as priced, its unpriced records
// contribute nothing to the total, and the count designed to disclose exactly
// this reports zero.
//
// This test PINS the current behaviour rather than asserting it is correct.
// It is recorded as a defect in the claim register, where RPL-C005 is
// REFUTED on the strength of it. It is not patched here: changing what a
// money figure reports is a production decision.
func TestC005_AMixedTranscriptHidesItsUnpricedRecords(t *testing.T) {
	dir := mixedPricedCorpus(t)

	var j, e bytes.Buffer
	if err := runCost([]string{"--json", dir}, &j, &e); err != nil {
		t.Fatalf("cost --json: %v (%s)", err, e.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("cost JSON does not parse: %v", err)
	}

	total, _ := doc["totalRequests"].(float64)
	unpriced, _ := doc["unpriced"].(float64)

	// POSITIVE CONTROL: the corpus must have reached the reader.
	if total != 4 {
		t.Fatalf("totalRequests = %v, want 4; the fixture is not reaching the reader "+
			"and nothing below is observable", total)
	}

	if unpriced != 0 {
		t.Fatalf("unpriced = %v. This test pins unpriced == 0 as the OBSERVED "+
			"behaviour. A non-zero value means the granularity was changed, which "+
			"would be the fix; update the claim register rather than this test.", unpriced)
	}

	t.Logf("DEFECT PINNED: 2 of 4 requests are unpriceable and `unpriced` reports %v. "+
		"The counter has transcript granularity and pricing has record granularity, "+
		"so a mixed transcript is counted as priced and its unpriceable records are "+
		"excluded from the total with no disclosure.", unpriced)
}

// CR2. The consequence at the human surface, which is what C005 is about.
//
// The report prints a total over a corpus it could only partly price, says
// nothing about the part it could not, and then asserts the figure is what
// the work cost.
func TestC005_TheHumanReportAssertsCompletenessOverAPartlyPricedCorpus(t *testing.T) {
	dir := mixedPricedCorpus(t)

	var out, e bytes.Buffer
	if err := runCost([]string{dir}, &out, &e); err != nil {
		t.Fatalf("cost: %v (%s)", err, e.String())
	}
	report := out.String()

	// POSITIVE CONTROL: a total must have been printed.
	if !strings.Contains(report, "total") || !dollarFigure.MatchString(report) {
		t.Fatalf("no total was printed, so this test cannot observe what it was "+
			"written for.\n%s", report)
	}

	// Does anything in the report disclose the unpriced half?
	disclosure := strings.Contains(report, "not priced") ||
		strings.Contains(report, "unpriced") ||
		strings.Contains(report, "excluded")

	if disclosure {
		t.Fatalf("the report DOES disclose the unpriced records. That is the fix, "+
			"and the claim register must be updated rather than this test.\n%s", report)
	}

	t.Logf("DEFECT PINNED at the human surface: the report prints a total over a " +
		"corpus half of which could not be priced, discloses nothing, and states " +
		"\"That is what your agent work actually cost\". The disclosure sentence " +
		"exists at cost.go:488 and fires only when a WHOLE transcript is unpriced.")
}

// CR3. The negative control for the whole file. On a fully priced corpus the
// absence of a disclosure is correct, so the two tests above are detecting a
// mixed corpus rather than detecting nothing at all.
func TestC005_AFullyPricedCorpusCorrectlyDisclosesNothing(t *testing.T) {
	dir := tierFixture(t) // every record on claude-opus-5

	var out, e bytes.Buffer
	if err := runCost([]string{dir}, &out, &e); err != nil {
		t.Fatalf("cost: %v (%s)", err, e.String())
	}
	if strings.Contains(out.String(), "not priced") {
		t.Error("a fully priced corpus reported unpriced transcripts; the disclosure " +
			"fires when it should not, and CR1/CR2 would then be measuring noise")
	}
}
