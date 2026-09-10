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
	// Dismissed is the reader saying "not doing this", from the triage
	// screen. Distinct from Applied because it records a decision rather
	// than a change: nothing about the corpus moved, so nothing should be
	// verified later, and re-suggesting it would be arguing with someone
	// who has already answered.
	Dismissed Status = "dismissed"
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
	// errorMeasured carries analysis.Figure.ErrorMeasured through to the
	// aggregate, because it was being thrown away here.
	//
	// note() takes a Figure and kept only its Value, so the one thing that
	// distinguishes a spread measured on this content from one borrowed off
	// different content stopped at this line. That is why the guard on the
	// unused-tools figure had to be a source grep reading advisor.go for the
	// name of a function — there was no behaviour left to assert. An audit
	// then defeated the grep with a rename, and restored the defect #121
	// exists to fix with the whole tree green.
	errorMeasured bool
	// reads counts file reads for hot-file targets.
	reads int
}

// sample is one session's reading of one target, and whether there was one.
//
// The distinction the previous []float64 could not carry. A map returns the
// zero value for a key it does not hold, so a session that never touched the
// target arrived as a hard 0.0 and averaged in as if the target had been
// measured at nothing. See mean_test.go: that scored a tool going unused for
// two sessions higher than actually halving its share.
//
// seen is false for two different reasons and deliberately does not say which.
// Either the session did not exercise the target, or note() dropped it for
// sitting under MinShare. By the time the aggregation runs, that information
// is already gone — so the honest reading of either is "no measurement here",
// and an unknown left out of a mean cannot corrupt it.
type sample struct {
	share float64
	seen  bool
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
	ob.targets[key(kind, target)] = evidence{at: ob.at, share: share, tokens: tokens.Value,
		estimated: estimated, errorMeasured: tokens.ErrorMeasured}
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

// builtinTools is the bucket for definitions that name no server.
//
// Not a server name and deliberately not shaped like one: a reader who sees it
// must not go looking for something to disable.
const builtinTools = "built-in"

// serverOf reads the MCP server out of a tool name, or reports a built-in.
//
// The convention is mcp__<server>__<tool>. Anything that does not match it is a
// built-in, and inventing a server for one would tell the reader to switch off
// something that does not exist. A malformed mcp__ name with no second
// separator is treated the same way, because a server named from a broken
// string is a worse answer than no server at all.
func serverOf(tool string) string {
	const p = "mcp__"
	if !strings.HasPrefix(tool, p) {
		return builtinTools
	}
	rest := tool[len(p):]
	i := strings.Index(rest, "__")
	if i <= 0 {
		return builtinTools
	}
	return rest[:i]
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
	// Grouped by the server that defines them, because the server is the unit
	// of action. A reader cannot disable one tool; they can disable a server.
	//
	// This needs no configuration to work out, which is the whole reason it is
	// free: MCP tools are named mcp__<server>__<tool>, so the attribution is
	// already in the name the ledger stores beside each byte size. Reading
	// .mcp.json to learn the same thing would cross the boundary WHAT-YOU-GET.md
	// draws — that file can hold credentials, and a CI job that parses it would
	// do so in the environment where secrets are most exposed.
	type bucket struct {
		bytes int
		names []string
	}
	buckets := map[string]*bucket{}
	for _, t := range first.Tools {
		if called[t.Name] {
			continue
		}
		g := serverOf(t.Name)
		b := buckets[g]
		if b == nil {
			b = &bucket{}
			buckets[g] = b
		}
		b.bytes += t.Bytes
		b.names = append(b.names, t.Name)
	}
	for target, b := range buckets {
		if b.bytes == 0 {
			continue
		}
		sort.Strings(b.names)
		// EstimateOutsideFit, not EstimateTokens with the fit's spread bolted
		// on. These are tool-definition bytes — exactly the content Fit
		// excludes from its sample because schemas "are denser than prose and
		// would drag the fit", which preflight.go:41-48 states independently.
		//
		// The estimate stays: a suggestion nobody can rank is not a
		// suggestion, and this ratio is the only number anyone has. What goes
		// is the error bar, which was a standard deviation over prose turns
		// and says nothing about schema density. Borrowing it dressed an
		// unquantified error as a measured one.
		//
		// The size of the real error is unknown and stays unknown until
		// somebody measures tokens-per-byte on schema JSON for this provider.
		// The direction is the code's own: denser, so this understates.
		est := fit.EstimateOutsideFit(b.bytes)
		est.Value *= len(lane.Requests)
		ob.note(KindUnusedTools, target, est, true)
		ob.titles[key(KindUnusedTools, target)] = [3]string{fmt.Sprint(len(b.names)), strings.Join(b.names, ", "), target}
	}
}

// agg is one target's evidence across the corpus.
type agg struct {
	// applied is set by the caller from the reader's own decision, never
	// inferred from the shares below.
	applied   bool
	kind      Kind
	target    string
	evidence  []evidence
	shares    []sample // per session in time order; seen=false where there was no reading
	titles    [3]string
	estimated bool
	tokens    int
	reads     int
}

// Suggest aggregates observations into suggestions, newest evidence
// last, and applies the tracking rules against earlier sessions.
// Suggest aggregates observations into suggestions.
//
// applied carries the ids the reader marked applied in `replay tui`, and is the
// only thing that lets a suggestion be judged. Nil means nobody has marked
// anything, which is the common case and yields Pending throughout — the
// honest answer, since two windows of a moving corpus are not a before and an
// after. See track for why that used to be inferred and why it cannot be.
func Suggest(obs []Observation, applied map[string]bool) []Suggestion {
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
			// The comma-ok is the whole fix. Without it this read is
			// indistinguishable from a measured zero.
			ev, ok := ob.targets[k]
			a.shares = append(a.shares, sample{share: ev.share, seen: ok})
		}
	}
	var out []Suggestion
	for _, k := range order {
		a := aggs[k]
		if a.kind == KindHotFile && a.reads < minReads {
			continue
		}
		sid := id(a.kind, a.target)
		a.applied = applied[sid]
		s := Suggestion{ID: sid, Kind: a.kind, Target: a.target, Sessions: len(a.evidence), PromptTokens: a.tokens, Estimated: a.estimated, FirstSeen: a.evidence[0].at, LastSeen: a.evidence[len(a.evidence)-1].at}
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
		// applied comes from the reader, not from the data. Suggest has no
		// access to the advice file, so the caller reconciles: a suggestion the
		// reader marked applied keeps that status through the next run and is
		// the only kind track will judge.
		s.Status, s.RealizedShare = track(a.kind, a.shares, s.PredictedShare, a.applied)
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PredictedTokens > out[j].PredictedTokens })
	return out
}

// track decides a suggestion's status from the target's share per session
// in time order. A drop of appliedDrop on the newest sessions against the
// earlier mean counts as applied; a realized drop of verifyShare of the
// prediction counts as verified.
func track(kind Kind, shares []sample, predicted float64, applied bool) (Status, float64) {
	if kind == KindHotFile || kind == KindCacheBreaks {
		return AdviceOnly, 0
	}
	// Without a recorded application there is no before and after — there are
	// two windows of a moving corpus.
	//
	// This function used to infer the application from the very number it then
	// measured: a fall of appliedDrop WAS the application, and the size of that
	// same fall decided whether the prediction held. The circularity is not
	// subtle once seen, and it was wrong in both directions. A target drifting
	// 30% to 22% over two sessions of different work returned VERIFIED with a
	// realized saving of 8 points, telling a reader their change was made and
	// confirmed when they had made none. Noise of 20/40/30/30 falling to 23
	// returned NOT VERIFIED, telling them a change they never made had failed.
	//
	// That is what produced 20 not-verified against 1 verified on this
	// machine's 140 suggestions. It is not evidence the advice fails. It is
	// evidence the verifier was measuring corpus drift and could not tell which
	// way it was being fooled.
	//
	// `replay tui` now lets a reader mark a finding applied, and that keystroke
	// is a recorded fact. Until one exists, the honest status is Pending: the
	// suggestion stands and nothing is claimed about it.
	if !applied {
		return Pending, 0
	}
	if len(shares) <= recentSessions {
		return Pending, 0
	}
	earlier, recent := shares[:len(shares)-recentSessions], shares[len(shares)-recentSessions:]

	// Averaged over the sessions that saw the target, not over all of them.
	//
	// A session that did not exercise a tool says nothing about what that tool
	// costs when it is used, and counting it as a zero says the opposite. That
	// arithmetic credited a target with its entire share as a "saving" for the
	// crime of not appearing — more than it credited actually halving it.
	before, nBefore := meanSeen(earlier)
	after, nAfter := meanSeen(recent)

	// No recent reading is not a reduction to nothing, it is the absence of a
	// measurement, and Pending is what this file says everywhere else when
	// nothing is known. Claiming a saving here is how the screen came to
	// report its largest successes for sessions that simply went elsewhere.
	if nBefore == 0 || nAfter == 0 {
		return Pending, 0
	}
	if before == 0 || after > before*(1-appliedDrop) {
		return Pending, 0
	}
	realized := before - after
	if realized >= verifyShare*predicted {
		return Verified, realized
	}
	return NotVerified, realized
}

// meanSeen averages the readings that exist, and reports how many there were.
//
// The count is returned rather than inferred from a zero mean because those
// are different facts: "measured at nothing" and "not measured" are exactly
// the pair this whole change exists to keep apart, and a helper that collapsed
// them again on the way out would put the defect back one level down.
func meanSeen(xs []sample) (float64, int) {
	sum, n := 0.0, 0
	for _, x := range xs {
		if !x.seen {
			continue
		}
		sum += x.share
		n++
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), n
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
		// A named server earns a different sentence from a built-in, because
		// only one of them can be switched off. Telling a reader to "disable
		// this server" for Glob would name something that does not exist.
		if target != builtinTools {
			return fmt.Sprintf("%s tool definitions from %s never called are %s of prompt tokens (%s)",
					titles[0], target, pct, titles[1]),
				fmt.Sprintf("disable this server if the work does not need it: its %s definitions are re-sent on every request, called or not", titles[0])
		}
		return fmt.Sprintf("%s built-in tool definitions never called are %s of prompt tokens (%s)", titles[0], pct, titles[1]),
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
