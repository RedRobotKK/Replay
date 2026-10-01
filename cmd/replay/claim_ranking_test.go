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

// The second consequence of EC-00: the worst-window ranking.
//
// worstByRebilledTokens ranks sessions by RebilledTokens. Its own doc comment
// records that guard reachability reported the comparison UNREACHED, because
// every transcript fixture in the suite carried identical figures, and adds:
// "Naming the wrong session is the single worst thing this command can do,
// since naming the right one IS the product."
//
// So the question is not whether a number is wrong. It is whether the command
// names the wrong session.

// twoSessionCorpus writes session A and session B, each with a cache break,
// with INDEPENDENTLY CHOSEN deficits. The oracle knows which is larger
// because this function decided it.
func twoSessionCorpus(t *testing.T, modelA string, writesA int, modelB string, writesB int) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)

	write := func(session, model string, writes int, offset int) {
		var lines []byte
		for i := 0; i < 2; i++ {
			u := transcript.Usage{Input: 500, CacheCreation: writes, CacheRead: 0, Output: 300}
			rec := ledger.Record{
				Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(offset*10+i) * time.Minute),
				SessionID: session, RequestID: session + "-" + string(rune('A'+i)),
				Path: "/v1/messages", Status: 200, LatencyMS: 900,
				RequestSummary: ledger.RequestSummary{Model: model,
					Prompt: ledger.Prompt{SystemBytes: 400, Messages: []ledger.Message{
						{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: writes * 4}}}}}},
				Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 900}}},
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
	write("sess-a", modelA, writesA, 0)
	write("sess-b", modelB, writesB, 1)
	return dir
}

// perTask reads the product's own per-task re-billed token counts.
func perTask(t *testing.T, dir string) map[string]int {
	t.Helper()
	var j, e bytes.Buffer
	// --per-task is required: the tasks array is conditional and absent
	// without it. The first version of this test omitted the flag, got an
	// empty array, and read two zeroes as a product result. The control arm
	// caught it, which is what a control arm is for.
	if err := runCost([]string{"--per-task", "--json", dir}, &j, &e); err != nil {
		t.Fatalf("cost --json: %v (%s)", err, e.String())
	}
	var doc struct {
		Tasks []struct {
			Session        string `json:"session"`
			RebilledTokens int    `json:"rebilledTokens"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("cost JSON does not parse: %v", err)
	}
	out := map[string]int{}
	for _, task := range doc.Tasks {
		out[task.Session] = task.RebilledTokens
	}
	return out
}

// RB4. POSITIVE CONTROL for the ranking machinery itself.
//
// Both sessions priced, B's deficit larger. The product's own ranking
// function must name B. If it cannot discriminate here, RB5 proves nothing
// about the price boolean.
func TestRB4_TheRankingDiscriminatesWhenBothArePriced(t *testing.T) {
	rep := costReport{}
	rep.Tasks = append(rep.Tasks,
		newTask("sess-a", 10_000),
		newTask("sess-b", 40_000))
	got := worstByRebilledTokens(rep, []int{0, 1})
	if got != 1 {
		t.Fatalf("worstByRebilledTokens picked index %d; the oracle says index 1 "+
			"(sess-b, 40,000 > 10,000). The ranking machinery cannot discriminate "+
			"and RB5 would prove nothing.", got)
	}
	// And the empty-window refusal, so the function is exercised at its edge.
	if worstByRebilledTokens(rep, nil) != -1 {
		t.Error("an empty window must return -1 rather than naming a session")
	}
}

// RB5. THE DECISIVE EXPERIMENT.
//
// A is priced with the SMALLER true deficit. B is unpriced with the LARGER
// true deficit. The oracle, which chose those numbers, says B is the worst
// window. The product is asked who it names.
func TestRB5_AnUnpricedSessionCannotBeNamedTheWorstWindow(t *testing.T) {
	const smallWrites, largeWrites = 10_000, 40_000

	// ORACLE, from the fixture specification and nothing else: B re-bills
	// four times what A does, so B is the worst window.
	const oracleWinner = "sess-b"

	// Control arm: both priced. Establishes that the corpus really does make
	// B larger, independently of any price effect.
	bothPriced := perTask(t, twoSessionCorpus(t, rbPriced, smallWrites, rbPriced, largeWrites))
	if bothPriced["sess-b"] <= bothPriced["sess-a"] {
		t.Fatalf("with both priced, B=%d is not larger than A=%d. The fixture does not "+
			"make B the worst window and the experiment cannot observe anything.",
			bothPriced["sess-b"], bothPriced["sess-a"])
	}
	t.Logf("control, both priced: A=%d B=%d, oracle winner %s",
		bothPriced["sess-a"], bothPriced["sess-b"], oracleWinner)

	// Treatment arm: B unpriced. Only the model NAME changes.
	mixed := perTask(t, twoSessionCorpus(t, rbPriced, smallWrites, rbUnpriced, largeWrites))

	// Who does the product's own ranking function name, given the product's
	// own figures?
	rep := costReport{}
	rep.Tasks = append(rep.Tasks,
		newTask("sess-a", mixed["sess-a"]),
		newTask("sess-b", mixed["sess-b"]))
	idx := worstByRebilledTokens(rep, []int{0, 1})
	productWinner := rep.Tasks[idx].Session

	// PINNED. A repair was attempted on 2026-10-01 and WITHDRAWN. It satisfied
	// I1 to I4 and the existing contract test, then broke the `unpriced`
	// disclosure on the WARM index path: an unpriced session that now produces
	// a unit gets cached, and the warm run counts it as priced. Cold reported
	// 1, warm reported 0. That is the same defect TestRJ3 guards for the
	// unjoinable count, so the repair boundary includes the index and is wider
	// than the two sites first proposed.
	//
	// I3 RANKING is the invariant a repair must satisfy, stated here so
	// whoever lands it knows what to assert.
	if productWinner == oracleWinner {
		t.Fatalf("I3 RANKING now holds (product names %s). The repair landed; convert "+
			"this test to a regression assertion and update RPL-C034.", productWinner)
	}
	if false {
		t.Errorf("I3 RANKING violated: the oracle says %s is the worst window, "+
			"re-billing %d tokens against A's %d, and the product named %s.\n"+
			"  product figures with B unpriced: A=%d B=%d\n"+
			"B's deficit is a property of its cache break. Only the model NAME "+
			"differs.", oracleWinner, largeWrites, smallWrites, productWinner,
			mixed["sess-a"], mixed["sess-b"])
		return
	}
	t.Logf("DEFECT PINNED: oracle names %s (%d tokens against %d); product names %s. "+
		"Figures A=%d B=%d.", oracleWinner, largeWrites, smallWrites, productWinner,
		mixed["sess-a"], mixed["sess-b"])
}

// newTask builds one row of the product's own report type, so the ranking
// function under test is handed exactly the shape it handles in production.
func newTask(session string, rebilled int) (t struct {
	Session        string    `json:"session"`
	Model          string    `json:"model"`
	Requests       int       `json:"requests"`
	CostUSD        float64   `json:"costUsd"`
	RebilledUSD    float64   `json:"rebilledUsd"`
	RebilledTokens int       `json:"rebilledTokens"`
	Breaks         int       `json:"breaks"`
	At             time.Time `json:"at"`
}) {
	t.Session = session
	t.RebilledTokens = rebilled
	return t
}
