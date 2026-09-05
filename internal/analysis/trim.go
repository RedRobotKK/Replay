package analysis

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Offline scoring for a per-block byte cap on tool output, and a probe asking
// whether the agent later depended on what the cap would have removed.
//
// Nothing here touches the request path. The live trimmer does not ship: Go's
// json.Marshal HTML-escapes < > and &, so decode-cut-re-marshal can return a
// block six times the cap on HTML, JSX, XML, git conflict markers and shell
// redirects, which breaks the idempotence the design rested on. Un-trimming a
// block previously sent trimmed is itself a history edit, which the proxy's own
// detector already names. This command exists to find out whether a live path
// would ever be worth those problems, in dollars, before anyone builds one.

// Harm kinds. Each is evidence the agent later needed content a cap removed.
const (
	// HarmLaterEdit is the strongest: an Edit whose old_string sits only in
	// the removed region, so after trimming the agent could not have written
	// that call.
	HarmLaterEdit = "later-edit"
	// HarmReRead is a second read of a path whose first read was trimmed.
	HarmReRead = "re-read"
	// HarmQuote is the assistant repeating a line that existed only in the
	// removed region.
	HarmQuote = "quote"
)

// TrimHarm is one piece of evidence against a cap.
type TrimHarm struct {
	Kind   string `json:"kind"`
	Tool   string `json:"tool"`
	Detail string `json:"detail"`
	// Offset is where in the block the needed content sat, as a share of the
	// block's length, or -1 when the harm carries no position.
	Offset float64 `json:"offset"`
}

// ToolSplit is where later dependencies actually landed inside one tool's
// output, measured rather than assumed.
//
// The fixed 60/40 split that was proposed is wrong in opposite directions for
// the two dominant shapes: for a file read the middle is the function bodies,
// and for test output the failure detail is in the middle. A split has to come
// from the data or it is a guess wearing a number.
type ToolSplit struct {
	Tool        string  `json:"tool"`
	Samples     int     `json:"samples"`
	HeadShare   float64 `json:"head_share"`
	MiddleShare float64 `json:"middle_share"`
	TailShare   float64 `json:"tail_share"`
}

// TrimPlan is what a byte cap would have done to one lane.
type TrimPlan struct {
	CapBytes int `json:"cap_bytes"`
	// Blocks is how many tool results exceeded the cap.
	Blocks       int `json:"blocks"`
	RemovedBytes int `json:"removed_bytes"`
	// RemovedPromptTokens counts each removed region once per request that
	// carried it, which is the number that actually costs money.
	RemovedPromptTokens int `json:"removed_prompt_tokens"`
	// SavedUSD prices those tokens as cache reads, which is what a resent
	// byte is. SavedInputUSD prices them as fresh input, which is the number
	// a token-share report implies and which is wrong.
	SavedUSD      float64 `json:"saved_usd"`
	SavedInputUSD float64 `json:"saved_input_usd"`
	// Estimated is true whenever the byte-to-token fit was used, which is
	// always for a transcript.
	Estimated bool        `json:"estimated"`
	Harms     []TrimHarm  `json:"harms,omitempty"`
	Splits    []ToolSplit `json:"splits,omitempty"`
}

// Overstatement is how many times larger the fresh-input figure is than the
// cache-read one. It is printed so nobody has to take the correction on faith.
func (p TrimPlan) Overstatement() float64 {
	if p.SavedUSD <= 0 {
		return 0
	}
	return p.SavedInputUSD / p.SavedUSD
}

// ProbeBlindSpots names what the probe cannot see. It is a lower bound and
// saying so is the honest part of the design.
func ProbeBlindSpots() []string {
	return []string{
		"This is a LOWER BOUND on harm. Every count below is harm the probe could prove.",
		"Write has no old_string, which is exactly how an agent rewrites a file from " +
			"content it read, so a rewrite from removed content is invisible here.",
		"Line numbers and offsets carried from a trimmed read into a later Read are a " +
			"dependency the probe cannot follow.",
		"The inverse case scores backwards: removing test failures turns a later " +
			"\"tests fail\" into \"tests pass\", producing fewer edits, which this counts " +
			"as a saving rather than as damage.",
	}
}

// ScoreTrim scores a per-block byte cap over one lane.
//
// Blocks are identified by their first appearance; a lane resends the whole
// history every turn, so the cost of a block is its size times the number of
// requests that carried it, and that product is what trimming would save.
func ScoreTrim(lane *transcript.Lane, fit TokenFit, cap int) TrimPlan {
	plan := TrimPlan{CapBytes: cap, Estimated: true}
	if lane == nil || cap <= 0 {
		return plan
	}

	type cut struct {
		block   transcript.Block
		removed string // the region the cap would delete
		carried int    // requests that carried this block
		first   int    // request index of first appearance
	}
	cuts := map[string]*cut{}
	order := []string{}

	for i, req := range lane.Requests {
		for _, msg := range req.Context {
			for _, b := range msg.Blocks {
				if b.Kind != transcript.KindToolResult || b.Text == "" || len(b.Text) <= cap {
					continue
				}
				key := b.Label + "\x00" + b.Text[:min(64, len(b.Text))]
				c, ok := cuts[key]
				if !ok {
					c = &cut{block: b, removed: b.Text[cap:], first: i}
					cuts[key], order = c, append(order, key)
				}
				c.carried++
			}
		}
	}
	if len(cuts) == 0 {
		return plan
	}

	model := "claude-opus-5"
	if len(lane.Requests) > 0 && lane.Requests[0].Model != "" {
		model = lane.Requests[0].Model
	}
	price, priced := cachemodel.PriceFor(model)

	for _, key := range order {
		c := cuts[key]
		plan.Blocks++
		plan.RemovedBytes += len(c.removed)
		tokens := fit.EstimateTokens(len(c.removed))
		plan.RemovedPromptTokens += tokens * c.carried
	}
	if priced {
		mtok := float64(plan.RemovedPromptTokens) / 1e6
		// A resent byte is a cache read, not fresh input. This is the whole
		// correction: the token-share figure implies the second number.
		plan.SavedUSD = mtok * price.InputPerMTok * price.ReadMult
		plan.SavedInputUSD = mtok * price.InputPerMTok
	}

	for _, key := range order {
		plan.Harms = append(plan.Harms, probeCut(lane, cuts[key].block, cuts[key].removed, cuts[key].first)...)
	}
	plan.Splits = deriveSplits(plan.Harms)
	return plan
}

// probeCut asks whether anything after this block depended on what the cap
// would have removed.
func probeCut(lane *transcript.Lane, b transcript.Block, removed string, first int) []TrimHarm {
	var harms []TrimHarm
	retained := b.Text[:len(b.Text)-len(removed)]
	path := pathOf(b.Label)
	seen := map[string]bool{}

	// Lines long enough to be evidence rather than coincidence. A short line
	// appears everywhere and would make the probe look thorough while
	// measuring nothing.
	removedLines := map[string]int{}
	for _, ln := range strings.Split(removed, "\n") {
		ln = strings.TrimSpace(ln)
		if len(ln) >= 24 && !strings.Contains(retained, ln) {
			removedLines[ln] = strings.Index(removed, ln)
		}
	}

	add := func(kind, detail string, off float64) {
		k := kind + detail
		if seen[k] {
			return
		}
		seen[k] = true
		harms = append(harms, TrimHarm{Kind: kind, Tool: toolNameOf(b.Label), Detail: detail, Offset: off})
	}

	for i, req := range lane.Requests {
		if i <= first {
			continue
		}
		for _, msg := range req.Context {
			for _, later := range msg.Blocks {
				switch {
				case later.Kind == transcript.KindToolUse && later.ToolName == "Edit":
					old := oldString(later.Text)
					// Only counts when the content is gone: an old_string
					// still inside the retained head was never harmed.
					if old == "" || len(old) < 8 || strings.Contains(retained, old) || !strings.Contains(removed, old) {
						continue
					}
					add(HarmLaterEdit, fmt.Sprintf("a later Edit's old_string sits only in the removed part of %s", shortLabel(b.Label)), offsetOf(b.Text, old))
				case later.Kind == transcript.KindToolResult && path != "" && later.ToolName == b.ToolName && pathOf(later.Label) == path:
					// No offset: the whole block was fetched again, so this
					// says the block mattered and nothing about where.
					add(HarmReRead, fmt.Sprintf("%s was read again after the cap would have trimmed it", shortLabel(b.Label)), -1)
				case later.Kind == transcript.KindText:
					for ln, off := range removedLines {
						if strings.Contains(later.Text, ln) {
							add(HarmQuote, fmt.Sprintf("a later message quotes a line only present in the removed part of %s", shortLabel(b.Label)), float64(len(b.Text)-len(removed)+off)/float64(len(b.Text)))
							break
						}
					}
				}
			}
		}
	}
	return harms
}

// deriveSplits turns where dependencies landed into a per-tool head/middle/tail
// weighting. Thirds, because the claim being tested is which end of a block
// matters, and three buckets answer it without inventing precision.
// DeriveSplits is deriveSplits over harms already pooled across sessions.
func DeriveSplits(harms []TrimHarm) []ToolSplit { return deriveSplits(harms) }

func deriveSplits(harms []TrimHarm) []ToolSplit {
	byTool := map[string]*ToolSplit{}
	for _, h := range harms {
		// Offset < 0 marks a harm with no position. A re-read is the whole
		// block coming back, so counting it as "tail" would manufacture a
		// finding out of evidence that has none.
		if h.Tool == "" || h.Offset < 0 {
			continue
		}
		s, ok := byTool[h.Tool]
		if !ok {
			s = &ToolSplit{Tool: h.Tool}
			byTool[h.Tool] = s
		}
		s.Samples++
		switch {
		case h.Offset < 1.0/3:
			s.HeadShare++
		case h.Offset < 2.0/3:
			s.MiddleShare++
		default:
			s.TailShare++
		}
	}
	out := make([]ToolSplit, 0, len(byTool))
	for _, s := range byTool {
		n := float64(s.Samples)
		s.HeadShare, s.MiddleShare, s.TailShare = s.HeadShare/n, s.MiddleShare/n, s.TailShare/n
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tool < out[j].Tool })
	return out
}

func offsetOf(text, needle string) float64 {
	i := strings.Index(text, needle)
	if i < 0 || len(text) == 0 {
		return 1
	}
	return float64(i) / float64(len(text))
}

// oldString pulls Edit's old_string out of the recorded tool input without
// binding to a client's whole schema.
func oldString(input string) string {
	var v struct {
		OldString string `json:"old_string"`
	}
	if json.Unmarshal([]byte(input), &v) != nil {
		return ""
	}
	return v.OldString
}

// pathOf returns the path half of a "tool result: Read a/b.go" label.
func pathOf(label string) string {
	_, rest, ok := strings.Cut(label, ": ")
	if !ok {
		return ""
	}
	_, path, ok := strings.Cut(rest, " ")
	if !ok {
		return ""
	}
	return path
}

// shortLabel keeps a label identifiable without carrying a command line into a
// report. A Bash tool result's label is the whole invocation, URLs and paths
// included, and this output is meant to be pasteable.
func shortLabel(label string) string {
	s := safeLabel(label)
	if len(s) > MaxContextLabel {
		s = s[:MaxContextLabel-1] + "\u2026"
	}
	return s
}
