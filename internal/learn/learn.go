// Package learn re-scores the policy catalog over recorded sessions and
// selects a policy the proxy may apply, with the statistics that keep a
// small personal corpus from electing a lucky candidate. It reads
// transcripts and ledger files, never the network, and writes a
// versioned, human-readable policy file (ADR-0006). The proxy reads that
// file only at a session's first request; learning never touches bytes.
package learn

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"time"

	"github.com/RedRobotKK/Buffy/internal/analysis"
	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// Selection rules. They are deliberately conservative: a developer's own
// corpus is tens of sessions, and the cost of a wrong policy is a session
// that caches worse than it would have.
const (
	// DefaultMinSessions is how many sessions must carry evidence for a
	// candidate before it can be selected. Evidence means the candidate's
	// score differed from as-run; a session whose prompts never reached a
	// trigger says nothing about that trigger.
	DefaultMinSessions = 5
	// holdoutShare is the fraction of sessions kept out of selection and
	// used only to confirm the winner.
	holdoutShare = 0.3
	// intervalWidth is the half-width of the reported interval in standard
	// errors; two is the conventional roughly 95% band.
	intervalWidth = 2.0
	// PolicyFileSchema is bumped on any incompatible change to the file.
	PolicyFileSchema = 1
)

// Context-edit candidates use a coarse, bounded, absolute grid: absolute
// because the proxy must know the trigger at a session's first request,
// bounded because every extra candidate is another chance to elect noise.
var contextEditTriggers = []int{50_000, 100_000, 200_000, 400_000}

// Candidate is one policy the learner scores.
type Candidate struct {
	Name string `json:"name"`
	// Family groups parameterizations of one mechanism; simplicity is
	// ordered by family, then by fewer parameters.
	Family string `json:"family"`
	// Live says how the candidate would be applied.
	Live string `json:"live"`
	// ContextEdit is set for the context-edit family.
	ContextEdit *analysis.ContextEditPolicy `json:"context_edit,omitempty"`
	// TTL is set for the ttl family.
	TTL time.Duration `json:"ttl,omitempty"`
}

// Families in order of increasing complexity.
const (
	FamilyAsRun       = "as-run"
	FamilyTTL         = "ttl"
	FamilyContextEdit = "context-edit"
)

// Catalog is the fixed candidate set.
func Catalog() []Candidate {
	out := []Candidate{
		{Name: "ttl-5m", Family: FamilyTTL, TTL: cachemodel.TTLShort, Live: "client setting promptCacheTtl=5m"},
		{Name: "ttl-1h", Family: FamilyTTL, TTL: cachemodel.TTLLong, Live: "client setting promptCacheTtl=1h"},
	}
	for _, trigger := range contextEditTriggers {
		p := analysis.ContextEditPolicy{KeepLast: analysis.ContextEditKeepLast, TriggerTokens: trigger}
		out = append(out, Candidate{Name: fmt.Sprintf("context-edit(keep=%d,trigger=%d)", p.KeepLast, p.TriggerTokens), Family: FamilyContextEdit, ContextEdit: &p, Live: fmt.Sprintf("buffy serve --context-edit-trigger %d --context-edit-keep %d", p.TriggerTokens, p.KeepLast)})
	}
	return out
}

// SessionScore is one session's relative saving per candidate: the share
// of as-run effective tokens the candidate would have avoided (negative
// when it would have cost more). Cached share is kept for the report.
type SessionScore struct {
	SessionID string
	Holdout   bool
	AsRun     analysis.Tally
	Saving    map[string]float64
	Cached    map[string]float64
	Estimated map[string]bool
}

// Score simulates every catalog candidate over one session's main lane.
// Sessions whose calibration fails are skipped with ok=false: the
// simulator cannot be trusted on them.
func Score(s *transcript.Session, candidates []Candidate) (SessionScore, bool) {
	lane := analysis.MainLane(s)
	if lane == nil {
		return SessionScore{}, false
	}
	rep := analysis.AnalyzeLane(s, lane)
	if !rep.Calibration.Passes() {
		return SessionScore{}, false
	}
	asRun := analysis.AsRun(lane)
	if asRun.EffectiveTokens <= 0 {
		return SessionScore{}, false
	}
	out := SessionScore{SessionID: s.ID, Holdout: isHoldout(s.ID), AsRun: asRun.Tally, Saving: map[string]float64{}, Cached: map[string]float64{}, Estimated: map[string]bool{}}
	for _, c := range candidates {
		var r analysis.PolicyResult
		switch {
		case c.ContextEdit != nil:
			r = analysis.WithContextEdit(rep.Calibration, *c.ContextEdit, rep.Fit)
		default:
			r = analysis.WithTTL(rep.Calibration, c.TTL)
		}
		out.Saving[c.Name] = (asRun.EffectiveTokens - r.EffectiveTokens) / asRun.EffectiveTokens
		out.Cached[c.Name] = r.CachedShare()
		out.Estimated[c.Name] = r.Estimated
	}
	return out, true
}

// isHoldout assigns a session to the holdout set by a stable hash of its
// id, so the split does not change between runs or with file order.
func isHoldout(id string) bool {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id)) // hash.Hash never fails
	return float64(h.Sum32()%1000)/1000 < holdoutShare
}

// Verdict is the learner's conclusion for one candidate.
type Verdict struct {
	Candidate
	// Sessions is how many sessions carried evidence, split by set.
	Sessions        int `json:"sessions"`
	HoldoutSessions int `json:"holdout_sessions"`
	// Mean and Interval are the training-set saving and its band.
	Mean     float64    `json:"mean_saving"`
	Interval [2]float64 `json:"interval"`
	// HoldoutMean is the saving on sessions selection never saw.
	HoldoutMean float64 `json:"holdout_mean_saving"`
	// CachedShareDelta is the change in cached share versus as-run.
	CachedShareDelta float64 `json:"cached_share_delta"`
	Estimated        bool    `json:"estimated"`
	// Decision says whether the candidate is selected and, when not, why.
	Decision string `json:"decision"`
}

// Selection rejection reasons. Each names the rule that fired.
const (
	rejectTooFew    = "rejected: fewer than %d sessions with evidence (%d)"
	rejectNoMargin  = "rejected: saving not above noise (interval includes zero)"
	rejectNoRepeat  = "rejected: did not repeat on held-out sessions"
	rejectNoHoldout = "rejected: no held-out session carried evidence"
	rejectSimpler   = "rejected: a simpler candidate saves the same within noise"
	rejectBeaten    = "rejected: another candidate saves more"
	selected        = "selected"
)

// Options bound the selection.
type Options struct {
	MinSessions int
}

// Result is the whole learning run: every verdict and the choice.
type Result struct {
	Schema    int       `json:"schema"`
	Generated time.Time `json:"generated"`
	Rules     string    `json:"rules"`
	// Sessions counts what was read and what was usable.
	Sessions struct {
		Found      int `json:"found"`
		Calibrated int `json:"calibrated"`
		Holdout    int `json:"holdout"`
	} `json:"sessions"`
	Verdicts []Verdict `json:"candidates"`
	// Selected is the chosen candidate, nil when nothing qualified.
	Selected *Candidate `json:"selected"`
	// Reason explains an empty selection.
	Reason string `json:"reason,omitempty"`
}

// Select applies the rules to scored sessions: minimum evidence, a
// margin above noise, a repeat on held-out sessions, and ties to the
// simpler candidate.
func Select(candidates []Candidate, scores []SessionScore, found int, opts Options, now time.Time) Result {
	if opts.MinSessions <= 0 {
		opts.MinSessions = DefaultMinSessions
	}
	res := Result{Schema: PolicyFileSchema, Generated: now.UTC(), Rules: cachemodel.RulesVersion}
	res.Sessions.Found = found
	res.Sessions.Calibrated = len(scores)
	for _, s := range scores {
		if s.Holdout {
			res.Sessions.Holdout++
		}
	}
	for _, c := range candidates {
		res.Verdicts = append(res.Verdicts, judge(c, scores, opts))
	}
	// Among qualifying candidates, the largest mean wins unless a simpler
	// one lies within its interval.
	var best *Verdict
	for i := range res.Verdicts {
		v := &res.Verdicts[i]
		if v.Decision != selected {
			continue
		}
		if best == nil || v.Mean > best.Mean {
			best = v
		}
	}
	if best == nil {
		res.Reason = "no candidate met the evidence, margin, and repeat rules"
		return res
	}
	for i := range res.Verdicts {
		v := &res.Verdicts[i]
		if v.Decision != selected || v == best {
			continue
		}
		if simpler(v.Candidate, best.Candidate) && pairedTie(best.Name, v.Name, scores) {
			best.Decision = rejectSimpler
			best = v
		}
	}
	for i := range res.Verdicts {
		v := &res.Verdicts[i]
		if v.Decision == selected && v != best {
			v.Decision = rejectBeaten
		}
	}
	chosen := best.Candidate
	res.Selected = &chosen
	return res
}

// judge scores one candidate against the rules that do not depend on
// other candidates.
func judge(c Candidate, scores []SessionScore, opts Options) Verdict {
	v := Verdict{Candidate: c}
	var train, holdout []float64
	var cachedDelta float64
	for _, s := range scores {
		saving, ok := s.Saving[c.Name]
		if !ok || saving == 0 {
			continue
		}
		v.Estimated = v.Estimated || s.Estimated[c.Name]
		cachedDelta += s.Cached[c.Name] - s.AsRun.CachedShare()
		if s.Holdout {
			holdout = append(holdout, saving)
		} else {
			train = append(train, saving)
		}
	}
	v.Sessions = len(train) + len(holdout)
	v.HoldoutSessions = len(holdout)
	if v.Sessions > 0 {
		v.CachedShareDelta = cachedDelta / float64(v.Sessions)
	}
	v.Mean, v.Interval = meanInterval(train)
	if len(holdout) > 0 {
		v.HoldoutMean, _ = meanInterval(holdout)
	}
	switch {
	case v.Sessions < opts.MinSessions:
		v.Decision = fmt.Sprintf(rejectTooFew, opts.MinSessions, v.Sessions)
	case len(train) < 2 || v.Interval[0] <= 0:
		v.Decision = rejectNoMargin
	case len(holdout) == 0:
		v.Decision = rejectNoHoldout
	case v.HoldoutMean <= 0:
		v.Decision = rejectNoRepeat
	default:
		v.Decision = selected
	}
	return v
}

// pairedTie reports whether candidate a's lead over b is not above noise:
// the per-session difference, taken on the training sessions both have
// evidence for, has a band that includes zero. Pairing removes the
// variance between sessions, which is most of the variance in a personal
// corpus, so a real lead of a few points is seen while a lucky one is not.
func pairedTie(a, b string, scores []SessionScore) bool {
	var diffs []float64
	for _, s := range scores {
		if s.Holdout || s.Saving[a] == 0 || s.Saving[b] == 0 {
			continue
		}
		diffs = append(diffs, s.Saving[a]-s.Saving[b])
	}
	_, iv := meanInterval(diffs)
	return len(diffs) < 2 || iv[0] <= 0
}

// meanInterval returns the mean and a two-standard-error band. One
// sample has no spread and gets an infinite band, which never clears the
// margin rule.
func meanInterval(xs []float64) (float64, [2]float64) {
	n := float64(len(xs))
	if n == 0 {
		return 0, [2]float64{}
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	mean := sum / n
	if n < 2 {
		return mean, [2]float64{math.Inf(-1), math.Inf(1)}
	}
	ss := 0.0
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	se := math.Sqrt(ss/(n-1)) / math.Sqrt(n)
	return mean, [2]float64{mean - intervalWidth*se, mean + intervalWidth*se}
}

// simpler orders candidates by family, then by fewer parameters.
func simpler(a, b Candidate) bool {
	rank := map[string]int{FamilyAsRun: 0, FamilyTTL: 1, FamilyContextEdit: 2}
	if rank[a.Family] != rank[b.Family] {
		return rank[a.Family] < rank[b.Family]
	}
	return false
}

// SortVerdicts orders a result's verdicts best first for the report.
func SortVerdicts(vs []Verdict) {
	sort.SliceStable(vs, func(i, j int) bool {
		if (vs[i].Decision == selected) != (vs[j].Decision == selected) {
			return vs[i].Decision == selected
		}
		return vs[i].Mean > vs[j].Mean
	})
}
