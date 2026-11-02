package analysis

import (
	"strings"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// ErrorClass names one kind of wasted work visible in a session.
type ErrorClass string

// Error classes. Provider-level retries are not visible in transcripts and
// are reported as such rather than as zero.
const (
	ErrorToolFailed      ErrorClass = "tool results flagged as errors"
	ErrorEditAnchor      ErrorClass = "failed edits (anchor text not found)"
	ErrorRepeatedCommand ErrorClass = "identical tool call repeated"
	ErrorContextOverflow ErrorClass = "context overflow or compaction"
)

// ErrorCost is the estimated prompt cost of one error class in a lane.
type ErrorCost struct {
	Class ErrorClass
	Count int
	// Tokens is the estimated size of the error content itself plus the
	// tool call that produced it, counted once.
	Tokens Figure
	// PromptTokens is the size multiplied by the requests that carried it.
	PromptTokens Figure
}

// ErrorEvidence summarizes how a lane's error classification was established.
//
// It exists so the evidence-aware path can distinguish a verified clean block
// from an occurrence whose is_error evidence conflicted (the duplicate-key
// trap), and from a block classified only from the collapsed decoded bool.
// A consumer that sums ErrorCost.Count alone would silently drop the
// distinction, so this travels beside the costs.
type ErrorEvidence struct {
	// Unresolved counts tool_result occurrences whose is_error evidence was
	// ambiguous and were counted as neither error nor clean.
	Unresolved int
	// Legacy counts tool_result occurrences classified from the decoded
	// Block.IsError because no raw evidence was available. They are
	// observations, not forensically verified occurrences.
	Legacy int
}

// Substrings the client uses in edit failures. They are matched on the
// tool result text, case-insensitively.
var editAnchorMarkers = []string{"string to replace not found", "not found in file", "old_string", "no match found"}

var overflowMarkers = []string{"prompt is too long", "context window", "compact"}

// overflowMarkerMaxBytes bounds how long a tool result may be and still be
// read as an overflow notice rather than content that merely mentions one.
const overflowMarkerMaxBytes = 400

// blockKey identifies one block across requests.
type blockKey struct {
	messageUUID string
	index       int
}

// ErrorCosts classifies error content across a lane and prices it with the
// fit. Repeated identical tool calls are counted from the second occurrence,
// keyed by the call's content-free identity so ledger data works too.
func ErrorCosts(cal *Calibration, fit TokenFit) ([]ErrorCost, ErrorEvidence) {
	requests := cal.Lane.Requests
	if len(requests) == 0 {
		return nil, ErrorEvidence{}
	}
	counts := map[ErrorClass]*labelAcc{}
	get := func(c ErrorClass) *labelAcc {
		a, ok := counts[c]
		if !ok {
			a = &labelAcc{}
			counts[c] = a
		}
		return a
	}
	// Every block is classified once, the first time any request carries
	// it, and priced by how many requests carry that exact block. Walking
	// each request's own context (rather than assuming the last request
	// holds the whole history) keeps the count right after a compaction or
	// a rewind removed blocks from later contexts.
	carriedBy := blockCarryCounts(requests)
	seenBlocks := map[blockKey]bool{}
	seenCalls := map[string]bool{}
	callBytes := map[string]int{}
	var ev ErrorEvidence
	for _, req := range requests {
		for _, m := range req.Context {
			for bi, b := range m.Blocks {
				key := blockKey{m.UUID, bi}
				if seenBlocks[key] {
					continue
				}
				seenBlocks[key] = true
				carried := carriedBy[key]
				switch b.Kind {
				case transcript.KindToolUse:
					callBytes[b.ToolUseID] = b.Bytes
					if b.CallKey != "" && seenCalls[b.CallKey] {
						get(ErrorRepeatedCommand).add(Estimated(fit.EstimateTokens(b.Bytes)), carried, false)
					}
					seenCalls[b.CallKey] = true
				case transcript.KindToolResult:
					isError, isErrorKnown := evidenceErrorState(b)
					switch {
					case isErrorKnown && isError:
						class, ok := classifyResult(b, true)
						if ok {
							size := Estimated(fit.EstimateTokens(b.Bytes + callBytes[b.ToolUseID]))
							get(class).add(size, carried, true)
						}
					case isErrorKnown:
						// Verified not-error, ambiguous conflict, or present
						// non-boolean. All are neither error nor clean: count
						// them as unresolved, never classify them.
						if b.Evidence != nil && (*b.Evidence == transcript.ErrorFlagConflict || *b.Evidence == transcript.ErrorFlagNonBoolean) {
							ev.Unresolved++
						}
					default:
						// Legacy: no raw evidence, use the decoded bool.
						ev.Legacy++
						class, ok := classifyResult(b, b.IsError)
						if ok {
							size := Estimated(fit.EstimateTokens(b.Bytes + callBytes[b.ToolUseID]))
							get(class).add(size, carried, true)
						}
					}
				}
			}
		}
	}
	order := []ErrorClass{ErrorEditAnchor, ErrorToolFailed, ErrorRepeatedCommand, ErrorContextOverflow}
	var out []ErrorCost
	for _, c := range order {
		a, ok := counts[c]
		if !ok {
			continue
		}
		out = append(out, ErrorCost{Class: c, Count: a.occurrences, Tokens: fit.Figure(a.once), PromptTokens: fit.Figure(a.prompt)})
	}
	return out, ev
}

// evidenceErrorState reports the evidence-backed error state of a tool_result
// block. The second return is false when no raw evidence was available (the
// legacy decoded path), in which case the first is meaningless.
func evidenceErrorState(b transcript.Block) (isError, known bool) {
	if b.Evidence == nil {
		return false, false
	}
	switch *b.Evidence {
	case transcript.ErrorFlagTrue:
		return true, true
	case transcript.ErrorFlagFalse, transcript.ErrorFlagAbsent:
		return false, true
	case transcript.ErrorFlagConflict, transcript.ErrorFlagNonBoolean:
		// Ambiguous or present-non-boolean: not clean, not error. Reported
		// as Unresolved by ErrorCosts.
		return false, true
	default:
		return false, false
	}
}

// classifyResult decides which error class a tool result belongs to. Text is
// only lower-cased when a class could match, so large results are not copied
// for nothing. isError is the evidence-aware error flag supplied by the caller
// (verified or legacy), so a conflicting duplicate is never read as clean.
func classifyResult(b transcript.Block, isError bool) (ErrorClass, bool) {
	if !isError && len(b.Text) >= overflowMarkerMaxBytes {
		return "", false
	}
	lower := strings.ToLower(b.Text)
	switch {
	case isError && containsAny(lower, editAnchorMarkers):
		return ErrorEditAnchor, true
	case isError:
		return ErrorToolFailed, true
	case containsAny(lower, overflowMarkers):
		return ErrorContextOverflow, true
	default:
		return "", false
	}
}

// blockCarryCounts counts, for every block, how many requests carried it.
func blockCarryCounts(requests []*transcript.Request) map[blockKey]int {
	counts := map[blockKey]int{}
	for _, r := range requests {
		for _, m := range r.Context {
			for bi := range m.Blocks {
				counts[blockKey{m.UUID, bi}]++
			}
		}
	}
	return counts
}

func containsAny(s string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
