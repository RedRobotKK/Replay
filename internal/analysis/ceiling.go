package analysis

import (
	"fmt"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// ContextCeilingStatus names why a model's observed context ceiling is
// available, or exactly why it is withheld. Panel decision, 2026-10-05
// (docs/evidence/compaction-panel-2026-10-05.md): a ceiling is reported
// after it clears the gate, never guessed to fill a gap.
type ContextCeilingStatus int

const (
	// ContextCeilingTooFewRecords means fewer than minContextCeilingRecords
	// compactions have been recorded for this model on this machine. A
	// model with zero recorded compactions is absent from the table
	// entirely, rather than carrying this status at N=0: the panel's three
	// named states are "too few, but some", "ambiguous", and "none at all",
	// and collapsing the first and third into one status would lose the
	// distinction a reader needs.
	ContextCeilingTooFewRecords ContextCeilingStatus = iota
	// ContextCeilingAmbiguousTier means this model's recorded pre-compaction
	// sizes split into widely separated clusters, the way a single model id
	// did on this machine (165k and 961k under "claude-fable-5-1", named in
	// docs/evidence/capability-map-2026-10-05.md): the id does not name one
	// tier here, so no single figure is reported as the ceiling.
	ContextCeilingAmbiguousTier
	// ContextCeilingAvailable means at least minContextCeilingRecords
	// compactions were recorded for this model, clustered tightly enough
	// that Max is reported as the observed ceiling.
	ContextCeilingAvailable
)

// minContextCeilingRecords is the panel's gate: a ceiling is reported only
// after at least this many compactions have been recorded for the model, on
// the reader's own machine. Fewer than this, and the status is
// ContextCeilingTooFewRecords instead.
const minContextCeilingRecords = 10

// ambiguousTierRatio is how far apart the smallest and largest recorded
// pre-compaction sizes for a model can be before they are read as two
// different tiers rather than noise around one. Chosen against the two real
// cases on this machine: claude-opus-5's cluster spans a ratio of about
// 1.04 (963,705 to 998,021) and is not ambiguous; claude-fable-5-1's two
// clusters span a ratio of about 5.84 (165,514 to 965,989) and are. A ratio
// of 2 sits between them with room on both sides.
const ambiguousTierRatio = 2.0

// ContextCeiling is one model's observed context ceiling on this machine, or
// the reason none is reported.
type ContextCeiling struct {
	Status ContextCeilingStatus
	// Max is the largest recorded pre-compaction size, valid only when
	// Status is ContextCeilingAvailable.
	Max int
	// N is the number of compactions counted, carried on every status so a
	// withheld figure still says how far from the gate it sits.
	N int
}

// BuildContextCeilings pools every dated, modelled compaction across every
// session given, buckets the recorded pre-compaction sizes by model, and
// classifies each model's result. A model with zero recorded compactions is
// not a key in the returned map: absence is itself the "no compaction
// recorded" state, not a zero-value entry a caller has to know to read as
// meaningless.
//
// Nothing here predicts. This counts what the client already recorded, on
// the machine running it, and reports it or withholds it by the panel's
// named rule — it does not say what a future session will do.
func BuildContextCeilings(sessions []*transcript.Session) map[string]ContextCeiling {
	byModel := map[string][]int{}
	for _, session := range sessions {
		for _, c := range session.Compactions {
			if !c.Sized() || c.At.IsZero() {
				continue
			}
			model := modelNearCompaction(session, c.At)
			if model == "" {
				continue
			}
			byModel[model] = append(byModel[model], c.PreTokens)
		}
	}
	table := map[string]ContextCeiling{}
	for model, sizes := range byModel {
		if len(sizes) < minContextCeilingRecords {
			table[model] = ContextCeiling{Status: ContextCeilingTooFewRecords, N: len(sizes)}
			continue
		}
		min, max := sizes[0], sizes[0]
		for _, s := range sizes[1:] {
			if s < min {
				min = s
			}
			if s > max {
				max = s
			}
		}
		if min <= 0 || float64(max)/float64(min) > ambiguousTierRatio {
			table[model] = ContextCeiling{Status: ContextCeilingAmbiguousTier, N: len(sizes)}
			continue
		}
		table[model] = ContextCeiling{Status: ContextCeilingAvailable, Max: max, N: len(sizes)}
	}
	return table
}

// modelNearCompaction is the model of the latest non-sidechain request
// timestamped at or before at, or "" when none is found. Compaction carries
// no model of its own, so this is the only way to attribute a boundary to
// the model that produced it: the same lane-walk firstPromptAfter already
// uses to search forward, run backward instead.
func modelNearCompaction(session *transcript.Session, at time.Time) string {
	if at.IsZero() {
		return ""
	}
	best := time.Time{}
	model := ""
	for _, lane := range session.Lanes {
		if lane.Sidechain {
			continue
		}
		for _, r := range lane.Requests {
			if r.Timestamp.After(at) {
				continue
			}
			if r.Timestamp.After(best) {
				best = r.Timestamp
				model = r.Model
			}
		}
	}
	return model
}

// ContextCeilingLine is one event's comparison against its model's observed
// ceiling, or "" when that model has no table entry at all (which cannot
// happen for a compaction that was itself just counted into the table, but
// can happen for a caller building a line from data this package did not
// produce).
//
// Distance is expressed as a share of the observed ceiling, never of "the
// window": the window itself is not in a Claude Code transcript, and saying
// so would claim a figure nobody recorded. Every line says "estimated".
func ContextCeilingLine(model string, preTokens int, ceilings map[string]ContextCeiling) string {
	c, ok := ceilings[model]
	if !ok {
		return ""
	}
	switch c.Status {
	case ContextCeilingTooFewRecords:
		return fmt.Sprintf("observed ceiling for %s: withheld, only %s recorded here (need %d)",
			model, plural(c.N, "compaction"), minContextCeilingRecords)
	case ContextCeilingAmbiguousTier:
		return fmt.Sprintf("observed ceiling for %s: withheld, this model id shows more than one ceiling on this machine across %d compactions", model, c.N)
	case ContextCeilingAvailable:
		share := float64(preTokens) / float64(c.Max) * 100
		return fmt.Sprintf("observed ceiling for %s on this machine: %s (estimated, n=%d); this compaction's pre-compaction size was %s (%.0f%% of the observed ceiling)",
			model, formatCount(c.Max), c.N, formatCount(preTokens), share)
	}
	return ""
}

// ContextCeilingDetail returns one line per element of session.Compactions,
// in the same order CompactionEvent (and therefore CompactionDetail) uses, so
// a caller can pair line i with CompactionDetail's event i+1. An element
// contributes "" when it has no usable comparison: unsized, undated, its
// model could not be resolved, or its model has no entry in ceilings.
func ContextCeilingDetail(session *transcript.Session, ceilings map[string]ContextCeiling) []string {
	lines := make([]string, len(session.Compactions))
	for i, c := range session.Compactions {
		if !c.Sized() || c.At.IsZero() {
			continue
		}
		model := modelNearCompaction(session, c.At)
		if model == "" {
			continue
		}
		lines[i] = ContextCeilingLine(model, c.PreTokens, ceilings)
	}
	return lines
}

// formatCount is a thousands-grouped integer, matching the rest of this
// package's rendering.
func formatCount(n int) string {
	s := fmt.Sprintf("%d", n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
