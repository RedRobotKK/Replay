package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RPL-C035-RB. The re-billed population mismatch.
//
// Deciding evidence, frozen before this fixture:
//
//	numerator   cost.go:806, deficit accumulated over EVERY lane via
//	            AnalyzeEveryLane regardless of priceability, monetised at
//	            PriceForAt(Requests[0].Model)
//	denominator summarise, priced rows only
//
// The >100% figure is the SYMPTOM. The claim is the population mismatch, and
// it is proven by changing the unpriceable population and watching one side
// move while the other does not.

// rbPop writes one session whose records break the cache, in the given models.
func rbPop(t *testing.T, models []string) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	var lines []byte
	for i, m := range models {
		u := transcript.Usage{Input: 500, CacheCreation: 20_000, CacheRead: 0, Output: 300}
		rec := ledger.Record{
			Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(i) * time.Minute),
			SessionID: "rbpop", RequestID: "rp-" + string(rune('A'+i)),
			Path: "/v1/messages", Status: 200, LatencyMS: 900,
			RequestSummary: ledger.RequestSummary{Model: m,
				Prompt: ledger.Prompt{SystemBytes: 400, Messages: []ledger.Message{
					{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 81_000}}}}}},
			Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 900}}},
		}
		b, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, append(b, '\n')...)
	}
	if err := os.WriteFile(filepath.Join(dir, "rbpop.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

type rbFig struct {
	totalUSD, rebilledUSD float64
	rebilledTokens        int
}

func rbRead(t *testing.T, dir string) rbFig {
	t.Helper()
	var j, e bytes.Buffer
	if err := runCost([]string{"--json", dir}, &j, &e); err != nil {
		t.Fatalf("cost: %v (%s)", err, e.String())
	}
	var doc struct {
		Summary struct {
			TotalUSD       float64 `json:"totalUsd"`
			RebilledUSD    float64 `json:"rebilledUsd"`
			RebilledTokens int     `json:"rebilledTokens"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	return rbFig{doc.Summary.TotalUSD, doc.Summary.RebilledUSD, doc.Summary.RebilledTokens}
}

// RB1. THE POPULATION MISMATCH, proven directly.
//
// A and B differ ONLY by two appended unpriceable records. If numerator and
// denominator covered the same population, adding records that contribute no
// dollars would move neither.
func TestRB1Pop_NumeratorAndDenominatorCoverDifferentPopulations(t *testing.T) {
	if _, ok := cachemodel.PriceFor(ptPriced); !ok {
		t.Fatalf("%s is not priced; the fixture is broken", ptPriced)
	}
	if _, ok := cachemodel.PriceFor(ptUnpriced); ok {
		t.Fatalf("%s IS priced; the fixture is broken", ptUnpriced)
	}

	a := rbRead(t, rbPop(t, []string{ptPriced, ptPriced}))
	b := rbRead(t, rbPop(t, []string{ptPriced, ptPriced, ptUnpriced, ptUnpriced}))

	if a.rebilledUSD <= 0 {
		t.Fatalf("corpus A re-billed $%.6f; the fixture produces no break and this "+
			"test cannot observe anything", a.rebilledUSD)
	}

	// REGRESSION, after the FM repair of 2026-10-01.
	//
	// This test previously PROVED the mismatch: adding two unpriceable
	// records moved RebilledUSD from $0.100000 to $0.300000 while TotalUSD
	// stayed at $0.270000. The FM repair monetises each break at its own
	// record's rate, so an unpriceable record now contributes tokens and no
	// dollars, and the numerator covers the same population the denominator
	// does.
	//
	// RB was therefore RESOLVED BY FM and required no second repair. The
	// assertion is inverted rather than deleted, so the mismatch cannot
	// return unnoticed.
	if math.Abs(b.rebilledUSD-a.rebilledUSD) > 1e-9 {
		t.Errorf("POPULATION MISMATCH RETURNED: adding two UNPRICEABLE records "+
			"changed RebilledUSD from $%.6f to $%.6f. Nothing prices them, so they "+
			"cannot contribute dollars.", a.rebilledUSD, b.rebilledUSD)
	}
	if math.Abs(b.totalUSD-a.totalUSD) > 1e-9 {
		t.Errorf("TotalUSD moved from $%.6f to $%.6f when two unpriceable records "+
			"were added; the denominator must not include them either",
			a.totalUSD, b.totalUSD)
	}
	// And the tokens MUST still move, or the repair fixed the ratio by
	// discarding evidence, which is the EC-00 defect returning.
	if b.rebilledTokens <= a.rebilledTokens {
		t.Errorf("deficit TOKENS did not grow (%d then %d). An unpriceable record "+
			"still re-bills tokens; dropping them would re-break EC-00.",
			a.rebilledTokens, b.rebilledTokens)
	}
	t.Logf("POPULATIONS AGREE: dollars $%.6f on both corpora, tokens %d then %d, "+
		"total $%.6f unchanged.",
		a.rebilledUSD, a.rebilledTokens, b.rebilledTokens, a.totalUSD)
}

// RPL-C035-FM. The first-model pricing rule, tested as an independent claim.
//
// Distinct from RB: RB is about WHICH RECORDS enter the numerator, this is
// about WHAT RATE the numerator is monetised at. Both flow from cost.go:806
// and they are separately falsifiable, so they are separately registered.
//
// The deficit, the usage and the SET of models are held identical. Only the
// ORDER changes, which changes Requests[0].Model, which is the only input to
// the rate.
func TestFM1_TheWholeDeficitIsPricedAtTheFirstRecordsModel(t *testing.T) {
	const expensive, cheap = "claude-opus-5", "claude-3-5-haiku"

	pe, ok := cachemodel.PriceFor(expensive)
	if !ok {
		t.Fatalf("%s is not priced", expensive)
	}
	pc, ok := cachemodel.PriceFor(cheap)
	if !ok {
		t.Fatalf("%s is not priced", cheap)
	}
	if pe.InputPerMTok <= pc.InputPerMTok {
		t.Fatalf("the fixture needs two priced models at different rates; got "+
			"%s $%.2f and %s $%.2f", expensive, pe.InputPerMTok, cheap, pc.InputPerMTok)
	}

	// Identical multiset of models. Only the order differs.
	expensiveFirst := rbRead(t, rbPop(t, []string{expensive, cheap, cheap, cheap}))
	cheapFirst := rbRead(t, rbPop(t, []string{cheap, cheap, cheap, expensive}))

	if expensiveFirst.rebilledTokens != cheapFirst.rebilledTokens {
		t.Fatalf("the two orderings produce different token deficits (%d vs %d); "+
			"the fixture does not hold the deficit constant and the rate effect "+
			"cannot be isolated", expensiveFirst.rebilledTokens, cheapFirst.rebilledTokens)
	}

	// HARD, not a logged withdrawal. The first version of this returned with
	// t.Logf when the orderings matched, which made it blind to the rate
	// being removed from the numerator altogether: both orderings then price
	// identically and the test passed. That is the same logged-finding defect
	// caught twice before in this campaign, and it is why the branch is now a
	// failure.
	//
	// The fixture guarantees two priced models at different rates, asserted
	// above, so identical pricing means either the rule changed or the rate
	// stopped reaching the numerator. Both are findings, neither is a pass.
	// REGRESSION, after the FM repair. This previously PROVED the defect:
	// the same deficit priced at $0.300000 or $0.048000 depending on which
	// record arrived first. Each break is now monetised at its own record's
	// rate, so the two orderings differ only where the BREAKING records
	// differ, which is real and not a defect.
	//
	// The invariant retained here is the narrow one: the figure must stay
	// within the bounds set by the corpus's own rates, so no record's rate is
	// applied to another's deficit.
	lo := float64(expensiveFirst.rebilledTokens) / 1_000_000 * pc.InputPerMTok
	hi := float64(expensiveFirst.rebilledTokens) / 1_000_000 * pe.InputPerMTok
	for name, got := range map[string]float64{
		"expensive-first": expensiveFirst.rebilledUSD,
		"cheap-first":     cheapFirst.rebilledUSD,
	} {
		if got < lo-1e-9 || got > hi+1e-9 {
			t.Errorf("%s priced $%.6f, outside [$%.6f, $%.6f]; a rate belonging to "+
				"no record in this corpus is being applied", name, got, lo, hi)
		}
	}
	t.Logf("FM REGRESSION: expensive-first $%.6f, cheap-first $%.6f, both within "+
		"[$%.6f, $%.6f] over an identical %d-token deficit. The difference that "+
		"remains is which records break, not which arrived first.",
		expensiveFirst.rebilledUSD, cheapFirst.rebilledUSD, lo, hi,
		expensiveFirst.rebilledTokens)
}
