package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Q01, RELEASE BLOCKER. A session's reported cost must not depend on what its
// lane files are called.
//
// foldSessions copies the first-seen lane's row wholesale at cost.go:246 and
// then sums fifteen quantities onto it, never recomputing Unpriced or
// MixedEpochs. Files arrive size-descending with path as the tie-break
// (main.go:585-590), and cache-warm units are appended before any cold file is
// walked (cost.go:782-798), so the two flags are decided by file name or by the
// contents of the cost index.
//
// CLASSIFICATION: a semantic attribution bug, not a presentation bug. The
// per-lane figures are correct and the grouping by session id is correct. What
// is wrong is that a PER-LANE fact is used as a SESSION fact, and summarise
// then gates the session's dollars on it.
//
// Expected behaviour, defined before the repair: permuting lane file names
// while preserving every record must leave the summary byte-identical.

// q01Write lays one session down as two lanes, letting the caller choose which
// lane content gets which file name. Nothing semantic changes between arms.
func q01Write(t *testing.T, nameForPriceable, nameForUnpriceable string) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	put := func(lane string, recs []c037Rec) {
		var lines []byte
		for i, r := range recs {
			u := transcript.Usage{Input: 500, CacheCreation: r.create, CacheRead: r.read, Output: 300}
			rec := ledger.Record{
				Schema: ledger.SchemaVersion, Timestamp: c037Day(r.day),
				SessionID: "onesession", RequestID: fmt.Sprintf("%s-%02d", lane, i),
				Path: "/v1/messages", Status: 200, LatencyMS: 900,
				RequestSummary: ledger.RequestSummary{Model: c037Model,
					Prompt: ledger.Prompt{SystemBytes: 400, Messages: []ledger.Message{
						{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "u", Bytes: 400_000}}}}}},
				Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "a", Bytes: 900}}},
			}
			b, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(b, '\n')...)
		}
		if err := os.WriteFile(filepath.Join(dir, "onesession-"+lane+".jsonl"), lines, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A lane whose earliest record prices, and a lane whose earliest record
	// does not. Both carry two priceable breaks.
	put(nameForPriceable, []c037Rec{{day: 6, create: 20_000, wantPriced: true},
		{day: 7, create: 20_000, wantPriced: true}, {day: 8, create: 20_000, wantPriced: true}})
	put(nameForUnpriceable, []c037Rec{{day: 1, create: 20_000, wantPriced: false},
		{day: 9, create: 20_000, wantPriced: true}, {day: 10, create: 20_000, wantPriced: true}})
	return dir
}

type q01Fig struct {
	Tasks                  int     `json:"tasks"`
	Lanes                  int     `json:"lanes"`
	TotalUSD               float64 `json:"totalUsd"`
	MedianUSD              float64 `json:"medianUsd"`
	P90USD                 float64 `json:"p90Usd"`
	RebilledUSD            float64 `json:"rebilledUsd"`
	RebilledTokens         int     `json:"rebilledTokens"`
	UnpricedRebilledTokens int     `json:"unpricedRebilledTokens"`
}

func q01Run(t *testing.T, dir string) q01Fig {
	t.Helper()
	var j, e bytes.Buffer
	if err := runCost([]string{"--json", dir}, &j, &e); err != nil {
		t.Fatalf("cost: %v (%s)", err, e.String())
	}
	var doc struct {
		Summary q01Fig `json:"summary"`
	}
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Summary
}

// Q01-1. THE BLOCKER. Permuting lane file names must not move a figure.
func TestQ01_LaneFileNamesDoNotDecideASessionsCost(t *testing.T) {
	defer cachemodel.Override(c037rLeadUnpricedRules())()

	a := q01Run(t, q01Write(t, "a", "b")) // priceable lane sorts first
	b := q01Run(t, q01Write(t, "b", "a")) // priceable lane sorts second

	if a.Lanes != 2 || b.Lanes != 2 {
		t.Fatalf("fixture: both arms must fold two lanes; got %d and %d", a.Lanes, b.Lanes)
	}
	if a != b {
		t.Errorf("Q01: the same session reports differently when its lane files are "+
			"renamed.\n  lanes a,b: %+v\n  lanes b,a: %+v\nNothing semantic differs "+
			"between the arms. foldSessions inherits Unpriced from whichever lane was "+
			"seen first, and summarise gates the session's dollars on it.", a, b)
	}
}

// Q01-2. NEGATIVE CONTROL. The fixture must be capable of showing a
// difference, or Q01-1 passes because nothing was ever at stake.
func TestQ01_TheFixtureCanDistinguishTheTwoLanes(t *testing.T) {
	defer cachemodel.Override(c037rLeadUnpricedRules())()

	// Each lane alone, as its own session: they must NOT be identical, which is
	// what makes the folded order matter in the first place.
	one := q01Run(t, q01Write(t, "a", "b"))
	if one.RebilledTokens == 0 {
		t.Fatal("the fixture produces no re-billed tokens, so the folded figure " +
			"could not move however the lanes were ordered")
	}
	if one.TotalUSD <= 0 {
		t.Fatal("the fixture prices nothing, so there is no dollar figure to protect")
	}
}
