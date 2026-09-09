package analysis

import (
	"sort"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// BlameEntry aggregates attributed tokens for one label across a lane.
type BlameEntry struct {
	Label string
	// Tokens is the size of all blocks with this label, counted once each.
	Tokens Figure
	// Occurrences is how many blocks carried the label.
	Occurrences int
	// PromptTokens is the total contribution to prompts across the lane:
	// each block's size multiplied by the number of requests that carried it.
	PromptTokens Figure
	// Errors counts blocks flagged as tool errors.
	Errors int
}

// RebillLabel names tokens re-written because a cache break forced the
// provider to process history again.
const RebillLabel = "cache breaks: history re-billed (see replay diff)"

// UnaccountedLabel names tokens a turn was billed that the blocks visible in
// that turn cannot account for.
//
// Almost always a shared prefix that grew where the transcript cannot see it:
// a second tool-definition re-lay, or an MCP server binding late. The provider
// billed the write, the transcript shows only whatever the agent was doing at
// the time, and shareByBytes used to hand the whole write to that. On session
// 7022b9f2 it charged 21k tokens to a `Bash echo one` result whose content is
// three bytes.
//
// It does not say where the tokens belong, because the transcript does not
// contain that. It says they are not the visible block's, which is the part
// that was being asserted wrongly.
// The first word matters: `replay context` groups labels by it, so this
// reads as "unaccounted" beside "system" and "Bash" rather than as "billed".
const UnaccountedLabel = "unaccounted: prefix grew where the transcript cannot see it"

// labelAcc accumulates attribution for one label.
type labelAcc struct {
	once, prompt        Tokens
	occurrences, errors int
}

func (a *labelAcc) add(t Tokens, carried int, isError bool) {
	a.once = a.once.Add(t)
	a.prompt = a.prompt.Add(t.Times(carried))
	a.occurrences++
	if isError {
		a.errors++
	}
}

// Blame attributes prompt tokens to content labels.
//
// Assistant output is measured: thinking blocks share the reported thinking
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
	seen := make(map[string]bool)
	var firstBlocks []transcript.Block
	for _, m := range first.Context {
		seen[m.UUID] = true
		firstBlocks = append(firstBlocks, m.Blocks...)
	}
	visible := first.Usage.PromptTotal() - fit.UnseenPrefix.Total() - fit.Injected.Total()
	shareByBytes(firstBlocks, Estimated(visible), total, get)
	if fit.UnseenPrefix.Total() > 0 {
		get(UnseenPrefixLabel).add(fit.UnseenPrefix, total, false)
	}
	if fit.Injected.Total() > 0 {
		get(InjectedLabel).add(fit.Injected, total, false)
	}

	for _, t := range cal.Turns {
		if t.Outcome == cachemodel.ReadFirst {
			markSeen(seen, t.Request)
			continue
		}
		carried := total - t.Index
		tc := splitTurn(t, seen)
		if tc.rebillTokens > 0 {
			// A break re-bills history that is already attributed to its
			// own labels; it is reported as its own line so the table
			// names the break, not the content that happened to follow it.
			get(RebillLabel).add(Measured(tc.rebillTokens), 1, false)
		}
		attributeOutput(t.Previous, carried, get)
		// Every block new to the lane, not only the user-role ones: when
		// tool definitions bind late the new content is a system-role
		// prefix message, and sharing this turn's write across user blocks
		// alone drops it on an empty list.
		shareWithinTheFit(tc.blocks, tc.newTokens, carried, fit, get)
	}

	entries := make([]BlameEntry, 0, len(byLabel))
	for label, a := range byLabel {
		entries = append(entries, BlameEntry{
			Label:        label,
			Tokens:       fit.Figure(a.once),
			Occurrences:  a.occurrences,
			PromptTokens: fit.Figure(a.prompt),
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
// output tokens: the thinking blocks share the reported thinking tokens
// equally (their bytes say nothing about their size), the remaining blocks
// share the rest by bytes. An output with no thinking block keeps every
// output token on its visible blocks.
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
	remaining := thinkingTokens
	for i, b := range thinking {
		share := thinkingTokens / len(thinking)
		if i == len(thinking)-1 {
			share = remaining
		}
		remaining -= share
		get(b.Label).add(Measured(share), carried, false)
	}
	shareByBytes(rest, Measured(req.Usage.Output-thinkingTokens), carried, get)
}

// shareByBytes splits tokens across blocks in proportion to bytes. The last
// block absorbs rounding so the sum is exact. carried is how many requests
// carry these blocks in their prompt, this one included.
// shareWithinTheFit attributes a turn's write to the blocks visible in it, up
// to what those bytes can be worth, and names the rest.
//
// The allowance is the fit's own claim about itself: accountable bytes at the
// fitted ratio, widened by the relative error it reports. For a well-fitted
// session that is a tight band; for an unfitted one RelativeError is 1, so the
// band is exactly double, which is deliberately generous. No new constant is
// introduced, and a turn whose content is denser than the ratio expects keeps
// its attribution as long as it stays inside the uncertainty the fit already
// publishes.
//
// Beyond that band the excess is not evidence about the visible blocks. It is
// reported under UnaccountedLabel rather than shared, because charging it to a
// three-byte tool result is an assertion the bytes cannot support — and the
// figure it produced, 21k tokens for `echo one`, went on to be the sole input
// to advisor.Observe.
func shareWithinTheFit(blocks []transcript.Block, newTokens, carried int, fit TokenFit, get func(string) *labelAcc) {
	if newTokens <= 0 || len(blocks) == 0 {
		shareByBytes(blocks, Estimated(newTokens), carried, get)
		return
	}
	bytes := 0
	for _, b := range blocks {
		bytes += b.Bytes
	}
	accountable := fit.EstimateTokens(bytes)
	allowance := int(float64(accountable) * (1 + fit.RelativeError))
	if newTokens <= allowance {
		shareByBytes(blocks, Estimated(newTokens), carried, get)
		return
	}
	shareByBytes(blocks, Estimated(accountable), carried, get)
	get(UnaccountedLabel).add(Estimated(newTokens-accountable), carried, false)
}

func shareByBytes(blocks []transcript.Block, tokens Tokens, carried int, get func(string) *labelAcc) {
	total := tokens.Total()
	if total <= 0 || len(blocks) == 0 {
		return
	}
	totalBytes := 0
	for _, b := range blocks {
		totalBytes += b.Bytes
	}
	remaining := total
	for i, b := range blocks {
		share := 0
		if totalBytes > 0 {
			share = total * b.Bytes / totalBytes
		}
		if i == len(blocks)-1 {
			share = remaining
		}
		remaining -= share
		part := Estimated(share)
		if tokens.Measured > 0 {
			part = Measured(share)
		}
		get(b.Label).add(part, carried, b.IsError)
	}
}
