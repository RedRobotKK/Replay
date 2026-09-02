package analysis

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// Tier labels every figure with its provenance.
type Tier string

// Tiers. The proxy will introduce TierMeasured for wire captures.
const (
	TierEstimated Tier = "estimated (transcripts only)"
	TierMeasured  Tier = "measured (proxy-recorded)"
)

// LaneReport is everything the commands need for one lane.
type LaneReport struct {
	Session     *transcript.Session
	Lane        *transcript.Lane
	Calibration *Calibration
	Fit         TokenFit
	Breaks      []Break
	Blame       []BlameEntry
	Errors      []ErrorCost
	Policies    []PolicyResult
	Tier        Tier
}

// Context-editing policies are scored at triggers relative to the session's
// largest prompt, so a policy is never evaluated at a threshold the session
// could not reach. These are starting points for the user, not
// recommendations.
var contextEditTriggerShares = []float64{0.5, 0.75}

const contextEditKeepLast = 6

// Report layout limits.
const (
	replayBlameLimit = 5
	labelColumnWidth = 64
)

func contextEditPolicies(lane *transcript.Lane) []ContextEditPolicy {
	largest := 0
	for _, r := range lane.Requests {
		largest = max(largest, r.Usage.PromptTotal())
	}
	var out []ContextEditPolicy
	for _, share := range contextEditTriggerShares {
		out = append(out, ContextEditPolicy{KeepLast: contextEditKeepLast, TriggerTokens: int(float64(largest) * share)})
	}
	return out
}

// AnalyzeLane runs every analysis for one lane.
func AnalyzeLane(s *transcript.Session, lane *transcript.Lane) *LaneReport {
	cal := Calibrate(lane)
	cal.PrefixVisible = s.PrefixVisible
	fit := Fit(cal)
	rep := &LaneReport{
		Session:     s,
		Lane:        lane,
		Calibration: cal,
		Fit:         fit,
		Breaks:      FindBreaks(cal, fit),
		Blame:       Blame(cal, fit),
		Errors:      ErrorCosts(cal, fit),
		Tier:        TierEstimated,
	}
	if s.Source == transcript.SourceLedger {
		rep.Tier = TierMeasured
	}
	rep.Policies = append(rep.Policies, AsRun(lane))
	if cal.Passes() {
		rep.Policies = append(rep.Policies, WithTTL(cal, cachemodel.TTLShort), WithTTL(cal, cachemodel.TTLLong))
		for _, p := range contextEditPolicies(lane) {
			rep.Policies = append(rep.Policies, WithContextEdit(cal, p, fit))
		}
	}
	return rep
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

// printer writes report text and remembers the first write error so the
// Write methods can return it without checking every line.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}

// WriteHeader prints the lines every report must carry.
func (r *LaneReport) WriteHeader(w io.Writer) error {
	p := &printer{w: w}
	r.header(p)
	return p.err
}

func (r *LaneReport) header(p *printer) {
	cal := r.Calibration
	p.printf("Session %s  client %s  model %s  requests %d\n", shortID(r.Session.ID), r.Session.ClientVersion, r.Lane.Requests[0].Model, len(r.Lane.Requests))
	p.printf("Tier: %s\n", r.Tier)
	p.printf("Calibration: reproduced provider cache reads on %d/%d turns", cal.Reproduced+cal.Exceeded, cal.Compared())
	if cal.Exceeded > 0 {
		p.printf(" (%d read more than predicted: a sibling request extended the prefix)", cal.Exceeded)
	}
	if cal.Broken > 0 {
		p.printf("; %d cache breaks", cal.Broken)
	}
	p.printf("\n")
	p.printf("Assumption: %s\n", AssumptionNote)
	prefix := "estimated"
	switch {
	case r.Session.PrefixVisible:
		prefix = "recorded on the wire"
	case r.Fit.UnseenPrefixMeasured:
		prefix = "measured from the first request's cache read"
	}
	p.printf("Rules: %s; user-content fit %.3f tokens/byte ±%.0f%% from %d turns; system prefix %s (%s)\n", cachemodel.RulesVersion, r.Fit.TokensPerByte, r.Fit.RelativeError*100, r.Fit.Turns, formatTokens(r.Fit.UnseenPrefixTokens), prefix)
	if r.Session.Skipped > 0 {
		p.printf("Note: %d transcript lines were not conversation content and were skipped\n", r.Session.Skipped)
	}
	p.printf("\n")
}

// WriteReplay prints the policy table, the error section, and the top
// token sources.
func (r *LaneReport) WriteReplay(w io.Writer) error {
	p := &printer{w: w}
	r.header(p)
	if !r.Calibration.Passes() {
		p.printf("Calibration below %.0f%%: alternatives are not scored for this session. Cache breaks:\n", CalibrationThreshold*100)
		r.breaks(p)
		return p.err
	}
	base := r.Policies[0]
	p.printf("  %-40s %14s %13s %10s %7s  %s\n", "policy", "prompt tokens", "cached share", "vs as-run", "misses", "guardrail")
	for _, pol := range r.Policies {
		delta := "-"
		if pol.Name != base.Name && base.EffectiveTokens > 0 {
			delta = fmt.Sprintf("%+.0f%%", (pol.EffectiveTokens-base.EffectiveTokens)/base.EffectiveTokens*100)
		}
		name := pol.Name
		if pol.Estimated {
			name += " *"
		}
		p.printf("  %-40s %14s %12.0f%% %10s %7d  %s\n", name, formatTokens(pol.PromptTokens), pol.CachedShare*100, delta, pol.Misses, pol.Guardrail)
	}
	p.printf("  vs as-run compares effective tokens (writes and reads at provider multipliers). * = estimated via the fit.\n\n")
	for _, pol := range r.Policies[1:] {
		p.printf("  %-40s live: %s\n", pol.Name, pol.ReachableLive)
	}
	p.printf("\n")
	r.errors(p)
	p.printf("\n")
	r.blame(p, replayBlameLimit)
	return p.err
}

// WriteBlame prints the full attribution table.
func (r *LaneReport) WriteBlame(w io.Writer, limit int) error {
	p := &printer{w: w}
	r.header(p)
	r.blame(p, limit)
	p.printf("\n")
	r.errors(p)
	return p.err
}

// WriteDiff prints every cache break with its cause and location.
func (r *LaneReport) WriteDiff(w io.Writer) error {
	p := &printer{w: w}
	r.header(p)
	r.breaks(p)
	return p.err
}

func (r *LaneReport) breaks(p *printer) {
	if len(r.Breaks) == 0 {
		p.printf("  no cache breaks: every turn read the full previous prefix\n")
		return
	}
	for _, b := range r.Breaks {
		t := b.Turn
		p.printf("  turn %d at %s (+%s): read %s of %s expected, %s re-billed\n", t.Index, t.Request.Timestamp.Format(time.TimeOnly), t.Gap.Round(time.Second), formatTokens(t.Actual), formatTokens(t.Expected), formatTokens(b.Deficit))
		p.printf("    cause: %s\n", b.Cause)
		if b.MessageIndex >= 0 {
			p.printf("    where: message %d (%s)\n", b.MessageIndex, b.Label)
		}
		p.printf("    evidence: %s\n", b.Detail)
	}
}

func (r *LaneReport) errors(p *printer) {
	p.printf("  cost of errors\n")
	if len(r.Errors) == 0 {
		p.printf("    none detected in tool results\n")
	}
	for i, e := range r.Errors {
		p.printf("    %d. %-44s x%-3d %s in prompts (±%s)\n", i+1, e.Class, e.Count, formatTokens(e.PromptTokens.Value), formatTokens(e.PromptTokens.Error))
	}
	p.printf("    provider retries: not visible in transcripts; run the proxy to capture\n")
}

func (r *LaneReport) blame(p *printer, limit int) {
	p.printf("  top token sources (size once, and total across every prompt that carried it)\n")
	for i, e := range r.Blame {
		if limit > 0 && i >= limit {
			break
		}
		errs := ""
		if e.Errors > 0 {
			errs = fmt.Sprintf("  %d errors", e.Errors)
		}
		p.printf("    %2d. %-*s x%-3d %8s once  %9s in prompts (±%s)%s\n", i+1, labelColumnWidth, transcript.TruncateLabel(e.Label, labelColumnWidth), e.Occurrences, formatTokens(e.Tokens.Value), formatTokens(e.PromptTokens.Value), formatTokens(e.PromptTokens.Error), errs)
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
