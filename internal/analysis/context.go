package analysis

import (
	"sort"
	"strings"
	"unicode"
)

// ContextEntry is one tool's share of what entered a session's context.
//
// The name is the honest one and it is load-bearing. Blame never subtracts:
// labelAcc only ever adds, and neither Blame nor the fit consults
// transcript.Request.ClearedTokens or AppliedEdits, though both exist and
// rereads.go reads them. So a block cleared by provider context editing, by
// compaction, or by a history-shrinking break is still counted here.
//
// The divergence is always in the over-reporting direction, and it is largest
// on exactly the sessions where this tool's own leading recommendation has been
// taken. Calling this "what is in your context" would be wrong in the one case
// the reader most cares about, so it is not called that anywhere.
type ContextEntry struct {
	// Label is a tool name, or a block kind for content that is not a tool.
	Label string
	// Tokens is the size of everything carrying this label, counted once each.
	Tokens int
	// Share is Tokens over the attributed total, not over the context window:
	// the window is not known here and claiming it would be a guess.
	Share float64
	// Occurrences is how many blocks carried this label.
	Occurrences int
	// Estimated is true when any contributing figure came from the
	// byte-to-token fit rather than from provider usage.
	Estimated bool
}

// MaxContextLabel bounds a rendered label. Labels are built from tool
// arguments, so they are model and tool supplied; observed labels reach 424
// runes against a 400-rune construction cap, because the prefix is added after
// truncation.
const MaxContextLabel = 48

// EnteredContext turns blame rows into a ranked attribution of what entered the
// context, grouped by tool.
//
// Ranked by Tokens, not PromptTokens. PromptTokens is a cost integral: a small
// file read on turn one is carried by every later request and dominates it
// while occupying almost no context. Tokens answers the question actually being
// asked.
func EnteredContext(entries []BlameEntry) []ContextEntry {
	byTool := map[string]*ContextEntry{}
	var order []string
	for _, e := range entries {
		// The rebill row is tokens the provider charged again after a cache
		// break. They are a cost, not content, and including them would put a
		// row describing absent content at the top of the list.
		if e.Label == RebillLabel {
			continue
		}
		name := toolNameOf(e.Label)
		cur, ok := byTool[name]
		if !ok {
			cur = &ContextEntry{Label: name}
			byTool[name] = cur
			order = append(order, name)
		}
		cur.Tokens += e.Tokens.Value
		cur.Occurrences += e.Occurrences
		// A non-zero error bar is the mark of a figure derived through the
		// byte-to-token fit rather than read from provider usage.
		cur.Estimated = cur.Estimated || e.Tokens.Error > 0
	}

	total := 0
	out := make([]ContextEntry, 0, len(order))
	for _, name := range order {
		total += byTool[name].Tokens
	}
	for _, name := range order {
		e := *byTool[name]
		if total > 0 {
			e.Share = float64(e.Tokens) / float64(total)
		}
		out = append(out, e)
	}
	// Stable: ties keep the order Blame produced, which is itself sorted.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Tokens > out[j].Tokens })
	return out
}

// toolNameOf reduces a blame label to the tool it belongs to.
//
// transcript.ToolLabel embeds the invocation, so "tool result: Bash gh pr
// create --base main" and "tool result: Bash npm test" are distinct labels.
// Real sessions carry 370 to 611 of them; grouped, a few dozen. Without this a
// single long-argument invocation outranks every other call to the same tool
// combined.
func toolNameOf(label string) string {
	s := label
	for _, prefix := range []string{"tool result: ", "tool call: "} {
		s = strings.TrimPrefix(s, prefix)
	}
	if i := strings.IndexByte(s, ' '); i > 0 {
		s = s[:i]
	}
	return safeLabel(s)
}

// safeLabel strips control bytes and bounds the width of a label that reaches a
// terminal. Labels are sanitised where they are constructed, but a renderer
// must not depend on that: this one is reached through a report, and a report
// can be read from a file.
func safeLabel(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == 0x1b || unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	out := []rune(b.String())
	if len(out) > MaxContextLabel {
		return string(out[:MaxContextLabel-1]) + "…"
	}
	return string(out)
}
