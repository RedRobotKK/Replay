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

// RPL-C031, the mixed-session priceability case. DISCOVERY ONLY.
//
// Treated as a separate claim from EC-00. EC-00 was a WHOLLY unpriceable
// session being dropped before its tokens could be counted. This is a session
// containing BOTH priceable and unpriceable records, and the question is
// whether PARTIAL survives aggregation or collapses into one of its
// neighbours.
//
// ORACLE INDEPENDENCE.
//
// Every expected value is a declared fixture fact. The fixture decides how
// many records are priceable and how many are not, and the oracle reads those
// declarations, never Replay's output.
//
// ALLOWED DEPENDENCIES, documented in full because that is the review:
//
//  1. cachemodel.PriceFor, in TestPT0_FixtureAssumptions ONLY. It asserts the
//     two model names still sit on the intended sides of the price table and
//     computes no expectation. Without it a table change makes this fixture
//     measure something else in silence, which is the MX8 defect.
//  2. runCost, as the SUBJECT. Its output is the observation, never the
//     expectation.
//
// Nothing else. No cost summary is used as an expected value, no aggregation
// path under test supplies an answer, and the price table is never asked what
// the disclosure OUGHT to be.

// discState is the disclosure the oracle requires. A TYPED enum, so PARTIAL
// cannot be expressed as UNPRICED, and UNAVAILABLE cannot be expressed as a
// numeric zero, even by accident. This is the lesson of usdKind.
type discState int

const (
	discPriced       discState = iota // every record priceable
	discUnpriced                      // no record priceable
	discPartial                       // SOME records priceable. Its own state
	discObservedZero                  // priceable and measured at zero
	discNotMeasured                   // no usable evidence at all
)

func (d discState) String() string {
	switch d {
	case discPriced:
		return "PRICED"
	case discUnpriced:
		return "UNPRICED"
	case discPartial:
		return "PARTIAL"
	case discObservedZero:
		return "OBSERVED_ZERO"
	}
	return "NOT_MEASURED"
}

const (
	ptPriced   = "claude-opus-5"
	ptUnpriced = "totally-unknown-model-x9"
)

// ptRecord is one declared record. Token quantities are held IDENTICAL across
// every arm; only the model name varies, so any difference observed is
// attributable to priceability and to nothing else.
type ptRecord struct{ model string }

func ptCorpus(t *testing.T, recs []ptRecord) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	var lines []byte
	for i, r := range recs {
		// Identical usage on every record of every arm.
		u := transcript.Usage{Input: 500, CacheCreation: 20_000, CacheRead: 0, Output: 300}
		rec := ledger.Record{
			Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(i) * time.Minute),
			SessionID: "mixed", RequestID: "pt-" + string(rune('A'+i)),
			Path: "/v1/messages", Status: 200, LatencyMS: 900,
			RequestSummary: ledger.RequestSummary{Model: r.model,
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
	if err := os.WriteFile(filepath.Join(dir, "mixed.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ptSeen is the observation. Not an expectation.
type ptSeen struct {
	pricedReqs   int
	unpricedReqs int
	unpriced     int
	unreadable   int
	tasks        int
	human        string
	jsonKeys     []string
}

func ptRun(t *testing.T, dir string) ptSeen {
	t.Helper()
	var j, e bytes.Buffer
	if err := runCost([]string{"--per-task", "--json", dir}, &j, &e); err != nil {
		t.Fatalf("cost: %v (%s)", err, e.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("cost JSON does not parse: %v", err)
	}
	num := func(k string) int {
		if v, ok := doc[k].(float64); ok {
			return int(v)
		}
		return -1
	}
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	var h bytes.Buffer
	_ = runCost([]string{dir}, &h, &e)
	tasks := -1
	if sm, ok := doc["summary"].(map[string]any); ok {
		if v, ok := sm["tasks"].(float64); ok {
			tasks = int(v)
		}
	}
	pr, upr := 0, 0
	if sm, ok := doc["summary"].(map[string]any); ok {
		if v, ok := sm["pricedRequests"].(float64); ok {
			pr = int(v)
		}
		if v, ok := sm["unpricedRequests"].(float64); ok {
			upr = int(v)
		}
	}
	return ptSeen{pr, upr, num("unpriced"), num("unreadable"), tasks, h.String(), keys}
}

// PT0. The only place the price table is consulted. Computes no expectation.
func TestPT0_FixtureAssumptions(t *testing.T) {
	if _, ok := cachemodel.PriceFor(ptPriced); !ok {
		t.Fatalf("%s is not priced; the priceable half of every arm is broken", ptPriced)
	}
	if p, ok := cachemodel.PriceFor(ptUnpriced); ok {
		t.Fatalf("%s IS priced (%v); the unpriceable half is broken", ptUnpriced, p)
	}
}

// PT1. The three arms. Declared facts in, observation out, no inference.
func TestPT1_DoesPartialSurviveSessionAggregation(t *testing.T) {
	arms := []struct {
		name string
		recs []ptRecord
		// DECLARED FACTS.
		pricedRecords   int
		unpricedRecords int
		oracle          discState
	}{
		{"positive control, all priced",
			[]ptRecord{{ptPriced}, {ptPriced}, {ptPriced}, {ptPriced}},
			4, 0, discPriced},
		{"negative control, none priced",
			[]ptRecord{{ptUnpriced}, {ptUnpriced}, {ptUnpriced}, {ptUnpriced}},
			0, 4, discUnpriced},
		{"TARGET, mixed",
			[]ptRecord{{ptPriced}, {ptPriced}, {ptUnpriced}, {ptUnpriced}},
			2, 2, discPartial},
	}

	var b strings.Builder
	b.WriteString("\nRPL-C031 MIXED-SESSION ARMS\n")
	b.WriteString("arm                            declared     oracle         unpriced tasks  human discloses\n")

	seen := map[string]ptSeen{}
	for _, a := range arms {
		got := ptRun(t, ptCorpus(t, a.recs))
		seen[a.oracle.String()] = got
		// Widened 2026-10-01 for the partial-coverage sentence the repair
		// added. The first version looked only for the whole-transcript
		// phrases and was blind to it, which is the C004 false-detector
		// failure and was caught by reading the output rather than the
		// detector. Discrimination is re-proven below.
		disc := strings.Contains(got.human, "not priced") ||
			strings.Contains(got.human, "could be priced") ||
			strings.Contains(got.human, "excluded") ||
			strings.Contains(got.human, "covers")
		b.WriteString(padRight(a.name, 31) +
			padRight(tierItoa(a.pricedRecords)+"P/"+tierItoa(a.unpricedRecords)+"U", 13) +
			padRight(a.oracle.String(), 15) +
			padRight(tierItoa(got.unpriced), 9) +
			padRight(tierItoa(got.tasks), 7) + yn(disc) + "\n")
	}
	t.Log(b.String())

	// The oracle's only question: are the three states distinguishable at any
	// user-visible surface? Compared on OBSERVATIONS, decided by DECLARATIONS.
	p, u, part := seen["PRICED"], seen["UNPRICED"], seen["PARTIAL"]

	if p.unpriced == u.unpriced {
		t.Fatalf("the two CONTROLS are indistinguishable (unpriced %d vs %d). The "+
			"fixture cannot separate even the easy cases, so nothing below means "+
			"anything.", p.unpriced, u.unpriced)
	}

	// REGRESSION, after the repair. The pin this replaced asserted the
	// collapse; the oracle itself is unchanged, only the direction.
	if part.unpriced != p.unpriced {
		t.Errorf("unexpected: PARTIAL and PRICED now differ on the session-level "+
			"unpriced counter (%d vs %d). That counter is about whole transcripts "+
			"and the repair deliberately did not change it.", part.unpriced, p.unpriced)
	}
	if part.pricedReqs == 0 || part.unpricedReqs == 0 {
		t.Errorf("MIXED reports pricedRequests=%d unpricedRequests=%d; the oracle "+
			"declared 2 and 2, so the coverage pair did not survive",
			part.pricedReqs, part.unpricedReqs)
	}
	if p.unpricedReqs != 0 {
		t.Errorf("ALL PRICEABLE reports unpricedRequests=%d, want 0", p.unpricedReqs)
	}
	if !strings.Contains(part.human, "covers") {
		t.Error("MIXED discloses no coverage. A partial total presented as a " +
			"complete one is the defect this repair closes.")
	}
	if strings.Contains(p.human, "covers") {
		t.Error("ALL PRICEABLE discloses partial coverage; the disclosure fires " +
			"when it must not, so its presence on MIXED proves nothing")
	}
	// The three states must now be mutually distinguishable.
	sig := func(x ptSeen) string {
		return tierItoa(x.unpriced) + "/" + tierItoa(x.pricedReqs) + "/" + tierItoa(x.unpricedReqs)
	}
	if sig(p) == sig(part) || sig(part) == sig(u) || sig(p) == sig(u) {
		t.Errorf("two states share a signature: PRICED=%s MIXED=%s UNPRICED=%s",
			sig(p), sig(part), sig(u))
	}
	t.Logf("THREE STATES DISTINGUISHABLE. signatures unpriced/priced/unpriceable: "+
		"PRICED=%s MIXED=%s UNPRICED=%s", sig(p), sig(part), sig(u))

	// Is there ANY json key that could carry a partial state?
	t.Logf("json keys available to a machine consumer: %v", p.jsonKeys)
}

// PT2. LAYER 4, measured rather than assumed.
//
// Added after the first freeze (3177f794) and the file re-hashed. The first
// pass reported Layer 4 as UNMEASURED on the grounds that no partial field
// exists, which is true of the FIELD question and was lazy about the other
// two: whether a mixed session survives the index at all, and whether the
// schema key would invalidate an older index if a field were added.
//
// Neither needs a production change to answer.
func TestPT2_Layer4ColdAndWarm(t *testing.T) {
	dir := ptCorpus(t, []ptRecord{{ptPriced}, {ptPriced}, {ptUnpriced}, {ptUnpriced}})

	cold := ptRun(t, dir)
	warm := ptRun(t, dir) // same home: reads the index the cold run wrote

	// Q1. Does the mixed session's reported state survive the index?
	if cold.unpriced != warm.unpriced || cold.tasks != warm.tasks {
		t.Errorf("the mixed session reports differently cold and warm: "+
			"unpriced %d/%d, tasks %d/%d. A partial-coverage repair would have to "+
			"carry its state through the index, and the state it carries today is "+
			"already unstable.", cold.unpriced, warm.unpriced, cold.tasks, warm.tasks)
	}
	t.Logf("Q1 cold==warm for the mixed arm: unpriced=%d tasks=%d on both passes. "+
		"The COLLAPSED state is stable across the index; that is not evidence a "+
		"PARTIAL state would be, because no such state exists to carry.",
		cold.unpriced, cold.tasks)

	// Q2. Would adding a field to costUnit invalidate an older index? The key
	// is derived from the struct's json tags, so the question is answerable
	// without adding one: check that the mechanism is tag-derived at all.
	before := unitSchema()
	if before == "" {
		t.Fatal("unitSchema() is empty; the index key does not depend on the unit's " +
			"shape and an older index would NOT invalidate")
	}
	if !strings.Contains(before, "unpriced") {
		t.Error("the existing session-level unpriced tag is absent from the schema " +
			"key; the mechanism is not reflecting what it claims to")
	}
	t.Logf("Q2 the index key IS tag-derived (%d tags). A new field would change it "+
		"and discard older indexes. Mechanism confirmed; no field added.",
		strings.Count(before, ",")+1)

	// Q3. What field would carry the distinction? NOT ANSWERED, and recorded
	// as such: that is a design decision, not a measurement, and proposing one
	// here would be the repair this pass is forbidden from making.
	t.Log("Q3 which field would carry PARTIAL is a design question, deliberately " +
		"left open. SessionSpend's Duplicated and Unidentified are the per-record " +
		"precedent; EC-00's session-level Unpriced bool is NOT, and is why it did " +
		"not reach this case.")
}
