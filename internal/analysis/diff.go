package analysis

import (
	"fmt"
	"math"
	"time"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// BreakCause classifies why a cached prefix stopped matching.
type BreakCause string

// Causes, from most to least specific.
const (
	CauseTTLExpired   BreakCause = "cache expired (gap longer than the TTL)"
	CauseModelChanged BreakCause = "model changed between requests"
	CauseEffortChange BreakCause = "effort or thinking setting changed"
	CauseHistoryEdit  BreakCause = "an earlier message was edited or removed"
	CauseRerendered   BreakCause = "client re-rendered history after the system prefix (no edit visible in transcript)"
	CausePrefixChange BreakCause = "system prompt or tool definitions changed (not visible in transcripts)"
	CauseUnknown      BreakCause = "prefix diverged inside the message history at an unknown block"
)

// Break describes one turn where the cache read fell short.
type Break struct {
	Turn Turn
	// Deficit is how many tokens were re-written that should have been read.
	Deficit int
	Cause   BreakCause
	// MessageIndex is the context position where divergence is located, or
	// -1 when it cannot be placed.
	MessageIndex int
	// Label describes the block at MessageIndex when known.
	Label string
	// Detail carries the evidence for the classification.
	Detail string
}

// rerenderTolerance is how far a read may sit above the unseen-prefix
// estimate and still be classified as "everything after the system prefix
// was re-billed". The fit's own error would be more precise but the unseen
// prefix estimate is coarse by nature.
const rerenderTolerance = 0.10

// FindBreaks classifies every broken turn of a calibrated lane.
func FindBreaks(cal *Calibration, fit TokenFit) []Break {
	var breaks []Break
	for _, t := range cal.Turns {
		if t.Outcome != cachemodel.ReadBroken {
			continue
		}
		breaks = append(breaks, classify(t, fit))
	}
	return breaks
}

func classify(t Turn, fit TokenFit) Break {
	b := Break{Turn: t, Deficit: t.Expected - t.Actual, MessageIndex: -1}
	prev, cur := t.Previous, t.Request

	ttl := cachemodel.TTLOf(prev.Usage)
	switch {
	case t.Gap > ttl:
		b.Cause = CauseTTLExpired
		b.Detail = fmt.Sprintf("gap %s exceeds TTL %s", t.Gap.Round(time.Second), ttl)
		return b
	case prev.Model != cur.Model:
		b.Cause = CauseModelChanged
		b.Detail = fmt.Sprintf("%s -> %s", prev.Model, cur.Model)
		return b
	case prev.Effort != cur.Effort && prev.Effort != "" && cur.Effort != "":
		b.Cause = CauseEffortChange
		b.Detail = fmt.Sprintf("%s -> %s", prev.Effort, cur.Effort)
		return b
	}

	if idx, label, ok := firstDivergence(prev.Context, cur.Context); ok {
		b.Cause = CauseHistoryEdit
		b.MessageIndex = idx
		b.Label = label
		b.Detail = fmt.Sprintf("message %d differs from the previous request (%s)", idx, label)
		return b
	}

	if t.Actual == 0 {
		b.Cause = CausePrefixChange
		b.Detail = "nothing was read; the divergence is before the first message"
		return b
	}

	unseen := float64(fit.UnseenPrefixTokens)
	if unseen > 0 && math.Abs(float64(t.Actual)-unseen) <= unseen*rerenderTolerance {
		b.Cause = CauseRerendered
		b.MessageIndex = 0
		if len(cur.Context) > 0 && len(cur.Context[0].Blocks) > 0 {
			b.Label = cur.Context[0].Blocks[0].Label
		}
		b.Detail = fmt.Sprintf("read %d tokens, about the size of the system prefix (%d); the message history was re-billed from the first message", t.Actual, fit.UnseenPrefixTokens)
		return b
	}

	b.Cause = CauseUnknown
	b.MessageIndex, b.Label = locateByTokens(cur.Context, t.Actual-fit.UnseenPrefixTokens, fit)
	b.Detail = fmt.Sprintf("read %d of %d expected tokens; position is an estimate from the byte-to-token fit", t.Actual, t.Expected)
	return b
}

// firstDivergence finds the first context position where the two requests'
// histories differ within their shared length. Identical histories return
// ok=false.
func firstDivergence(prev, cur []*transcript.Message) (int, string, bool) {
	n := min(len(prev), len(cur))
	for i := 0; i < n; i++ {
		if prev[i].UUID != cur[i].UUID || prev[i].Bytes() != cur[i].Bytes() {
			return i, firstLabel(cur[i]), true
		}
	}
	if len(cur) < len(prev) {
		return len(cur), "history truncated", true
	}
	return -1, "", false
}

// locateByTokens walks the context until the estimated token offset is
// reached and returns that message's index and label.
func locateByTokens(ctx []*transcript.Message, tokens int, fit TokenFit) (int, string) {
	if tokens <= 0 {
		return 0, firstLabelOrEmpty(ctx, 0)
	}
	acc := 0
	for i, m := range ctx {
		acc += fit.EstimateTokens(m.Bytes())
		if acc >= tokens {
			return i, firstLabel(m)
		}
	}
	return len(ctx) - 1, firstLabelOrEmpty(ctx, len(ctx)-1)
}

func firstLabel(m *transcript.Message) string {
	if len(m.Blocks) == 0 {
		return m.Role
	}
	return m.Blocks[0].Label
}

func firstLabelOrEmpty(ctx []*transcript.Message, i int) string {
	if i < 0 || i >= len(ctx) {
		return ""
	}
	return firstLabel(ctx[i])
}
