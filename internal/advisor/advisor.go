// Package advisor turns the largest token sources across a developer's
// sessions into concrete suggestions with a predicted saving, and tracks
// each suggestion from pending to applied to verified against later
// sessions (PRD AD-1 to AD-3). Every prediction is on the scale-free
// metric first: the share of prompt tokens a target accounts for.
package advisor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Kind names what a suggestion asks the user to change.
type Kind string

// Suggestion kinds.
const (
	KindToolInputs   Kind = "tool-inputs"
	KindLargeResults Kind = "large-results"
	KindHotFile      Kind = "hot-file"
	KindFirstTurn    Kind = "first-turn-content"
	KindUnusedTools  Kind = "unused-tools"
	KindCacheBreaks  Kind = "cache-breaks"
)

// Thresholds. A target has to matter before it is worth a suggestion,
// and a suggestion predicts what a plausible change would do, never a
// perfect one.
const (
	// MinShare is the share of a session's prompt tokens a target needs to
	// be suggested at all.
	MinShare = 0.10
	// trimShare is the cut a suggestion assumes the user can achieve on a
	// target: halving it. Predictions are trimShare of the target.
	trimShare = 0.5
	// minInjectedTokens is the first-turn injected content size below
	// which splitting instruction files is not worth suggesting.
	minInjectedTokens = 10_000
	// minReads is how many reads of one file, across the corpus, make it a
	// hot file.
	minReads = 3
	// recentSessions is how many of the newest sessions decide whether a
	// suggestion was applied.
	recentSessions = 2
	// appliedDrop is the drop in a target's share, newest sessions against
	// the earlier ones, that counts as the suggestion having been applied.
	appliedDrop = 0.2
	// verifyShare is the fraction of the predicted saving that must be
	// realized for a suggestion to count as verified.
	verifyShare = 0.5
	// AdviceFileSchema is bumped on any incompatible change to the file.
	AdviceFileSchema = 1
)

// Status is where a tracked suggestion stands.
type Status string

// Statuses, in order of progress. AdviceOnly marks kinds whose target
// comes and goes with the work rather than with a change the user made,
// so applying them cannot be detected.
const (
	Pending     Status = "pending"
	Applied     Status = "applied"
	Verified    Status = "verified"
	NotVerified Status = "not verified"
	AdviceOnly  Status = "advice only"
)

// Suggestion is one piece of advice with its evidence.
type Suggestion struct {
	ID     string `json:"id"`
	Kind   Kind   `json:"kind"`
	Target string `json:"target"`
	Title  string `json:"title"`
	Action string `json:"action"`
	// Sessions is how many sessions carried the target above threshold.
	Sessions int `json:"sessions"`
	// Share is the target's mean share of prompt tokens across those
	// sessions; PromptTokens the total it cost across the corpus.
	Share        float64 `json:"share"`
	PromptTokens int     `json:"prompt_tokens"`
	// PredictedShare is the share of prompt tokens the suggestion expects
	// to remove per session; PredictedTokens the same over the corpus.
	PredictedShare  float64 `json:"predicted_share"`
	PredictedTokens int     `json:"predicted_tokens"`
	Estimated       bool    `json:"estimated"`
	Status          Status  `json:"status"`
	// RealizedShare is the drop in the target's share on the newest
	// sessions, once the suggestion counts as applied.
	RealizedShare float64   `json:"realized_share,omitempty"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
}

// evidence is one session's contribution to one target.
type evidence struct {
	at        time.Time
	share     float64
	tokens    int
	estimated bool
	// reads counts file reads for hot-file targets.
	reads int
}

// Observation is everything the advisor extracts from one session.
type Observation struct {
	at      time.Time
	prompt  int
	targets map[string]evidence // keyed by kind + target
	titles  map[string][3]string
}

// Observe extracts targets from one session's main lane. Sessions that
// do not calibrate are skipped: their figures cannot be trusted.
func Observe(s *transcript.Session) (Observation, bool) {
	lane := analysis.MainLane(s)
	if lane == nil || len(lane.Requests) == 0 {
		return Observation{}, false
	}
	rep := analysis.AnalyzeLane(s, lane)
	if !rep.Calibration.Passes() {
		return Observation{}, false
	}
	ob := Observation{at: lane.Requests[0].Timestamp, targets: map[string]evidence{}, titles: map[string][3]string{}}
	for _, req := range lane.Requests {
		ob.prompt += req.Usage.PromptTotal()
	}
	if ob.prompt == 0 {
		return Observation{}, false
	}
	byTool := map[string]analysis.Figure{}
	byToolEstimated := map[string]bool{}
	for _, e := range rep.Blame {
		switch {
		case strings.HasPrefix(e.Label, transcript.LabelToolCallPrefix):
			ob.note(KindToolInputs, strings.TrimPrefix(e.Label, transcript.LabelToolCallPrefix), e.PromptTokens, e.PromptTokens.Error > 0)
		case strings.HasPrefix(e.Label, transcript.LabelToolResultPrefix):
			name := strings.TrimPrefix(e.Label, transcript.LabelToolResultPrefix)
			if analysis.IsFileRead(name) {
				ob.noteReads(fileTarget(name), e)
			}
			tool, _, _ := strings.Cut(name, " ")
			f := byTool[tool]
			f.Value += e.PromptTokens.Value
			f.Error += e.PromptTokens.Error
			byTool[tool] = f
			byToolEstimated[tool] = byToolEstimated[tool] || e.PromptTokens.Error > 0
		case e.Label == analysis.InjectedLabel && e.Tokens.Value >= minInjectedTokens:
			ob.note(KindFirstTurn, "first turn", e.PromptTokens, true)
		case e.Label == analysis.RebillLabel:
			ob.note(KindCacheBreaks, "cache breaks", e.PromptTokens, false)
		}
	}
	for tool, f := range byTool {
		ob.note(KindLargeResults, tool, f, byToolEstimated[tool])
	}
	ob.unusedTools(lane, rep.Fit)
	return ob, true
}

func key(kind Kind, target string) string { return string(kind) + "\x00" + target }

// fileTarget reduces a read label to the tool and the file's base name,
// so the advice file never holds a full path; ledger labels are already
// hashed and pass through unchanged.
func fileTarget(label string) string {
	tool, p, _ := strings.Cut(label, " ")
	return tool + " " + path.Base(p)
}

// note records a target when it clears the share threshold, or always
// for kinds whose threshold is elsewhere.
func (ob *Observation) note(kind Kind, target string, tokens analysis.Figure, estimated bool) {
	share := float64(tokens.Value) / float64(ob.prompt)
	if kind != KindCacheBreaks && kind != KindFirstTurn && share < MinShare {
		return
	}
	if tokens.Value <= 0 {
		return
	}
	ob.targets[key(kind, target)] = evidence{at: ob.at, share: share, tokens: tokens.Value, estimated: estimated}
}

// noteReads records a file read at any size; the corpus decides whether
// it is hot.
func (ob *Observation) noteReads(name string, e analysis.BlameEntry) {
	k := key(KindHotFile, name)
	ev := ob.targets[k]
	ev.at, ev.estimated = ob.at, true
	ev.tokens += e.PromptTokens.Value
	ev.share = float64(ev.tokens) / float64(ob.prompt)
	ev.reads += e.Occurrences
	ob.targets[k] = ev
}

// unusedTools finds tool definitions the session carried on every request
// and never called. Only ledger sessions know their definitions.
func (ob *Observation) unusedTools(lane *transcript.Lane, fit analysis.TokenFit) {
	first := lane.Requests[0]
	if len(first.Tools) == 0 {
		return
	}
	called := map[string]bool{}
	for _, req := range lane.Requests {
		for _, m := range req.Context {
			for _, b := range m.Blocks {
				if b.Kind == transcript.KindToolUse {
					called[b.ToolName] = true
				}
			}
		}
	}
	bytes, names := 0, []string{}
	for _, t := range first.Tools {
		if !called[t.Name] {
			bytes += t.Bytes
			names = append(names, t.Name)
		}
	}
	if bytes == 0 {
		return
	}
	sort.Strings(names)
	tokens := fit.EstimateTokens(bytes) * len(lane.Requests)
	ob.note(KindUnusedTools, "tools never called", analysis.Figure{Value: tokens, Error: int(float64(tokens) * fit.RelativeError)}, true)
	ob.titles[key(KindUnusedTools, "tools never called")] = [3]string{fmt.Sprint(len(names)), strings.Join(names, ", ")}
}

// agg is one target's evidence across the corpus.
type agg struct {
	kind      Kind
	target    string
	evidence  []evidence
	shares    []float64 // per session in time order, zero when absent
	titles    [3]string
	estimated bool
	tokens    int
	reads     int
}

// Suggest aggregates observations into suggestions, newest evidence
// last, and applies the tracking rules against earlier sessions.
func Suggest(obs []Observation) []Suggestion {
	sort.SliceStable(obs, func(i, j int) bool { return obs[i].at.Before(obs[j].at) })
	aggs := map[string]*agg{}
	var order []string
	for _, ob := range obs {
		for k, ev := range ob.targets {
			a, ok := aggs[k]
			if !ok {
				kind, target, _ := strings.Cut(k, "\x00")
				a = &agg{kind: Kind(kind), target: target}
				aggs[k] = a
				order = append(order, k)
			}
			a.evidence = append(a.evidence, ev)
			a.estimated = a.estimated || ev.estimated
			a.tokens += ev.tokens
			a.reads += ev.reads
			if t, ok := ob.titles[k]; ok {
				a.titles = t
			}
		}
	}
	sort.Strings(order)
	for _, ob := range obs {
		for k, a := range aggs {
			a.shares = append(a.shares, ob.targets[k].share)
		}
	}
	var out []Suggestion
	for _, k := range order {
		a := aggs[k]
		if a.kind == KindHotFile && a.reads < minReads {
			continue
		}
		s := Suggestion{ID: id(a.kind, a.target), Kind: a.kind, Target: a.target, Sessions: len(a.evidence), PromptTokens: a.tokens, Estimated: a.estimated, FirstSeen: a.evidence[0].at, LastSeen: a.evidence[len(a.evidence)-1].at}
		for _, ev := range a.evidence {
			s.Share += ev.share
		}
		s.Share /= float64(len(a.evidence))
		switch a.kind {
		case KindHotFile:
			// Only the repeats are avoidable.
			s.Share *= float64(a.reads-1) / float64(a.reads)
			s.PredictedShare = s.Share
			s.PredictedTokens = a.tokens * (a.reads - 1) / a.reads
		case KindCacheBreaks:
			// A break that does not happen re-bills nothing.
			s.PredictedShare = s.Share
			s.PredictedTokens = a.tokens
		default:
			s.PredictedShare = trimShare * s.Share
			s.PredictedTokens = int(trimShare * float64(a.tokens))
		}
		s.Title, s.Action = describe(a, s)
		s.Status, s.RealizedShare = track(a.kind, a.shares, s.PredictedShare)
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PredictedTokens > out[j].PredictedTokens })
	return out
}

// track decides a suggestion's status from the target's share per session
// in time order. A drop of appliedDrop on the newest sessions against the
// earlier mean counts as applied; a realized drop of verifyShare of the
// prediction counts as verified.
func track(kind Kind, shares []float64, predicted float64) (Status, float64) {
	if kind == KindHotFile || kind == KindCacheBreaks {
		return AdviceOnly, 0
	}
	if len(shares) <= recentSessions {
		return Pending, 0
	}
	earlier, recent := shares[:len(shares)-recentSessions], shares[len(shares)-recentSessions:]
	before, after := mean(earlier), mean(recent)
	if before == 0 || after > before*(1-appliedDrop) {
		return Pending, 0
	}
	realized := before - after
	if realized >= verifyShare*predicted {
		return Verified, realized
	}
	return NotVerified, realized
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// describe renders the title and the action for a suggestion.
func describe(a *agg, s Suggestion) (string, string) {
	kind, target, titles := a.kind, a.target, a.titles
	pct := fmt.Sprintf("%.0f%%", s.Share*100)
	switch kind {
	case KindToolInputs:
		return fmt.Sprintf("%s inputs are %s of prompt tokens", target, pct),
			"keep tool inputs short: run scripts from files instead of inline heredocs, and pass paths instead of contents"
	case KindLargeResults:
		return fmt.Sprintf("%s results are %s of prompt tokens", target, pct),
			"truncate outputs before they enter the conversation: head, tail, grep with limits, or a summarizing wrapper"
	case KindHotFile:
		return fmt.Sprintf("%s read %d times across %d sessions, repeats are %s of prompt tokens", target, a.reads, len(a.evidence), pct),
			"put a summary header at the top of the file and read only the lines needed; a stable summary caches, a full re-read does not"
	case KindFirstTurn:
		return fmt.Sprintf("first-turn instructions and attachments are %s of prompt tokens", pct),
			"split instruction files: keep what every turn needs, move the rest to on-demand files or skills the agent loads when relevant"
	case KindUnusedTools:
		return fmt.Sprintf("%s tool definitions never called are %s of prompt tokens (%s)", titles[0], pct, titles[1]),
			"defer-load tools the session does not use (Claude Code tool search) or trim their descriptions"
	case KindCacheBreaks:
		return fmt.Sprintf("cache breaks re-billed %s of prompt tokens", pct),
			"run replay diff on the session to see the cause of each break; most are a changed prefix or an edited turn"
	}
	return string(kind), ""
}

// id is a stable identity for a suggestion across runs.
func id(kind Kind, target string) string {
	sum := sha256.Sum256([]byte(key(kind, target)))
	return hex.EncodeToString(sum[:])[:12]
}
