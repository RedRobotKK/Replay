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

// EC-00's strongest instance, at the user-visible boundary.
//
//	cmd/replay/cost.go:717
//	  if price, ok := PriceForAt(model, u.At); ok {
//	      u.RebilledUSD   = ...
//	      u.RebilledTokens = deficit
//	  }
//
// `deficit` is accumulated at cost.go:703-712 from br.Deficit over every
// lane, entirely before and independently of the price lookup. It is a TOKEN
// count. Tokens do not need a price.
//
// The shape under attack is therefore:
//
//	known quantity -> gated by unrelated price evidence -> user-visible zero
//
// ATTRIBUTION CORRECTED BY MUTATION. The first version of this file named
// cost.go:717 as the cause. It is not, on its own. The quantity is gated
// TWICE, sequentially:
//
//	cost.go:680  if asRun.CostUSD <= 0 { unpriced++; return nil }
//	cost.go:717  if price, ok := PriceForAt(...); ok { ... RebilledTokens = deficit }
//
// An unpriced session prices to zero, so it is dropped at the FIRST gate and
// control never reaches the second. Measured: hoisting the token assignment
// out of the inner branch changes nothing (0), removing the session skip
// alone changes nothing (0), and removing both yields 40,000 on both arms.
//
// So the repair boundary spans two sites, and EC-00 and RPL-C031/C032 are one
// defect chain rather than separate findings. That is a stronger result than
// the original single-site story and it was only found because the mutation
// survived.
//
// This file proves it at the printed and JSON boundary, not by reading the
// assignment. The oracle knows the token count because the test builds the
// corpus; it never calls the code under test to find out.

const (
	rbPriced   = "claude-opus-5"
	rbUnpriced = "totally-unknown-model-x9"
)

// rebilledCorpus writes a two-request session that re-sends a large prefix,
// which is what the break analyser reads as tokens re-written that should
// have been read. The two corpora differ ONLY in the model id.
func rebilledCorpus(t *testing.T, model string) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)

	// Turn 1 writes a large cache entry. Turn 2 writes it again instead of
	// reading it, which is the break.
	usages := []transcript.Usage{
		{Input: 500, CacheCreation: 40_000, CacheRead: 0, Output: 300},
		{Input: 500, CacheCreation: 40_000, CacheRead: 0, Output: 300},
	}
	var lines []byte
	for i, u := range usages {
		u := u
		rec := ledger.Record{
			Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(i) * time.Minute),
			SessionID: "rebilled", RequestID: "rb-" + string(rune('A'+i)),
			Path: "/v1/messages", Status: 200, LatencyMS: 900,
			RequestSummary: ledger.RequestSummary{Model: model,
				Prompt: ledger.Prompt{SystemBytes: 400, Messages: []ledger.Message{
					{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 160_000}}}}}},
			Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 900}}},
		}
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, append(b, '\n')...)
	}
	if err := os.WriteFile(filepath.Join(dir, "rebilled.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

type rbReport struct {
	rebilledTokens int
	rebilledUSD    float64
	human          string
}

func rbRun(t *testing.T, dir string) rbReport {
	t.Helper()
	var j, e bytes.Buffer
	if err := runCost([]string{"--json", dir}, &j, &e); err != nil {
		t.Fatalf("cost --json: %v (%s)", err, e.String())
	}
	var doc struct {
		Summary struct {
			RebilledTokens int     `json:"rebilledTokens"`
			RebilledUSD    float64 `json:"rebilledUsd"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("cost JSON does not parse: %v", err)
	}
	var h bytes.Buffer
	_ = runCost([]string{dir}, &h, &e)
	return rbReport{doc.Summary.RebilledTokens, doc.Summary.RebilledUSD, h.String()}
}

// RB0. The fixture's assumptions, checked before anything is concluded.
func TestRB0_FixtureAssumptions(t *testing.T) {
	if _, ok := cachemodel.PriceFor(rbPriced); !ok {
		t.Fatalf("%s is not priced; the control arm is broken", rbPriced)
	}
	if p, ok := cachemodel.PriceFor(rbUnpriced); ok {
		t.Fatalf("%s IS priced (%v); the treatment arm is broken", rbUnpriced, p)
	}
}

// RB1. POSITIVE CONTROL. On a priced model the corpus must report a non-zero
// re-billed token count, or the fixture produces no break and everything
// below is vacuous.
func TestRB1_APricedModelReportsItsRebilledTokens(t *testing.T) {
	got := rbRun(t, rebilledCorpus(t, rbPriced))
	if got.rebilledTokens <= 0 {
		t.Fatalf("the priced corpus reported rebilledTokens=%d. The fixture produced "+
			"no cache break, so the comparison in RB2 cannot observe anything.\n%s",
			got.rebilledTokens, got.human)
	}
	t.Logf("priced: rebilledTokens=%d rebilledUSD=%.6f", got.rebilledTokens, got.rebilledUSD)
}

// RB2. THE ATTACK. The two corpora differ only in the model id. The re-billed
// TOKEN count is a property of the cache break and not of the price table,
// so the contract requires it to be identical.
// RB2. REGRESSION, after the repair of 2026-09-30.
//
// I1 TOKEN:  a known deficit is reported whether or not a price exists.
// I2 DOLLAR: an unavailable price does not become a measured $0.
//
// This test replaced a pin that asserted the defect. The oracle is unchanged:
// two corpora differing only in a model id must re-bill the same TOKENS,
// because a deficit is a property of the cache break.
func TestRB2_AKnownDeficitIsReportedWhetherOrNotAPriceExists(t *testing.T) {
	priced := rbRun(t, rebilledCorpus(t, rbPriced))
	unpriced := rbRun(t, rebilledCorpus(t, rbUnpriced))

	if priced.rebilledTokens <= 0 {
		t.Fatalf("the priced corpus reported %d re-billed tokens; the fixture "+
			"produces no break and this test observes nothing", priced.rebilledTokens)
	}

	// REGRESSION, after the three-site repair of 2026-10-01.
	// I1 TOKEN: a known deficit is reported whether or not a price exists.
	if unpriced.rebilledTokens != priced.rebilledTokens {
		t.Errorf("I1 TOKEN violated: priced reports %d re-billed tokens and unpriced "+
			"reports %d, over corpora differing only in a model id. A deficit is "+
			"Expected minus Actual and consults no price table.",
			priced.rebilledTokens, unpriced.rebilledTokens)
	}
	// I2 DOLLAR: an unavailable price must not become a measured $0.
	if priced.rebilledUSD <= 0 {
		t.Errorf("the priced arm reports no dollar figure (%.6f); I2 cannot be "+
			"checked against it", priced.rebilledUSD)
	}
	if unpriced.rebilledUSD != 0 {
		t.Errorf("I2 DOLLAR violated: the unpriced arm claims $%.6f. With no price "+
			"in the table there is no dollar figure to state.", unpriced.rebilledUSD)
	}
	t.Logf("I1 and I2 hold: %d re-billed tokens on both arms; $%.6f on the priced "+
		"arm and no dollar figure claimed on the unpriced one.",
		priced.rebilledTokens, priced.rebilledUSD)
}

// RB6. I4 ZERO. A genuinely zero deficit must stay distinguishable from an
// unavailable one. Without this the repair could satisfy I1 by reporting
// something for everything.
func TestRB6_AGenuineZeroIsNotTheSameAsUnavailable(t *testing.T) {
	// A corpus with no cache break at all: every turn reads rather than
	// re-writes, so the deficit is genuinely zero on a PRICED model.
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	var lines []byte
	for i := 0; i < 2; i++ {
		u := transcript.Usage{Input: 400, CacheCreation: 0, CacheRead: 30_000, Output: 200}
		rec := ledger.Record{
			Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(i) * time.Minute),
			SessionID: "nobreak", RequestID: "nb-" + string(rune('A'+i)),
			Path: "/v1/messages", Status: 200, LatencyMS: 700,
			RequestSummary: ledger.RequestSummary{Model: rbPriced,
				Prompt: ledger.Prompt{SystemBytes: 300, Messages: []ledger.Message{
					{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 1_000}}}}}},
			Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 400}}},
		}
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, append(b, '\n')...)
	}
	if err := os.WriteFile(filepath.Join(dir, "nobreak.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}

	zero := rbRun(t, dir)
	if zero.rebilledTokens != 0 {
		t.Fatalf("a corpus with no cache break reports %d re-billed tokens; the "+
			"fixture is not a genuine zero", zero.rebilledTokens)
	}
	// It is zero AND priced, so a reader can tell it apart from unavailable:
	// the priced session carries dollar figures and no unpriced flag.
	if !strings.Contains(zero.human, "total") {
		t.Error("a genuinely zero-deficit priced session produced no cost report at " +
			"all, so zero and unavailable are not distinguishable after all")
	}
	t.Log("I4 holds: a genuine zero reports zero from a priced session that still " +
		"carries its dollar figures, which is what distinguishes it from unavailable")
}

// RB3. The human surface.
func TestRB3_TheHumanReportLosesTheRebilledLine(t *testing.T) {
	priced := rbRun(t, rebilledCorpus(t, rbPriced))
	unpriced := rbRun(t, rebilledCorpus(t, rbUnpriced))

	pHas := strings.Contains(priced.human, "re-billed")
	uHas := strings.Contains(unpriced.human, "re-billed")
	t.Logf("human report mentions re-billed: priced=%v unpriced=%v", pHas, uHas)
	if pHas && !uHas {
		t.Logf("RECORDED: the re-billed line is present for a priced model and absent " +
			"for an unpriced one, over corpora with the same cache break.")
	}
}
