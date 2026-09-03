package analysis

import (
	"strings"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// readTools are the tools whose result is a file the agent already had if
// it read the same path before. Names cover the clients the parser knows.
var readTools = map[string]bool{"Read": true, "Grep": true, "Glob": true, "NotebookRead": true, "view": true, "read_file": true}

// ReReads measures how often the agent reads a file it already read in
// the same session. It is the guardrail for context editing: clearing old
// tool results saves prompt tokens only if the agent does not fetch them
// again, so the rate after the provider's first clear is reported next to
// the rate before it.
type ReReads struct {
	Reads    int `json:"reads"`
	Repeated int `json:"repeated"`
	// Tokens is the estimated prompt cost of the repeated reads: their
	// size times the requests that carried them.
	Tokens Figure `json:"-"`
	// ContextEdits and ClearedTokens are the provider's applied edits
	// across the lane, zero for transcripts.
	ContextEdits  int `json:"context_edits,omitempty"`
	ClearedTokens int `json:"cleared_tokens,omitempty"`
	// ReadsAfterClear and RepeatedAfterClear cover requests after the
	// first applied edit.
	ReadsAfterClear    int `json:"reads_after_clear,omitempty"`
	RepeatedAfterClear int `json:"repeated_after_clear,omitempty"`
}

// Rate is repeated reads over reads, zero when nothing was read.
func (r ReReads) Rate() float64 {
	if r.Reads == 0 {
		return 0
	}
	return float64(r.Repeated) / float64(r.Reads)
}

// RateAfterClear is the rate over requests after the first applied edit.
func (r ReReads) RateAfterClear() float64 {
	if r.ReadsAfterClear == 0 {
		return 0
	}
	return float64(r.RepeatedAfterClear) / float64(r.ReadsAfterClear)
}

// RateBeforeClear is the rate over requests up to the first applied edit.
func (r ReReads) RateBeforeClear() float64 {
	reads := r.Reads - r.ReadsAfterClear
	if reads == 0 {
		return 0
	}
	return float64(r.Repeated-r.RepeatedAfterClear) / float64(reads)
}

// CountReReads walks each request's new messages in order and counts
// file reads whose label (tool and path) already appeared in the session.
func CountReReads(cal *Calibration, fit TokenFit) ReReads {
	requests := cal.Lane.Requests
	var out ReReads
	seen := map[string]bool{}
	carriedBy := blockCarryCounts(requests)
	var tokens Tokens
	prevLen := 0
	cleared := false
	for _, req := range requests {
		if req.AppliedEdits > 0 {
			out.ContextEdits += req.AppliedEdits
			out.ClearedTokens += req.ClearedTokens
		}
		start := min(prevLen, len(req.Context))
		for _, m := range req.Context[start:] {
			for bi, b := range m.Blocks {
				if b.Kind != transcript.KindToolResult || !isFileRead(b.ToolName) {
					continue
				}
				out.Reads++
				if cleared {
					out.ReadsAfterClear++
				}
				if !seen[b.ToolName] {
					seen[b.ToolName] = true
					continue
				}
				out.Repeated++
				if cleared {
					out.RepeatedAfterClear++
				}
				tokens = tokens.Add(Estimated(fit.EstimateTokens(b.Bytes)).Times(carriedBy[blockKey{m.UUID, bi}]))
			}
		}
		prevLen = len(req.Context)
		// The response to this request is what applied the edit, so later
		// requests are the ones that saw a cleared context.
		if req.AppliedEdits > 0 {
			cleared = true
		}
	}
	out.Tokens = fit.Figure(tokens)
	return out
}

// isFileRead reports whether a tool-result label names a read of a path:
// a read tool followed by the path label the client or the ledger gave it.
func isFileRead(label string) bool {
	name, path, ok := strings.Cut(label, " ")
	return ok && path != "" && readTools[name]
}
