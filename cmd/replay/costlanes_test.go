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

// An anonymous request is skipped by the overlap count, not keyed on "".
//
// The Claude Code parser cannot produce one: it groups assistant lines BY
// request id and drops those without. The ledger and the Codex reader are not
// bound by that, and keying every anonymous request on the empty string would
// collapse them into one and report the rest as duplicates of it — publishing a
// re-render rate that is an artefact of the key rather than a fact about the
// corpus. The note built on that count is the one that says "430 of 38,186
// requests appear in more than one transcript".
func TestAnonymousRequestsDoNotBecomeDuplicatesOfEachOther(t *testing.T) {
	lane := func(ids ...string) *transcript.Lane {
		l := &transcript.Lane{}
		for _, id := range ids {
			l.Requests = append(l.Requests, &transcript.Request{ID: id, Model: "claude-opus-5"})
		}
		return l
	}
	s := &transcript.Session{ID: "sess", Lanes: []*transcript.Lane{
		lane("a", "", "b", "", ""),
	}}

	seen := map[string]bool{}
	ids, total, dup := requestIDs(s, seen)
	if total != 2 {
		t.Errorf("two identified requests among five counted as %d", total)
	}
	if dup != 0 {
		t.Errorf("three anonymous requests produced %d duplicate(s); they were keyed "+
			"on the empty string and became duplicates of each other", dup)
	}
	if len(ids) != 2 {
		t.Errorf("collected %d ids, want 2", len(ids))
	}
	if seen[""] {
		t.Error(`the empty string was recorded as a seen request id, so the next ` +
			`anonymous request in any file counts as a re-render of this one`)
	}
}

// And a genuine cross-file re-render is still counted, so the skip above is not
// hiding the thing the counter exists for.
func TestARepeatedIDIsCountedAsADuplicate(t *testing.T) {
	lane := &transcript.Lane{Requests: []*transcript.Request{
		{ID: "a", Model: "claude-opus-5"},
		{ID: "b", Model: "claude-opus-5"},
	}}
	s := &transcript.Session{ID: "sess", Lanes: []*transcript.Lane{lane}}

	seen := map[string]bool{"a": true} // "a" already arrived in an earlier file
	_, total, dup := requestIDs(s, seen)
	if total != 2 {
		t.Errorf("counted %d requests, want 2", total)
	}
	if dup != 1 {
		t.Errorf("one request already seen in another file counted as %d duplicate(s)", dup)
	}
}

// A session whose model nothing prices is excluded and disclosed, never
// counted as free.
//
// This is the branch that decides whether unpriced spend is reported at all.
// `replay cost` says of it: "They are excluded rather than counted as free."
func TestAnUnpricedSessionIsExcludedAndCounted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REPLAY_TRANSCRIPTS", "")

	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	// One priced session and one on a model no table carries.
	twoLaneTranscript(t, filepath.Join(proj, "aaaa1111-priced.jsonl"), "aaaa1111-priced", 2)
	unpricedTranscript(t, filepath.Join(proj, "bbbb2222-unpriced.jsonl"), "bbbb2222-unpriced")

	var out bytes.Buffer
	if err := runCost([]string{"--json", proj}, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("cost failed: %v", err)
	}
	var doc struct {
		Unpriced int `json:"unpriced"`
		Summary  struct {
			Tasks int `json:"tasks"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if doc.Unpriced != 1 {
		t.Errorf("one unpriceable session reported as %d unpriced; it would otherwise "+
			"be counted as free or not counted at all", doc.Unpriced)
	}
	if doc.Summary.Tasks != 1 {
		t.Errorf("%d task(s) priced, want 1: the unpriceable session was included in "+
			"the total", doc.Summary.Tasks)
	}
}

// unpricedTranscript writes a session on a model no price table carries.
func unpricedTranscript(t *testing.T, path, session string) {
	t.Helper()
	line := map[string]any{
		"type": "assistant", "requestId": session + "-req", "sessionId": session,
		"uuid": session + "-u", "parentUuid": nil,
		"timestamp": "2026-09-01T00:00:00.000Z", "version": "2.1.233",
		"message": map[string]any{
			"role": "assistant", "type": "message", "model": "no-such-model-anywhere",
			"content": []any{map[string]any{"type": "text", "text": "x"}},
			"usage": map[string]any{
				"input_tokens": 1000, "output_tokens": 100,
				"cache_read_input_tokens": 4000, "cache_creation_input_tokens": 500,
			},
		},
	}
	b, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The second run of cost over an unchanged corpus reads the index instead of
// re-parsing, and reports the same figures.
//
// The index took `replay cost` from 6.474s to 0.046s on this corpus, so the
// warm path is the one almost every real run takes — and it is a second source
// of truth for every number the command prints. A cache that disagreed with a
// cold read would be the worst kind of wrong: fast and confident.
func TestTheWarmIndexAgreesWithTheColdRead(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("REPLAY_TRANSCRIPTS", "")

	proj := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	twoLaneTranscript(t, filepath.Join(proj, "aaaa1111-x.jsonl"), "aaaa1111-x", 3)

	read := func() string {
		var out, errs bytes.Buffer
		if err := runCost([]string{"--per-task", "--json", proj}, &out, &errs); err != nil {
			t.Fatalf("cost failed: %v", err)
		}
		return out.String()
	}
	cold := read()
	var errs bytes.Buffer
	var out bytes.Buffer
	if err := runCost([]string{"--per-task", "--json", proj}, &out, &errs); err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if !strings.Contains(errs.String(), "reused from the index") {
		t.Fatalf("the second run did not read the index, so this test never entered "+
			"the warm path:\n%s", errs.String())
	}
	if out.String() != cold {
		t.Errorf("the warm read disagrees with the cold one.\ncold:\n%s\nwarm:\n%s",
			cold, out.String())
	}
}
