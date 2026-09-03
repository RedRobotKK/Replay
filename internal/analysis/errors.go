package analysis

import (
	"strings"

	"github.com/RedRobotKK/Buffy/internal/transcript"
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
func ErrorCosts(cal *Calibration, fit TokenFit) []ErrorCost {
	requests := cal.Lane.Requests
	if len(requests) == 0 {
		return nil
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
					class, ok := classifyResult(b)
					if ok {
						size := Estimated(fit.EstimateTokens(b.Bytes + callBytes[b.ToolUseID]))
						get(class).add(size, carried, true)
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
	return out
}

// classifyResult decides which error class a tool result belongs to. Text
// is only lower-cased when a class could match, so large results are not
// copied for nothing.
func classifyResult(b transcript.Block) (ErrorClass, bool) {
	if !b.IsError && len(b.Text) >= overflowMarkerMaxBytes {
		return "", false
	}
	lower := strings.ToLower(b.Text)
	switch {
	case b.IsError && containsAny(lower, editAnchorMarkers):
		return ErrorEditAnchor, true
	case b.IsError:
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
