package observation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Pooling: a figure that can be walked back to its parts.
//
// "$847,000 of agent spend audited" is unfalsifiable — there is no operation a
// reader can perform that would show it to be wrong. "$847,000 across 41
// contributed corpora, each one downloadable" is a different kind of sentence:
// it names an n, it names 41 files, and anyone who disagrees can add them up.
// This file exists so that the second sentence is the only one this repository
// is able to produce.
//
// Four design choices carry that, and each is a refusal:
//
//   - The totals are METHODS OVER THE ROSTER, never fields. There is no way to
//     construct a Pool that states a number its listed submissions do not sum
//     to, because there is nowhere to put one. A field would have been one
//     keystroke and one merge away from a figure that had drifted from its
//     evidence.
//   - Every entry carries the digest of the file it came from, so a roster row
//     names something a reader can fetch and re-hash. Add re-derives the digest
//     and refuses a submission whose content no longer matches its own name.
//   - There is NO pooled median, and MedianSpan says why.
//   - An empty pool does not render and does not serialise. Nothing measured is
//     not a pass, here as everywhere else in this repository.
//
// This package still has no transport, and its import allowlist keeps it that
// way. Pooling happens where the submissions were collected — a pull request, a
// checked-in file — never over a wire. That is why this file writes to a string
// rather than an io.Writer: nothing here needs a stream, and not importing one
// is one less thing for the allowlist to have to forbid.

// PoolSchema versions the pooled document independently of the submissions in
// it, because a change to how pooling works is not a change to what was
// contributed.
const PoolSchema = "replay.pool.v1"

// PoolEntry is one submission's row in the roster.
//
// It is a flattened Corpus rather than an embedded one, so that the roster
// reads as a table — the form in which a reader can actually check it — and so
// that a field added to Corpus later does not silently widen what a published
// pool discloses.
type PoolEntry struct {
	SourceTag string `json:"sourceTag"`
	// TagBasis travels because a tag derived from a machine-local secret is not
	// an identity. See DistinctTags for what that costs the count.
	TagBasis string `json:"tagBasis"`
	Digest   string `json:"digest"`
	// File is the submission's filename, so a roster row names something a
	// reader can go and fetch rather than a number they must take on trust.
	File    string `json:"file"`
	TakenAt string `json:"takenAt"`

	Tasks          int     `json:"tasks"`
	TotalUSD       float64 `json:"totalUsd"`
	AvoidableUSD   float64 `json:"avoidableUsd"`
	AvoidableShare float64 `json:"avoidableShare"`
	MedianTaskUSD  float64 `json:"medianTaskUsd"`

	PricedAt     string `json:"pricedAt"`
	RulesVersion string `json:"rulesVersion"`
	Unpriced     int    `json:"unpriced"`

	// CacheBreaks and ReReads are optional in a submission, so they are
	// pointers here for the reason ADR-0018 gives: absence, zero and unknown
	// are three values. A submission from a build that did not report breaks
	// is not a submission that observed none, and flattening the two would
	// make a pool of quiet contributors look like a pool of clean ones.
	CacheBreaks *int `json:"cacheBreaks,omitempty"`
	ReReads     *int `json:"reReads,omitempty"`
}

// Pool is a roster of submissions and nothing else.
//
// Note what is absent: no Tasks, no TotalUSD, no n. Every figure a reader is
// shown is computed from Roster at the moment it is shown, so the claim and its
// evidence cannot come apart.
type Pool struct {
	Schema   string      `json:"schema"`
	PooledAt string      `json:"pooledAt"`
	Roster   []PoolEntry `json:"roster"`
	// Superseded holds submissions displaced by a later one from the same
	// machine. They are kept and published, and they are not summed.
	//
	// Keeping them is the difference between a total that dropped something
	// and a total that says what it dropped. A reader who fetches every file
	// named in this document and adds them up will get a bigger number than
	// the pooled total, and this list is the explanation waiting for them.
	Superseded []PoolEntry `json:"superseded,omitempty"`
}

// NewPool starts an empty pool.
//
// pooledAt is passed in rather than read from the clock, because a document
// that reports a different timestamp each time it is regenerated cannot be
// diffed against the one that was published.
func NewPool(pooledAt string) *Pool {
	return &Pool{Schema: PoolSchema, PooledAt: pooledAt}
}

// Add admits one submission, or explains why it cannot be pooled.
//
// Every refusal here is a case where adding the submission would produce a
// number that looks fine and means nothing.
func (p *Pool) Add(c Corpus, file string) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("this submission cannot be pooled: %w", err)
	}
	// The digest is recomputed, not trusted. A roster row's whole function is
	// to let a reader fetch a file and check it against the pool; admitting a
	// submission whose content does not match its own digest would publish a
	// row that fails the check the row exists to invite.
	if want := c.Digested().Digest; want != c.Digest {
		return fmt.Errorf("the submission's content does not match its digest (claims %s, "+
			"content hashes to %s); it was changed after it was written, so this roster row "+
			"would not survive the check it exists to invite", shortDigest(c.Digest), shortDigest(want))
	}
	for _, e := range p.Roster {
		if e.Digest == c.Digest {
			return fmt.Errorf("digest %s is already in this pool; counting one submission "+
				"twice raises the total and the n together, so the pooled share survives "+
				"unchanged and nothing looks wrong", shortDigest(c.Digest))
		}
	}
	// The caching rules decide what counts as avoidable. Two corpora scored
	// under different rules produce avoidable totals that are not the same
	// quantity, and adding them yields a number that is of nothing. The price
	// table date is handled differently, and PricedAtLow says why.
	if len(p.Roster) > 0 && p.Roster[0].RulesVersion != c.RulesVersion {
		return fmt.Errorf("this pool is scored under %s and the submission under %s; "+
			"avoidable spend means a different thing under each, so these totals are not "+
			"addable. Rescore one corpus, or pool them separately",
			p.Roster[0].RulesVersion, c.RulesVersion)
	}
	if file == "" {
		file = corpusFileName(c)
	}
	// A second submission from the same machine SUPERSEDES the first; it is
	// never added to it.
	//
	// This is the defect that would have silently inflated every pooled figure
	// this project published. `replay cost` reads the whole transcript root on
	// every run, so a corpus taken on Tuesday CONTAINS the tasks that were in
	// Monday's. The two are nested, not disjoint. Summing them would count
	// almost all of that machine's money twice, and nothing about the result
	// would look wrong: the total rises, the task count rises with it, and the
	// avoidable share — the figure a reader actually checks — barely moves,
	// because both numerator and denominator were inflated together.
	//
	// The duplicate-digest check above does not catch this. Two runs on
	// different days produce different figures, so different digests, so two
	// files that are honestly distinct submissions of overlapping corpora.
	// Identity of content was never the right test; identity of SOURCE is.
	//
	// TakenAt decides which one wins, and it is compared lexicographically,
	// which is what RFC 3339 is for.
	incoming := PoolEntry{
		SourceTag: c.SourceTag, TagBasis: c.TagBasis, Digest: c.Digest, File: file,
		TakenAt: c.TakenAt, Tasks: c.Tasks, TotalUSD: c.TotalUSD,
		AvoidableUSD: c.AvoidableUSD, AvoidableShare: c.AvoidableShare,
		MedianTaskUSD: c.MedianTaskUSD, PricedAt: c.PricedAt,
		RulesVersion: c.RulesVersion, Unpriced: c.Unpriced,
		CacheBreaks: c.CacheBreaks, ReReads: c.ReReads,
	}
	for i, e := range p.Roster {
		if e.SourceTag != c.SourceTag {
			continue
		}
		switch {
		case e.TakenAt == incoming.TakenAt:
			// Same machine, same hour, different content. Nothing in either
			// file says which measurement came second, so there is no
			// defensible way to pick one, and picking arbitrarily would make
			// the pooled total depend on directory order.
			return fmt.Errorf("tag %s already has a submission taken at %s and this one "+
				"carries the same timestamp with different figures; these are overlapping "+
				"corpora and nothing here says which is later. Keep one and remove the other",
				shortDigest(c.SourceTag), e.TakenAt)
		case e.TakenAt > incoming.TakenAt:
			// The one already in the roster is newer, so the arriving
			// submission is the stale one.
			p.Superseded = append(p.Superseded, incoming)
			return nil
		default:
			p.Superseded = append(p.Superseded, e)
			p.Roster[i] = incoming
			return nil
		}
	}
	p.Roster = append(p.Roster, incoming)
	return nil
}

// PoolTotals is what the roster adds up to.
//
// Every field here is derived. It is returned by value from Totals rather than
// stored on the Pool, so there is no cached copy to go stale against the roster
// it came from.
type PoolTotals struct {
	// Submissions is len(Roster). It is the n that belongs beside every figure
	// below, and it is deliberately not called Contributors.
	Submissions int `json:"submissions"`
	// DistinctTags is how many distinct source tags those submissions carry.
	//
	// It is NOT a count of people, and TagsAreIdentities says so. A local tag
	// is derived from a secret the machine minted, so one person with three
	// machines is three tags, and anyone can mint unlimited ones. It is here
	// because it is strictly more informative than Submissions alone — it says
	// whether a pool is 41 machines or one machine 41 times — and for nothing
	// else.
	DistinctTags int `json:"distinctTags"`
	// TagsAreIdentities is false whenever any submission's tag came from a
	// machine-local secret, which is the only basis the CLI currently emits.
	//
	// It travels so a consumer of this JSON cannot read DistinctTags as a
	// population without also reading the field that says it is not one.
	TagsAreIdentities bool `json:"tagsAreIdentities"`
	// SupersededSubmissions is how many contributed files are named in this
	// document but not counted in it, because a later submission from the same
	// machine replaced them.
	//
	// It is published because the alternative is a reader downloading every
	// file the pool names, adding them up, getting a larger number, and having
	// no way to tell whether they found an error or a design decision.
	SupersededSubmissions int `json:"supersededSubmissions"`

	Tasks        int     `json:"tasks"`
	TotalUSD     float64 `json:"totalUsd"`
	AvoidableUSD float64 `json:"avoidableUsd"`
	// AvoidableShare is sum(avoidable) / sum(total): a share of the pooled
	// money.
	//
	// It is NOT the mean of the submissions' own shares. That would weight a
	// three-task corpus like a thousand-task one, so a handful of tiny
	// contributions could move a population figure further than all the real
	// spend in the pool. The two differ whenever the corpora differ in size,
	// which is always.
	AvoidableShare float64 `json:"avoidableShare"`
	// Unpriced is how many transcripts the pool's members read and left out.
	// It is the pooled figure's own footnote: what this total does not cover.
	Unpriced int `json:"unpriced"`

	// MedianTaskUSDLow and MedianTaskUSDHigh bracket the submitted medians.
	// There is no pooled median — see MedianSpan.
	MedianTaskUSDLow  float64 `json:"medianTaskUsdLow"`
	MedianTaskUSDHigh float64 `json:"medianTaskUsdHigh"`
	// MedianTaskUSDs is every submission's own median, sorted ascending.
	//
	// The bracket above is a min and a max, which is two submissions' worth of
	// evidence however many are in the pool. It also never converges: as the
	// pool grows the bracket widens until "inside it" means nothing, and at
	// small n it is badly calibrated in the other direction. The min and max of
	// n exchangeable draws contain an n+1th with probability (n-1)/(n+1), so at
	// five submissions one ordinary reader in three falls outside by luck.
	//
	// Publishing the sorted vector costs one number per submission and lets a
	// consumer state a rank: "above 31 of the 41 submitted medians". That uses
	// every submission rather than two, it sharpens as the pool grows instead of
	// decaying, and it still names no central figure, so the refusal to compute
	// a median of medians stands.
	//
	// It is the submitted medians themselves, not task costs. Publishing task
	// costs would be publishing the corpus.
	MedianTaskUSDs []float64 `json:"medianTaskUsds"`

	// RulesVersion is shared by every member; Add refuses a pool where it is
	// not.
	RulesVersion string `json:"rulesVersion"`
	// PricedAtLow, PricedAtHigh and PricedAtDistinct describe the price-table
	// dates in the pool.
	//
	// Unlike RulesVersion this is disclosed rather than refused. These totals
	// are dollars, and the table this dates is the currency reference used for
	// DISPLAY conversion, so a spread of dates does not make the USD figures
	// unaddable. It is reported because a reader deciding how much to trust a
	// pooled figure should be able to see how far apart its members were taken.
	//
	// The bounds are lexicographic, which is what YYYY-MM-DD is for.
	// CacheBreaks and ReReads are sums over the submissions that REPORTED
	// them, which is not necessarily every submission in the pool.
	//
	// Both fields are optional in a submission, so their population is its own
	// and BreaksTasks carries the denominator that goes with it. Dividing
	// CacheBreaks by Tasks would divide a partial numerator by a whole
	// denominator, and would understate breaks per session by exactly the
	// share of the pool that stayed quiet. Nothing here does that division;
	// the fields travel so a reader can do it correctly or not at all.
	//
	// Nil when no submission reported, rather than zero. A pool nobody
	// reported breaks for has not observed zero breaks.
	CacheBreaks *int `json:"cacheBreaks,omitempty"`
	ReReads     *int `json:"reReads,omitempty"`
	// BreaksReportedBy is how many submissions carried those counts, and
	// BreaksTasks is how many tasks those particular submissions covered.
	BreaksReportedBy int `json:"breaksReportedBy"`
	BreaksTasks      int `json:"breaksTasks"`

	PricedAtLow      string `json:"pricedAtLow"`
	PricedAtHigh     string `json:"pricedAtHigh"`
	PricedAtDistinct int    `json:"pricedAtDistinct"`
}

// Totals reduces the roster, or refuses.
//
// An empty pool has no figure. Returning zeroes would hand a caller a $0.00
// total across 0 submissions that renders exactly like a real result, which is
// this repository's oldest defect: nothing measured, presented as something
// measured.
func (p Pool) Totals() (PoolTotals, error) {
	if len(p.Roster) == 0 {
		return PoolTotals{}, fmt.Errorf("NOT MEASURED: this pool has no submissions, so there " +
			"is no pooled figure to state; a zero here would be a total nobody contributed to")
	}
	first := p.Roster[0]
	t := PoolTotals{
		Submissions:           len(p.Roster),
		SupersededSubmissions: len(p.Superseded),
		TagsAreIdentities:     true,
		RulesVersion:          first.RulesVersion,
		MedianTaskUSDLow:      first.MedianTaskUSD,
		MedianTaskUSDHigh:     first.MedianTaskUSD,
		PricedAtLow:           first.PricedAt,
		PricedAtHigh:          first.PricedAt,
	}
	tags := make(map[string]bool, len(p.Roster))
	dates := make(map[string]bool, len(p.Roster))
	for _, e := range p.Roster {
		tags[e.SourceTag] = true
		dates[e.PricedAt] = true
		if e.TagBasis != BasisAccount {
			t.TagsAreIdentities = false
		}
		t.Tasks += e.Tasks
		t.TotalUSD += e.TotalUSD
		t.AvoidableUSD += e.AvoidableUSD
		t.Unpriced += e.Unpriced
		// Counted only where reported, with the tasks that came with it.
		// An entry reporting one of the pair and not the other still counts
		// its tasks once: the denominator is the submission, not the field.
		if e.CacheBreaks != nil || e.ReReads != nil {
			t.BreaksReportedBy++
			t.BreaksTasks += e.Tasks
		}
		if e.CacheBreaks != nil {
			n := e.CacheBreaks
			if t.CacheBreaks == nil {
				zero := 0
				t.CacheBreaks = &zero
			}
			*t.CacheBreaks += *n
		}
		if e.ReReads != nil {
			n := e.ReReads
			if t.ReReads == nil {
				zero := 0
				t.ReReads = &zero
			}
			*t.ReReads += *n
		}
		if e.MedianTaskUSD < t.MedianTaskUSDLow {
			t.MedianTaskUSDLow = e.MedianTaskUSD
		}
		if e.MedianTaskUSD > t.MedianTaskUSDHigh {
			t.MedianTaskUSDHigh = e.MedianTaskUSD
		}
		if e.PricedAt < t.PricedAtLow {
			t.PricedAtLow = e.PricedAt
		}
		if e.PricedAt > t.PricedAtHigh {
			t.PricedAtHigh = e.PricedAt
		}
	}
	// Every submission's own median, sorted, so a consumer can state a rank
	// instead of a bracket. See the field comment for why the bracket alone is
	// not enough.
	t.MedianTaskUSDs = make([]float64, 0, len(p.Roster))
	for _, e := range p.Roster {
		t.MedianTaskUSDs = append(t.MedianTaskUSDs, e.MedianTaskUSD)
	}
	sort.Float64s(t.MedianTaskUSDs)

	t.DistinctTags = len(tags)
	t.PricedAtDistinct = len(dates)
	if t.TotalUSD > 0 {
		t.AvoidableShare = t.AvoidableUSD / t.TotalUSD
	}
	return t, nil
}

// MedianSpan is why this type has no pooled median.
//
// A median of medians is not a median. Given corpora of 3 and 3,000 tasks, the
// midpoint of their two medians describes no distribution that exists. The
// statistic that would be correct needs every task's cost, and a submission
// carries five figures rather than a per-task table precisely because the
// per-task table is the disclosure the contributor declined to make.
//
// So the honest thing to publish is the range the submitted medians fall in,
// stated as a range of medians and not as a median.
func MedianSpan(t PoolTotals) string {
	return fmt.Sprintf("median task $%.2f-$%.2f (a RANGE of the submitted medians, not a "+
		"pooled median: a median of medians is not a median, and computing one would need "+
		"per-task costs that no submission carries)", t.MedianTaskUSDLow, t.MedianTaskUSDHigh)
}

// TagNote states what DistinctTags is, and what it is not.
func TagNote(t PoolTotals) string {
	if t.TagsAreIdentities {
		return fmt.Sprintf("%d submissions from %d distinct provider accounts",
			t.Submissions, t.DistinctTags)
	}
	return fmt.Sprintf("%d submissions carrying %d distinct tags. A tag is derived from a "+
		"secret the contributing machine generated, so this is a count of MACHINES AT MOST "+
		"and not of people: one person with several machines contributes several tags, and "+
		"anyone can mint unlimited ones", t.Submissions, t.DistinctTags)
}

// poolDoc is what a Pool serialises to: the roster it holds, plus totals
// computed at the moment of marshalling.
//
// The indirection is the guarantee. A stored totals field could be written once
// and then disagree with a roster edited afterwards; a derived one cannot,
// because there is no moment at which the two exist separately.
type poolDoc struct {
	Schema      string      `json:"schema"`
	PooledAt    string      `json:"pooledAt"`
	Totals      PoolTotals  `json:"totals"`
	MedianNote  string      `json:"medianNote"`
	TagNote     string      `json:"tagNote"`
	RosterCount int         `json:"rosterCount"`
	Roster      []PoolEntry `json:"roster"`
	// Superseded is named and not counted. See Pool.Superseded.
	Superseded     []PoolEntry `json:"superseded,omitempty"`
	SupersededNote string      `json:"supersededNote,omitempty"`
}

// MarshalJSON emits the roster with its derived totals, or fails.
//
// An empty pool does not serialise. Same rule as Totals: there is no such
// document as a pooled figure over nothing, so there is no way to write one to
// a file and mail it to somebody.
func (p Pool) MarshalJSON() ([]byte, error) {
	t, err := p.Totals()
	if err != nil {
		return nil, err
	}
	return json.Marshal(poolDoc{
		Schema:   PoolSchema,
		PooledAt: p.PooledAt,
		Totals:   t,
		// The two caveats are fields rather than prose in a README, because a
		// consumer parses this document and never reads the README. ADR-0018:
		// provenance is a field, not a comment.
		MedianNote:     MedianSpan(t),
		TagNote:        TagNote(t),
		RosterCount:    len(p.Roster),
		Roster:         p.Roster,
		Superseded:     p.Superseded,
		SupersededNote: SupersededNote(t),
	})
}

// SupersededNote explains why the files in Superseded are named but not summed.
//
// Empty when there are none, so the note only appears where it is load-bearing.
func SupersededNote(t PoolTotals) string {
	if t.SupersededSubmissions == 0 {
		return ""
	}
	return fmt.Sprintf("%d further submission(s) are named below and NOT counted. Each was "+
		"replaced by a later corpus from the same machine. A corpus is cumulative, because the "+
		"tool reads the whole transcript root on every run, so a machine's later submission "+
		"contains the tasks in its earlier one. Adding both would count that spend twice "+
		"while raising the task count with it, which leaves the avoidable share looking "+
		"untouched", t.SupersededSubmissions)
}

// Render returns the roster and the figure it adds up to, in that order.
//
// The order is the argument. A reader who stops after the first screen has seen
// the submissions; a reader who stops before it has seen nothing to stop at.
func (p Pool) Render() (string, error) {
	t, err := p.Totals()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Pooled corpora, scored under %s\n\n", t.RulesVersion)
	fmt.Fprintf(&b, "  %-14s %-14s %7s %12s %12s %8s  %s\n",
		"tag", "digest", "tasks", "total", "avoidable", "share", "file")
	for _, e := range p.Roster {
		fmt.Fprintf(&b, "  %-14s %-14s %7d %12s %12s %7.1f%%  %s\n",
			shortDigest(e.SourceTag), shortDigest(e.Digest), e.Tasks,
			fmt.Sprintf("$%.2f", e.TotalUSD), fmt.Sprintf("$%.2f", e.AvoidableUSD),
			e.AvoidableShare*100, e.File)
	}
	fmt.Fprintf(&b, "\n  $%.2f across %d contributed corpora, %d tasks.\n",
		t.TotalUSD, t.Submissions, t.Tasks)
	fmt.Fprintf(&b, "  $%.2f of it avoidable, %.1f%% of the pooled total.\n",
		t.AvoidableUSD, t.AvoidableShare*100)
	b.WriteString("  That share is the pooled money's own, not the mean of the submissions'\n" +
		"  shares, which would weight a three-task corpus like a thousand-task one.\n")
	fmt.Fprintf(&b, "  %s\n", MedianSpan(t))
	fmt.Fprintf(&b, "  %s\n", TagNote(t))
	if t.Unpriced > 0 {
		fmt.Fprintf(&b, "  %d further transcripts were read by these contributors and left out,\n"+
			"  because their model is not in the price table. Excluded, not counted as free.\n", t.Unpriced)
	}
	if t.PricedAtDistinct > 1 {
		fmt.Fprintf(&b, "  Price tables span %s to %s across %d distinct dates.\n",
			t.PricedAtLow, t.PricedAtHigh, t.PricedAtDistinct)
	}
	if note := SupersededNote(t); note != "" {
		fmt.Fprintf(&b, "\n  %s:\n", note)
		for _, e := range p.Superseded {
			fmt.Fprintf(&b, "    %-14s %-14s %7d %12s  superseded  %s\n",
				shortDigest(e.SourceTag), shortDigest(e.Digest), e.Tasks,
				fmt.Sprintf("$%.2f", e.TotalUSD), e.File)
		}
	}
	b.WriteString("  Every row above names a file you can fetch and re-hash.\n")
	return b.String(), nil
}

// shortDigest truncates a digest or tag for a table cell. Twelve hex characters
// tell submissions apart in a roster and stay readable; the full value is in the
// JSON and in the filename.
func shortDigest(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
