package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

// A pooled figure that can be walked back to its parts.
//
// The claim these tests defend is not "the arithmetic is right". It is that
// there is no way to produce a pooled number this repository could not hand
// somebody the parts of. Every test below is a way that could stop being true.

func corpusFor(tag, takenAt string, tasks int, total, avoidable, median float64) Corpus {
	share := 0.0
	if total > 0 {
		share = avoidable / total
	}
	return Corpus{
		Schema: CorpusSchema, TakenAt: takenAt,
		Tasks: tasks, TotalUSD: total, AvoidableUSD: avoidable,
		AvoidableShare: share, MedianTaskUSD: median,
		PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		SourceTag: tag, TagBasis: BasisLocal,
	}.Digested()
}

func mustAdd(t *testing.T, p *Pool, c Corpus) {
	t.Helper()
	if err := p.Add(c, ""); err != nil {
		t.Fatalf("Add: %v", err)
	}
}

// PL1: the pooled total is the sum of the roster, and the roster names files.
//
// PASS: the total equals what the listed rows add to, and every row carries a
// digest and a filename.
// FAIL: a headline figure with nothing behind it a reader could fetch.
func TestPL1_TheTotalIsItsPartsAndThePartsAreNamed(t *testing.T) {
	p := NewPool("2026-09-09T00:00Z")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70))
	mustAdd(t, p, corpusFor("bbb", "2026-09-02T00:00Z", 20, 500, 25, 0.90))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.Submissions != 2 || got.Tasks != 120 || got.TotalUSD != 1500 || got.AvoidableUSD != 75 {
		t.Fatalf("the pooled figures do not match the roster: %+v", got)
	}
	var sum float64
	for _, e := range p.Roster {
		sum += e.TotalUSD
		if e.Digest == "" || e.File == "" {
			t.Errorf("roster row %s names nothing a reader could fetch", e.SourceTag)
		}
	}
	if sum != got.TotalUSD {
		t.Errorf("the roster sums to %v and the pool reports %v", sum, got.TotalUSD)
	}
}

// PL2: a pool with no submissions has no figure, and cannot be serialised.
//
// PASS: both Totals and MarshalJSON refuse.
// FAIL: a $0.00-across-0-corpora document that renders exactly like a result.
func TestPL2_AnEmptyPoolHasNoFigure(t *testing.T) {
	p := NewPool("2026-09-09T00:00Z")
	if _, err := p.Totals(); err == nil {
		t.Error("an empty pool produced totals")
	}
	if _, err := json.Marshal(p); err == nil {
		t.Error("an empty pool serialised; there is no such document as a pooled figure over nothing")
	}
	if _, err := p.Render(); err == nil {
		t.Error("an empty pool rendered")
	}
}

// PL3: the same file twice is refused.
func TestPL3_TheSameSubmissionTwiceIsRefused(t *testing.T) {
	p := NewPool("2026-09-09T00:00Z")
	c := corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70)
	mustAdd(t, p, c)
	if err := p.Add(c, ""); err == nil {
		t.Error("the identical submission was pooled twice")
	}
}

// PL4: one machine's later corpus SUPERSEDES its earlier one.
//
// This is the defect the pool was built with and would have shipped with. A
// corpus is cumulative: `replay cost` reads the whole transcript root on every
// run, so Tuesday's submission CONTAINS Monday's tasks. The two files are
// honestly distinct — different figures, different digests — so the
// duplicate-digest check does not fire, and summing them counts nearly all of
// that machine's money twice.
//
// What makes it dangerous is that the result looks fine. The total rises, the
// task count rises with it, and the avoidable share — the one figure a reader
// checks — barely moves, because numerator and denominator inflate together.
//
// PASS: the later submission replaces the earlier; the total is Tuesday's, not
// Monday-plus-Tuesday; the displaced file is still named.
// FAIL: a pooled total inflated by every contributor who ran the tool twice.
func TestPL4_ALaterCorpusSupersedesAnEarlierOne(t *testing.T) {
	p := NewPool("2026-09-09T00:00Z")
	monday := corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70)
	tuesday := corpusFor("aaa", "2026-09-02T00:00Z", 140, 1400, 70, 0.72)
	mustAdd(t, p, monday)
	mustAdd(t, p, tuesday)

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.Submissions != 1 {
		t.Errorf("one machine's two overlapping corpora counted as %d submissions", got.Submissions)
	}
	if got.TotalUSD != 1400 {
		t.Errorf("pooled total is $%.2f; Tuesday's corpus contains Monday's, so the answer "+
			"is $1400.00 and $2400.00 would be Monday counted twice", got.TotalUSD)
	}
	if got.Tasks != 140 {
		t.Errorf("pooled tasks is %d, want 140", got.Tasks)
	}
	if got.SupersededSubmissions != 1 {
		t.Errorf("the displaced submission is not reported; a reader who fetches every named "+
			"file and adds them up gets %v with no explanation", got.TotalUSD)
	}
	if len(p.Superseded) != 1 || p.Superseded[0].Digest != monday.Digest {
		t.Errorf("the displaced file is not named: %+v", p.Superseded)
	}
}

// PL5: order of arrival does not change the answer.
//
// The same two files, pooled newest-first, must give the same total. A pooled
// figure that depended on directory order would be unreproducible by the
// reader it invites to check it.
func TestPL5_SupersessionIsOrderIndependent(t *testing.T) {
	monday := corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70)
	tuesday := corpusFor("aaa", "2026-09-02T00:00Z", 140, 1400, 70, 0.72)

	forward := NewPool("x")
	mustAdd(t, forward, monday)
	mustAdd(t, forward, tuesday)
	backward := NewPool("x")
	mustAdd(t, backward, tuesday)
	mustAdd(t, backward, monday)

	a, err := forward.Totals()
	if err != nil {
		t.Fatal(err)
	}
	b, err := backward.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if a.TotalUSD != b.TotalUSD || a.Submissions != b.Submissions || a.Tasks != b.Tasks {
		t.Errorf("directory order changed the pooled figure: %+v vs %+v", a, b)
	}
	if b.SupersededSubmissions != 1 {
		t.Errorf("arriving stale, the older submission was not recorded as superseded")
	}
}

// PL6: two submissions from one machine at the same hour are refused.
//
// Nothing in either file says which measurement came second, so there is no
// defensible way to choose, and choosing arbitrarily makes the total depend on
// directory order — which PL5 forbids.
func TestPL6_AnAmbiguousSupersessionIsRefused(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70))
	err := p.Add(corpusFor("aaa", "2026-09-01T00:00Z", 140, 1400, 70, 0.72), "")
	if err == nil {
		t.Fatal("two same-hour corpora from one machine were silently reconciled")
	}
	if !strings.Contains(err.Error(), "which is later") {
		t.Errorf("the refusal does not say why it cannot choose: %v", err)
	}
}

// PL7: the pooled share is of the pooled money, not the mean of shares.
//
// The fixture is chosen so the two answers differ. A 10-task corpus at a 50%
// avoidable rate and a 1,000-task corpus at 1%: the mean of the shares is
// 25.5%, and the share of the pooled money is 1.5%. Publishing the first would
// let two tiny contributions move a population figure further than all the real
// spend in the pool.
func TestPL7_TheShareIsOfPooledMoneyNotAMeanOfShares(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 50, 5.0))
	mustAdd(t, p, corpusFor("bbb", "2026-09-01T00:00Z", 1000, 10000, 100, 0.10))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	want := 150.0 / 10100.0
	if diff := got.AvoidableShare - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("pooled share is %.4f, want %.4f", got.AvoidableShare, want)
	}
	meanOfShares := (0.50 + 0.01) / 2
	if diff := got.AvoidableShare - meanOfShares; diff < 1e-6 && diff > -1e-6 {
		t.Error("the pooled share equals the mean of the submissions' shares, which weights " +
			"a ten-task corpus like a thousand-task one")
	}
}

// PL8: there is no pooled median, and the range says so in words.
//
// A median of medians is not a median. The statistic that would be correct
// needs every task's cost, and a submission carries five figures rather than a
// per-task table precisely because that table is the disclosure the contributor
// declined to make.
func TestPL8_ThereIsNoPooledMedian(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 50, 0.20))
	mustAdd(t, p, corpusFor("bbb", "2026-09-01T00:00Z", 1000, 10000, 100, 3.40))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.MedianTaskUSDLow != 0.20 || got.MedianTaskUSDHigh != 3.40 {
		t.Errorf("the submitted medians are not bracketed: %v-%v",
			got.MedianTaskUSDLow, got.MedianTaskUSDHigh)
	}
	note := MedianSpan(got)
	if !strings.Contains(note, "RANGE") || !strings.Contains(note, "not a median") {
		t.Errorf("the span does not say it is a range rather than a median: %q", note)
	}
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	totals, _ := doc["totals"].(map[string]any)
	if _, found := totals["medianTaskUsd"]; found {
		t.Error("the document publishes a single pooled median; a median of medians is not a median")
	}
	if doc["medianNote"] == "" {
		t.Error("the caveat is not a field, so a consumer parsing this JSON never sees it")
	}
}

// PL9: a submission edited after it was written is refused.
//
// The roster invites a reader to fetch a file and re-hash it. Admitting a
// submission whose content no longer matches its own digest would publish a row
// that fails the check the row exists to invite.
func TestPL9_ATamperedSubmissionIsRefused(t *testing.T) {
	c := corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70)
	c.TotalUSD = 999999 // the digest still names the original figures

	p := NewPool("x")
	if err := p.Add(c, ""); err == nil {
		t.Fatal("a submission whose content does not match its digest was pooled")
	}
	if len(p.Roster) != 0 {
		t.Error("the refusal left a row in the roster")
	}
}

// PL10: corpora scored under different caching rules are not addable.
//
// Avoidable spend means a different quantity under each rules version, so the
// sum is of nothing. The price-table date is treated differently on purpose:
// these totals are dollars and that table is the display-currency reference, so
// a spread of dates is disclosed rather than refused.
func TestPL10_MismatchedRulesAreRefusedAndPriceDatesAreDisclosed(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70))

	other := corpusFor("bbb", "2026-09-01T00:00Z", 100, 1000, 50, 0.70)
	other.RulesVersion = "anthropic-2027-01-01"
	other = other.Digested()
	if err := p.Add(other, ""); err == nil {
		t.Error("corpora scored under different caching rules were added together")
	}

	later := corpusFor("ccc", "2026-09-01T00:00Z", 100, 1000, 50, 0.70)
	later.PricedAt = "2026-09-14"
	later = later.Digested()
	if err := p.Add(later, ""); err != nil {
		t.Fatalf("a differing price-table date was refused rather than disclosed: %v", err)
	}
	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.PricedAtDistinct != 2 || got.PricedAtLow != "2026-09-07" || got.PricedAtHigh != "2026-09-14" {
		t.Errorf("the price-table spread is not disclosed: %+v", got)
	}
}

// PL11: n is not a count of people, and the document says so where it is read.
//
// The tag comes from a secret the contributing machine minted. One person with
// three machines is three tags, and anyone can mint unlimited ones. That caveat
// has to be a field, because a consumer parses this JSON and never reads the
// README.
func TestPL11_SubmissionsAreNotPeople(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 5, 0.5))
	mustAdd(t, p, corpusFor("bbb", "2026-09-01T00:00Z", 10, 100, 5, 0.5))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.TagsAreIdentities {
		t.Error("local tags are reported as identities")
	}
	note := TagNote(got)
	if !strings.Contains(note, "MACHINES AT MOST") || !strings.Contains(note, "not of people") {
		t.Errorf("the tag note does not say what the count is not: %q", note)
	}
	// One entry per tag is now an invariant of Add, so these must agree. If
	// they ever diverge, supersession has stopped working and the pool is
	// double-counting a machine again.
	if got.DistinctTags != got.Submissions {
		t.Errorf("%d submissions carry %d distinct tags; Add admits one row per machine, so "+
			"a mismatch means supersession is broken", got.Submissions, got.DistinctTags)
	}
}

// PL12: the rendered report shows the parts before the figure.
//
// A reader who stops after the first screen has seen the submissions; a reader
// who stops before it has seen nothing to stop at.
func TestPL12_TheRosterPrecedesTheHeadline(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70))
	mustAdd(t, p, corpusFor("bbb", "2026-09-02T00:00Z", 20, 500, 25, 0.90))

	out, err := p.Render()
	if err != nil {
		t.Fatal(err)
	}
	headline := strings.Index(out, "across 2 contributed corpora")
	if headline < 0 {
		t.Fatalf("the report does not state the figure with its n:\n%s", out)
	}
	firstRow := strings.Index(out, p.Roster[0].File)
	if firstRow < 0 {
		t.Fatalf("the report does not name the first submission's file:\n%s", out)
	}
	if firstRow > headline {
		t.Error("the headline figure appears before the submissions it is made of")
	}
	for _, e := range p.Roster {
		if !strings.Contains(out, e.File) {
			t.Errorf("submission %s is counted but not named", e.SourceTag)
		}
	}
	if !strings.Contains(out, "fetch and re-hash") {
		t.Error("the report does not tell the reader the rows are checkable")
	}
}
