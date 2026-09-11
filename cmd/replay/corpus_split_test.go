package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/observation"
)

// The corpus report published one number where there are three.
//
// `docs/evidence/calibration-corpus-2026-09-10.md` says "34750 compared, 33983
// matched, 767 breaks" and never breaks matched into reproduced and exceeded.
// An exceeded turn is one where the provider served MORE cached prefix than
// the model predicted — usually a concurrent sibling lane extended it — so it
// is a turn the model got wrong, folded into the figure the README quotes.
//
// Measured on the real corpus on 2026-09-11: 37925 compared, 35686 reproduced
// exactly (94.10%), 1433 exceeded (3.78%), 806 broken (2.13%). The 97.87%
// headline is 94.10% exact, and 3.86% of every matched turn is an exceeded
// one. 257 of 1816 transcripts clear the 95% calibration gate ONLY because
// exceeded counts as a match.
//
// MatchRate is not redefined — other documents quote it — so the report prints
// the exact rate beside it and names the split in its totals.

// laneWithAnExceededTurn writes one transcript whose three API requests
// produce one reproduced turn and one exceeded turn.
//
// The expected read is the previous request's cache creation plus its cache
// read: req1 reading exactly 100 reproduces, req2 reading 500 against an
// expectation of 120 exceeds.
func laneWithAnExceededTurn(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	usage := func(in, create, read int) string {
		return fmt.Sprintf(`"usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":5}`, in, create, read)
	}
	req := func(i int, parent, u string) string {
		return fmt.Sprintf(`{"type":"assistant","uuid":"a%d","parentUuid":"%s","sessionId":"split-session","requestId":"req-%d","timestamp":"2026-09-11T10:00:%02dZ","apiBlockIndex":0,`+
			`"message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],%s}}`, i, parent, i, i, u)
	}
	body := strings.Join([]string{
		`{"type":"user","uuid":"u0","sessionId":"split-session","timestamp":"2026-09-11T10:00:00Z","message":{"role":"user","content":"hello"}}`,
		req(0, "u0", usage(10, 100, 0)),
		req(1, "a0", usage(2, 20, 100)),
		req(2, "a1", usage(2, 0, 500)),
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(dir, "lane.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// corpusOf runs the report over a fixture directory.
func corpusOf(t *testing.T, dir string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if err := run([]string{"corpus", dir}, &out, &errb); err != nil {
		t.Fatalf("corpus: %v (stderr %s)", err, errb.String())
	}
	return out.String()
}

// CS1: the fixture really does produce one reproduced and one exceeded turn.
//
// Without this the assertions below could hold against a corpus with no
// exceeded turn at all, where exact and match agree for free.
func TestCS1_TheFixtureProducesOneExceededTurn(t *testing.T) {
	got := corpusOf(t, laneWithAnExceededTurn(t))
	if !strings.Contains(got, "Compared turns: 2") {
		t.Fatalf("the fixture did not produce two compared turns, so nothing below is under test:\n%s", totalsOf(got))
	}
}

// CS2: the totals name the three-way split.
//
// PASS: reproduced, exceeded and broken are each counted, and the exceeded
// count is not silently inside "matched".
// FAIL: "compared, matched, breaks" alone, which is what the published
// evidence file says and what a reader cannot decompose.
func TestCS2_TotalsReportTheThreeWaySplit(t *testing.T) {
	got := totalsOf(corpusOf(t, laneWithAnExceededTurn(t)))
	if !strings.Contains(got, "reproduced exactly: 1") {
		t.Errorf("the totals do not report how many turns were reproduced EXACTLY. The match "+
			"rate counts turns where the provider served more prefix than predicted, and a "+
			"reader cannot subtract a number the report never prints:\n%s", got)
	}
	if !strings.Contains(got, "read more than predicted: 1") {
		t.Errorf("the totals do not report the exceeded count separately, so the share of the "+
			"headline that is a wrong prediction stays hidden:\n%s", got)
	}
	if !strings.Contains(got, "exact reproduction rate: 50.00%") {
		t.Errorf("the totals do not print an exact reproduction rate beside the match rate:\n%s", got)
	}
	if !strings.Contains(got, "Overall match rate: 100.00%") {
		t.Errorf("the published match rate was dropped or redefined. It keeps its meaning; the "+
			"exact rate is added BESIDE it:\n%s", got)
	}
}

// CS3: the per-transcript table carries the split too.
//
// The table is the body of the evidence document. A totals line alone would
// let every row still read as exact.
//
// PASS: an Exceeded column and an Exact rate column, with this row's values.
// FAIL: Matched and Match rate alone.
func TestCS3_TheRowTableCarriesExactAndExceeded(t *testing.T) {
	got := corpusOf(t, laneWithAnExceededTurn(t))
	header := headerRowOf(got)
	for _, col := range []string{"Exact", "Exceeded", "Exact rate", "Match rate"} {
		if !strings.Contains(header, "| "+col+" |") {
			t.Errorf("the per-transcript table has no %q column:\n%s", col, header)
		}
	}
	row := dataRowOf(got)
	if !strings.Contains(row, "| 50.0% | 100.0% |") {
		t.Errorf("the row does not carry the exact rate beside the match rate. One of its two "+
			"compared turns read more than predicted, so 100%% match is 50%% exact:\n%s", row)
	}
}

// CS4: the per-model table carries the split.
//
// This table is what a reader uses to decide whether a model is calibrated,
// and its verdict column is read against a rate. Printing only the folded rate
// puts an exceeded turn on the calibrated side of that judgement invisibly.
//
// PASS: an exact rate column, and the recent window gets one too.
// FAIL: match rate alone.
func TestCS4_ThePerModelTableCarriesTheExactRate(t *testing.T) {
	got := corpusOf(t, laneWithAnExceededTurn(t))
	modelHeader := ""
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "| Model |") {
			modelHeader = line
		}
	}
	if modelHeader == "" {
		t.Fatalf("no per-model table in the report:\n%s", got)
	}
	for _, col := range []string{"Exact rate", "Match rate", "Recent exact rate", "Recent match rate"} {
		if !strings.Contains(modelHeader, "| "+col+" |") {
			t.Errorf("the per-model table has no %q column:\n%s", col, modelHeader)
		}
	}
	// Sessions, Lanes, then exact before match. The Lanes column arrived on
	// main while this branch was open — two denominations of the same corpus,
	// and this assertion pins their order so a future edit cannot quietly
	// reintroduce the conflation by printing one of them under the other's
	// heading.
	if !strings.Contains(got, "| claude-opus-5 | 1 | 1 | 50.0% | 100.0% |") {
		t.Errorf("the model row does not print sessions, lanes, then the exact rate "+
			"before the match rate:\n%s", got)
	}
}

// CS5: the poolable contribution record carries the exact count.
//
// ADR-0007's record is the unit of contribution and it is the one place these
// figures leave the machine. A pooled reading built on `matched` alone would
// reconstruct the same conflated headline across every contributor, and no
// reader of the pool could decompose it either.
//
// PASS: Validate refuses a row claiming more exact turns than matched ones.
// FAIL: the field is unchecked, so a nonsense row pools as if it were evidence.
func TestCS5_ContributionRefusesMoreExactThanMatched(t *testing.T) {
	c := observation.Calibration{
		Schema:       observation.CalibrationSchema,
		RulesVersion: "test",
		SourceTag:    "tag",
		TagBasis:     "basis",
		Models: []observation.ModelCalibrationRow{
			{Model: "m", Sessions: 1, Compared: 10, Matched: 8, Exact: 9},
		},
	}
	err := c.Digested().Validate()
	if err == nil {
		t.Fatal("a row claiming 9 exact turns out of 8 matched was accepted. Exact is a subset of " +
			"matched by construction, so the pool would be counting a state that cannot occur.")
	}
	if !strings.Contains(err.Error(), "exact") {
		t.Errorf("the refusal does not name the field that is wrong: %v", err)
	}
}

// CS6: and a well-formed row still pools.
//
// Without this, refusing everything would satisfy CS5 and silently end
// contribution.
func TestCS6_AWellFormedContributionStillPools(t *testing.T) {
	c := observation.Calibration{
		Schema:       observation.CalibrationSchema,
		RulesVersion: "test",
		SourceTag:    "tag",
		TagBasis:     "basis",
		Models: []observation.ModelCalibrationRow{
			{Model: "m", Sessions: 1, Compared: 10, Matched: 9, Exact: 8},
		},
	}
	if err := c.Digested().Validate(); err != nil {
		t.Fatalf("a report with 8 exact of 9 matched of 10 compared was refused: %v", err)
	}
}

// CS7: contributeCalibration puts the measured exact count on the record.
//
// Validate can only refuse what it is given. If the builder leaves Exact at
// zero, every submission pools as "nothing was reproduced exactly" and the
// field is worse than absent.
func TestCS7_ContributionCarriesTheMeasuredExactCount(t *testing.T) {
	cals := []analysis.ModelCalibration{{
		Model: "claude-opus-5", Sessions: 1, Compared: 10, Matched: 9, Exact: 8,
		RecentLanes: 1, RecentCompared: 10, RecentMatched: 9, RecentExact: 8,
	}}
	rows := []corpusRow{{id: "abc", client: "2.1.0", requests: 11, compared: 10, matched: 9, exact: 8}}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfg := filepath.Join(home, ".config", "replay")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "corpus-consent.toml"), []byte("corpus_opt_in = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := contributeCalibration("c", t.TempDir(), rows, cals, time.Date(2026, 9, 11, 5, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("contributeCalibration: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"exact": 8`) {
		t.Errorf("the written report does not carry the exact count, so a pooled reading is back "+
			"to one conflated number:\n%s", body)
	}
}

// headerRowOf returns the per-transcript table's header line.
func headerRowOf(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "| Session |") {
			return line
		}
	}
	return s
}

// dataRowOf returns the first data row of the per-transcript table.
func dataRowOf(s string) string {
	seenHeader := false
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "| Session |") {
			seenHeader = true
			continue
		}
		if seenHeader && strings.HasPrefix(line, "| ") && !strings.HasPrefix(line, "|---") {
			return line
		}
	}
	return s
}

// CS8: the note explaining exceeded reads appears only when there are any.
//
// The paragraph tells a reader what a read larger than predicted means. On a
// corpus with none, printing it explains a number that is not on the page —
// and the number it explains is zero, so a reader who stops on it learns that
// something they do not have is behaving normally.
//
// guard-reachability reported the branch INERT: the note was printed on every
// fixture that had exceeded turns and nothing checked the empty case, so the
// condition could be deleted with the suite green.
func TestCS8_TheExceededNoteIsAbsentWhenNothingExceeded(t *testing.T) {
	const phrase = "larger than predicted"

	withExceeded := []corpusRow{{
		id: "s1", compared: 4, matched: 4, exact: 3, exceeded: 1,
	}}
	var sb strings.Builder
	if err := writeCorpus(&sb, withExceeded, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), phrase) {
		t.Errorf("a corpus WITH exceeded reads does not explain them:\n%s", sb.String())
	}

	none := []corpusRow{{
		id: "s1", compared: 4, matched: 4, exact: 4, exceeded: 0,
	}}
	sb.Reset()
	if err := writeCorpus(&sb, none, nil, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sb.String(), phrase) {
		t.Errorf("a corpus with NO exceeded reads still explains them, so the report "+
			"describes a condition the reader does not have:\n%s", sb.String())
	}
}

// CS9: a row that compared nothing has an exact rate of zero, not 0/0.
//
// Unreached: every fixture compared something. A lane with no compared turns
// is the one MatchRate returned 1 for until 2026-09-06, admitting 18 of 1,450
// lanes to alternative scoring for having tested nothing. exactRate is the
// same shape, added later, and nothing had asked it the question.
func TestCS9_ARowThatComparedNothingRatesZero(t *testing.T) {
	r := corpusRow{id: "s1", compared: 0, matched: 0, exact: 0}
	if got := r.exactRate(); got != 0 {
		t.Errorf("exactRate = %v on a row that compared nothing, want 0", got)
	}
	if got := r.matchRate(); got != 0 {
		t.Errorf("matchRate = %v on a row that compared nothing, want 0", got)
	}
}
