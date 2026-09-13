package observation

import (
	"math"
	"strings"
	"testing"
	"time"
)

// Placing a reader against the pool.
//
// A four-reviewer QA pass rejected the first version of this feature. Every
// test below exists because something was wrong, and the comment on each says
// what. The coverage finding was the worst of them: every over-claiming
// sentence in the file was unexecuted, so the wording nobody had run was the
// wording nobody could catch.

func usd(v float64) *float64 { return &v }

// pool builds a pool that passes every gate, so a test can break exactly one.
func pool(medians ...float64) PoolTotals {
	p := PoolTotals{
		Submissions: 40, DistinctTags: 40, Tasks: 5000,
		TotalUSD: 1000, AvoidableUSD: 200, AvoidableShare: 0.20,
		RulesVersion: "anthropic-2026-09-01",
		PricedAtLow:  "2026-09-01", PricedAtHigh: "2026-09-07", PricedAtDistinct: 2,
	}
	if len(medians) == 0 {
		for i := 0; i < 40; i++ {
			medians = append(medians, 0.10+float64(i)*0.02)
		}
	}
	p.MedianTaskUSDs = medians
	p.MedianTaskUSDLow, p.MedianTaskUSDHigh = medians[0], medians[len(medians)-1]
	return p
}

func me(median *float64, share *float64) Benchmark {
	return Benchmark{MedianTaskUSD: median, AvoidableShare: share, Tasks: 500, Unit: PoolUnit, RulesVersion: "anthropic-2026-09-01"}
}

// ---- refusals ----

func TestBM_RefusesAndAlwaysSaysWhy(t *testing.T) {
	// Every refusal must render a reason. A section that silently vanishes
	// leaves the reader with the same null and no explanation, which is the
	// thing the feature exists to fix.
	cases := []struct {
		name string
		pool func(PoolTotals) PoolTotals
		mine func(Benchmark) Benchmark
		want string
	}{
		{"empty pool", func(p PoolTotals) PoolTotals { p.Submissions, p.DistinctTags = 0, 0; return p }, nil, "empty"},
		{"more tags than submissions", func(p PoolTotals) PoolTotals { p.DistinctTags = 99; return p }, nil, "cannot both be true"},
		{"below the tag floor", func(p PoolTotals) PoolTotals { p.DistinctTags = BenchmarkMinTags - 1; return p }, nil, "at least"},
		{"no per-submission medians", func(p PoolTotals) PoolTotals { p.MedianTaskUSDs = nil; return p }, nil, "predates"},
		{"pooled share is NaN", func(p PoolTotals) PoolTotals { p.AvoidableShare = math.NaN(); return p }, nil, "not a share"},
		{"pooled share above 1", func(p PoolTotals) PoolTotals { p.AvoidableShare = 3.5; return p }, nil, "not a share"},
		{"per-lane reader", nil, func(b Benchmark) Benchmark { b.Unit = "lane"; return b }, "different quantities"},
		{"rules version differs", nil, func(b Benchmark) Benchmark { b.RulesVersion = "anthropic-2025-01-01"; return b }, "not comparable"},
	}
	for _, c := range cases {
		p := pool()
		if c.pool != nil {
			p = c.pool(p)
		}
		b := me(usd(0.40), usd(0.05))
		if c.mine != nil {
			b = c.mine(b)
		}
		got := b.Against(p)
		if got.Comparable {
			t.Errorf("%s: compared anyway", c.name)
			continue
		}
		if !strings.Contains(got.Lines(), c.want) {
			t.Errorf("%s: reason does not mention %q:\n%s", c.name, c.want, got.Lines())
		}
	}
}

// The rules-version guard mirrors Pool.Add, which refuses to ADD corpora scored
// under different rules because avoidable spend means a different thing under
// each. Comparing them is the same error; the codebase forbade the addition and
// permitted the comparison until a reviewer noticed.
func TestBM_RulesGuardMatchesPoolAdd(t *testing.T) {
	same := me(usd(0.40), usd(0.05)).Against(pool())
	if !same.Comparable {
		t.Fatal("matching rules versions were refused")
	}
}

// ---- the rank ----

// The bracket this replaced was min and max: two submissions' worth of evidence
// however many are in the pool, badly calibrated at small n (an ordinary reader
// falls outside one run in three at n=5) and vacuous at large n (99.6% "inside"
// at n=500). A rank uses every submission and sharpens as the pool grows.
func TestBM_RankUsesEverySubmission(t *testing.T) {
	p := pool(0.10, 0.20, 0.30, 0.40, 0.50)
	p.Submissions, p.DistinctTags = 40, 40
	cases := []struct {
		mine  float64
		below int
	}{{0.05, 0}, {0.15, 1}, {0.35, 3}, {0.99, 5}}
	for _, c := range cases {
		got := me(usd(c.mine), usd(0.20)).Against(p)
		if got.Below != c.below || got.Of != 5 {
			t.Errorf("$%.2f ranked above %d of %d, want %d of 5", c.mine, got.Below, got.Of, c.below)
		}
	}
}

func TestBM_RankIsStatedWithItsDenominator(t *testing.T) {
	// A rank without its n is unreadable, and "above 3" says nothing.
	line := me(usd(0.35), usd(0.20)).Against(pool(0.10, 0.20, 0.30, 0.40, 0.50)).Lines()
	if !strings.Contains(line, "above 3 of the 5 submitted medians") {
		t.Errorf("the rank is not stated with its denominator:\n%s", line)
	}
}

func TestBM_RankIsDecidedOnThePrintedFigures(t *testing.T) {
	// "$0.80 costs more than $0.79" was printed for a difference that rounds
	// away on screen. A verdict computed on more precision than it prints
	// contradicts the numbers beside it.
	p := pool(0.7892915)
	p.Submissions, p.DistinctTags = 40, 40
	got := me(usd(0.7893001), usd(0.20)).Against(p)
	if got.Below != 0 {
		t.Errorf("a difference invisible at printed precision was ranked as above: below=%d", got.Below)
	}
}

func TestBM_AReaderWithTooFewTasksIsNotRanked(t *testing.T) {
	// The median of three rows is not a median anybody should be placed by.
	b := me(usd(0.40), usd(0.05))
	b.Tasks = BenchmarkMinTasks - 1
	got := b.Against(pool())
	if got.MedianPlace != MedianUnknown {
		t.Errorf("a reader with %d task(s) was ranked", b.Tasks)
	}
	if !strings.Contains(got.Lines(), "is the floor") {
		t.Errorf("the reason is not given:\n%s", got.Lines())
	}
}

// ---- absence is not zero, on both figures ----

func TestBM_AbsentMedianAndAbsentShareAreNotZero(t *testing.T) {
	// The share was a bare float64 and a reader with nothing priced arrived
	// with 0.0, took the "your share is lower" branch, and was congratulated on
	// evidence that did not exist.
	got := Benchmark{MedianTaskUSD: nil, AvoidableShare: nil, Tasks: 500, Unit: PoolUnit}.Against(pool())
	if got.MedianPlace != MedianUnknown || got.SharePlace != ShareUnknown {
		t.Fatalf("absence was classified: median=%q share=%q", got.MedianPlace, got.SharePlace)
	}
	line := got.Lines()
	if strings.Contains(line, "$0.00") || strings.Contains(line, "0.0%") {
		t.Errorf("absence was printed as zero:\n%s", line)
	}
	if !strings.Contains(line, "NOT MEASURED") {
		t.Errorf("the share section vanished instead of saying it was not measured:\n%s", line)
	}
}

func TestBM_AMeasuredZeroMedianIsNotAbsence(t *testing.T) {
	// The mirror defect, at the call site: `s.MedianUSD > 0` turned a measured
	// $0.00 median into "nothing here was priced", which is false.
	got := me(usd(0), usd(0.20)).Against(pool())
	if strings.Contains(got.Lines(), "nothing here was priced") {
		t.Errorf("a measured zero median was reported as absence:\n%s", got.Lines())
	}
}

// ---- the share band ----

func TestBM_ShareBandIsSymmetric(t *testing.T) {
	// 0.18 against 0.20 was "below" and 0.22 against 0.20 was "near", purely
	// from float representation error. Two readers equidistant from the pool
	// were given opposite verdicts.
	p := pool()
	for _, e := range []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1} {
		lo := me(usd(0.40), usd(p.AvoidableShare-e)).Against(p).SharePlace
		hi := me(usd(0.40), usd(p.AvoidableShare+e)).Against(p).SharePlace
		mirror := map[string]string{ShareBelow: ShareAbove, ShareAbove: ShareBelow, ShareNear: ShareNear}
		if mirror[lo] != hi {
			t.Errorf("±%.3f around %.3f gave %q and %q, which are not mirrors", e, p.AvoidableShare, lo, hi)
		}
	}
}

func TestBM_ShareBandIsRelativeNotAbsolute(t *testing.T) {
	// An absolute two-point band against the shipped pooled share of 2.79%
	// called a reader at 4.75%, 1.7x the pool, "the same figure". That is the
	// case the tool exists to find, suppressed by its own band.
	small := pool()
	small.AvoidableShare = 0.0279
	if got := me(usd(0.40), usd(0.0475)).Against(small).SharePlace; got != ShareAbove {
		t.Errorf("1.7x the pooled share reported as %q, want %q", got, ShareAbove)
	}
	// And the other end: a hair-trigger absolute band fires on noise at 40%.
	big := pool()
	big.AvoidableShare = 0.40
	if got := me(usd(0.40), usd(0.425)).Against(big).SharePlace; got != ShareNear {
		t.Errorf("a 6%% relative difference at a pooled 40%% reported as %q, want %q", got, ShareNear)
	}
}

func TestBM_EveryShareBranchIsReachable(t *testing.T) {
	// ADR-0014: a branch that cannot fire is not evidence. Every one of these
	// was unexecuted by the original tests.
	p := pool()
	seen := map[string]bool{}
	for _, share := range []float64{0.0, 0.19, 0.20, 0.21, 0.60} {
		seen[me(usd(0.40), usd(share)).Against(p).SharePlace] = true
	}
	for _, want := range []string{ShareBelow, ShareNear, ShareAbove} {
		if !seen[want] {
			t.Errorf("%q is unreachable", want)
		}
	}
}

// ---- what it may not say ----

func TestBM_NeverClaimsAPooledMedianOrACause(t *testing.T) {
	// Every phrase here was in the shipped draft and was removed on review.
	// "your caches are holding better" was a causal claim the data cannot
	// support; "typical of the corpora" claimed a central tendency from
	// sum/sum; "worth a look" manufactured a concern; "your tasks are cheaper"
	// generalised one order statistic to a whole corpus.
	banned := []string{
		"the pooled median", "the median is", "pool median",
		"caches are holding", "your tasks are cheaper", "your tasks cost more",
		"worth a look", "typical of the corpora", "the same figure for any purpose",
		"\u2014", "\u2013",
	}
	p := pool()
	for _, share := range []float64{0.0, 0.20, 0.60} {
		for _, median := range []*float64{usd(0.05), usd(0.40), usd(9.99), nil} {
			line := strings.ToLower(me(median, usd(share)).Against(p).Lines())
			for _, bad := range banned {
				if strings.Contains(line, strings.ToLower(bad)) {
					t.Errorf("rendered text contains %q:\n%s", bad, line)
				}
			}
		}
	}
}

func TestBM_AlwaysCarriesProvenanceAndTheDollarCaveat(t *testing.T) {
	line := me(usd(0.40), usd(0.20)).Against(pool()).Lines()
	for _, want := range []string{
		"40 submission(s)", "40 machine tag(s)", "never people",
		"Pooled " + PooledAt, "anthropic-2026-09-01",
		"2026-09-01 to 2026-09-07", "not priced on the same day",
		"list price", "subscription seat",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("the output is missing %q:\n%s", want, line)
		}
	}
}

// ---- non-finite and out-of-domain ----

func TestBM_NonFiniteNeverReachesTheReader(t *testing.T) {
	// NaN satisfied neither comparison and inherited "inside" from a default
	// clause, then printed "$NaN" and "which is the same figure".
	p := pool()
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -5, 1e300} {
		line := me(usd(bad), usd(bad)).Against(p).Lines()
		for _, tell := range []string{"NaN", "Inf", "$-", "-0.0%"} {
			if strings.Contains(line, tell) {
				t.Errorf("input %v rendered %q to the reader:\n%s", bad, tell, line)
			}
		}
		if len(line) > 1500 {
			t.Errorf("input %v rendered %d characters, which destroys the layout", bad, len(line))
		}
	}
}

// ---- staleness ----

func TestBM_TheVendoredPoolHasAClockAndItFires(t *testing.T) {
	// The price table has two clocks, a 60 day window and a printed warning.
	// The pool had none, while pooled.go claimed the release checklist checked
	// it and RELEASE-CRITERIA.md does not mention the pool at all.
	at, err := time.Parse("2006-01-02", PooledAt)
	if err != nil {
		t.Fatalf("PooledAt %q is not a date", PooledAt)
	}
	if note := AgeNote(at.AddDate(0, 0, PooledStaleDays)); note != "" {
		t.Errorf("warned inside the window: %q", note)
	}
	note := AgeNote(at.AddDate(0, 0, PooledStaleDays+1))
	if note == "" {
		t.Fatal("a pool past its window produced no warning")
	}
	if !strings.Contains(note, "not refreshed") {
		t.Errorf("the warning does not say what stale means here: %q", note)
	}
}
