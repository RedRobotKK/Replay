package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RPL-C037, FROZEN CANDIDATE. Break-level coverage of RebilledUSD.
//
// `replay cost` prints a re-billed dollar figure beside a re-billed token
// count as one result. RebilledUSD covers only the breaks whose own record
// could be priced; RebilledTokens covers every break. No rendered or
// serialized field says which, so two corpora with different break-level
// priceability collapse to the same re-billing surface.
//
// REPAIRED 2026-10-02. P1, P1b and P2 are INVERTED rather than deleted,
// exactly as RB1Pop was inverted after the FM repair, so the collapse cannot
// return unnoticed. Each one records the figures it measured BEFORE the
// repair, which are the RED evidence the claim was frozen on.
//
// The names are kept so the claim register's Tests list and the evidence
// trail stay valid. P1 no longer asserts a collapse; it asserts that the
// collapse it is named for is gone.
//
// SEPARATION, deliberate and load-bearing:
//
//	C031/C035  request-level coverage of TotalUSD     (requests)
//	C036       which rate a break's deficit pays      (rate selection)
//	C037       break-level coverage of RebilledUSD    (breaks)         <- here
//
// Nothing in this file imports a C036 helper, so C036's frozen fixture cannot
// be perturbed by work on this claim.

// ---------------------------------------------------------------- fixture

const c037Model = "c037-break-coverage-model"

// Three dated windows on ONE model name. Holding the name constant keeps the
// route line and the model identity identical across corpora, so the only
// thing that varies between the arms is the rate in force on a given day.
//
// $1.00/MTok, $0.50/MTok, and a window where the model is NOT PRICED. Output
// scales with input so every cost leg scales linearly with the row and two
// arms can be made to agree on the totals exactly.
func c037Rules() *cachemodel.Rules {
	return &cachemodel.Rules{
		Schema: cachemodel.RulesSchema, Version: "c037",
		Models: []cachemodel.ModelRule{
			{Match: c037Model, MinPrefix: 512, InputPerMTok: 1.0, OutputPerMTok: 5.0, ReadMult: 0.1,
				Priced: true, EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-05"},
			{Match: c037Model, MinPrefix: 512, InputPerMTok: 0.5, OutputPerMTok: 2.5, ReadMult: 0.1,
				Priced: true, EffectiveFrom: "2026-09-06", EffectiveUntil: "2026-09-10"},
			{Match: c037Model, MinPrefix: 512, InputPerMTok: 0, OutputPerMTok: 0, ReadMult: 0,
				Priced: false, EffectiveFrom: "2026-09-11", EffectiveUntil: "2026-09-15"},
		},
	}
}

func c037Day(d int) time.Time { return time.Date(2026, 9, d, 9, 0, 0, 0, time.UTC) }

// c037Rec is one record, specified in the terms the claim is about: the
// tokens it writes, the tokens it reads, the day it ran, and whether the
// fixture DECLARES it priceable. The declaration is checked against the price
// table in TestC037_FixtureAssumptions rather than taken on trust.
type c037Rec struct {
	day        int
	create     int
	read       int
	wantPriced bool
}

// brk writes 20,000 tokens and reads none, so the record after it has a
// 20,000-token deficit. clean reads exactly what the record before it wrote,
// so it has none.
func c037Brk(day int, priced bool) c037Rec {
	return c037Rec{day: day, create: 20_000, wantPriced: priced}
}
func c037Clean(day int, priced bool) c037Rec {
	return c037Rec{day: day, read: 20_000, wantPriced: priced}
}

func c037Write(t *testing.T, recs []c037Rec) string {
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
			Schema: ledger.SchemaVersion, Timestamp: c037Day(r.day),
			SessionID: "c037", RequestID: fmt.Sprintf("c037-%02d", i),
			Path: "/v1/messages", Status: 200, LatencyMS: 900,
			RequestSummary: ledger.RequestSummary{Model: c037Model,
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
	if err := os.WriteFile(filepath.Join(dir, "c037.jsonl"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// ----------------------------------------------------------------- oracle

// c037Oracle is the break-token ledger, computed from the FIXTURE
// SPECIFICATION and nothing the production cost path produces. It never reads
// RebilledUSD or RebilledTokens.
type c037Oracle struct {
	PricedTokens   int
	UnpricedTokens int
}

func (o c037Oracle) Total() int { return o.PricedTokens + o.UnpricedTokens }

// Coverage is the quantity the surface does not disclose.
func (o c037Oracle) Coverage() (float64, bool) {
	if o.Total() == 0 {
		return 0, false
	}
	return float64(o.PricedTokens) / float64(o.Total()), true
}

// c037Expect reimplements the break rule longhand: a request reads what the
// one before it wrote, and whatever it did not read is a deficit ON THAT
// RECORD. Priceability comes from the fixture's own declaration.
func c037Expect(recs []c037Rec) c037Oracle {
	var o c037Oracle
	for i := 1; i < len(recs); i++ {
		expected := recs[i-1].create + recs[i-1].read
		if recs[i].read >= expected {
			continue
		}
		d := expected - recs[i].read
		if recs[i].wantPriced {
			o.PricedTokens += d
		} else {
			o.UnpricedTokens += d
		}
	}
	return o
}

// ---------------------------------------------------------------- surfaces

// c037Prod is everything the production surfaces actually expose about
// re-billing, text and JSON together.
type c037Prod struct {
	Text             string
	RebilledUSD      float64
	RebilledTokens   int
	RebilledShare    float64
	Breaks           int
	Requests         int
	PricedRequests   int
	UnpricedRequests int
	UnitUnpriced     bool
}

// RebillProjection is the part of the surface that reports re-billing. If two
// corpora agree here, no consumer of the re-billed result can tell them
// apart.
func (p c037Prod) RebillProjection() string {
	var lines []string
	for _, l := range strings.Split(p.Text, "\n") {
		if strings.Contains(l, "re-billed") {
			lines = append(lines, strings.TrimRight(l, " "))
		}
	}
	return fmt.Sprintf("TEXT[%s] JSON[usd=%.9f tokens=%d share=%.9f breaks=%d]",
		strings.Join(lines, " | "), p.RebilledUSD, p.RebilledTokens, p.RebilledShare, p.Breaks)
}

// RebillPair is the re-billed RESULT itself: the dollar figure, the token
// count and the number of breaks they describe. It excludes RebilledShare,
// which is RebilledUSD/TotalUSD and therefore moves with the as-run total for
// reasons that have nothing to do with break coverage.
func (p c037Prod) RebillPair() string {
	var lines []string
	for _, l := range strings.Split(p.Text, "\n") {
		if strings.Contains(l, "tokens re-billed") {
			lines = append(lines, strings.TrimRight(l, " "))
		}
		if strings.Contains(l, "  re-billed ") {
			// Drop the "(N% of the total)" tail, which is the share.
			if k := strings.Index(l, "("); k > 0 {
				l = l[:k]
			}
			lines = append(lines, strings.TrimRight(l, " "))
		}
	}
	return fmt.Sprintf("TEXT[%s] JSON[usd=%.9f tokens=%d breaks=%d]",
		strings.Join(lines, " | "), p.RebilledUSD, p.RebilledTokens, p.Breaks)
}

func c037Run(t *testing.T, recs []c037Rec) c037Prod {
	t.Helper()
	dir := c037Write(t, recs)
	var txt, e bytes.Buffer
	if err := runCost([]string{dir}, &txt, &e); err != nil {
		t.Fatalf("cost: %v (%s)", err, e.String())
	}
	var j bytes.Buffer
	e.Reset()
	if err := runCost([]string{"--per-task", "--json", c037Write(t, recs)}, &j, &e); err != nil {
		t.Fatalf("cost --json: %v (%s)", err, e.String())
	}
	var doc struct {
		Summary struct {
			RebilledUSD    float64 `json:"rebilledUsd"`
			RebilledTokens int     `json:"rebilledTokens"`
			RebilledShare  float64 `json:"rebilledShare"`
		} `json:"summary"`
		Tasks []struct {
			Requests         int  `json:"requests"`
			PricedRequests   int  `json:"pricedRequests"`
			UnpricedRequests int  `json:"unpricedRequests"`
			Breaks           int  `json:"breaks"`
			Unpriced         bool `json:"unpriced"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(j.Bytes(), &doc); err != nil {
		t.Fatalf("cost JSON does not parse: %v", err)
	}
	p := c037Prod{Text: txt.String(),
		RebilledUSD: doc.Summary.RebilledUSD, RebilledTokens: doc.Summary.RebilledTokens,
		RebilledShare: doc.Summary.RebilledShare}
	for _, tk := range doc.Tasks {
		p.Breaks += tk.Breaks
		p.Requests += tk.Requests
		p.PricedRequests += tk.PricedRequests
		p.UnpricedRequests += tk.UnpricedRequests
		p.UnitUnpriced = p.UnitUnpriced || tk.Unpriced
	}
	return p
}

// c037RequestCoverageLine is the C035 sentence, which is about a DIFFERENT
// population. Extracted so a test can hold it constant.
func c037RequestCoverageLine(text string) string {
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, "of the requests read") {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

// ---------------------------------------------------------------- detector

// c037DisclosedBreakCoverage looks for a statement about how much of the
// RE-BILLED result the dollar figure covers. It must not fire on the C035
// sentence, which is about requests and about the total.
//
// Today it finds nothing, and that absence is the claim. The detector is
// still written now, and proven to discriminate now, because a detector
// written after a repair is a detector tuned to the repair.
func c037DisclosedBreakCoverage(text string) (float64, bool) {
	// PARAGRAPH-scoped, not line-scoped. The first version of this detector
	// read one line at a time and could not see its own planted control,
	// because a disclosure sentence wraps and the subject and the pricing
	// word land on different lines. That is the fourth detector in this
	// campaign to be blind to the string its control planted, and it was
	// found the same way as the other three: by reading the output.
	for _, para := range strings.Split(text, "\n\n") {
		low := strings.ToLower(para)
		if strings.Contains(low, "of the requests read") {
			continue // C035. Different population, different claim.
		}
		if !strings.Contains(low, "%") {
			continue
		}
		subject := strings.Contains(low, "re-billed") || strings.Contains(low, "break")
		pricing := strings.Contains(low, "priced") || strings.Contains(low, "price table") ||
			strings.Contains(low, "unpriceable")
		if !subject || !pricing {
			continue
		}
		i := strings.Index(para, "%")
		j := i
		for j > 0 && (para[j-1] >= '0' && para[j-1] <= '9' || para[j-1] == '.') {
			j--
		}
		if j == i {
			continue
		}
		var pct float64
		if _, err := fmt.Sscanf(para[j:i], "%g", &pct); err != nil {
			continue
		}
		return pct / 100, true
	}
	return 0, false
}

// ------------------------------------------------------------- the corpora
//
// PAIR 1. Every rendered and serialized re-billing number identical.
//
//	P  lead $1.00 | break $0.50 | break $0.50
//	     break tokens 40,000, ALL PRICEABLE          coverage 40,000/40,000
//	Q  lead $1.00 | break $1.00 | break UNPRICED
//	     break tokens 40,000, 20,000 UNPRICEABLE     coverage 20,000/40,000
//
// Both re-bill $0.020000 over 40,000 tokens across 2 breaks. The cost legs
// agree because 1.00+0.50+0.50 == 1.00+1.00+0.

func c037PairOneP() []c037Rec {
	return []c037Rec{c037Brk(1, true), c037Brk(6, true), c037Brk(7, true)}
}
func c037PairOneQ() []c037Rec {
	return []c037Rec{c037Brk(1, true), c037Brk(2, true), c037Brk(11, false)}
}

// PAIR 2. The orthogonal control. Both arms carry exactly one unpriceable
// REQUEST, so C035's request-coverage sentence is identical, and the
// re-billing projection is identical too. Break coverage still differs.
//
//	R   lead $0.50 | break $0.50 | break $0.50 | clean UNPRICED
//	      break tokens 40,000, ALL PRICEABLE
//	Q2  lead $1.00 | break $1.00 | break UNPRICED | clean $1.00
//	      break tokens 40,000, 20,000 UNPRICEABLE

func c037PairTwoR() []c037Rec {
	return []c037Rec{c037Brk(6, true), c037Brk(7, true), c037Brk(8, true), c037Clean(11, false)}
}
func c037PairTwoQ() []c037Rec {
	return []c037Rec{c037Brk(1, true), c037Brk(2, true), c037Brk(11, false), c037Clean(3, true)}
}

// A corpus with no break at all.
func c037NoBreaks() []c037Rec { return []c037Rec{c037Brk(1, true), c037Clean(2, true)} }

// ------------------------------------------------------------------ tests

// FIXTURE PRECONDITIONS. Everything downstream is worthless if the price
// table does not agree with what the fixture declares, or if the two arms do
// not actually differ in break coverage.
func TestC037_FixtureAssumptions(t *testing.T) {
	defer cachemodel.Override(c037Rules())()

	for _, c := range []struct {
		name string
		recs []c037Rec
	}{
		{"P1/P", c037PairOneP()}, {"P1/Q", c037PairOneQ()},
		{"P2/R", c037PairTwoR()}, {"P2/Q2", c037PairTwoQ()},
		{"no-breaks", c037NoBreaks()},
	} {
		for i, r := range c.recs {
			_, ok := cachemodel.PriceForAt(c037Model, c037Day(r.day))
			if ok != r.wantPriced {
				t.Fatalf("%s record %d (day %d): the price table says priced=%v, the "+
					"fixture declares %v. Every result in this file rests on the two "+
					"agreeing.", c.name, i, r.day, ok, r.wantPriced)
			}
		}
	}

	for _, c := range []struct {
		name                     string
		recs                     []c037Rec
		wantPriced, wantUnpriced int
	}{
		{"P1/P", c037PairOneP(), 40_000, 0},
		{"P1/Q", c037PairOneQ(), 20_000, 20_000},
		{"P2/R", c037PairTwoR(), 40_000, 0},
		{"P2/Q2", c037PairTwoQ(), 20_000, 20_000},
		{"no-breaks", c037NoBreaks(), 0, 0},
	} {
		o := c037Expect(c.recs)
		if o.PricedTokens != c.wantPriced || o.UnpricedTokens != c.wantUnpriced {
			t.Fatalf("%s: oracle computed priced=%d unpriced=%d, the specification "+
				"says %d and %d", c.name, o.PricedTokens, o.UnpricedTokens,
				c.wantPriced, c.wantUnpriced)
		}
		// And the production token total must agree, or the fixture is not
		// producing the breaks the oracle thinks it is. This is the ONE place
		// the oracle is reconciled against production, and it is about the
		// quantity (settled by C036), never about the dollars.
		got := c037Run(t, c.recs)
		if got.RebilledTokens != o.Total() {
			t.Fatalf("%s: production counted %d re-billed tokens, the oracle %d",
				c.name, got.RebilledTokens, o.Total())
		}
	}
}

// P1. THE COLLAPSE. Two corpora, 100% and 50% break coverage, one
// re-billing surface.
func TestC037_P1_TheRebillingSurfaceCollapses(t *testing.T) {
	defer cachemodel.Override(c037Rules())()

	p, q := c037Run(t, c037PairOneP()), c037Run(t, c037PairOneQ())
	cp, _ := c037Expect(c037PairOneP()).Coverage()
	cq, _ := c037Expect(c037PairOneQ()).Coverage()

	if cp == cq {
		t.Fatalf("the two arms have the same break coverage (%.2f); this observes nothing", cp)
	}

	// BEFORE the repair, both arms rendered:
	//   re-billed $0.02 (37% of the total) | 40k tokens re-billed
	//   JSON usd=0.020000000 tokens=40000 share=0.370370370 breaks=2
	// identical, at 100% and 50% break coverage. INVERTED: they must differ.
	if p.RebillProjection() == q.RebillProjection() {
		t.Fatalf("THE COLLAPSE HAS RETURNED. 100%% and %.0f%% break coverage both "+
			"render:\n  %s", cq*100, p.RebillProjection())
	}

	// And the difference must be the coverage statement, not something else.
	dp, pOK := c037DisclosedBreakCoverage(p.Text)
	dq, qOK := c037DisclosedBreakCoverage(q.Text)
	if pOK {
		t.Errorf("the fully covered arm discloses %.0f%% break coverage; it must "+
			"disclose nothing", dp*100)
	}
	if !qOK {
		t.Fatalf("the half-covered arm discloses no break coverage")
	}
	if math.Abs(dq-cq) > 1e-9 {
		t.Errorf("the half-covered arm discloses %.1f%% and the oracle says %.1f%%",
			dq*100, cq*100)
	}
	// RebilledUSD and RebilledTokens must NOT have moved. The repair is a
	// disclosure, not an arithmetic change.
	if p.RebilledUSD != 0.02 || q.RebilledUSD != 0.02 ||
		p.RebilledTokens != 40_000 || q.RebilledTokens != 40_000 {
		t.Errorf("a dollar or token figure moved: P $%.6f/%d, Q $%.6f/%d; both were "+
			"$0.020000 over 40,000 tokens before the repair and must still be",
			p.RebilledUSD, p.RebilledTokens, q.RebilledUSD, q.RebilledTokens)
	}
	t.Logf("C037 REPAIRED: 100%% arm discloses nothing, %.0f%% arm discloses %.0f%%, "+
		"and both still report $%.6f over %d tokens",
		cq*100, dq*100, p.RebilledUSD, p.RebilledTokens)
}

// P1b. THE ORTHOGONAL CONTROL. C035's request-level sentence is identical in
// both arms, so it cannot be the field that distinguishes them.
func TestC037_P1b_RequestCoverageCarriesNoBreakCoverage(t *testing.T) {
	defer cachemodel.Override(c037Rules())()

	r, q := c037Run(t, c037PairTwoR()), c037Run(t, c037PairTwoQ())
	cr, _ := c037Expect(c037PairTwoR()).Coverage()
	cq, _ := c037Expect(c037PairTwoQ()).Coverage()

	if cr == cq {
		t.Fatalf("both arms have break coverage %.2f; this observes nothing", cr)
	}
	lr, lq := c037RequestCoverageLine(r.Text), c037RequestCoverageLine(q.Text)
	if lr == "" {
		t.Fatalf("neither arm printed a C035 request-coverage line, so this control "+
			"is not holding anything constant. Text:\n%s", r.Text)
	}
	// C035 is untouched by the repair and must stay identical in both arms.
	if lr != lq {
		t.Fatalf("the request-coverage sentences differ:\n  R  %q\n  Q2 %q\nC035 was "+
			"not supposed to change.", lr, lq)
	}
	// The re-billed RESULT is still identical, deliberately: the repair
	// changed no dollar and no token count. BEFORE the repair that identity
	// was the whole defect, because nothing else distinguished the arms.
	if r.RebillPair() != q.RebillPair() {
		t.Fatalf("a re-billed dollar or token figure moved:\n  R  %s\n  Q2 %s\nThe "+
			"repair is a disclosure and must not have touched either.",
			r.RebillPair(), q.RebillPair())
	}
	// INVERTED: the disclosure is now what tells them apart, and the C035
	// sentence held identical above is proven not to be doing that work.
	dr, rOK := c037DisclosedBreakCoverage(r.Text)
	dq, qOK := c037DisclosedBreakCoverage(q.Text)
	if rOK {
		t.Errorf("the fully covered arm discloses %.0f%% break coverage", dr*100)
	}
	if !qOK {
		t.Fatalf("THE COLLAPSE HAS RETURNED. With the C035 sentence identical and the "+
			"re-billed result identical, nothing distinguishes %.0f%% from %.0f%% "+
			"break coverage.", cr*100, cq*100)
	}
	if math.Abs(dq-cq) > 1e-9 {
		t.Errorf("the half-covered arm discloses %.1f%% and the oracle says %.1f%%",
			dq*100, cq*100)
	}
	t.Logf("C037 REPAIRED: request coverage identical (%q) and the re-billed result "+
		"identical (%s), and break coverage now reads absent against %.0f%%",
		lr, r.RebillPair(), dq*100)
}

// P2. The disclosed coverage must equal pricedBreakTokens over all break
// tokens, computed from the fixture specification.
//
// BEFORE the repair this test recorded the ABSENCE of any disclosure and
// logged the oracle's answer beside the figure it was missing from. INVERTED:
// the disclosure must now be present and correct.
func TestC037_P2_NoBreakCoverageIsDisclosed(t *testing.T) {
	defer cachemodel.Override(c037Rules())()

	for _, c := range []struct {
		name string
		recs []c037Rec
	}{{"P1/Q", c037PairOneQ()}, {"P2/Q2", c037PairTwoQ()}} {
		o := c037Expect(c.recs)
		want, ok := o.Coverage()
		if !ok {
			t.Fatalf("%s: the oracle has no coverage to compare against", c.name)
		}
		got := c037Run(t, c.recs)
		disclosed, present := c037DisclosedBreakCoverage(got.Text)
		if !present {
			t.Errorf("%s: %d of %d re-billed tokens are outside $%.6f and the report "+
				"discloses nothing. This is RPL-C037 returning.",
				c.name, o.UnpricedTokens, o.Total(), got.RebilledUSD)
			continue
		}
		if math.Abs(disclosed-want) > 1e-9 {
			t.Errorf("%s: disclosed %.1f%%, oracle %.1f%%. P2 requires "+
				"pricedBreakTokens/(priced+unpriced).", c.name, disclosed*100, want*100)
			continue
		}
		t.Logf("%s: disclosed %.0f%%, oracle %.0f%%, %d of %d tokens inside $%.6f",
			c.name, disclosed*100, want*100, o.PricedTokens, o.Total(), got.RebilledUSD)
	}
}

// P3. A fully priceable corpus must never carry a partial-coverage
// disclosure. Live after a repair; today it also guards against a disclosure
// that fires on everything.
func TestC037_P3_FullCoverageEmitsNoPartialDisclosure(t *testing.T) {
	defer cachemodel.Override(c037Rules())()
	for _, c := range []struct {
		name string
		recs []c037Rec
	}{{"P1/P", c037PairOneP()}, {"P2/R", c037PairTwoR()}} {
		o := c037Expect(c.recs)
		if o.UnpricedTokens != 0 {
			t.Fatalf("%s is not a fully covered corpus: %d unpriceable break tokens",
				c.name, o.UnpricedTokens)
		}
		got := c037Run(t, c.recs)
		if d, present := c037DisclosedBreakCoverage(got.Text); present && d < 1 {
			t.Errorf("%s: every break token is priceable and the report discloses "+
				"%.1f%% break coverage", c.name, d*100)
		}
	}
}

// P4. A corpus with no break at all must carry no coverage disclosure, and no
// re-billed token line either.
func TestC037_P4_ZeroBreakCorpusEmitsNoDisclosure(t *testing.T) {
	defer cachemodel.Override(c037Rules())()
	o := c037Expect(c037NoBreaks())
	if o.Total() != 0 {
		t.Fatalf("the zero-break corpus produced %d break tokens", o.Total())
	}
	got := c037Run(t, c037NoBreaks())
	if got.RebilledTokens != 0 {
		t.Fatalf("production found %d re-billed tokens in a corpus with no break",
			got.RebilledTokens)
	}
	if d, present := c037DisclosedBreakCoverage(got.Text); present {
		t.Errorf("a corpus with no break disclosed %.1f%% break coverage", d*100)
	}
	if strings.Contains(got.Text, "tokens re-billed") {
		t.Errorf("a corpus with no break printed a re-billed token line")
	}
}

// THE DETECTOR MUST DISCRIMINATE. Three times in this campaign a detector was
// blind to the string its own control planted, so it is proven here against a
// planted sentence BEFORE any repair can tune it.
func TestC037_TheDetectorDiscriminates(t *testing.T) {
	const planted = "The re-billed figure above covers 50% of the re-billed tokens: 20,000 of\n" +
		"40,000 were priced, 20,000 ran on a model no price table carries."
	got, ok := c037DisclosedBreakCoverage(planted)
	if !ok {
		t.Fatalf("the detector cannot see a plausible break-coverage sentence:\n%s", planted)
	}
	if math.Abs(got-0.5) > 1e-9 {
		t.Errorf("the detector read %.3f from a sentence stating 50%%", got)
	}
	// And it must NOT fire on C035's request-level sentence, which is the
	// whole point of keeping the two claims apart.
	const c035 = "The total above covers 75% of the requests read: 3 of 4 priced, 1 on a\n" +
		"model no price table carries. Those 1 are not in the figure and are not free;\n" +
		"what they cost is not established here."
	if _, ok := c037DisclosedBreakCoverage(c035); ok {
		t.Errorf("the detector fires on C035's REQUEST-coverage sentence. The two " +
			"claims are about different populations and this would conflate them.")
	}
	// Nor on the current report, which carries neither.
	defer cachemodel.Override(c037Rules())()
	if _, ok := c037DisclosedBreakCoverage(c037Run(t, c037PairOneP()).Text); ok {
		t.Errorf("the detector fires on today's report, which discloses no break coverage")
	}
}

// ------------------------------------------------- candidate-repair mutants
//
// No production disclosure exists yet, so these mutate a SIMULATED one. That
// tests the ORACLE's discriminating power, not a repair, which is what this
// phase is for. Each must be shown to reach the deciding path first: a mutant
// that returns the oracle's own answer on every fixture proves nothing.

type c037Disclosure struct {
	name string
	fn   func(o c037Oracle, p c037Prod) (coverage float64, tokens int)
}

func c037Candidates() []c037Disclosure {
	return []c037Disclosure{
		{"CORRECT: priced break tokens over all break tokens", func(o c037Oracle, p c037Prod) (float64, int) {
			c, _ := o.Coverage()
			return c, o.Total()
		}},
		{"M1 force the disclosure to 100%", func(o c037Oracle, p c037Prod) (float64, int) {
			return 1, o.Total()
		}},
		{"M2 derive coverage from REQUEST counts", func(o c037Oracle, p c037Prod) (float64, int) {
			n := p.PricedRequests + p.UnpricedRequests
			if n == 0 {
				return 0, o.Total()
			}
			return float64(p.PricedRequests) / float64(n), o.Total()
		}},
		{"M3 derive coverage from RebilledUSD > 0", func(o c037Oracle, p c037Prod) (float64, int) {
			if p.RebilledUSD > 0 {
				return 1, o.Total()
			}
			return 0, o.Total()
		}},
		{"M4 read coverage off the unit-level Unpriced flag", func(o c037Oracle, p c037Prod) (float64, int) {
			if p.UnitUnpriced {
				return 0, o.Total()
			}
			return 1, o.Total()
		}},
		{"M5 drop unpriceable break tokens and claim full coverage", func(o c037Oracle, p c037Prod) (float64, int) {
			return 1, o.PricedTokens
		}},
	}
}

// c037Judge is the frozen acceptance rule: the disclosed coverage must equal
// the oracle's, AND the token total must still be every break token, so a
// candidate cannot buy agreement by discarding evidence.
func c037Judge(o c037Oracle, coverage float64, tokens int) (bool, string) {
	want, ok := o.Coverage()
	if !ok {
		return tokens == 0, "zero-break corpus"
	}
	if tokens != o.Total() {
		return false, fmt.Sprintf("token total %d, every break accounts for %d", tokens, o.Total())
	}
	if math.Abs(coverage-want) > 1e-9 {
		return false, fmt.Sprintf("coverage %.4f, oracle %.4f", coverage, want)
	}
	return true, ""
}

func TestC037_TheOracleRejectsEveryWrongCandidate(t *testing.T) {
	defer cachemodel.Override(c037Rules())()

	corpora := []struct {
		name string
		recs []c037Rec
	}{
		{"P1/P full", c037PairOneP()},
		{"P1/Q partial", c037PairOneQ()},
		{"P2/R full", c037PairTwoR()},
		{"P2/Q2 partial", c037PairTwoQ()},
		{"no-breaks", c037NoBreaks()},
	}
	prods := make([]c037Prod, len(corpora))
	oracles := make([]c037Oracle, len(corpora))
	for i, c := range corpora {
		prods[i] = c037Run(t, c.recs)
		oracles[i] = c037Expect(c.recs)
	}

	for _, cand := range c037Candidates() {
		correct := strings.HasPrefix(cand.name, "CORRECT")
		accepted, reachedDecider := true, false
		var why string
		for i := range corpora {
			cov, tok := cand.fn(oracles[i], prods[i])
			refCov, refTok := c037Candidates()[0].fn(oracles[i], prods[i])
			if math.Abs(cov-refCov) > 1e-9 || tok != refTok {
				// This mutant actually computes something different from the
				// correct implementation on this fixture.
				reachedDecider = true
			}
			ok, reason := c037Judge(oracles[i], cov, tok)
			if !ok && accepted {
				accepted, why = false, corpora[i].name+": "+reason
			}
		}
		switch {
		case correct && !accepted:
			t.Errorf("the CORRECT candidate was rejected (%s). The acceptance rule is "+
				"wrong and every kill below is meaningless.", why)
		case correct:
			t.Logf("control: the correct candidate is accepted on all %d corpora", len(corpora))
		case !reachedDecider:
			t.Errorf("MIS-TARGETED: %s computes exactly what the correct candidate "+
				"computes on every fixture here, so its survival or death says nothing. "+
				"It needs a fixture where the two diverge.", cand.name)
		case accepted:
			t.Errorf("SURVIVED: %s differs from the correct candidate and the "+
				"acceptance rule still accepted it. The oracle is too weak.", cand.name)
		default:
			t.Logf("KILLED: %s (%s)", cand.name, why)
		}
	}
}
