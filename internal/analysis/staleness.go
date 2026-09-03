package analysis

import (
	"fmt"
	"sort"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
)

// Staleness detection (ST-1). Calibration is judged per model with the
// newest sessions on their own: when the rules reproduced a model's cache
// reads on earlier sessions and stop reproducing them on the newest ones,
// the provider's behavior changed, alternatives are no longer scored for
// that model, and the one rule usage can bound, the minimum cacheable
// prefix, is refit from the sessions. The lookback window cannot be
// inferred from usage and stays in the rules file (ST-2).
const (
	// StalenessRecentSessions is how many of a model's newest sessions
	// form the recent window.
	StalenessRecentSessions = 5
	// StalenessMinSessions and StalenessMinTurns are the evidence a window
	// needs before its match rate means anything.
	StalenessMinSessions = 3
	StalenessMinTurns    = 20
)

// ModelCalibration is one model's calibration across sessions.
type ModelCalibration struct {
	Model    string
	Sessions int
	Compared int
	Matched  int
	// The recent window: the newest sessions judged on their own.
	RecentSessions int
	RecentCompared int
	RecentMatched  int
	// RecentFailing counts recent sessions that individually fall below
	// the calibration threshold.
	RecentFailing int
	// Stale is set when earlier sessions calibrated and enough recent
	// sessions each do not; Reason says so in words. One bad session is
	// not a rule change, and a model whose sessions never calibrated is
	// not stale: the per-session gate already refuses it.
	Stale  bool
	Reason string
	// MinPrefix is the refit of the minimum cacheable prefix.
	MinPrefix MinPrefixFit
}

// MatchRate is the share of compared turns reproduced across all sessions.
func (m ModelCalibration) MatchRate() float64 { return rate(m.Matched, m.Compared) }

// RecentMatchRate is the same over the recent window.
func (m ModelCalibration) RecentMatchRate() float64 { return rate(m.RecentMatched, m.RecentCompared) }

func rate(matched, compared int) float64 {
	if compared == 0 {
		return 1
	}
	return float64(matched) / float64(compared)
}

// MinPrefixFit bounds the provider's minimum cacheable prefix from usage:
// the largest prompt that saw no cache activity lies below it, the
// smallest cached prefix at or above it.
type MinPrefixFit struct {
	// Rule is the rules file's value for the model.
	Rule int
	// LargestUncached is the largest prompt with neither a cache write
	// nor a read; zero when every prompt was cached.
	LargestUncached int
	// SmallestCached is the smallest cached prefix seen; zero when none.
	SmallestCached int
}

// Conclusive reports whether both bounds were seen and are consistent.
func (f MinPrefixFit) Conclusive() bool {
	return f.LargestUncached > 0 && f.SmallestCached > 0 && f.LargestUncached < f.SmallestCached
}

// Disagrees reports whether the rule lies outside the observed bounds:
// prompts the rule says should cache did not, or prompts it says cannot
// did.
func (f MinPrefixFit) Disagrees() bool {
	if f.LargestUncached > 0 && f.Rule <= f.LargestUncached {
		return true
	}
	return f.SmallestCached > 0 && f.Rule > f.SmallestCached
}

// String renders the fit for a report.
func (f MinPrefixFit) String() string {
	switch {
	case f.SmallestCached == 0 && f.LargestUncached == 0:
		return fmt.Sprintf("minimum cacheable prefix: no evidence (rules say %d tokens)", f.Rule)
	case f.SmallestCached == 0:
		return fmt.Sprintf("minimum cacheable prefix: above %d tokens, nothing cached yet (rules say %d)", f.LargestUncached, f.Rule)
	case f.LargestUncached == 0:
		return fmt.Sprintf("minimum cacheable prefix: at most %d tokens, no uncached prompt seen (rules say %d)", f.SmallestCached, f.Rule)
	case !f.Conclusive():
		return fmt.Sprintf("minimum cacheable prefix: inconclusive, a %d-token prompt was uncached while a %d-token prefix was cached (rules say %d)", f.LargestUncached, f.SmallestCached, f.Rule)
	default:
		return fmt.Sprintf("minimum cacheable prefix: between %d and %d tokens (rules say %d)", f.LargestUncached+1, f.SmallestCached, f.Rule)
	}
}

// ModelCalibrations groups calibrated lanes by the model of their first
// request, in model order. Sessions are ordered by their first request so
// the newest form the recent window.
func ModelCalibrations(reports []*LaneReport) []ModelCalibration {
	byModel := map[string][]*LaneReport{}
	for _, rep := range reports {
		if rep == nil || rep.Lane == nil || rep.Calibration == nil || len(rep.Lane.Requests) == 0 {
			continue
		}
		model := rep.Lane.Requests[0].Model
		byModel[model] = append(byModel[model], rep)
	}
	models := make([]string, 0, len(byModel))
	for m := range byModel {
		models = append(models, m)
	}
	sort.Strings(models)
	out := make([]ModelCalibration, 0, len(models))
	for _, model := range models {
		out = append(out, modelCalibration(model, byModel[model]))
	}
	return out
}

func modelCalibration(model string, reps []*LaneReport) ModelCalibration {
	sort.SliceStable(reps, func(i, j int) bool {
		return reps[i].Lane.Requests[0].Timestamp.Before(reps[j].Lane.Requests[0].Timestamp)
	})
	m := ModelCalibration{Model: model, Sessions: len(reps), MinPrefix: MinPrefixFit{Rule: cachemodel.MinCacheablePrefix(model)}}
	recentFrom := len(reps) - StalenessRecentSessions
	if recentFrom < 0 {
		recentFrom = 0
	}
	for i, rep := range reps {
		cal := rep.Calibration
		matched := cal.Reproduced + cal.Exceeded
		m.Compared += cal.Compared()
		m.Matched += matched
		if i >= recentFrom {
			m.RecentSessions++
			m.RecentCompared += cal.Compared()
			m.RecentMatched += matched
			if !cal.Passes() {
				m.RecentFailing++
			}
		}
		for _, req := range rep.Lane.Requests {
			u := req.Usage
			cached := u.CacheCreation + u.CacheRead
			if cached == 0 {
				m.MinPrefix.LargestUncached = max(m.MinPrefix.LargestUncached, u.Input)
				continue
			}
			if m.MinPrefix.SmallestCached == 0 || cached < m.MinPrefix.SmallestCached {
				m.MinPrefix.SmallestCached = cached
			}
		}
	}
	earlierCompared := m.Compared - m.RecentCompared
	earlierMatched := m.Matched - m.RecentMatched
	recentEvidence := m.RecentFailing >= StalenessMinSessions && m.RecentCompared >= StalenessMinTurns
	earlierEvidence := earlierCompared >= StalenessMinTurns && rate(earlierMatched, earlierCompared) >= CalibrationThreshold
	if recentEvidence && earlierEvidence {
		m.Stale = true
		m.Reason = fmt.Sprintf("provider behavior changed: %d of the newest %d sessions fall below the calibration threshold (%.0f%% together) after %.0f%% on the %d before them; alternatives are not scored for this model", m.RecentFailing, m.RecentSessions, m.RecentMatchRate()*100, rate(earlierMatched, earlierCompared)*100, m.Sessions-m.RecentSessions)
	}
	return m
}

// StaleModels returns the models flagged stale, for callers that filter.
func StaleModels(cals []ModelCalibration) map[string]bool {
	out := map[string]bool{}
	for _, c := range cals {
		if c.Stale {
			out[c.Model] = true
		}
	}
	return out
}
