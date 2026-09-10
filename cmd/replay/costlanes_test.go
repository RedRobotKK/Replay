package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// `replay cost` must price every lane of a session.
//
// It priced MainLane(session) alone, which on the corpus that surfaced it
// dropped 21,854 of 60,401 requests — 36.2% — concentrated in 8 files out of
// 1,812. Those eight are the fan-out sessions, so 36% of the requests was 2.8x
// of the dollars: `replay cost` reported $3,721 where `replay burn`, pricing
// every lane, reported $10,499 on the same corpus in the same minute.
//
// internal/analysis has unit tests for the arithmetic. This one is about the
// wiring, because the arithmetic being right and the command not calling it is
// how the original defect looked from the outside — and is a shape that has
// slipped past a green suite twice in this repository already.

// twoLaneTranscript writes a session whose requests root in two different
// parent chains, which is what makes them two lanes.
func twoLaneTranscript(t *testing.T, path, session string, perLane int) {
	t.Helper()
	var b bytes.Buffer
	write := func(root string, i int) {
		uuid := root + "-" + string(rune('a'+i))
		parent := any(nil)
		if i > 0 {
			parent = root + "-" + string(rune('a'+i-1))
		}
		line := map[string]any{
			"type": "assistant", "requestId": uuid + "-req", "sessionId": session,
			"uuid": uuid, "parentUuid": parent,
			"timestamp": "2026-09-01T00:00:00.000Z", "version": "2.1.233",
			"message": map[string]any{
				"role": "assistant", "type": "message", "model": "claude-opus-5",
				"content": []any{map[string]any{"type": "text", "text": "x"}},
				"usage": map[string]any{
					"input_tokens": 1000, "output_tokens": 100,
					"cache_read_input_tokens": 4000, "cache_creation_input_tokens": 500,
				},
			},
		}
		enc, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(enc)
		b.WriteByte('\n')
	}
	for i := 0; i < perLane; i++ {
		write("laneone", i)
		write("lanetwo", i)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCostPricesEveryLaneOfASession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REPLAY_TRANSCRIPTS", "")

	const sess = "aaaa1111-2222-3333-4444-555555555555"
	proj := filepath.Join(home, ".claude", "projects", "proj-a")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	twoLaneTranscript(t, filepath.Join(proj, sess+".jsonl"), sess, 3)

	var out bytes.Buffer
	if err := runCost([]string{"--per-task", "--json", proj}, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("cost failed: %v", err)
	}
	var doc struct {
		Tasks []struct {
			Session      string  `json:"session"`
			Requests     int     `json:"requests"`
			CostUSD      float64 `json:"costUsd"`
			AvoidableUSD float64 `json:"avoidableUsd"`
			Breaks       int     `json:"breaks"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("cost --json is not JSON: %v\n%s", err, out.String())
	}
	if len(doc.Tasks) != 1 {
		t.Fatalf("one session produced %d rows: %s", len(doc.Tasks), out.String())
	}
	// Six requests: three in each of two lanes. Main-lane pricing sees three.
	if got := doc.Tasks[0].Requests; got != 6 {
		t.Errorf("a two-lane session of 6 requests priced %d. Main-lane pricing "+
			"reports 3 here, which is the defect this test exists for", got)
	}
	if doc.Tasks[0].CostUSD <= 0 {
		t.Fatalf("the session priced at zero: %s", out.String())
	}
	// And the dollars scale with the lanes, not just the count. Six identical
	// requests must cost twice what three of them cost, or the row is
	// reporting a request count it did not price.
	var one bytes.Buffer
	dir := t.TempDir()
	oneLane := filepath.Join(dir, "proj")
	if err := os.MkdirAll(oneLane, 0o755); err != nil {
		t.Fatal(err)
	}
	oneLaneTranscript(t, filepath.Join(oneLane, sess+".jsonl"), sess, 3)
	if err := runCost([]string{"--per-task", "--json", oneLane}, &one, &bytes.Buffer{}); err != nil {
		t.Fatalf("cost failed on the one-lane corpus: %v", err)
	}
	var single struct {
		Tasks []struct {
			Requests     int     `json:"requests"`
			CostUSD      float64 `json:"costUsd"`
			AvoidableUSD float64 `json:"avoidableUsd"`
			Breaks       int     `json:"breaks"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(one.Bytes(), &single); err != nil || len(single.Tasks) != 1 {
		t.Fatalf("the one-lane control did not produce a row: %v\n%s", err, one.String())
	}
	if single.Tasks[0].Requests != 3 {
		t.Fatalf("the control has %d requests, want 3", single.Tasks[0].Requests)
	}
	ratio := doc.Tasks[0].CostUSD / single.Tasks[0].CostUSD
	if ratio < 1.9 || ratio > 2.1 {
		t.Errorf("six requests over two lanes cost %.3fx three over one, want 2x. "+
			"The request count covers both lanes and the dollars do not", ratio)
	}

	// The causes have to cover the same traffic as the cost. A total that
	// doubled beside an unchanged avoidable figure reads as "the waste got
	// proportionally smaller", which is a claim nobody made and nothing
	// measured.
	if single.Tasks[0].Breaks == 0 {
		t.Fatal("the control session records no cache breaks, so the cause check " +
			"below cannot fail and is not a check")
	}
	if got, want := doc.Tasks[0].Breaks, single.Tasks[0].Breaks*2; got != want {
		t.Errorf("two lanes report %d cache breaks, one lane %d; want %d. The causes "+
			"are still being counted on the main lane alone",
			got, single.Tasks[0].Breaks, want)
	}
	if single.Tasks[0].AvoidableUSD > 0 {
		ar := doc.Tasks[0].AvoidableUSD / single.Tasks[0].AvoidableUSD
		if ar < 1.9 || ar > 2.1 {
			t.Errorf("avoidable over two lanes is %.3fx one lane, want 2x", ar)
		}
	}
}

// oneLaneTranscript is the same traffic in a single parent chain: the control
// for the ratio check above.
func oneLaneTranscript(t *testing.T, path, session string, n int) {
	t.Helper()
	var b bytes.Buffer
	for i := 0; i < n; i++ {
		uuid := "only-" + string(rune('a'+i))
		var parent any
		if i > 0 {
			parent = "only-" + string(rune('a'+i-1))
		}
		line := map[string]any{
			"type": "assistant", "requestId": uuid + "-req", "sessionId": session,
			"uuid": uuid, "parentUuid": parent,
			"timestamp": "2026-09-01T00:00:00.000Z", "version": "2.1.233",
			"message": map[string]any{
				"role": "assistant", "type": "message", "model": "claude-opus-5",
				"content": []any{map[string]any{"type": "text", "text": "x"}},
				"usage": map[string]any{
					"input_tokens": 1000, "output_tokens": 100,
					"cache_read_input_tokens": 4000, "cache_creation_input_tokens": 500,
				},
			},
		}
		enc, err := json.Marshal(line)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(enc)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// And the fixture is only meaningful if it really has two lanes. A one-lane
// fixture would make the test above pass against the defect it names.
func TestTheTwoLaneFixtureReallyHasTwoLanes(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	twoLaneTranscript(t, p, "sess", 3)
	s, err := parseForTest(p)
	if err != nil {
		t.Fatalf("the fixture does not parse: %v", err)
	}
	if len(s.Lanes) != 2 {
		t.Fatalf("the fixture has %d lane(s), want 2; every assertion about "+
			"multi-lane pricing above is then vacuous", len(s.Lanes))
	}
	total := 0
	for _, l := range s.Lanes {
		total += len(l.Requests)
	}
	if total != 6 {
		t.Errorf("the fixture holds %d requests across its lanes, want 6", total)
	}
	if strings.TrimSpace(s.ID) == "" {
		t.Error("the fixture carries no session id, so nothing can fold it")
	}
}

// parseForTest is the parser the walk uses, named here so the fixture check
// above reads as what it is.
var parseForTest = transcript.ParseClaudeCodeFile
