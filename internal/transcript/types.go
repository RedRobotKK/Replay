// Package transcript reads coding-agent session transcripts and reconstructs
// the sequence of provider requests they represent.
//
// A transcript is a log of messages; a request is what the client actually
// sent to the provider at one point in time. The two differ: one request
// produces several assistant lines, and the request's input is the whole
// parent chain leading up to it. This package rebuilds that view so the
// analysis packages can reason about prefixes, not lines.
package transcript

import "time"

// Block kinds as they appear on the provider wire.
const (
	KindText       = "text"
	KindThinking   = "thinking"
	KindToolUse    = "tool_use"
	KindToolResult = "tool_result"
	KindImage      = "image"
	KindDocument   = "document"
	KindOther      = "other"
)

// Roles of a message in a request context. RoleSystem is used by ledger
// data for the system prompt and tool definitions that precede messages.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Block is one content block of a message. Text is kept in memory for
// analysis and is never persisted by this package.
type Block struct {
	Kind string
	// Label identifies the block for attribution, for example
	// "tool result: Read internal/foo.go" or "assistant text".
	Label string
	// Bytes is the size of the block's textual content. It is the input to
	// the byte-to-token fit and is unaffected by JSON escaping so redacted
	// and original transcripts measure the same.
	Bytes int
	// Text is the block's textual content where it has one.
	Text string
	// ToolUseID links a tool_use block to its tool_result and back.
	ToolUseID string
	// ToolName is set on tool_use blocks and on the tool_result they produced.
	ToolName string
	// IsError is set on tool_result blocks the client flagged as errors.
	IsError bool
}

// Message is one turn of context as sent to the provider.
type Message struct {
	UUID      string
	Role      string
	Timestamp time.Time
	Blocks    []Block
}

// Bytes is the total approximate wire size of the message content.
func (m *Message) Bytes() int {
	total := 0
	for _, b := range m.Blocks {
		total += b.Bytes
	}
	return total
}

// Usage is the provider's accounting for one request. Every field comes from
// the transcript verbatim; nothing here is estimated.
type Usage struct {
	Input         int
	CacheCreation int
	CacheRead     int
	Output        int
	// ThinkingTokens is the share of Output spent on reasoning. The provider
	// replays those tokens when the thinking block is sent back, so they are
	// the block's measured input size.
	ThinkingTokens int
	// Create5m and Create1h split CacheCreation by the TTL it was written with.
	// Both are zero when the client did not report the breakdown.
	Create5m int
	Create1h int
}

// PromptTotal is the size of the whole prompt the provider processed.
func (u Usage) PromptTotal() int {
	return u.Input + u.CacheCreation + u.CacheRead
}

// Request is one call to the provider: its input context, its output, and
// the usage the provider reported for it.
type Request struct {
	ID        string
	Model     string
	Effort    string
	Timestamp time.Time
	Usage     Usage
	// Context is every message the request carried as input, oldest first.
	Context []*Message
	// Output is the assistant message the request produced.
	Output *Message
	// Sidechain marks requests made by a sub-agent rather than the main loop.
	Sidechain bool
}

// Lane is one linear sequence of requests sharing a conversation root.
// The main loop is one lane; each sub-agent conversation is another.
type Lane struct {
	ID        string
	Sidechain bool
	Requests  []*Request
}

// Where a session's data came from. The source decides the truth tier of
// every figure derived from it.
const (
	SourceTranscript = "claude-code-transcript"
	SourceLedger     = "buffy-ledger"
)

// Session is a parsed transcript or ledger.
type Session struct {
	ID            string
	Path          string
	ClientVersion string
	Source        string
	// PrefixVisible is true when the system prompt and tool definitions are
	// present in each request's context (ledger data), so nothing ahead of
	// the first message needs estimating.
	PrefixVisible bool
	Lanes         []*Lane
	// Skipped counts lines the parser could not interpret. Non-zero is not an
	// error, but it is reported so a format change does not pass silently.
	Skipped int
}

// RequestCount is the number of provider requests across all lanes.
func (s *Session) RequestCount() int {
	n := 0
	for _, l := range s.Lanes {
		n += len(l.Requests)
	}
	return n
}
