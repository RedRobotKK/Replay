package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RPL-C036. The record-local deficit-monetisation oracle.
//
// The quantity was always record-correct: Deficit is Expected minus Actual
// (internal/analysis/diff.go:48) and consults no price table. The DOLLARS were
// not: cost.go monetised the whole session's deficit once, at
// PriceForAt(Requests[0].Model, Requests[0].Timestamp).
//
// This file computes the expected figure independently of the production
// arithmetic and holds the product to it. It calls PriceForAt for the rate,
// which is the subject's own price table and the only shared surface; the
// break detection, the deficit quantity and the monetisation are all
// recomputed here longhand.

const (
	c036Expensive = "claude-opus-5"    // $5.00/MTok input
	c036Cheap     = "claude-3-5-haiku" // $0.80/MTok input
	c036Unpriced  = "totally-unknown-model-x9"
)

// c036Rec is one ledger record, specified by the only three things that
// matter to this experiment.
type c036Rec struct {
	model  string
	create int // CacheCreation
	read   int // CacheRead
	// at overrides the default one-minute spacing. Needed only by the dated
	// case, where two records must fall on different sides of a price change.
	at time.Time
}

// c036Write lays the records down in the order given, one per minute.
func c036Write(t *testing.T, recs []c036Rec) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []byte
	for i, r := range recs {
		u := transcript.Usage{Input: 500, CacheCreation: r.create, CacheRead: r.read, Output: 300}
		rec := ledger.Record{
			Schema: ledger.SchemaVersion, Timestamp: c036TS(recs, i),
			SessionID: "c036", RequestID: fmt.Sprintf("c036-%02d", i),
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
	if err := os.WriteFile(filepath.Join(dir, "c036.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func c036At(i int) time.Time {
	return time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute)
}

// c036TS is the one clock the writer and the oracle share, so a timestamp the
// oracle prices at is the timestamp the record carries.
func c036TS(recs []c036Rec, i int) time.Time {
	if !recs[i].at.IsZero() {
		return recs[i].at
	}
	return c036At(i)
}

// c036Expect is the ORACLE. Per record, from that record's own evidence.
type c036Expect struct {
	usd            float64
	tokens         int
	pricedBreaks   int
	unpricedBreaks int
	perRecord      []string
}

// c036Oracle recomputes the expected figure longhand.
//
// The break rule is reimplemented here rather than called: a request reads
// what the one before it wrote, so expected = prev.create + prev.read, and
// anything it did not read is a deficit on THIS record. The rate is then
// looked up for THIS record's own model and timestamp.
func c036Oracle(t *testing.T, recs []c036Rec) c036Expect {
	t.Helper()
	var e c036Expect
	for i := 1; i < len(recs); i++ {
		expected := recs[i-1].create + recs[i-1].read
		actual := recs[i].read
		if actual >= expected {
			e.perRecord = append(e.perRecord, fmt.Sprintf(
				"#%d %-26s no deficit            $0.000000", i, recs[i].model))
			continue
		}
		deficit := expected - actual
		e.tokens += deficit
		p, ok := cachemodel.PriceForAt(recs[i].model, c036TS(recs, i))
		if !ok {
			e.unpricedBreaks++
			e.perRecord = append(e.perRecord, fmt.Sprintf(
				"#%d %-26s %6d tok  NO PRICE  $0.000000 (withheld, not zero)",
				i, recs[i].model, deficit))
			continue
		}
		e.pricedBreaks++
		c := float64(deficit) / 1_000_000 * p.InputPerMTok
		e.usd += c
		e.perRecord = append(e.perRecord, fmt.Sprintf(
			"#%d %-26s %6d tok  $%5.2f/MTok  $%.6f", i, recs[i].model, deficit, p.InputPerMTok, c))
	}
	return e
}

type c036Got struct {
	rebilledUSD float64
	tokens      int
}

func c036Run(t *testing.T, dir string) c036Got {
	t.Helper()
	var j, errb bytes.Buffer
	if err := runCost([]string{"--json", dir}, &j, &errb); err != nil {
		t.Fatalf("cost: %v (%s)", err, errb.String())
	}
	var doc struct {
		Summary struct {
			RebilledUSD    float64 `json:"rebilledUsd"`
			RebilledTokens int     `json:"rebilledTokens"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	return c036Got{doc.Summary.RebilledUSD, doc.Summary.RebilledTokens}
}

// c036Check holds the product to the oracle on one corpus.
func c036Check(t *testing.T, label string, recs []c036Rec) {
	t.Helper()
	want := c036Oracle(t, recs)
	got := c036Run(t, c036Write(t, recs))
	for _, line := range want.perRecord {
		t.Log("  oracle ", line)
	}
	if got.tokens != want.tokens {
		t.Errorf("%s: deficit TOKENS %d, oracle %d. The quantity is not in dispute "+
			"in C036, so a mismatch here means the fixture is not producing the "+
			"breaks the oracle thinks it is.", label, got.tokens, want.tokens)
	}
	if math.Abs(got.rebilledUSD-want.usd) > 1e-9 {
		t.Errorf("%s: re-billed $%.6f, oracle $%.6f. Each deficit must be priced at "+
			"its OWN record's rate.", label, got.rebilledUSD, want.usd)
	}
	t.Logf("%s: $%.6f over %d tokens (%d priced breaks, %d unpriceable)",
		label, got.rebilledUSD, got.tokens, want.pricedBreaks, want.unpricedBreaks)
}

// c036Lead is the fixed priming record. It can never break, because a break
// needs a predecessor, so it is held CONSTANT across every permutation below.
// Permuting it would change which records break and the total would move for
// a legitimate reason, which is a different experiment.
var c036Lead = c036Rec{model: c036Cheap, create: 20_000}

func c036Breaking(model string) c036Rec { return c036Rec{model: model, create: 20_000} }

// ---------------------------------------------------------------------------
// PHASE 3. Cases A-E.
// ---------------------------------------------------------------------------

// A. A priceable record with a deficit pays its own rate.
func TestC036_A_PricedRecordWithDeficit(t *testing.T) {
	c036Check(t, "A priced+deficit", []c036Rec{c036Lead, c036Breaking(c036Expensive)})
}

// B. An unpriceable record with a deficit keeps its tokens and pays nothing.
func TestC036_B_UnpriceableRecordWithDeficit(t *testing.T) {
	if _, ok := cachemodel.PriceFor(c036Unpriced); ok {
		t.Fatalf("%s IS priced; the fixture is broken", c036Unpriced)
	}
	recs := []c036Rec{c036Lead, c036Breaking(c036Unpriced)}
	want := c036Oracle(t, recs)
	if want.tokens == 0 {
		t.Fatalf("the oracle expects no deficit; this case observes nothing")
	}
	if want.usd != 0 {
		t.Fatalf("the oracle expects dollars from an unpriceable record; it is wrong")
	}
	c036Check(t, "B unpriceable+deficit", recs)
}

// C. A record with no deficit contributes no deficit dollars however
// expensive it is. Its CacheRead matches what the record before it wrote.
func TestC036_C_PricedRecordWithoutDeficit(t *testing.T) {
	recs := []c036Rec{c036Lead, {model: c036Expensive, read: 20_000}}
	want := c036Oracle(t, recs)
	if want.tokens != 0 || want.usd != 0 {
		t.Fatalf("case C must have no deficit; the oracle computed %d tokens and "+
			"$%.6f, so the fixture does not produce a clean read", want.tokens, want.usd)
	}
	c036Check(t, "C priced, no deficit", recs)
}

// D. A mixed session: expensive, cheap, unpriceable and clean, all at once.
func TestC036_D_MixedSession(t *testing.T) {
	c036Check(t, "D mixed", []c036Rec{
		c036Lead,
		c036Breaking(c036Expensive),
		c036Breaking(c036Cheap),
		c036Breaking(c036Unpriced),
		{model: c036Cheap, read: 20_000}, // clean read of what #3 wrote
	})
}

// E. The same records reversed. The lead is held fixed, so every record that
// breaks in one arrangement breaks in the other.
func TestC036_E_ReversedOrder(t *testing.T) {
	tail := []c036Rec{c036Breaking(c036Expensive), c036Breaking(c036Cheap), c036Breaking(c036Unpriced)}
	fwd := append([]c036Rec{c036Lead}, tail...)
	rev := []c036Rec{c036Lead, tail[2], tail[1], tail[0]}

	wf, wr := c036Oracle(t, fwd), c036Oracle(t, rev)
	if math.Abs(wf.usd-wr.usd) > 1e-9 {
		t.Fatalf("the oracle itself is order-dependent ($%.6f vs $%.6f); it cannot "+
			"be used to judge the product", wf.usd, wr.usd)
	}
	c036Check(t, "E forward", fwd)
	c036Check(t, "E reversed", rev)
}

// F1. ORDER INVARIANCE over every permutation of the same three breaking
// records, lead held fixed.
func TestC036_F1_OrderInvariance(t *testing.T) {
	tail := []c036Rec{c036Breaking(c036Expensive), c036Breaking(c036Cheap), c036Breaking(c036Unpriced)}
	perms := [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}

	first := -1.0
	for _, p := range perms {
		recs := []c036Rec{c036Lead, tail[p[0]], tail[p[1]], tail[p[2]]}
		want := c036Oracle(t, recs)
		if first < 0 {
			first = want.usd
		} else if math.Abs(want.usd-first) > 1e-9 {
			t.Fatalf("the ORACLE is not permutation-invariant ($%.6f vs $%.6f) on a "+
				"fixture whose lead is fixed; the experiment is unsound",
				want.usd, first)
		}
		got := c036Run(t, c036Write(t, recs))
		if math.Abs(got.rebilledUSD-want.usd) > 1e-9 {
			t.Errorf("F1 violated for order %v: product $%.6f, oracle $%.6f. The same "+
				"records in a different arrangement must not change the figure.",
				[]string{tail[p[0]].model, tail[p[1]].model, tail[p[2]].model},
				got.rebilledUSD, want.usd)
		}
		if got.tokens != want.tokens {
			t.Errorf("F1 tokens for order %v: product %d, oracle %d",
				[]string{tail[p[0]].model, tail[p[1]].model, tail[p[2]].model},
				got.tokens, want.tokens)
		}
	}
	t.Logf("F1: six permutations, oracle $%.6f in every one", first)
}

// F3/F5. The decisive negative case, stated as a difference rather than a
// level: swapping the UNPRICEABLE record for a priceable one must change the
// figure, and swapping it for a different unpriceable one must not.
//
// If an unpriceable record borrowed a rate, the first arm would move by less
// than the full difference and the second would move at all.
func TestC036_F3_UnpriceableBorrowsNoRate(t *testing.T) {
	base := []c036Rec{c036Lead, c036Breaking(c036Cheap), c036Breaking(c036Unpriced)}
	swapPriced := []c036Rec{c036Lead, c036Breaking(c036Cheap), c036Breaking(c036Expensive)}
	swapOther := []c036Rec{c036Lead, c036Breaking(c036Cheap), c036Breaking("another-unknown-model-q7")}

	if _, ok := cachemodel.PriceFor("another-unknown-model-q7"); ok {
		t.Fatalf("the second decoy model IS priced; the fixture is broken")
	}

	b := c036Run(t, c036Write(t, base))
	sp := c036Run(t, c036Write(t, swapPriced))
	so := c036Run(t, c036Write(t, swapOther))

	if b.tokens != sp.tokens || b.tokens != so.tokens {
		t.Fatalf("the three arms carry different deficits (%d, %d, %d); they do not "+
			"isolate the rate", b.tokens, sp.tokens, so.tokens)
	}
	pe, ok := cachemodel.PriceFor(c036Expensive)
	if !ok {
		t.Fatalf("%s not priced", c036Expensive)
	}
	// Arm 1: the unpriceable record's 20,000 tokens become priceable at the
	// expensive rate, so the figure must rise by exactly that and no less.
	wantRise := 20_000.0 / 1_000_000 * pe.InputPerMTok
	if math.Abs((sp.rebilledUSD-b.rebilledUSD)-wantRise) > 1e-9 {
		t.Errorf("F3 violated: making the unpriceable record priceable moved the "+
			"figure by $%.6f, and its own rate accounts for $%.6f. The difference is "+
			"a rate it was already borrowing.",
			sp.rebilledUSD-b.rebilledUSD, wantRise)
	}
	// Arm 2: a different unpriceable model must change nothing at all.
	if math.Abs(so.rebilledUSD-b.rebilledUSD) > 1e-9 {
		t.Errorf("F3 violated: swapping one unpriceable model for another moved the "+
			"figure from $%.6f to $%.6f, so something about the record is reaching "+
			"the price.", b.rebilledUSD, so.rebilledUSD)
	}
	t.Logf("F3: unpriceable $%.6f, priceable $%.6f (rise $%.6f = its own rate), "+
		"other unpriceable $%.6f", b.rebilledUSD, sp.rebilledUSD, wantRise, so.rebilledUSD)
}

// F5. A record with no deficit contributes nothing even at the highest rate
// in the table, proven by difference against the same corpus priced cheaply.
func TestC036_F5_ZeroDeficitContributesNothing(t *testing.T) {
	cheapClean := []c036Rec{c036Lead, {model: c036Cheap, read: 20_000}}
	dearClean := []c036Rec{c036Lead, {model: c036Expensive, read: 20_000}}

	a := c036Run(t, c036Write(t, cheapClean))
	b := c036Run(t, c036Write(t, dearClean))
	if a.tokens != 0 || b.tokens != 0 {
		t.Fatalf("these corpora must carry no deficit; got %d and %d tokens",
			a.tokens, b.tokens)
	}
	if a.rebilledUSD != 0 || b.rebilledUSD != 0 {
		t.Errorf("F5 violated: a session with no deficit re-billed $%.6f and $%.6f",
			a.rebilledUSD, b.rebilledUSD)
	}
	t.Logf("F5: no deficit, $%.6f at the cheap rate and $%.6f at the expensive one",
		a.rebilledUSD, b.rebilledUSD)
}

// F2T. The TIMESTAMP half of record-local attribution.
//
// Mutation 2 of the C036 campaign — substitute the first record's timestamp —
// SURVIVED the fixtures above, and that was a property of the fixtures, not
// of the repair: with no rules document installed PriceForAt falls through to
// PriceFor (priceat_prod.go:17) and the clock is inert. Every record in those
// corpora also sat one minute apart.
//
// This case installs a dated document and puts the two records on opposite
// sides of the change, so the timestamp decides the rate.
func TestC036_F2T_TheRecordsOwnTimestampSetsTheRate(t *testing.T) {
	const dated = "c036-dated-model"
	r := &cachemodel.Rules{
		Schema:  cachemodel.RulesSchema,
		Version: "c036",
		Models: []cachemodel.ModelRule{
			{Match: dated, MinPrefix: 512, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true},
			{Match: dated, MinPrefix: 512, InputPerMTok: 1, OutputPerMTok: 5, ReadMult: 0.1, Priced: true,
				EffectiveFrom: "2026-09-10", EffectiveUntil: "2026-09-30"},
		},
	}
	defer cachemodel.Override(r)()

	lead := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)    // base row, $10/MTok
	break1 := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC) // dated row, $1/MTok

	pLead, ok := cachemodel.PriceForAt(dated, lead)
	if !ok {
		t.Fatalf("the lead date does not price; the fixture is broken")
	}
	pBreak, ok := cachemodel.PriceForAt(dated, break1)
	if !ok {
		t.Fatalf("the break date does not price; the fixture is broken")
	}
	if pLead.InputPerMTok == pBreak.InputPerMTok {
		t.Fatalf("both dates price at $%.2f/MTok; the fixture cannot tell the two "+
			"clocks apart and this case observes nothing", pLead.InputPerMTok)
	}

	recs := []c036Rec{
		{model: dated, create: 20_000, at: lead},
		{model: dated, create: 20_000, at: break1},
	}
	want := c036Oracle(t, recs)
	if want.tokens != 20_000 {
		t.Fatalf("expected a 20,000-token deficit, oracle computed %d", want.tokens)
	}
	got := c036Run(t, c036Write(t, recs))

	// Longhand, so the assertion does not depend on the oracle being right.
	wantUSD := 20_000.0 / 1_000_000 * pBreak.InputPerMTok
	leadUSD := 20_000.0 / 1_000_000 * pLead.InputPerMTok
	if math.Abs(got.rebilledUSD-wantUSD) > 1e-9 {
		t.Errorf("F2T violated: the breaking record ran on %s and prices at "+
			"$%.2f/MTok there, so its 20,000-token deficit is $%.6f. The product "+
			"reported $%.6f; the first record's date would give $%.6f.",
			break1.Format("2006-01-02"), pBreak.InputPerMTok, wantUSD,
			got.rebilledUSD, leadUSD)
	}
	t.Logf("F2T: $%.6f at the record's own date ($%.2f/MTok), not $%.6f at the "+
		"lead record's ($%.2f/MTok)", got.rebilledUSD, pBreak.InputPerMTok,
		leadUSD, pLead.InputPerMTok)
}
