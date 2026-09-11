package analysis

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// LaneReport is everything the commands need for one lane. Policies are
// simulated on first use, since only replay prints them.
type LaneReport struct {
	Session     *transcript.Session
	Lane        *transcript.Lane
	Calibration *Calibration
	Fit         TokenFit
	Breaks      []Break
	Blame       []BlameEntry
	Errors      []ErrorCost
	ReReads     ReReads
	// Dollars adds a list-price column to the policy table when the
	// session's model is in the price table.
	Dollars bool

	policiesOnce sync.Once
	policies     []PolicyResult
}

// Context-editing policies are scored at triggers relative to the session's
// largest prompt, so a policy is never evaluated at a threshold the session
// could not reach. These are starting points for the user, not
// recommendations.
var contextEditTriggerShares = []float64{0.5, 0.75}

// ContextEditKeepLast is the default number of recent tool results a
// clear keeps, for the simulated grid and the live policy alike.
const ContextEditKeepLast = 6

// Report layout limits.
const (
	replayBlameLimit = 5
	labelColumnWidth = 64
)

// errorBar renders a figure's uncertainty, or says there is not one.
//
// Fit assigns RelativeError = 1 when it measured no spread — no fittable turn,
// or a single one — and the old `(±%s)` printed that placeholder as a number
// indistinguishable from a real bar. On one session the top row read
// `1.50M in prompts (±1.50M)`: an uncertainty equal to the value, presented as
// a measurement, on the line a reader is most likely to act on.
//
// "not measured" is the phrase the rest of this project uses for exactly this,
// and it is deliberately not a number: there is nothing here to round.
func errorBar(f Figure) string {
	if !f.ErrorMeasured {
		return "not measured"
	}
	return formatTokens(f.Error)
}

// AnalyzeLane runs the analyses every command needs for one lane.
func AnalyzeLane(s *transcript.Session, lane *transcript.Lane) *LaneReport {
	cal := Calibrate(lane)
	fit := Fit(cal, s.Source.PrefixVisible())
	return &LaneReport{
		Session:     s,
		Lane:        lane,
		Calibration: cal,
		Fit:         fit,
		Breaks:      FindBreaks(cal, fit),
		Blame:       Blame(cal, fit),
		Errors:      ErrorCosts(cal, fit),
		ReReads:     CountReReads(cal, fit),
	}
}

// Policies scores as-run and, when calibration passes, the alternative
// layouts. The as-run entry is always first.
func (r *LaneReport) Policies() []PolicyResult {
	r.policiesOnce.Do(func() {
		r.policies = append(r.policies, AsRun(r.Lane))
		if !r.Calibration.Passes() {
			return
		}
		r.policies = append(r.policies, WithTTL(r.Calibration, cachemodel.TTLShort), WithTTL(r.Calibration, cachemodel.TTLLong))
		for _, p := range contextEditPolicies(r.Lane) {
			r.policies = append(r.policies, WithContextEdit(r.Calibration, p, r.Fit))
		}
	})
	return r.policies
}

func contextEditPolicies(lane *transcript.Lane) []ContextEditPolicy {
	largest := 0
	for _, r := range lane.Requests {
		largest = max(largest, r.Usage.PromptTotal())
	}
	var out []ContextEditPolicy
	for _, share := range contextEditTriggerShares {
		out = append(out, ContextEditPolicy{KeepLast: ContextEditKeepLast, TriggerTokens: int(float64(largest) * share)})
	}
	return out
}

// MainLane picks the lane to report on: the largest non-sidechain lane.
func MainLane(s *transcript.Session) *transcript.Lane {
	var best *transcript.Lane
	for _, l := range s.Lanes {
		if l.Sidechain {
			continue
		}
		if best == nil || len(l.Requests) > len(best.Requests) {
			best = l
		}
	}
	if best == nil && len(s.Lanes) > 0 {
		best = s.Lanes[0]
	}
	return best
}

// Printer writes report text and remembers the first write error so
// callers can return it without checking every line.
type Printer struct {
	w   io.Writer
	err error
}

// NewPrinter wraps a writer.
func NewPrinter(w io.Writer) *Printer { return &Printer{w: w} }

// Printf writes formatted text unless an earlier write failed.
func (p *Printer) Printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}

// Err is the first write error, if any.
func (p *Printer) Err() error { return p.err }

func (r *LaneReport) header(p *Printer) {
	cal := r.Calibration
	p.Printf("Session %s  client %s  model %s  requests %d\n", shortID(r.Session.ID), r.Session.ClientVersion, r.Lane.Requests[0].Model, len(r.Lane.Requests))
	p.Printf("Tier: %s\n", r.Session.Source.Tier())
	p.Printf("Calibration: reproduced provider cache reads on %d/%d turns", cal.Reproduced+cal.Exceeded, cal.Compared())
	if cal.Exceeded > 0 {
		p.Printf(" (%d read more than predicted: a sibling request extended the prefix)", cal.Exceeded)
	}
	if cal.Broken > 0 {
		p.Printf("; %d cache breaks", cal.Broken)
	}
	p.Printf("\n")
	p.Printf("Assumption: %s\n", AssumptionNote)
	prefix := "estimated"
	switch {
	case r.Session.Source.PrefixVisible():
		prefix = "recorded on the wire"
	case r.Fit.UnseenPrefix.Measured > 0:
		prefix = "measured from the first request's cache read"
	}
	// The spread, or the fact that there is not one. This line already printed
	// the turn count beside the percentage, so a reader who knew that two turns
	// are needed for a spread could work it out; nobody should have to.
	spread := fmt.Sprintf("±%.0f%%", r.Fit.RelativeError*100)
	if !r.Fit.ErrorMeasured() {
		spread = "spread not measured"
	}
	p.Printf("Rules: %s; user-content fit %.3f tokens/byte %s from %d turns; system prefix %s (%s)\n", cachemodel.RulesVersionInEffect(), r.Fit.TokensPerByte, spread, r.Fit.Turns, formatTokens(r.Fit.UnseenPrefix.Total()), prefix)
	if r.Session.Skipped > 0 {
		p.Printf("Note: %d transcript lines were not conversation content and were skipped\n", r.Session.Skipped)
	}
	r.laneScope(p)
	p.Printf("\n")
}

// laneScope says how much of the session these figures cover.
//
// Every offline command analyses the lane MainLane picks and discards the
// rest, and until 2026-09-08 none of them said so. Measured on the corpus
// this was built from: the whole set holds 817 breaks and 55,294,260
// re-billed tokens, while `replay diff` printed 758 and 33.8M. So 21.5
// million tokens, 38.9% of the total, sat in sub-agent lanes of the same
// files, and every cause share the report printed was a share of 61% of the
// tokens with nothing on the page to say so.
//
// Reporting one lane is still the right default. A sub-agent lane has its own
// prefix and its own tool set, and folding them together is what forged 31 of
// 34 events in the lane-isolation incident. What was wrong was the silence,
// which is the same thing ContextGap.LanesTotal was added to fix, in a
// different command.
//
// Silent when there is nothing to disclose. A note printed on every
// single-lane session is noise, and a disclosure readers learn to skip has
// stopped being one.
func (r *LaneReport) laneScope(p *Printer) {
	total := len(r.Session.Lanes)
	if total < 2 {
		return
	}
	omitted := 0
	for _, l := range r.Session.Lanes {
		if l != r.Lane {
			omitted += len(l.Requests)
		}
	}
	if omitted == 0 {
		return
	}
	p.Printf("Scope: 1 of %d lanes; %d requests in this session's other lanes are not counted here.\n", total, omitted)
}

// WriteReplay prints the policy table, the error section, and the top
// token sources.
func (r *LaneReport) WriteReplay(w io.Writer) error {
	p := NewPrinter(w)
	r.header(p)
	if !r.Calibration.Passes() {
		p.Printf("Calibration below %.0f%%: alternatives are not scored for this session. Cache breaks:\n", CalibrationThreshold*100)
		r.breaks(p)
		return p.Err()
	}
	policies := r.Policies()
	base := policies[0]
	priced := r.Dollars && base.CostUSD > 0
	costHeader, costNote := "", ""
	if priced {
		costHeader = fmt.Sprintf(" %10s", "list cost")
		costNote = fmt.Sprintf(" list cost uses the first-party price table dated %s; other platforms and discounts differ.", cachemodel.PriceTableVersion)
		costNote += cachemodel.PriceTableAgeNote(time.Now())
	} else if r.Dollars {
		costNote = " no list price is known for this model, so no dollar column."
	}
	p.Printf("  %-40s %14s %13s %10s %7s%s  %s\n", "policy", "prompt tokens", "cached share", "vs as-run", "misses", costHeader, "guardrail")
	for _, pol := range policies {
		delta := "-"
		if pol.Name != base.Name && base.EffectiveTokens > 0 {
			delta = fmt.Sprintf("%+.0f%%", (pol.EffectiveTokens-base.EffectiveTokens)/base.EffectiveTokens*100)
		}
		name := pol.Name
		if pol.Estimated {
			name += " *"
		}
		cost := ""
		if priced {
			cost = fmt.Sprintf(" %10s", fmt.Sprintf("$%.2f", pol.CostUSD))
		}
		p.Printf("  %-40s %14s %12.0f%% %10s %7d%s  %s\n", name, formatTokens(pol.PromptTokens), pol.CachedShare()*100, delta, pol.Misses, cost, pol.Guardrail)
	}
	p.Printf("  vs as-run compares effective tokens (writes and reads at provider multipliers). * = estimated via the fit.%s\n\n", costNote)
	for _, pol := range policies[1:] {
		p.Printf("  %-40s live: %s\n", pol.Name, pol.ReachableLive)
	}
	p.Printf("\n")
	r.errors(p)
	r.reReads(p)
	p.Printf("\n")
	r.blame(p, replayBlameLimit)
	return p.Err()
}

// reReads prints the context-editing guardrail: how often the agent read
// a file it already had, and whether that rose after the provider began
// clearing old results.
func (r *LaneReport) reReads(p *Printer) {
	rr := r.ReReads
	if rr.Reads == 0 {
		return
	}
	p.Printf("  file re-reads\n")
	p.Printf("    %d of %d file reads repeated a path already in context (%.0f%%), %s in prompts (±%s)\n", rr.Repeated, rr.Reads, rr.Rate()*100, formatTokens(rr.Tokens.Value), errorBar(rr.Tokens))
	if rr.ContextEdits > 0 {
		p.Printf("    provider context edits: %d applied, %s prompt tokens cleared; re-read rate after the first clear %.0f%% (%d of %d) vs %.0f%% before\n", rr.ContextEdits, formatTokens(rr.ClearedTokens), rr.RateAfterClear()*100, rr.RepeatedAfterClear, rr.ReadsAfterClear, rr.RateBeforeClear()*100)
	}
}

// WriteBlame prints the full attribution table.
func (r *LaneReport) WriteBlame(w io.Writer, limit int) error {
	p := NewPrinter(w)
	r.header(p)
	r.blame(p, limit)
	p.Printf("\n")
	r.errors(p)
	return p.Err()
}

// WriteDiff prints every cache break with its cause and location.
func (r *LaneReport) WriteDiff(w io.Writer) error {
	p := NewPrinter(w)
	r.header(p)
	r.breaks(p)
	return p.Err()
}

func (r *LaneReport) breaks(p *Printer) {
	if len(r.Breaks) == 0 {
		p.Printf("  no cache breaks: every turn read the full previous prefix\n")
		return
	}
	for _, b := range r.Breaks {
		t := b.Turn
		p.Printf("  turn %d at %s (+%s): read %s of %s expected, %s re-billed\n", t.Index, t.Request.Timestamp.Format(time.TimeOnly), t.Gap.Round(time.Second), formatTokens(t.Actual), formatTokens(t.Expected), formatTokens(b.Deficit))
		p.Printf("    cause: %s\n", b.Cause)
		if b.MessageIndex >= 0 {
			p.Printf("    where: message %d (%s)\n", b.MessageIndex, b.Label)
		}
		p.Printf("    evidence: %s\n", b.Detail)
	}
	p.Printf("  Anthropic Console cache diagnostics name a byte offset; this names a cause.\n")
}

func (r *LaneReport) errors(p *Printer) {
	p.Printf("  cost of errors\n")
	if len(r.Errors) == 0 {
		p.Printf("    none detected in tool results\n")
	}
	for i, e := range r.Errors {
		p.Printf("    %d. %-44s x%-3d %s in prompts (±%s)\n", i+1, e.Class, e.Count, formatTokens(e.PromptTokens.Value), errorBar(e.PromptTokens))
	}
	p.Printf("    provider retries: not visible in transcripts; run the proxy to capture\n")
}

func (r *LaneReport) blame(p *Printer, limit int) {
	p.Printf("  top token sources (size once, and total across every prompt that carried it)\n")
	for i, e := range r.Blame {
		if limit > 0 && i >= limit {
			break
		}
		errs := ""
		if e.Errors > 0 {
			errs = fmt.Sprintf("  %d errors", e.Errors)
		}
		p.Printf("    %2d. %-*s x%-3d %8s once  %9s in prompts (±%s)%s\n", i+1, labelColumnWidth, transcript.TruncateLabel(e.Label, labelColumnWidth), e.Occurrences, formatTokens(e.Tokens.Value), formatTokens(e.PromptTokens.Value), errorBar(e.PromptTokens), errs)
	}
}

func formatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 10_000:
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func shortID(id string) string {
	if i := strings.Index(id, "-"); i > 0 {
		return id[:i]
	}
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
