package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RPL-C004. "Every figure Replay prints carries a truth tier."
//
// This is the claim that the epistemic model reaches the user. Everything
// else in the register can be right and this can still be false, because a
// provenance field that never reaches the screen protects nobody.
//
// The deciding layer is NONE. There is no tier type, field or enum: the
// vocabulary is free string literals at more than fifteen sites across eleven
// packages. A property held by scattered literals and no type is one that
// drifts at the first new report, which is why this is worth attacking.
//
// Boundary, stated before measuring. ADR-0002 says "the two never share a
// table without the label", so the claimed unit is the report, not each
// digit. The assumption is that a tier is conveyed by the words measured,
// estimated or structural; a report conveying provenance by other wording
// would be understated by this measurement.

var (
	dollarFigure = regexp.MustCompile(`\$[0-9][0-9,]*\.?[0-9]*`)
	tierWord     = regexp.MustCompile(`(?i)\b(measured|estimated|structural)\b`)
)

// tierFixture writes a small two-session ledger corpus and returns its dir.
func tierFixture(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	for si, session := range []string{"tier-a", "tier-b"} {
		var lines []byte
		for i := 0; i < 3; i++ {
			u := transcript.Usage{Input: 1_200 + i*100, CacheRead: 9_000, CacheCreation: 800, Output: 350}
			rec := ledger.Record{
				Schema:    ledger.SchemaVersion,
				Timestamp: at.Add(time.Duration(si*10+i) * time.Minute),
				SessionID: session,
				RequestID: session + "-req-" + string(rune('A'+i)),
				Path:      "/v1/messages",
				Status:    200,
				LatencyMS: 850,
				RequestSummary: ledger.RequestSummary{
					Model:  "claude-opus-5",
					Prompt: ledger.Prompt{SystemBytes: 60, Messages: []ledger.Message{{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 200}}}}},
				},
				Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 150}}},
			}
			b, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(b, '\n')...)
		}
		if err := os.WriteFile(filepath.Join(dir, session+".jsonl"), lines, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TC1. The headline report prints dollars. Does it say how they were obtained,
// IN A PLACE ATTACHED TO THEM?
//
// The first version of this test searched the whole report for the words
// measured, estimated or structural, and it PASSED. It passed on the sentence
// "was measured here across 3.09M tokens", which is about a rate-limit null
// result and labels nothing. A detector that fires on an unrelated sentence
// is not evidence, and this is the defect the campaign exists to find in
// other people's checks.
//
// Rewritten to ask the question that matters: does the basis of the dollars
// appear where a reader meets the dollars, which is the report header.
func TestC004_TheCostReportStatesHowItsDollarsWereObtained(t *testing.T) {
	dir := tierFixture(t)

	var out, errOut bytes.Buffer
	if err := runCost([]string{dir}, &out, &errOut); err != nil {
		t.Fatalf("cost: %v (stderr: %s)", err, errOut.String())
	}
	report := out.String()
	lines := strings.Split(report, "\n")

	// POSITIVE CONTROL: there must be figures, or the assertion is vacuous.
	figures := dollarFigure.FindAllString(report, -1)
	if len(figures) == 0 {
		t.Fatalf("the report printed no dollar figure, so this test cannot observe "+
			"what it was written for.\n%s", report)
	}

	// Where does the reader first meet money?
	firstFigureLine := -1
	for i, l := range lines {
		if dollarFigure.MatchString(l) {
			firstFigureLine = i
			break
		}
	}
	if firstFigureLine <= 0 {
		t.Fatalf("the first dollar figure is on line %d; there is no header above it "+
			"to carry a basis", firstFigureLine)
	}

	// The basis must be stated in the header, above the figures. This is the
	// actual mechanism: a price basis and the date it was read.
	header := strings.Join(lines[:firstFigureLine], " ")
	hasBasis := regexp.MustCompile(`(?i)(list price|measured|estimated|at list)`).MatchString(header)
	hasDate := regexp.MustCompile(`\b20\d\d-\d\d-\d\d\b`).MatchString(header)

	if !hasBasis {
		t.Errorf("RPL-C004: the header above the figures states no price basis.\n"+
			"header: %q", header)
	}
	if !hasDate {
		t.Errorf("RPL-C004: the header states a basis with no date, so a reader "+
			"cannot tell how stale it is.\nheader: %q", header)
	}
	// RPL-C025, the specific date. "Any date" let a header that had dropped
	// the price-table date pass on the strength of the rules version's date
	// beside it (the 2026-10-02 wiring gate planted exactly that and this
	// test stayed green). The date a reader needs is the price table's, and
	// it is the one constant that says which table priced these dollars.
	if !strings.Contains(header, "dated "+cachemodel.PriceTableVersion) {
		t.Errorf("RPL-C025: the header does not name the price table's own date %q; "+
			"a date from elsewhere in the line is not the basis of the dollars.\nheader: %q",
			cachemodel.PriceTableVersion, header)
	}
}

// TC1b. The tier VOCABULARY is not the mechanism, and that is worth pinning.
//
// README.md:293 names three tiers, measured / estimated / structural.
// ADR-0002 is titled "two tiers of truth" and names only two: it does not
// contain the word structural at all. Meanwhile the report labels its dollars
// by naming a dated price basis and saying plainly that they are list price
// on a subscription seat.
//
// The property the claim protects is present. The wording of the claim does
// not describe how. This test records the gap between them rather than
// pretending either is wrong.
func TestC004_TheTierVocabularyIsNotHowTheCostReportLabelsMoney(t *testing.T) {
	dir := tierFixture(t)
	var out, errOut bytes.Buffer
	if err := runCost([]string{dir}, &out, &errOut); err != nil {
		t.Fatalf("cost: %v", err)
	}
	lines := strings.Split(out.String(), "\n")

	firstFigureLine := -1
	for i, l := range lines {
		if dollarFigure.MatchString(l) {
			firstFigureLine = i
			break
		}
	}
	header := strings.Join(lines[:firstFigureLine], " ")

	if tierWord.MatchString(header) {
		t.Logf("the header DOES use the tier vocabulary: %q", header)
		return
	}
	t.Logf("RECORDED: the header labels its dollars without using any of the three "+
		"tier words. It says %q instead. The claim's wording describes a mechanism "+
		"the report does not use; the property it protects is nonetheless present.",
		strings.TrimSpace(header))
}

// TC2. The detector must be able to fail and able to pass, or TC1 means
// nothing either way.
func TestC004_TheTierDetectorDiscriminates(t *testing.T) {
	if tierWord.MatchString("total $12.00 across 4 sessions") {
		t.Fatal("the tier detector fires on a line with no tier word; a positive " +
			"result would be meaningless")
	}
	if !tierWord.MatchString("total $12.00, measured on the wire") {
		t.Fatal("the tier detector does not fire on a labelled line; a negative " +
			"result would be meaningless")
	}
	if !dollarFigure.MatchString("total $12.00") {
		t.Fatal("the figure detector does not find a dollar figure")
	}
	if dollarFigure.MatchString("4 sessions, 12 requests") {
		t.Fatal("the figure detector fires on a bare integer; it is too broad")
	}
}

// TC3. The machine-readable surface. A JSON consumer cannot read a prose
// caveat, so if the tier is carried anywhere for them it must be a field.
func TestC004_TheJSONSurfaceCarriesNoTierField(t *testing.T) {
	dir := tierFixture(t)

	var out, errOut bytes.Buffer
	if err := runCost([]string{"--json", dir}, &out, &errOut); err != nil {
		t.Fatalf("cost --json: %v (stderr: %s)", err, errOut.String())
	}

	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("cost JSON does not parse: %v", err)
	}
	if len(doc) == 0 {
		t.Fatal("the JSON document is empty; this test cannot observe anything")
	}

	var tierKeys []string
	for k := range doc {
		if strings.Contains(strings.ToLower(k), "tier") ||
			strings.Contains(strings.ToLower(k), "provenance") ||
			strings.Contains(strings.ToLower(k), "basis") {
			tierKeys = append(tierKeys, k)
		}
	}
	if len(tierKeys) == 0 {
		t.Logf("RECORDED: the cost JSON has %d top-level keys and none of them names "+
			"a tier, a provenance or a basis. A machine consumer receives dollar "+
			"figures with no way to tell how they were obtained. Keys: %v",
			len(doc), tierKeysOf(doc))
	}
}

func tierKeysOf(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
