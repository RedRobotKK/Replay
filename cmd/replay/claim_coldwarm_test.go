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

// RPL-C034, the repair's behavioural oracle. Written BEFORE the production
// change and not altered to accommodate it.
//
// The decisive property is not that a count comes out right once. It is that
// the COLD walk and the WARM index path agree, because the previous attempt
// at this repair satisfied every behavioural invariant and then lost the
// disclosure the moment the same corpus was read twice.

// cwObserved is everything a reader can see, from both surfaces.
type cwObserved struct {
	rebilledTokens int
	rebilledUSD    float64
	unpriced       int
	unreadable     int
	tasks          int
	taskTokens     map[string]int
}

func cwRun(t *testing.T, dir string) cwObserved {
	t.Helper()
	var j, e bytes.Buffer
	if err := runCost([]string{"--per-task", "--json", dir}, &j, &e); err != nil {
		t.Fatalf("cost: %v (%s)", err, e.String())
	}
	var doc struct {
		Unpriced   int `json:"unpriced"`
		Unreadable int `json:"unreadable"`
		Summary    struct {
			Tasks          int     `json:"tasks"`
			RebilledTokens int     `json:"rebilledTokens"`
			RebilledUSD    float64 `json:"rebilledUsd"`
		} `json:"summary"`
		Tasks []struct {
			Session        string `json:"session"`
			RebilledTokens int    `json:"rebilledTokens"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("cost JSON does not parse: %v", err)
	}
	out := cwObserved{
		rebilledTokens: doc.Summary.RebilledTokens,
		rebilledUSD:    doc.Summary.RebilledUSD,
		unpriced:       doc.Unpriced,
		unreadable:     doc.Unreadable,
		tasks:          doc.Summary.Tasks,
		taskTokens:     map[string]int{},
	}
	for _, task := range doc.Tasks {
		out.taskTokens[task.Session] = task.RebilledTokens
	}
	return out
}

// cwCorpus writes one session per spec into a SHARED home, so a second run
// over the same directory reads the index this one wrote.
type cwSpec struct {
	session string
	model   string
	writes  int  // cache_creation on both turns; the deficit the break produces
	free    bool // every usage field zero: a genuine $0
	broken  bool // malformed JSONL: the unreadable path
}

func cwCorpus(t *testing.T, specs ...cwSpec) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	for n, sp := range specs {
		if sp.broken {
			if err := os.WriteFile(filepath.Join(dir, sp.session+".jsonl"),
				[]byte("{not json at all\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		var lines []byte
		for i := 0; i < 2; i++ {
			u := transcript.Usage{Input: 500, CacheCreation: sp.writes, CacheRead: 0, Output: 300}
			if sp.free {
				u = transcript.Usage{}
			}
			rec := ledger.Record{
				Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(n*10+i) * time.Minute),
				SessionID: sp.session, RequestID: sp.session + "-" + string(rune('A'+i)),
				Path: "/v1/messages", Status: 200, LatencyMS: 900,
				RequestSummary: ledger.RequestSummary{Model: sp.model,
					Prompt: ledger.Prompt{SystemBytes: 400, Messages: []ledger.Message{
						{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: sp.writes*4 + 1000}}}}}},
				Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 900}}},
			}
			b, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(b, '\n')...)
		}
		if err := os.WriteFile(filepath.Join(dir, sp.session+".jsonl"), lines, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// CW1. The four states, each on its own corpus so one does not mask another.
//
// ORACLE INDEPENDENCE, stated because it is the whole value of this test.
//
// Every expected value below is a DECLARED FIXTURE FACT, written here before
// the product is run. None is read back from Replay. Specifically:
//
//	expectedTokens  the deficit the fixture constructs, which is the prefix
//	                turn 2 re-writes instead of reading. Derived from the
//	                DEFINITION of Break.Deficit as Expected minus Actual
//	                (internal/analysis/diff.go:48), read rather than executed.
//	hasPrice        a property of the MODEL NAME chosen, not of any lookup.
//	expectedUSD     stated as a kind, not a number: measured, zero, or
//	                unavailable. "unavailable" and "zero" are deliberately
//	                different kinds and are never both encoded as 0.
//
// The `priced` row is the positive control for the token model itself: if the
// fixture's declared deficit and the product's reported deficit agree there,
// the model of Deficit is sound, and the other rows can rely on it. If they
// disagree the test fails loudly rather than silently adopting whatever the
// product returned.
//
// cachemodel.PriceFor appears in this campaign's tests ONLY in
// precondition checks, never in expectation generation:
// TestRB0_FixtureAssumptions here, and the equivalent guards in
// claim_refusal_matrix_test.go and claim_refusal_surface_test.go. Each asserts
// that the model names used really do sit on the intended sides of the price
// table, so that a table change makes the fixture fail instead of quietly
// measuring something else. None computes an expected token count, dollar
// figure or winner.
//
// The distinction is the one that matters: an oracle is contaminated when
// production decides WHAT THE ANSWER IS, not when a test checks its own
// premises. The guard is justified by a defect it would have caught: MX8's
// first version zeroed input and output while leaving cache usage priced, so
// its "genuine zero" was not zero at all.

// usdKind keeps "unavailable" and "zero" apart at the type level, so the
// oracle cannot express them with the same value even by accident.
type usdKind int

const (
	usdMeasured    usdKind = iota // a real dollar figure, greater than zero
	usdZero                       // measured, and the measurement is zero
	usdUnavailable                // no price exists; there is no figure to state
)

func TestCW1_TheFourStates(t *testing.T) {
	cases := []struct {
		name string
		spec cwSpec
		// Declared fixture facts. Not read back from the product.
		expectedTokens int
		hasPrice       bool
		expectedUSD    usdKind
		wantUnpriced   int
		wantUnreadable int
	}{
		{"priced", cwSpec{"s", rbPriced, 40_000, false, false},
			40_000, true, usdMeasured, 0, 0},
		{"unpriced", cwSpec{"s", rbUnpriced, 40_000, false, false},
			40_000, false, usdUnavailable, 1, 0},
		{"genuine zero", cwSpec{"s", rbPriced, 0, true, false},
			0, true, usdZero, 0, 0},
		{"unreadable", cwSpec{"s", rbPriced, 0, false, true},
			0, false, usdUnavailable, 0, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := cwRun(t, cwCorpus(t, c.spec))

			if got.rebilledTokens != c.expectedTokens {
				t.Errorf("re-billed tokens = %d, fixture declares %d. A deficit is "+
					"Expected minus Actual and consults no price table.",
					got.rebilledTokens, c.expectedTokens)
			}

			switch c.expectedUSD {
			case usdMeasured:
				if got.rebilledUSD <= 0 {
					t.Errorf("re-billed USD = %.6f, fixture declares a measured figure "+
						"greater than zero", got.rebilledUSD)
				}
			case usdZero, usdUnavailable:
				// Both read as 0 on the wire today, which is exactly why they
				// are told apart by the `unpriced` disclosure below and not by
				// the number. The oracle never conflates them.
				if got.rebilledUSD != 0 {
					t.Errorf("re-billed USD = %.6f, fixture declares none", got.rebilledUSD)
				}
			}

			if got.unpriced != c.wantUnpriced {
				t.Errorf("unpriced = %d, want %d. This is what separates an "+
					"unavailable dollar figure from a measured zero; the number "+
					"alone cannot.", got.unpriced, c.wantUnpriced)
			}
			if got.unreadable != c.wantUnreadable {
				t.Errorf("unreadable = %d, want %d. An unreadable file must not "+
					"become an unpriced unit.", got.unreadable, c.wantUnreadable)
			}
			// The two zero-valued states must never be reported identically.
			if c.expectedUSD == usdZero && got.unpriced != 0 {
				t.Error("a genuinely free session was disclosed as unpriced; zero and " +
					"unavailable have collapsed")
			}
			if c.expectedUSD == usdUnavailable && c.hasPrice {
				t.Fatal("fixture states no price is available and also that the model " +
					"has one; the declared facts contradict each other")
			}
		})
	}
}

// CW2. COLD/WARM CONSERVATION. The regression that killed the last attempt.
func TestCW2_ColdAndWarmAgree(t *testing.T) {
	for _, model := range []string{rbPriced, rbUnpriced} {
		t.Run(model, func(t *testing.T) {
			dir := cwCorpus(t,
				cwSpec{"sess-a", rbPriced, 10_000, false, false},
				cwSpec{"sess-b", model, 40_000, false, false})

			cold := cwRun(t, dir)
			warm := cwRun(t, dir) // same home, same corpus: reads the index just written

			if cold.rebilledTokens != warm.rebilledTokens {
				t.Errorf("re-billed tokens: cold %d, warm %d", cold.rebilledTokens, warm.rebilledTokens)
			}
			if cold.rebilledUSD != warm.rebilledUSD {
				t.Errorf("re-billed USD: cold %.6f, warm %.6f", cold.rebilledUSD, warm.rebilledUSD)
			}
			if cold.unpriced != warm.unpriced {
				t.Errorf("UNPRICED DISCLOSURE: cold %d, warm %d. A disclosure that "+
					"survives only while the cache is cold disappears the moment "+
					"anyone uses the tool twice.", cold.unpriced, warm.unpriced)
			}
			if cold.unreadable != warm.unreadable {
				t.Errorf("unreadable: cold %d, warm %d", cold.unreadable, warm.unreadable)
			}
			if cold.tasks != warm.tasks {
				t.Errorf("tasks: cold %d, warm %d", cold.tasks, warm.tasks)
			}
			for _, s := range []string{"sess-a", "sess-b"} {
				if cold.taskTokens[s] != warm.taskTokens[s] {
					t.Errorf("%s re-billed tokens: cold %d, warm %d",
						s, cold.taskTokens[s], warm.taskTokens[s])
				}
			}
		})
	}
}

// CW3. Ranking, cold AND warm, control and treatment.
func TestCW3_RankingHoldsColdAndWarm(t *testing.T) {
	const oracleWinner = "sess-b" // 40,000 against 10,000, chosen here

	for _, tc := range []struct{ name, modelB string }{
		{"control both priced", rbPriced},
		{"treatment B unpriced", rbUnpriced},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := cwCorpus(t,
				cwSpec{"sess-a", rbPriced, 10_000, false, false},
				cwSpec{"sess-b", tc.modelB, 40_000, false, false})

			for _, pass := range []string{"cold", "warm"} {
				got := cwRun(t, dir)
				rep := costReport{}
				rep.Tasks = append(rep.Tasks,
					newTask("sess-a", got.taskTokens["sess-a"]),
					newTask("sess-b", got.taskTokens["sess-b"]))
				idx := worstByRebilledTokens(rep, []int{0, 1})
				if idx < 0 {
					t.Fatalf("%s: no winner at all; the corpus did not reach the report", pass)
				}
				if w := rep.Tasks[idx].Session; w != oracleWinner {
					t.Errorf("%s: product named %s, oracle says %s (A=%d B=%d)",
						pass, w, oracleWinner, got.taskTokens["sess-a"], got.taskTokens["sess-b"])
				}
			}
		})
	}
}
