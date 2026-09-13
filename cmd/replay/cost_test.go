package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Cost per task is the unit-economics figure: what one session cost, and what
// share of it nobody chose to spend. The aggregation is where a report like
// this lies most easily, so the rules are pinned here.
//
// A mean is the wrong summary. One 40-hour session drags it somewhere no real
// task lives, and a person reading "average task: $12" plans against a number
// that describes none of their work. The median and the p90 are what a unit
// economics conversation actually needs.

func TestCostSummaryReportsMedianAndP90NotMean(t *testing.T) {
	var units []costUnit
	for i := 0; i < 99; i++ {
		units = append(units, costUnit{CostUSD: 1})
	}
	units = append(units, costUnit{CostUSD: 1000}) // the outlier that ruins a mean

	s := summarise(units)
	if s.MedianUSD != 1 {
		t.Fatalf("median: %v, want 1", s.MedianUSD)
	}
	if s.P90USD != 1 {
		t.Fatalf("p90: %v, want 1", s.P90USD)
	}
	if math.Abs(s.TotalUSD-1099) > 0.001 {
		t.Fatalf("total: %v, want 1099", s.TotalUSD)
	}
}

// The re-billed share is the point of the whole report, and it has to be a
// share of what was actually priced, not of everything walked past.
func TestCostSummaryRebilledIsAShareOfWhatWasPriced(t *testing.T) {
	units := []costUnit{
		{CostUSD: 10, RebilledUSD: 2},
		{CostUSD: 10, RebilledUSD: 0},
	}
	s := summarise(units)
	if math.Abs(s.RebilledShare-0.1) > 0.0001 {
		t.Fatalf("re-billed share: %v, want 0.1", s.RebilledShare)
	}
}

// A model with no price yields no cost, and a session with no cost must not be
// counted as a task that cost nothing: that would drag every figure down and
// make the corpus look cheaper than it is.
func TestCostSummaryExcludesUnpricedSessionsRatherThanCountingThemAsZero(t *testing.T) {
	units := []costUnit{{CostUSD: 4, RebilledUSD: 1}, {CostUSD: 6, RebilledUSD: 1}}
	s := summarise(units)
	if s.Tasks != 2 {
		t.Fatalf("tasks: %d, want 2", s.Tasks)
	}
	if s.MedianUSD != 5 {
		t.Fatalf("median of 4 and 6 should be 5, got %v", s.MedianUSD)
	}
	// An empty corpus is not a zero-cost corpus.
	empty := summarise(nil)
	if empty.Tasks != 0 || empty.MedianUSD != 0 || empty.RebilledShare != 0 {
		t.Fatalf("an empty corpus must summarise to nothing, got %+v", empty)
	}
}

// The headline is the number a person can act on: what the re-billed share
// would have been worth across the corpus.
func TestCostSummaryRendersTheActionableNumber(t *testing.T) {
	s := summarise([]costUnit{
		{CostUSD: 100, RebilledUSD: 12},
		{CostUSD: 50, RebilledUSD: 3},
	})
	out := renderCost(s, 0, 0, io.Discard, "")
	for _, want := range []string{"$150.00", "10%", "$15.00"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report is missing %q:\n%s", want, out)
		}
	}
}

func TestCostPrintsFourBilledLegs(t *testing.T) {
	s := summarise([]costUnit{{
		CostUSD: 10, UncachedUSD: 1, WriteUSD: 4, ReadUSD: 3.5, OutputUSD: 1.5,
	}})
	out := renderCost(s, 0, 0, io.Discard, "")
	for _, want := range []string{"cache write", "cache read", "uncached", "output"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report is missing billed leg %q:\n%s", want, out)
		}
	}
}

// Sessions the engine could not reproduce must be named, not silently dropped,
// because a cost report that quietly ignores what it could not read is exactly
// the kind of number this tool exists to distrust.
func TestCostReportNamesWhatItCouldNotPrice(t *testing.T) {
	out := renderCost(summarise([]costUnit{{CostUSD: 1}}), 7, 0, io.Discard, "")
	if !strings.Contains(out, "7") {
		t.Fatalf("the report must say how many sessions it could not price:\n%s", out)
	}
}

// A row that says `session` must be one session.
//
// It was not. The unit of the whole report is a transcript FILE, and Claude
// Code writes one file per agent lane: a session that fanned out to sub-agents
// writes `<session>/subagents/agent-*.jsonl`, every one of them carrying the
// parent's `sessionId`. `cost --per-task --json` emitted one row per lane while
// naming the field `session` and the count `tasks`, so on the machine this was
// found on it reported 1614 "tasks" over 114 distinct sessions, with a single
// session id on 1014 separate rows.
//
// This project has already retracted exactly this conflation once:
// docs/evidence/README.md records a published figure of "1363 sessions" that
// was a file count, with one session supplying 1020 of them. It was corrected
// in the calibration corpus and left live in this command. A reader told "the
// top 22 of 1613 tasks are half your spend" is being shown agent lanes and
// believes they are being shown work they did.
//
// PASS: every session id appears on exactly one row, and summary.tasks counts
// those rows.
// FAIL: any id on two rows, which is the defect.
func TestCostPerTaskRowsAreSessionsNotAgentLanes(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)

	// Three transcripts of one session: the shape a fanned-out session writes.
	// The fixture's sessionId is the same in every copy, which is precisely
	// what the client does for a sub-agent lane.
	proj := filepath.Join(home, ".claude", "projects", "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := filepath.Abs(filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	const lanes = 3
	for i := 0; i < lanes; i++ {
		if err := os.Symlink(src, filepath.Join(proj, fmt.Sprintf("lane-%d.jsonl", i))); err != nil {
			t.Fatal(err)
		}
	}

	var out, errOut bytes.Buffer
	if err := runCost([]string{"--per-task", "--json", proj}, &out, &errOut); err != nil {
		t.Fatalf("cost --per-task --json: %v (stderr: %s)", err, errOut.String())
	}
	var got struct {
		Tasks []struct {
			Session  string  `json:"session"`
			Requests int     `json:"requests"`
			CostUSD  float64 `json:"costUsd"`
		} `json:"tasks"`
		Summary struct {
			Tasks    int     `json:"tasks"`
			TotalUSD float64 `json:"totalUsd"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("cost JSON does not parse: %v\n%s", err, out.String())
	}
	if len(got.Tasks) == 0 {
		t.Fatalf("no rows were priced, so this test cannot observe the defect it "+
			"was written for (stderr: %s)", errOut.String())
	}

	seen := map[string]int{}
	for _, row := range got.Tasks {
		seen[row.Session]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("session %q is on %d rows. A row keyed `session` must be one "+
				"session; these are agent lanes wearing their parent's id", id, n)
		}
	}
	if len(seen) != len(got.Tasks) {
		t.Errorf("%d rows over %d distinct sessions: the unit of the report is a "+
			"transcript file, not a session", len(got.Tasks), len(seen))
	}
	if got.Summary.Tasks != len(got.Tasks) {
		t.Errorf("summary.tasks = %d but there are %d rows; the count must count "+
			"whatever the rows are", got.Summary.Tasks, len(got.Tasks))
	}
}

// Folding lanes into a session must sum only what is additive.
//
// The fast way to get an aggregation wrong is to add up something that is not
// a quantity. Every field is pinned here with a value that would be visibly
// wrong under the other rule: the timestamps are out of order in the input so
// a first-seen `at` differs from the earliest one, and the cheap lane is
// listed first with a different model so a first-seen `model` differs from the
// one that spent the money.
func TestFoldSessionsSumsOnlyTheAdditiveFields(t *testing.T) {
	early := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	mid := time.Date(2026, 8, 1, 13, 0, 0, 0, time.UTC)
	late := time.Date(2026, 8, 1, 17, 0, 0, 0, time.UTC)

	// Three lanes, and the earliest is neither the first nor the last of them.
	// With only two, "keep the earliest" and "keep whichever came last" agree
	// on this input and a mutant that swapped one for the other survived the
	// check that was meant to catch it.
	got := foldSessions([]costUnit{
		{ID: "aaaa", Lane: "main", Model: "claude-haiku", At: late,
			Requests: 3, CostUSD: 1, RebilledUSD: 0.25, RebilledTokens: 10, Breaks: 1},
		{ID: "aaaa", Lane: "b0b0", Model: "claude-opus", At: early,
			Requests: 5, CostUSD: 9, RebilledUSD: 0.5, RebilledTokens: 60, Breaks: 2},
		{ID: "aaaa", Lane: "c1c1", Model: "claude-haiku", At: mid,
			Requests: 2, CostUSD: 0.5, RebilledUSD: 0.25, RebilledTokens: 30, Breaks: 0},
		{ID: "zzzz", Lane: "main", Model: "claude-opus", At: late,
			Requests: 1, CostUSD: 4, RebilledUSD: 0, RebilledTokens: 0, Breaks: 0},
	})

	if len(got) != 2 {
		t.Fatalf("%d rows over 2 sessions: %+v", len(got), got)
	}
	a := got[0]
	if a.ID != "aaaa" {
		t.Fatalf("rows must keep first-seen order, got %q first", a.ID)
	}
	// Counts and amounts inside a lane: additive.
	if a.Requests != 10 || a.Breaks != 3 || a.RebilledTokens != 100 {
		t.Errorf("counts must add: requests=%d breaks=%d tokens=%d, want 10/3/100",
			a.Requests, a.Breaks, a.RebilledTokens)
	}
	if math.Abs(a.CostUSD-10.5) > 1e-9 || math.Abs(a.RebilledUSD-1) > 1e-9 {
		t.Errorf("money must add: cost=%v re-billed=%v, want 10.5/1", a.CostUSD, a.RebilledUSD)
	}
	// A point in time, not a quantity. Summing or averaging two timestamps is
	// meaningless; the session began when its earliest lane began.
	if !a.At.Equal(early) {
		t.Errorf("at = %v, want the earliest lane's %v. A session did not start "+
			"when the file listed first happened to start", a.At, early)
	}
	// A category. The row names the model that spent the money, which is the
	// one a routing decision is about.
	if a.Model != "claude-opus" {
		t.Errorf("model = %q, want the costliest lane's claude-opus", a.Model)
	}
	// The fan-out, so the two units are visible rather than inferred.
	if a.Lanes != 3 || got[1].Lanes != 1 {
		t.Errorf("lanes = %d and %d, want 3 and 1", a.Lanes, got[1].Lanes)
	}
	// A session is not one lane, so it does not wear one lane's id.
	if a.Lane != "" {
		t.Errorf("a session row carries lane %q; a folded row is not one lane", a.Lane)
	}
}

// Folding moves where a figure is shown. It must not change what is in it.
//
// This is the property that makes the change safe to ship against a published
// total: the corpus was already summing every lane, so the grand total, the
// re-billed total and the re-billed share are identical before and after. Only
// the median and the p90 move, and they move because they are now percentiles
// over sessions rather than over files, which is the correction.
func TestFoldSessionsConservesTheTotals(t *testing.T) {
	lanes := []costUnit{
		{ID: "aaaa", CostUSD: 1, RebilledUSD: 0.5, RebilledTokens: 10, Breaks: 1, Requests: 2},
		{ID: "aaaa", CostUSD: 2, RebilledUSD: 0.5, RebilledTokens: 20, Breaks: 1, Requests: 3},
		{ID: "aaaa", CostUSD: 3, RebilledUSD: 1.0, RebilledTokens: 30, Breaks: 0, Requests: 4},
		{ID: "bbbb", CostUSD: 4, RebilledUSD: 0.0, RebilledTokens: 0, Breaks: 2, Requests: 5},
	}
	before, after := summarise(lanes), summarise(foldSessions(lanes))

	if math.Abs(before.TotalUSD-after.TotalUSD) > 1e-9 {
		t.Errorf("total moved: %v -> %v. Folding may not add or lose money",
			before.TotalUSD, after.TotalUSD)
	}
	if math.Abs(before.RebilledUSD-after.RebilledUSD) > 1e-9 {
		t.Errorf("re-billed moved: %v -> %v", before.RebilledUSD, after.RebilledUSD)
	}
	if before.RebilledTokens != after.RebilledTokens {
		t.Errorf("re-billed tokens moved: %d -> %d", before.RebilledTokens, after.RebilledTokens)
	}
	if math.Abs(before.RebilledShare-after.RebilledShare) > 1e-9 {
		t.Errorf("re-billed share moved: %v -> %v. It is a ratio of two conserved "+
			"sums and must not survive the fold as a different number",
			before.RebilledShare, after.RebilledShare)
	}
	if after.Tasks != 2 {
		t.Errorf("tasks = %d, want 2 sessions", after.Tasks)
	}
}

// Fan-out analysis genuinely needs the lanes, so --per-lane keeps them — and
// keeps them under a name that cannot be mistaken for a session.
//
// The rows arrive under `lanes`, not `tasks`, and each one's id field is
// `ofSession`: several lanes share one session id by construction, so a bare
// `session` on a lane row would put the original defect straight back into the
// output under a flag nobody reads twice.
func TestCostPerLaneKeepsTheLanesAndNamesThemAsLanes(t *testing.T) {
	home := t.TempDir()
	isolateHome(t, home)
	proj := filepath.Join(home, ".claude", "projects", "proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := filepath.Abs(filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	const lanes = 3
	for i := 0; i < lanes; i++ {
		if err := os.Symlink(src, filepath.Join(proj, fmt.Sprintf("lane-%d.jsonl", i))); err != nil {
			t.Fatal(err)
		}
	}

	var out, errOut bytes.Buffer
	if err := runCost([]string{"--per-task", "--per-lane", "--json", proj}, &out, &errOut); err != nil {
		t.Fatalf("cost --per-task --per-lane --json: %v (stderr: %s)", err, errOut.String())
	}
	var got struct {
		Tasks []json.RawMessage `json:"tasks"`
		Lanes []struct {
			Lane      string  `json:"lane"`
			OfSession string  `json:"ofSession"`
			Session   string  `json:"session"`
			CostUSD   float64 `json:"costUsd"`
		} `json:"lanes"`
		Summary struct {
			Tasks int    `json:"tasks"`
			Unit  string `json:"unit"`
			Lanes int    `json:"lanes"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("cost JSON does not parse: %v\n%s", err, out.String())
	}
	if len(got.Lanes) != lanes {
		t.Fatalf("%d lane rows, want %d. Lane detail must stay reachable, not be "+
			"deleted:\n%s", len(got.Lanes), lanes, out.String())
	}
	if len(got.Tasks) != 0 {
		t.Errorf("lane rows appeared under `tasks`; they must not share a key with " +
			"session rows")
	}
	for _, row := range got.Lanes {
		if row.Session != "" {
			t.Errorf("a lane row carries a bare `session` field (%q). Several lanes "+
				"share one session id; the field is `ofSession`", row.Session)
		}
		if row.OfSession == "" {
			t.Errorf("a lane row does not say which session it belongs to, which is " +
				"the thing fan-out analysis is for")
		}
	}
	// The unit governs the whole report, not only the rows.
	if got.Summary.Unit != unitLane {
		t.Errorf("summary.unit = %q, want %q", got.Summary.Unit, unitLane)
	}
	if got.Summary.Tasks != lanes {
		t.Errorf("summary.tasks = %d, want %d: the count must count whatever the "+
			"rows are", got.Summary.Tasks, lanes)
	}
}

// A session that changed tool set mid-run says so in the report.
//
// This is the reader `MixedEpochs` was added for and shipped without. A review
// panel found the field had zero production consumers: its own comment said "a
// reader adding two epochs together should be told so", and no reader was told
// anything. A label nobody is shown cannot be wrong, which is not the same as
// being right.
func TestCostReportsASessionThatChangedToolSet(t *testing.T) {
	mixed := renderCost(summarise([]costUnit{
		{CostUSD: 10, MixedEpochs: true},
		{CostUSD: 5},
	}), 0, 0, io.Discard, "")
	if !strings.Contains(mixed, "changed tool set mid-run") {
		t.Fatalf("a session spanning two tool sets was priced and not disclosed:\n%s", mixed)
	}
	if !strings.Contains(mixed, "1 session(s)") {
		t.Fatalf("the count is wrong or missing:\n%s", mixed)
	}

	// And the ordinary case says nothing, because a caveat printed on every
	// run is a caveat nobody reads.
	plain := renderCost(summarise([]costUnit{{CostUSD: 10}, {CostUSD: 5}}), 0, 0, io.Discard, "")
	if strings.Contains(plain, "changed tool set") {
		t.Fatalf("a run with no mixed-epoch session printed the notice anyway:\n%s", plain)
	}
}
