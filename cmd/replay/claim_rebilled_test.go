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
func TestRB2_AnUnpricedModelSuppressesAPriceIndependentTokenCount(t *testing.T) {
	priced := rbRun(t, rebilledCorpus(t, rbPriced))
	unpriced := rbRun(t, rebilledCorpus(t, rbUnpriced))

	if priced.rebilledTokens <= 0 {
		t.Skip("no break produced; RB1 reports the fixture problem")
	}

	// The oracle: identical corpora differing only in a model NAME must
	// re-bill the identical number of TOKENS.
	if unpriced.rebilledTokens == priced.rebilledTokens {
		t.Fatalf("both corpora report rebilledTokens=%d. The collapse is gone and the "+
			"claim register must be updated rather than this test.", priced.rebilledTokens)
	}

	// PINNED, not failed. The suite stays green and this test flips the moment
	// the defect is repaired, telling whoever repairs it to update the claim
	// register rather than this file.
	if unpriced.rebilledTokens != 0 {
		t.Fatalf("the unpriced corpus now reports rebilledTokens=%d. The defect is "+
			"repaired; update RPL-C034 in the claim register rather than this test.",
			unpriced.rebilledTokens)
	}
	t.Logf("EC-00 CONFIRMED at the user-visible boundary.\n\n"+
		"  priced model   %-26s rebilledTokens=%d  rebilledUSD=%.6f\n"+
		"  unpriced model %-26s rebilledTokens=%d  rebilledUSD=%.6f\n\n"+
		"The two corpora are byte-identical apart from the model id. A re-billed "+
		"TOKEN count is a property of the cache break, computed at cost.go:703-712 "+
		"before any price lookup, and it is suppressed because an unrelated price "+
		"lookup failed.\n\n"+
		"docs/TOKEN-PRICES.md:53: \"An unpriced model must never count as zero.\"\n\n"+
		"This test is EXPECTED TO FAIL. It documents a defect that is deliberately "+
		"not repaired in this campaign.",
		rbPriced, priced.rebilledTokens, priced.rebilledUSD,
		rbUnpriced, unpriced.rebilledTokens, unpriced.rebilledUSD)
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
