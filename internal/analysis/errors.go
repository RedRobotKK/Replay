package analysis

import (
	"strings"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// ErrorClass names one kind of wasted work visible in a transcript.
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
	Tokens Estimated
	// PromptTokens is the size multiplied by the requests that carried it.
	PromptTokens Estimated
}

// Substrings the client uses in edit failures. They are matched on the
// tool result text, case-insensitively.
var editAnchorMarkers = []string{"string to replace not found", "not found in file", "old_string", "no match found"}

var overflowMarkers = []string{"prompt is too long", "context window", "compact"}

// ErrorCosts classifies error content across a lane and prices it with the
// fit. Repeated identical tool calls are counted from the second occurrence.
func ErrorCosts(cal *Calibration, fit TokenFit) []ErrorCost {
	requests := cal.Lane.Requests
	total := len(requests)
	if total == 0 {
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
	seenCalls := map[string]bool{}
	callBytes := map[string]int{}

	// Walk the longest context (the last request) so each block is seen
	// once, and price by how many requests carried it. Requests earlier in
	// the lane carry a prefix of the same history.
	last := requests[total-1]
	for i, m := range last.Context {
		carried := requestsCarrying(requests, i)
		for _, b := range m.Blocks {
			switch b.Kind {
			case transcript.KindToolUse:
				key := b.ToolName + "\x00" + b.Text
				callBytes[b.ToolUseID] = b.Bytes
				if seenCalls[key] {
					get(ErrorRepeatedCommand).add(fit.EstimateTokens(b.Bytes), carried, false, false)
				}
				seenCalls[key] = true
			case transcript.KindToolResult:
				size := fit.EstimateTokens(b.Bytes + callBytes[b.ToolUseID])
				lower := strings.ToLower(b.Text)
				switch {
				case containsAny(lower, editAnchorMarkers) && b.IsError:
					get(ErrorEditAnchor).add(size, carried, false, true)
				case b.IsError:
					get(ErrorToolFailed).add(size, carried, false, true)
				case containsAny(lower, overflowMarkers) && len(b.Text) < 400:
					get(ErrorContextOverflow).add(size, carried, false, true)
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
		out = append(out, ErrorCost{Class: c, Count: a.occurrences, Tokens: fit.Estimate(0, a.estimated), PromptTokens: fit.Estimate(0, a.promptEstimated)})
	}
	return out
}

// requestsCarrying counts requests whose context includes position i.
func requestsCarrying(requests []*transcript.Request, i int) int {
	n := 0
	for _, r := range requests {
		if len(r.Context) > i {
			n++
		}
	}
	return n
}

func containsAny(s string, markers []string) bool {
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
