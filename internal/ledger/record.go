// Package ledger stores what the proxy observed about each request as
// derived data: block kinds, sizes, labels, timings, and provider usage.
// It never stores message text, headers, or credentials.
//
// The block and usage types are the transcript package's own, whose JSON
// tags define exactly the content-free subset that is persisted. One JSONL
// file per client session lives under the ledger directory, and the reader
// turns those files back into transcript sessions so the analysis commands
// work on measured data exactly as they do on transcripts.
package ledger

import (
	"time"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// SchemaVersion is written on every record so a future reader can tell
// what it is looking at. Bump it on any incompatible change.
const SchemaVersion = 2

// Block is the transcript block type; its Text is never serialized.
type Block = transcript.Block

// Usage is the transcript usage type, serialized with the provider's names.
type Usage = transcript.Usage

// Record is one proxied request and its response.
type Record struct {
	Schema    int       `json:"schema"`
	Timestamp time.Time `json:"ts"`
	// SessionID is the client-supplied session header, or a hash of the
	// stable prefix when the client sent none.
	SessionID string `json:"session_id"`
	// AgentID is the client-supplied sub-agent header, empty for the main loop.
	AgentID string `json:"agent_id,omitempty"`
	// RequestID is the provider's request id from the response headers.
	RequestID string `json:"request_id,omitempty"`
	Path      string `json:"path"`
	RequestSummary
	Status    int   `json:"status"`
	LatencyMS int64 `json:"latency_ms"`
	// Response is the structure of what the provider returned. Usage is
	// absent on error responses and on endpoints that report none.
	Response Response `json:"response"`
	// Cache is the proxy's live classification of this response's cache
	// read against the previous request in the session, when known.
	Cache *CacheOutcome `json:"cache,omitempty"`
}

// RequestSummary is what the proxy learns from a request body: its
// structure and the attributes the analysis keys on. Embedded in Record so
// the fields serialize flat.
type RequestSummary struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
	Stream bool   `json:"stream"`
	Prompt Prompt `json:"prompt"`
	// PrefixHash identifies the session by its stable prefix (system prompt
	// and first message) when the client sends no session header. It
	// contains no content.
	PrefixHash string `json:"-"`
}

// Prompt is the request body reduced to structure.
type Prompt struct {
	SystemBytes int `json:"system_bytes"`
	// ToolBytes is the decoded size of the tool definitions; ToolCount how many.
	ToolBytes int `json:"tool_bytes"`
	ToolCount int `json:"tool_count"`
	// CacheControlCount is how many cache markers the client placed.
	CacheControlCount int       `json:"cache_control"`
	Messages          []Message `json:"messages"`
	// ContextEdits is true when the client sent its own context editing.
	ContextEdits bool `json:"context_edits,omitempty"`
}

// Message is one message reduced to its blocks.
type Message struct {
	Role   string  `json:"role"`
	Blocks []Block `json:"blocks"`
}

// Response is the reply reduced to structure and usage.
type Response struct {
	Blocks []Block `json:"blocks,omitempty"`
	Usage  *Usage  `json:"usage,omitempty"`
}

// CacheOutcome is the live classification of one response's cache read.
type CacheOutcome struct {
	Outcome  string                `json:"outcome"`
	Expected int                   `json:"expected,omitempty"`
	Deficit  int                   `json:"deficit,omitempty"`
	Cause    cachemodel.BreakCause `json:"cause,omitempty"`
}
