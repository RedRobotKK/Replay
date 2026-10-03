package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RPL-C035-LF. The lane fold.
//
// foldSessions merges rows that share a session id into one session row. Those
// rows come from separate FILES: Claude Code writes a sub-agent lane to
// <session>/subagents/agent-<id>.jsonl and every one carries the parent's
// session id (cost.go:35,158). C035's fixture was single-file, so the
// coverage pair's fold at cost.go:236 was never exercised, and a mutation
// against that line survived.
//
// This is a PROOF gap rather than desirable coverage: the C035 claim is about
// sessions, sessions can span lanes, and a broken fold would falsify the claim
// for every multi-lane session while leaving the single-lane proof intact.

// lfLane writes one lane file. Same session id across calls, different model
// per lane, identical usage so only priceability varies.
func lfLane(t *testing.T, dir, session, lane, model string, n int, at time.Time) {
	t.Helper()
	var lines []byte
	for i := 0; i < n; i++ {
		u := transcript.Usage{Input: 500, CacheCreation: 20_000, CacheRead: 0, Output: 300}
		rec := ledger.Record{
			Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(i) * time.Minute),
			SessionID: session, RequestID: lane + "-" + string(rune('A'+i)),
			Path: "/v1/messages", Status: 200, LatencyMS: 900,
			RequestSummary: ledger.RequestSummary{Model: model,
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
	if err := os.WriteFile(filepath.Join(dir, lane+".jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
}

// LF1. Two lanes of ONE session, priceability differing between them.
//
// DECLARED FACTS: lane one is 2 records on a priced model, lane two is 2
// records on an unpriceable one. The session total must therefore be 2 priced
// and 2 unpriceable, which only holds if the fold sums both lanes.
func TestLF1_TheLaneFoldSumsCoverageAcrossLanes(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	const session = "lf-session"
	lfLane(t, dir, session, "lane-one", ptPriced, 2, at)
	lfLane(t, dir, session, "lane-two", ptUnpriced, 2, at.Add(time.Hour))

	var j, e bytes.Buffer
	if err := runCost([]string{"--per-task", "--json", dir}, &j, &e); err != nil {
		t.Fatalf("cost: %v (%s)", err, e.String())
	}
	var doc struct {
		Summary struct {
			Tasks            int `json:"tasks"`
			Lanes            int `json:"lanes"`
			PricedRequests   int `json:"pricedRequests"`
			UnpricedRequests int `json:"unpricedRequests"`
		} `json:"summary"`
		Tasks []struct {
			Session          string `json:"session"`
			Lanes            int    `json:"lanes"`
			Requests         int    `json:"requests"`
			PricedRequests   int    `json:"pricedRequests"`
			UnpricedRequests int    `json:"unpricedRequests"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("cost JSON does not parse: %v", err)
	}

	// POSITIVE CONTROL: the fold must actually have happened, or this test
	// measures a single-lane case under a two-lane name.
	if len(doc.Tasks) != 1 {
		t.Fatalf("got %d task rows, want 1: two lane files sharing a session id "+
			"must fold into one session row, and this fixture is not exercising "+
			"the fold at all", len(doc.Tasks))
	}
	row := doc.Tasks[0]
	if row.Lanes < 2 {
		t.Fatalf("the session row reports %d lane(s); the fold was not exercised",
			row.Lanes)
	}

	// THE CLAIM. Declared: 2 priced, 2 unpriceable, across two lanes.
	if row.PricedRequests != 2 || row.UnpricedRequests != 2 {
		t.Errorf("folded session reports priced=%d unpriceable=%d; the fixture "+
			"declares 2 and 2 across two lanes. The coverage pair is not being "+
			"summed by foldSessions.", row.PricedRequests, row.UnpricedRequests)
	}
	if row.PricedRequests+row.UnpricedRequests != row.Requests {
		t.Errorf("INVARIANT VIOLATED after the fold: priced %d + unpriceable %d != "+
			"requests %d", row.PricedRequests, row.UnpricedRequests, row.Requests)
	}
	if doc.Summary.PricedRequests != 2 || doc.Summary.UnpricedRequests != 2 {
		t.Errorf("corpus summary reports priced=%d unpriceable=%d, want 2 and 2",
			doc.Summary.PricedRequests, doc.Summary.UnpricedRequests)
	}
	t.Logf("LF1: %d lanes folded into 1 session row, priced=%d unpriceable=%d "+
		"requests=%d", row.Lanes, row.PricedRequests, row.UnpricedRequests, row.Requests)
}
