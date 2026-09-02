package analysis

import (
	"sort"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// BlameEntry aggregates attributed tokens for one label across a lane.
type BlameEntry struct {
	Label string
	// Tokens is the size of all blocks with this label, counted once each.
	Tokens Estimated
	// Occurrences is how many blocks carried the label.
	Occurrences int
	// PromptTokens is the total contribution to prompts across the lane:
	// each block's size multiplied by the number of requests that carried it.
	PromptTokens Estimated
	// Errors counts blocks flagged as tool errors.
	Errors int
}

// labelAcc accumulates attribution for one label, keeping measured and
// estimated tokens apart so uncertainty is reported only where it exists.
type labelAcc struct {
	measured, estimated             int
	promptMeasured, promptEstimated int
	occurrences, errors             int
}

func (a *labelAcc) add(tokens, carried int, measured bool, isError bool) {
	if measured {
		a.measured += tokens
		a.promptMeasured += tokens * carried
	} else {
		a.estimated += tokens
		a.promptEstimated += tokens * carried
	}
	a.occurrences++
	if isError {
		a.errors++
	}
}

// Blame attributes prompt tokens to content labels.
//
// Assistant output is measured: thinking blocks get the reported thinking
// tokens and the remaining output blocks share the rest of the reported
// output tokens by bytes. User-side content of each turn shares the turn's
// reported new-content tokens by bytes, so per-turn sums match provider
// usage exactly and only the split within a turn is estimated.
func Blame(cal *Calibration, fit TokenFit) []BlameEntry {
	byLabel := make(map[string]*labelAcc)
	get := func(label string) *labelAcc {
		a, ok := byLabel[label]
		if !ok {
			a = &labelAcc{}
			byLabel[label] = a
		}
		return a
	}

	requests := cal.Lane.Requests
	total := len(requests)
	if total == 0 {
		return nil
	}

	first := requests[0]
	var firstBlocks []transcript.Block
	for _, m := range first.Context {
		firstBlocks = append(firstBlocks, m.Blocks...)
	}
	visible := first.Usage.PromptTotal() - fit.UnseenPrefixTokens - fit.InjectedTokens
	shareByBytes(firstBlocks, visible, total, false, get)
	if fit.UnseenPrefixTokens > 0 {
		get(unseenPrefixLabel).add(fit.UnseenPrefixTokens, total, fit.UnseenPrefixMeasured, false)
	}
	if fit.InjectedTokens > 0 {
		get(injectedLabel).add(fit.InjectedTokens, total, false, false)
	}

	for _, t := range cal.Turns {
		if t.Outcome == cachemodel.ReadFirst {
			continue
		}
		carried := total - t.Index
		tc := splitTurn(t)
		attributeOutput(t.Previous, carried, get)
		var userBlocks []transcript.Block
		for _, m := range t.Request.Context[min(len(t.Previous.Context), len(t.Request.Context)):] {
			if m.Role == transcript.RoleUser {
				userBlocks = append(userBlocks, m.Blocks...)
			}
		}
		shareByBytes(userBlocks, tc.userTokens, carried, false, get)
	}

	entries := make([]BlameEntry, 0, len(byLabel))
	for label, a := range byLabel {
		entries = append(entries, BlameEntry{
			Label:        label,
			Tokens:       fit.Estimate(a.measured, a.estimated),
			Occurrences:  a.occurrences,
			PromptTokens: fit.Estimate(a.promptMeasured, a.promptEstimated),
			Errors:       a.errors,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].PromptTokens.Value != entries[j].PromptTokens.Value {
			return entries[i].PromptTokens.Value > entries[j].PromptTokens.Value
		}
		return entries[i].Label < entries[j].Label
	})
	return entries
}

// attributeOutput credits a request's output blocks with its reported
// output tokens: the thinking blocks share the reported thinking tokens,
// the remaining blocks share the rest by bytes. An output with no thinking
// block keeps every output token on its visible blocks.
func attributeOutput(req *transcript.Request, carried int, get func(string) *labelAcc) {
	if req.Output == nil {
		return
	}
	var thinking, rest []transcript.Block
	for _, b := range req.Output.Blocks {
		if b.Kind == transcript.KindThinking {
			thinking = append(thinking, b)
		} else {
			rest = append(rest, b)
		}
	}
	thinkingTokens := req.Usage.ThinkingTokens
	if len(thinking) == 0 {
		thinkingTokens = 0
	}
	shareEqually(thinking, thinkingTokens, carried, get)
	shareByBytes(rest, req.Usage.Output-thinkingTokens, carried, true, get)
}

// shareEqually splits measured tokens evenly across blocks whose bytes say
// nothing about their size (thinking blocks with omitted text).
func shareEqually(blocks []transcript.Block, tokens int, carried int, get func(string) *labelAcc) {
	if tokens <= 0 || len(blocks) == 0 {
		return
	}
	remaining := tokens
	for i, b := range blocks {
		share := tokens / len(blocks)
		if i == len(blocks)-1 {
			share = remaining
		}
		remaining -= share
		get(b.Label).add(share, carried, true, false)
	}
}

// shareByBytes splits tokens across blocks in proportion to bytes. The last
// block absorbs rounding so the sum is exact. carried is how many requests
// carry these blocks in their prompt, this one included.
func shareByBytes(blocks []transcript.Block, tokens int, carried int, measured bool, get func(string) *labelAcc) {
	if tokens <= 0 || len(blocks) == 0 {
		return
	}
	totalBytes := 0
	for _, b := range blocks {
		totalBytes += b.Bytes
	}
	remaining := tokens
	for i, b := range blocks {
		share := 0
		if totalBytes > 0 {
			share = tokens * b.Bytes / totalBytes
		}
		if i == len(blocks)-1 {
			share = remaining
		}
		remaining -= share
		get(b.Label).add(share, carried, measured, b.IsError)
	}
}
