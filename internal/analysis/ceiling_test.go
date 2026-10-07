package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

func sessionWithCompactions(model string, preTokens ...int) *transcript.Session {
	t0 := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{Requests: []*transcript.Request{
		{Timestamp: t0, Model: model, Usage: transcript.Usage{Input: 100}},
	}}
	var comps []transcript.Compaction
	for i, p := range preTokens {
		comps = append(comps, transcript.Compaction{
			Trigger: "auto", PreTokens: p, PostTokens: p / 30,
			At: t0.Add(time.Duration(i+1) * time.Minute),
		})
	}
	return &transcript.Session{Lanes: []*transcript.Lane{lane}, Compactions: comps}
}

// Ten or more tightly clustered recordings for a model are enough to report
// an observed ceiling: the panel's gate, 2026-10-05.
//
// PASS: ten recordings close together report Available with the max.
// FAIL: a lower threshold reports early, or the max is wrong.
func TestContextCeilingAvailableAtTenTightlyClusteredRecordings(t *testing.T) {
	pre := []int{963_705, 964_000, 965_000, 966_870, 970_000, 980_000, 990_000, 995_000, 996_000, 998_021}
	sessions := []*transcript.Session{sessionWithCompactions("claude-opus-5", pre...)}
	table := BuildContextCeilings(sessions)
	c, ok := table["claude-opus-5"]
	if !ok {
		t.Fatalf("want an entry for claude-opus-5, got none: %+v", table)
	}
	if c.Status != ContextCeilingAvailable {
		t.Fatalf("want Available at n=10 tightly clustered, got status %v", c.Status)
	}
	if c.Max != 998_021 {
		t.Errorf("want the largest recorded pre-compaction size, 998021, got %d", c.Max)
	}
	if c.N != 10 {
		t.Errorf("want N=10, got %d", c.N)
	}
}

// Nine recordings is not enough, even tightly clustered.
//
// PASS: a model with 9 compactions reports TooFewRecords, not a ceiling.
// FAIL: a ceiling is reported from fewer than the panel's gate of ten.
func TestContextCeilingTooFewRecordsBelowTen(t *testing.T) {
	pre := []int{963_705, 964_000, 965_000, 966_870, 970_000, 980_000, 990_000, 995_000, 996_000}
	sessions := []*transcript.Session{sessionWithCompactions("claude-opus-5", pre...)}
	table := BuildContextCeilings(sessions)
	c := table["claude-opus-5"]
	if c.Status != ContextCeilingTooFewRecords {
		t.Errorf("want TooFewRecords at n=9, got %v", c.Status)
	}
	if c.N != 9 {
		t.Errorf("want N=9 carried even when withheld, got %d", c.N)
	}
}

// The same model id can show two different ceilings on this machine (the
// capability map's fable example: 165k and 961k under one id). Reporting the
// max of the two as THE ceiling would be wrong in a specific, checkable way:
// it would silently discard that the id does not name one tier. The panel's
// gate withholds a ceiling in that case rather than guess which cluster
// applies to a given reading.
//
// PASS: ten recordings split across two widely separated clusters report
// AmbiguousTier, not a single ceiling.
// FAIL: the max of the mixed clusters is reported as if it were one tier's
// ceiling.
func TestContextCeilingAmbiguousTierAcrossTwoWidelySeparatedClusters(t *testing.T) {
	pre := []int{165_514, 165_600, 165_700, 165_800, 165_900, 961_009, 962_000, 963_000, 964_000, 965_989}
	sessions := []*transcript.Session{sessionWithCompactions("claude-fable-5-1", pre...)}
	table := BuildContextCeilings(sessions)
	c := table["claude-fable-5-1"]
	if c.Status != ContextCeilingAmbiguousTier {
		t.Errorf("want AmbiguousTier across two 5x-separated clusters, got %v, max %d", c.Status, c.Max)
	}
}

// A model with no recorded compactions is simply absent from the table: the
// absence itself is the "no compaction recorded" state, not a zero-value
// Available entry.
//
// PASS: a model that never compacted is not a key in the table at all.
// FAIL: an entry exists with Status's zero value read as meaningful.
func TestContextCeilingAbsentWhenNeverRecorded(t *testing.T) {
	sessions := []*transcript.Session{sessionWithCompactions("claude-opus-5")}
	table := BuildContextCeilings(sessions)
	if _, ok := table["claude-opus-5"]; ok {
		t.Errorf("a model with zero compactions must not appear in the table, got %+v", table["claude-opus-5"])
	}
}

// Compactions across every session in the corpus count toward the same
// model's total, not just one session's.
//
// PASS: 5 compactions in one session plus 5 in another reach the gate of 10.
// FAIL: each session is judged alone and neither reaches 10.
func TestContextCeilingCountsAcrossTheWholeCorpusNotOneSession(t *testing.T) {
	a := sessionWithCompactions("claude-opus-5", 963_705, 964_000, 965_000, 966_870, 970_000)
	b := sessionWithCompactions("claude-opus-5", 980_000, 990_000, 995_000, 996_000, 998_021)
	table := BuildContextCeilings([]*transcript.Session{a, b})
	c := table["claude-opus-5"]
	if c.Status != ContextCeilingAvailable || c.N != 10 {
		t.Errorf("want Available at N=10 pooled across both sessions, got status %v N %d", c.Status, c.N)
	}
}

// A compaction with no recorded instant cannot be attributed to a model: the
// nearest-request lookup needs a time to search before, and a boundary the
// client did not date must not be paired with whichever request happens to
// be first, the same rule firstPromptAfter already applies forward.
//
// PASS: an undated compaction contributes to no model's count.
// FAIL: an undated compaction is attributed to the session's only model
// anyway.
func TestContextCeilingSkipsAnUndatedCompaction(t *testing.T) {
	t0 := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{Requests: []*transcript.Request{
		{Timestamp: t0, Model: "claude-opus-5", Usage: transcript.Usage{Input: 100}},
	}}
	session := &transcript.Session{Lanes: []*transcript.Lane{lane}, Compactions: []transcript.Compaction{
		{Trigger: "auto", PreTokens: 998_021, PostTokens: 30_000}, // At is the zero value
	}}
	table := BuildContextCeilings([]*transcript.Session{session})
	if _, ok := table["claude-opus-5"]; ok {
		t.Errorf("an undated compaction must not be attributed to any model, got %+v", table["claude-opus-5"])
	}
}

// The rendered line never claims a percentage of "the window" (not in the
// Claude Code record, per the panel), never uses forward-looking language,
// and is always marked estimated.
//
// PASS: an Available ceiling renders a line naming the model, the observed
// max, the count, "estimated", and this event's own size, with none of the
// banned words.
// FAIL: any banned word appears, or the line claims a window percentage.
func TestContextCeilingLineForAnAvailableCeiling(t *testing.T) {
	table := map[string]ContextCeiling{
		"claude-opus-5": {Status: ContextCeilingAvailable, Max: 998_021, N: 86},
	}
	line := ContextCeilingLine("claude-opus-5", 969_218, table)
	for _, bad := range []string{"approach", "will ", "expect", "forecast", "window", "predict"} {
		if strings.Contains(strings.ToLower(line), bad) {
			t.Errorf("line contains the banned word %q: %s", bad, line)
		}
	}
	if !strings.Contains(line, "998,021") || !strings.Contains(line, "969,218") {
		t.Errorf("line must name both the observed ceiling and this event's own size: %s", line)
	}
	if !strings.Contains(strings.ToLower(line), "estimated") {
		t.Errorf("line must say estimated: %s", line)
	}
}

// A model with too few records or an ambiguous tier says so by name, rather
// than silently printing nothing (which reads as "nothing to report" when
// the true state is "withheld, and here is why").
//
// PASS: TooFewRecords and AmbiguousTier each produce a line naming the
// reason, not an empty string.
// FAIL: either status renders nothing, or claims a ceiling it does not have.
func TestContextCeilingLineNamesWhyAStatusIsWithheld(t *testing.T) {
	tooFew := map[string]ContextCeiling{"claude-sonnet-5": {Status: ContextCeilingTooFewRecords, N: 3}}
	if line := ContextCeilingLine("claude-sonnet-5", 500_000, tooFew); line == "" ||
		!strings.Contains(line, "3") {
		t.Errorf("a too-few-records status must name the count withheld, got %q", line)
	}
	ambiguous := map[string]ContextCeiling{"claude-fable-5-1": {Status: ContextCeilingAmbiguousTier, N: 10}}
	if line := ContextCeilingLine("claude-fable-5-1", 500_000, ambiguous); line == "" ||
		strings.Contains(line, "998,021") {
		t.Errorf("an ambiguous tier must not print a ceiling figure, got %q", line)
	}
}

// A model absent from the table entirely (no compactions recorded anywhere
// in the corpus) renders nothing: there is no observed value to compare this
// event against, and no event can be reported for a model with zero
// compactions in the first place, since this function is only ever called
// for a compaction event that already occurred.
//
// PASS: an unknown model key renders an empty string.
// FAIL: a lookup miss panics or fabricates a figure.
func TestContextCeilingLineForAModelNotInTheTable(t *testing.T) {
	if line := ContextCeilingLine("claude-haiku-4-5", 100_000, map[string]ContextCeiling{}); line != "" {
		t.Errorf("want nothing for a model with no table entry, got %q", line)
	}
}

// ContextCeilingDetail returns one slot per element of session.Compactions,
// in order, so a caller can pair it with CompactionDetail's own per-event
// lines by index without the two ever disagreeing about which event is
// which.
//
// PASS: three compactions (one resolvable, one undated, one unsized) produce
// three slots in the same order, with the unresolvable two empty.
// FAIL: slots are dropped, reordered, or an unresolvable one fabricates a
// line.
func TestContextCeilingDetailAlignsWithCompactionOrder(t *testing.T) {
	t0 := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	lane := &transcript.Lane{Requests: []*transcript.Request{
		{Timestamp: t0, Model: "claude-opus-5", Usage: transcript.Usage{Input: 100}},
	}}
	session := &transcript.Session{Lanes: []*transcript.Lane{lane}, Compactions: []transcript.Compaction{
		{Trigger: "auto", PreTokens: 969_218, PostTokens: 26_970, At: t0.Add(time.Minute)},
		{Trigger: "auto", PreTokens: 500_000, PostTokens: 20_000},     // undated
		{Trigger: "auto", PostTokens: 0, At: t0.Add(2 * time.Minute)}, // unsized
	}}
	ceilings := map[string]ContextCeiling{"claude-opus-5": {Status: ContextCeilingAvailable, Max: 998_021, N: 50}}
	lines := ContextCeilingDetail(session, ceilings)
	if len(lines) != 3 {
		t.Fatalf("want 3 slots for 3 compactions, got %d", len(lines))
	}
	if lines[0] == "" || !strings.Contains(lines[0], "969,218") {
		t.Errorf("slot 0 must resolve, got %q", lines[0])
	}
	if lines[1] != "" {
		t.Errorf("slot 1 is undated, must be empty, got %q", lines[1])
	}
	if lines[2] != "" {
		t.Errorf("slot 2 is unsized, must be empty, got %q", lines[2])
	}
}
