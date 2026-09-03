// Package ledger stores what the proxy observed about each request as
// derived data: block kinds, sizes, labels, timings, and provider usage.
// It never stores message text, headers, or credentials.
//
// One JSONL file per client session lives under the ledger directory. The
// Reader turns those files back into transcript.Sessions so the analysis
// commands work on measured data exactly as they do on transcripts.
package ledger

import "time"

// SchemaVersion is written on every record so a future reader can tell
// what it is looking at. Bump it on any incompatible change.
const SchemaVersion = 1

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
	Model     string `json:"model,omitempty"`
	Effort    string `json:"effort,omitempty"`
	Stream    bool   `json:"stream"`
	Status    int    `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
	// Prompt is the structure of what the client sent.
	Prompt Prompt `json:"prompt"`
	// Response is the structure of what the provider returned. Usage is
	// absent on error responses and on endpoints that report none.
	Response Response `json:"response"`
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

// Block is one content block reduced to kind, size, and label.
type Block struct {
	Kind  string `json:"kind"`
	Bytes int    `json:"bytes"`
	// Label is the analysis label: tool name and a short argument for tool
	// calls, the tool that produced it for tool results.
	Label string `json:"label,omitempty"`
	// ToolUseID links calls and results; it is an opaque provider id.
	ToolUseID string `json:"tool_use_id,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Response is the reply reduced to structure and usage.
type Response struct {
	Blocks []Block `json:"blocks,omitempty"`
	Usage  *Usage  `json:"usage,omitempty"`
}

// Usage mirrors the provider's usage object field for field.
type Usage struct {
	Input          int `json:"input_tokens"`
	CacheCreation  int `json:"cache_creation_input_tokens"`
	CacheRead      int `json:"cache_read_input_tokens"`
	Output         int `json:"output_tokens"`
	ThinkingTokens int `json:"thinking_tokens,omitempty"`
	Create5m       int `json:"ephemeral_5m_input_tokens,omitempty"`
	Create1h       int `json:"ephemeral_1h_input_tokens,omitempty"`
}
