package observation

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Placing a reader against the pool.
//
// WHY THIS EXISTS.
//
// Measured across 1,506 transcripts, 56% of sessions have no avoidable spend at
// all. Those readers run the tool, get an honest null, and walk away with
// nothing to act on. Placing them against the contributed corpora is a finding
// for everyone rather than for the 44% whose caches happen to be broken.
//
// WHAT IT IS NOT ALLOWED TO SAY, AND WHAT IT SAID BEFORE REVIEW.
//
// There is no pooled median. PoolTotals refuses to compute one, because a
// median of medians is not a median. This file places a reader inside the span
// the submitted medians occupy and never names a centre.
//
// A four-reviewer QA pass found the first version of this file claiming a great
// deal more than that, and every sentence below is the corrected one:
//
//	"your tasks are cheaper than any corpus in the pool" generalised one order
//	statistic to a whole corpus. A median says nothing about the tail above it.
//
//	"Your caches are holding better than the pool's" was a causal claim the
//	data cannot support. AvoidableShare falls when caches hold, and equally
//	when there were few cache writes to break, or when caching is off
//	entirely. A reader with caching disabled has a share of zero and was being
//	congratulated on it.
//
//	"more than is typical of the corpora here" claimed a central tendency from
//	sum(avoidable)/sum(total), which is an aggregate over pooled dollars that
//	one large submission can set. That is the median-of-medians fallacy
//	committed in the opposite direction, at the only surface a reader reads.
//
//	"That is worth a look" manufactured a concern the data does not contain,
//	and only the expensive branch had one, which encoded "cheap is better".
//
// WHEN IT REFUSES.
//
// Below BenchmarkMinTags the pool is not a population. It also refuses on any
// pool it cannot trust: impossible counts, an inverted span, non-finite
// figures, a share outside [0,1], or a rules version that differs from the
// reader's. Pool.Add already refuses to ADD corpora scored under different
// rules, because avoidable spend means a different thing under each; comparing
// them is the same error and is refused here for the same reason.
const BenchmarkMinTags = 20

// BenchmarkMinTasks is the reader's own floor. A median over three rows is not
// a median anybody should be placed by.
const BenchmarkMinTasks = 20

// PoolUnit is what every contributed median is a median OF.
//
// "session", which is the default unit `replay cost` reports in and therefore
// the only unit any contributed corpus has ever carried. cost.go PRINTS a
// session row as a "task", which is why the rendered sentences say task; the
// constant names the internal unit so the comparison is made on the right one.
//
// Corpus has no unit field, so a per-lane corpus contributed by a future build
// would be indistinguishable in the pool. That is a real gap and it is the
// reason this guard sits on the reader's side, where the unit IS known.
const PoolUnit = "session"

// Whether the reader's median could be ranked at all.
const (
	MedianRanked  = "ranked"
	MedianUnknown = "unknown"
)

// Where a reader's avoidable share sits against the pooled share.
const (
	ShareBelow   = "below"
	ShareNear    = "near"
	ShareAbove   = "above"
	ShareUnknown = "unknown"
)

// shareNearBand is how close counts as not worth reporting as a difference.
//
// Half a percentage point, not the two points this started at. Against the
// pooled share in the shipped snapshot, 2.79%, a two point band covered nearly
// every real reader and left the other two branches almost unreachable, which
// ADR-0014 calls what it is: a branch that cannot fire is not evidence.
const shareNearBand = 0.005

// shareNearRatio is the other half of the band. A reader reasons in ratios, and
// the quantity spans two orders of magnitude across the plausible range: a flat
// band is hair-trigger against a pooled 40% and swallows a 3x difference
// against a pooled 2.8%. Within a quarter either way is not a difference to act
// on; outside it is. The absolute floor above stops 0.3% against 0.6% firing.
const shareNearRatio = 0.25

// displayCent is one printed cent. Placement is decided on the rounded figures
// that actually appear in the output, because a verdict computed on more
// precision than it prints produces "$0.79 costs more than $0.79", which a
// reviewer found and a reader would screenshot.
const displayCent = 0.005

// maxPlausibleMedianUSD fences a median that cannot be a task cost. Without it
// math.MaxFloat64 renders a 309 digit number inside a wrapped sentence.
const maxPlausibleMedianUSD = 1e6

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// roundCents rounds to the precision the output prints.
func roundCents(v float64) float64 { return math.Round(v*100) / 100 }

// roundShare rounds to the 0.1 percentage point the output prints, so two
// readers equidistant from the pool get mirror verdicts. Comparing the raw
// difference made 0.18 against 0.20 "below" and 0.22 against 0.20 "near",
// purely from float representation error.
func roundShare(v float64) float64 { return math.Round(v*1000) / 1000 }

// Benchmark is one reader's own figures, as `replay cost` computed them.
type Benchmark struct {
	// MedianTaskUSD is nil when there is no median, never 0.
	//
	// A pointer rather than a float plus a bool: this repository's own idiom for
	// absence (see CacheBreaks and ReReads on PoolEntry) and ADR-0018, absence,
	// zero and unknown are three values. A measured median of zero is zero and
	// must not be collapsed into absence, which is what the first call site did.
	MedianTaskUSD *float64

	// AvoidableShare is nil when nothing was priced, never 0.
	//
	// The same ADR-0018 defect the median already avoids. A reader with no
	// priced spend has no share; leaving it a bare float meant 0.0 arrived
	// looking like a perfect score, took the "your share is lower" branch, and
	// congratulated somebody on evidence that did not exist.
	AvoidableShare *float64

	// RulesVersion is what the reader's figures were scored under.
	RulesVersion string

	// Tasks is how many rows the reader's own median was taken over, and Unit
	// says what those rows are.
	//
	// Unit matters because `--per-lane` makes MedianUSD a median over lanes
	// rather than tasks, and placing a per-lane median among per-task medians
	// compares two different quantities. costSummary carries Unit for exactly
	// this reason and the first version of this code ignored it.
	Tasks int
	Unit  string
}

// Placement is the finished comparison.
type Placement struct {
	Comparable  bool
	Why         string
	MedianPlace string
	SharePlace  string

	// Below is how many submitted medians the reader's own median exceeds, out
	// of Of. A rank, not a bracket: it uses every submission rather than the two
	// at the ends, and it sharpens as the pool grows instead of decaying into
	// "inside the range", which at 500 submissions is true of 99.6% of readers.
	Below int
	Of    int

	mine Benchmark
	pool PoolTotals
}

// Against places these figures in the pool, or explains why it will not.
func (b Benchmark) Against(pool PoolTotals) Placement {
	p := Placement{mine: b, pool: pool, MedianPlace: MedianUnknown, SharePlace: ShareUnknown}

	if why := refuse(b, pool); why != "" {
		p.Why = why
		return p
	}
	p.Comparable = true

	if b.MedianTaskUSD != nil && b.Tasks >= BenchmarkMinTasks {
		m := roundCents(*b.MedianTaskUSD)
		if m > 0 && finite(m) && m <= maxPlausibleMedianUSD {
			// Ranked on the printed figures, so the verdict never contradicts
			// the numbers beside it.
			below := 0
			for _, v := range pool.MedianTaskUSDs {
				if finite(v) && roundCents(v) < m {
					below++
				}
			}
			p.Below, p.Of = below, len(pool.MedianTaskUSDs)
			if p.Of > 0 {
				p.MedianPlace = MedianRanked
			}
		}
	}

	if b.AvoidableShare != nil && finite(*b.AvoidableShare) && *b.AvoidableShare >= 0 && *b.AvoidableShare <= 1 {
		mine, pooled := roundShare(*b.AvoidableShare), roundShare(pool.AvoidableShare)
		d := mine - pooled
		// Near unless BOTH tests say otherwise: far enough in absolute terms to
		// be worth printing, and far enough in relative terms to be worth
		// acting on.
		// The epsilon is not cosmetic. Without it 0.15 against 0.20 is "below"
		// and 0.25 against 0.20 is "near", because 0.15-0.20 and 0.25-0.20 do
		// not have the same magnitude in binary. Two readers equidistant from
		// the pool got opposite verdicts, and the first attempt at this fix
		// moved the same defect from the absolute band into the ratio.
		const eps = 1e-9
		far := math.Abs(d) > shareNearBand+eps &&
			(pooled <= 0 || math.Abs(d)/pooled > shareNearRatio+eps)
		switch {
		case !far:
			p.SharePlace = ShareNear
		case d < 0:
			p.SharePlace = ShareBelow
		default:
			p.SharePlace = ShareAbove
		}
	}
	return p
}

// refuse returns the reason this comparison must not be made, or "".
func refuse(b Benchmark, pool PoolTotals) string {
	switch {
	case pool.Submissions < 1 || pool.DistinctTags < 1:
		return "No comparison: the pool is empty, so there is nothing to compare against."
	case pool.DistinctTags > pool.Submissions:
		return fmt.Sprintf("No comparison: the pool reports %d machine tag(s) across %d submission(s),\n"+
			"which cannot both be true. The pooled document is not trustworthy and nothing\n"+
			"is derived from it.", pool.DistinctTags, pool.Submissions)
	case pool.DistinctTags < BenchmarkMinTags:
		return fmt.Sprintf(
			"No comparison: the pool carries %d machine tag(s) and a comparison needs at least\n"+
				"%d. Below that a reader lands outside the pool often enough by luck that the\n"+
				"placement would be noise, and at the smallest sizes the pool is this tool's own\n"+
				"author. A tag counts machines, so %d tags can still be one person.\n"+
				"`replay cost --contribute` is how it grows.",
			pool.DistinctTags, BenchmarkMinTags, BenchmarkMinTags)
	case len(pool.MedianTaskUSDs) < 1:
		return "No comparison: this pooled document predates the per-submission medians a rank\nneeds. Re-vendor it from a pool built by a current `replay pool`."
	case !finite(pool.AvoidableShare) || pool.AvoidableShare < 0 || pool.AvoidableShare > 1:
		return "No comparison: the pooled re-billed share is not a share. The pooled document is\nnot trustworthy and nothing is derived from it."
	case b.Unit != "" && b.Unit != PoolUnit:
		return fmt.Sprintf("No comparison: your figures are per %s and every contributed median is per\n"+
			"%s. Those are different quantities. Run without --per-lane to compare.",
			b.Unit, PoolUnit)
	case b.RulesVersion != "" && pool.RulesVersion != "" && b.RulesVersion != pool.RulesVersion:
		return fmt.Sprintf("No comparison: your figures are scored under %s and the pool under %s.\n"+
			"Avoidable spend means a different thing under each, so these are not comparable.\n"+
			"`replay pool` refuses to add such corpora together for the same reason.",
			b.RulesVersion, pool.RulesVersion)
	}
	return ""
}

// Lines renders the placement for `replay cost`.
func (p Placement) Lines() string {
	if !p.Comparable {
		return p.Why
	}
	var b strings.Builder

	fmt.Fprintf(&b, "Against the contributed pool: %d submission(s) carrying %d machine tag(s),\n"+
		"%d task(s). A tag counts machines at most, never people.\n",
		p.pool.Submissions, p.pool.DistinctTags, p.pool.Tasks)
	fmt.Fprintf(&b, "Pooled %s", pooledAtOrUnknown())
	if p.pool.RulesVersion != "" {
		fmt.Fprintf(&b, ", scored under %s", p.pool.RulesVersion)
	}
	if p.pool.PricedAtLow != "" && p.pool.PricedAtHigh != "" {
		fmt.Fprintf(&b, ", priced from %s to %s across %d price table date(s)",
			p.pool.PricedAtLow, p.pool.PricedAtHigh, p.pool.PricedAtDistinct)
	} else {
		b.WriteString(", priced at a date the pool does not record")
	}
	b.WriteString(".\nThe two sides were not priced on the same day and nothing below corrects for\nthat.\n")
	if note := AgeNote(time.Now()); note != "" {
		b.WriteString(strings.TrimSpace(note) + "\n")
	}

	switch p.MedianPlace {
	case MedianRanked:
		fmt.Fprintf(&b, "\nYour median task costs $%.2f, above %d of the %d submitted medians. That is a\n"+
			"rank among submitted medians, not a statement about your tasks: a median says\n"+
			"nothing about the tail above it.\n",
			roundCents(*p.mine.MedianTaskUSD), p.Below, p.Of)
	default:
		fmt.Fprintf(&b, "\nNo median to rank: %s\n", p.medianWhy())
	}

	poolPct := roundShare(p.pool.AvoidableShare) * 100
	switch p.SharePlace {
	case ShareBelow:
		fmt.Fprintf(&b, "\nRe-billed share: %.1f%% of your priced spend against %.1f%% of the pool's. Your\n"+
			"share is lower. A lower share is consistent with caches that held and equally\n"+
			"with fewer cache writes to break. This figure does not separate the two.\n",
			roundShare(*p.mine.AvoidableShare)*100, poolPct)
	case ShareNear:
		fmt.Fprintf(&b, "\nRe-billed share: %.1f%% of your priced spend against %.1f%% of the pool's. The\n"+
			"two are within %.0f%% of each other, which is inside the band this tool declines\n"+
			"to report as a difference.\n", roundShare(*p.mine.AvoidableShare)*100, poolPct, shareNearRatio*100)
	case ShareAbove:
		fmt.Fprintf(&b, "\nRe-billed share: %.1f%% of your priced spend against %.1f%% of the pool's. Your\n"+
			"share is higher. The pooled figure is the pool's total re-billed dollars over its\n"+
			"total spend, not a typical corpus: one large submission can set it.\n",
			roundShare(*p.mine.AvoidableShare)*100, poolPct)
	default:
		// An explicit arm. ShareUnknown used to fall through and the whole
		// section vanished with no reason given, which is absence rendered as
		// silence rather than as absence.
		b.WriteString("\nNo re-billed share to compare: NOT MEASURED here, which is not the same as\nzero.\n")
	}

	// The caveat the tool prints whenever it prints a dollar. It was missing on
	// the one path built for readers with no avoidable tokens, which is exactly
	// where cost.go gates its own caveat off.
	b.WriteString("\nThese dollars are list price. On a subscription seat you are not billed per\n" +
		"token, so neither side of this comparison is money anyone paid.\n")
	return b.String()
}

// medianWhy names which precondition the median failed, because "no median"
// with no reason is the same silence this file keeps removing.
func (p Placement) medianWhy() string {
	switch {
	case p.mine.MedianTaskUSD == nil:
		return "nothing here was priced, which is absent rather than zero."
	case p.mine.Tasks < BenchmarkMinTasks:
		return fmt.Sprintf("a median over %d task(s) is not worth ranking; %d is the floor.",
			p.mine.Tasks, BenchmarkMinTasks)
	default:
		return "your median is not a figure that can be placed."
	}
}

// AgeNote warns when the vendored pool is old enough that a reader should not
// lean on it, mirroring cachemodel.PriceTableAgeNote.
//
// The vendored price table has two clocks and a 60 day window and prints a
// warning when it passes. The vendored pool had none of that, while pooled.go
// claimed "the release checklist is what checks the date" and RELEASE-CRITERIA.md
// does not mention the pool at all. A claim with nothing behind it is the defect
// this repository exists to refuse, so the clock is here and a test fails on it.
func AgeNote(now time.Time) string {
	at, err := time.Parse("2006-01-02", PooledAt)
	if err != nil {
		return " The vendored pool carries no readable date, so its age is unknown."
	}
	days := int(now.UTC().Sub(at) / (24 * time.Hour))
	if days <= PooledStaleDays {
		return ""
	}
	return fmt.Sprintf(" The pooled document is %d days old. It is produced daily, so a pool this\n"+
		"stale means the vendored copy in this build was not refreshed, not that nobody\n"+
		"contributed.", days)
}

// pooledAtOrUnknown keeps the promise pooled.go makes, that PooledAt is printed
// next to every figure derived from it. It was printed nowhere.
func pooledAtOrUnknown() string {
	if PooledAt == "" {
		return "at a date this build does not record"
	}
	return PooledAt
}
