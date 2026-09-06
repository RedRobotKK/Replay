package analysis

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/RedRobotKK/Replay/internal/transcript"
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

// ContextGap measures how much an attribution overstates a session, rather than
// warning that it might.
//
// Blame never subtracts, so every attribution is an upper bound. Warning about
// that on every session teaches a reader to skip the warning. The provider
// reports what it cleared on the request itself, so the sessions where the gap
// is real can be named, sized, and separated from the sessions where the
// attribution is exact.
type ContextGap struct {
	// ClearedTokens is what the provider reported removing.
	ClearedTokens int
	// ContextEdits is how many times it did so.
	ContextEdits int
	// Compactions counts history rewrites.
	Compactions int
	// CompactedTokens is how much those rewrites removed, where the client
	// recorded it. Zero means unrecorded, never "removed nothing" - the two
	// are different claims and Compactions above keeps them apart.
	CompactedTokens int
	// AttributedTokens is the total this gap applies to.
	AttributedTokens int
}

// Overstated reports whether content is known to have left this context.
func (g ContextGap) Overstated() bool {
	return g.ClearedTokens > 0 || g.ContextEdits > 0 || g.Compactions > 0 || g.CompactedTokens > 0
}

// OverstatedShare is the measured overstatement as a share of the attributed
// total, and zero when nothing measurable is available.
//
// Compaction used to be excluded here on the grounds that it "reports no
// size". It does: Claude Code writes preTokens and postTokens on every
// compaction record, and the parser was not reading them. A compaction with no
// recorded size still adds nothing, because a count is not a size - but it is
// counted in Compactions, so "compacted, size unknown" stays distinguishable
// from "compacted, dropped nothing".
func (g ContextGap) OverstatedShare() float64 {
	measured := g.ClearedTokens + g.CompactedTokens
	if measured <= 0 || g.AttributedTokens <= 0 {
		return 0
	}
	return float64(measured) / float64(g.AttributedTokens)
}

// Note is the one line a reader needs about how far to trust the figures.
func (g ContextGap) Note() string {
	if !g.Overstated() {
		return "Complete: nothing was cleared or compacted in this session, so every " +
			"block counted here is still in the context."
	}
	var b strings.Builder
	b.WriteString("OVERSTATED: content left this context and the attribution above does not subtract it.")
	if g.ClearedTokens > 0 {
		b.WriteString(" The provider cleared ")
		b.WriteString(shortCount(g.ClearedTokens))
		b.WriteString(" tokens over ")
		b.WriteString(plural(g.ContextEdits, "context edit"))
		if s := g.OverstatedShare(); s > 0 {
			b.WriteString(", so these figures overstate by at least ")
			b.WriteString(percent(s))
		}
		b.WriteString(".")
	} else if g.ContextEdits > 0 {
		b.WriteString(" The provider applied ")
		b.WriteString(plural(g.ContextEdits, "context edit"))
		b.WriteString(" without reporting a size.")
	}
	if g.Compactions > 0 {
		b.WriteString(" The history was compacted ")
		b.WriteString(plural(g.Compactions, "time"))
		if g.CompactedTokens > 0 {
			b.WriteString(", dropping ")
			b.WriteString(shortCount(g.CompactedTokens))
			b.WriteString(" tokens the client recorded")
			// A share at or above 1 is not an overstatement, it is a sign the
			// denominator is wrong: more has passed through this session than
			// is attributed, because the attribution describes what survived.
			// Printing 700% would read as a defect and discredit the absolute
			// figure standing next to it.
			if s := g.OverstatedShare(); s > 0 && s < 1 {
				b.WriteString("; these figures overstate by at least ")
				b.WriteString(percent(s))
				b.WriteString(".")
			} else {
				b.WriteString(", which is more than is attributed above: the attribution " +
					"describes what remains, not everything that passed through.")
			}
		} else {
			// The client recorded the rewrite and not its size. Distinct from
			// dropping nothing, and worth saying rather than implying.
			b.WriteString(" without recording a size, so that part of the overstatement " +
				"cannot be measured.")
		}
	}
	return b.String()
}

func shortCount(n int) string {
	switch {
	case n >= 1_000_000:
		return strconv.FormatFloat(float64(n)/1e6, 'f', 1, 64) + "M"
	case n >= 1_000:
		return strconv.Itoa(n/1000) + "k"
	}
	return strconv.Itoa(n)
}

func percent(f float64) string {
	return strconv.Itoa(int(f*100+0.5)) + "%"
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// MeasureGap reads what the provider reported clearing across a lane.
//
// The fields have been on transcript.Request all along and rereads.go already
// consumes them; the attribution never did, which is precisely why it
// overstates without knowing it.
func MeasureGap(session *transcript.Session, lane *transcript.Lane, attributed int) ContextGap {
	g := ContextGap{AttributedTokens: attributed}
	// Recorded compactions beat inferred ones. The client writes the sizes it
	// dropped, so a prompt that shrank is only evidence when nothing better is
	// on disk - and until now nothing better was ever read.
	var recorded int
	if session != nil {
		for _, c := range session.Compactions {
			recorded++
			g.CompactedTokens += c.Dropped()
		}
		g.Compactions = recorded
	}
	if lane == nil {
		return g
	}
	prev := 0
	for i, r := range lane.Requests {
		g.ClearedTokens += r.ClearedTokens
		g.ContextEdits += r.AppliedEdits
		// A prompt that shrank between turns is history leaving the context by
		// some route the provider did not report: compaction, a rewind, or a
		// resume. The size is not recoverable, only the fact.
		// Only when the client recorded nothing. A shrinking prompt is a weak
		// signal - it also fires on a rewind or a resume - so it is the
		// fallback rather than a second count added to the recorded one.
		if cur := r.Usage.PromptTotal(); recorded == 0 && i > 0 && prev > 0 && cur < prev {
			g.Compactions++
		}
		prev = r.Usage.PromptTotal()
	}
	return g
}
