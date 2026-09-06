// Package transcript reads coding-agent session transcripts and reconstructs
// the sequence of provider requests they represent.
//
// A transcript is a log of messages; a request is what the client actually
// sent to the provider at one point in time. The two differ: one request
// produces several assistant lines, and the request's input is the whole
// parent chain leading up to it. This package rebuilds that view so the
// analysis packages can reason about prefixes, not lines.
//
// The types here are also the ledger's wire model: the proxy persists the
// content-free subset (everything but Text) and reads it back as the same
// structures, so one block model serves both tiers.
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

// Labels shared by every decoder so both tiers group content identically.
const (
	LabelUserText          = "user text"
	LabelAssistantText     = "assistant text"
	LabelAssistantThinking = "assistant thinking"
	LabelToolCallPrefix    = "tool call: "
	LabelToolResultPrefix  = "tool result: "
	LabelUnknownTool       = "unknown tool"
	LabelOtherPrefix       = "other: "
)

// TextLabel is the label of a text block by the role that produced it.
func TextLabel(role string) string {
	if role == RoleAssistant {
		return LabelAssistantText
	}
	return LabelUserText
}

// Block is one content block of a message. Text is kept in memory for
// analysis and is never persisted: the JSON tags define exactly what the
// ledger stores.
type Block struct {
	Kind string `json:"kind"`
	// Label identifies the block for attribution, for example
	// "tool result: Read internal/foo.go" or "assistant text".
	Label string `json:"label,omitempty"`
	// Bytes is the size of the block's textual content. It is the input to
	// the byte-to-token fit and is unaffected by JSON escaping so redacted
	// and original transcripts measure the same.
	Bytes int `json:"bytes"`
	// Text is the block's textual content where it has one.
	Text string `json:"-"`
	// ToolUseID links a tool_use block to its tool_result and back.
	ToolUseID string `json:"tool_use_id,omitempty"`
	// ToolName is set on tool_use blocks and on the tool_result they produced.
	ToolName string `json:"tool_name,omitempty"`
	// CallKey identifies a tool call by tool name and input without holding
	// the input: identical calls share a key. Set on tool_use blocks.
	CallKey string `json:"call_key,omitempty"`
	// IsError is set on tool_result blocks the client flagged as errors.
	IsError bool `json:"is_error,omitempty"`
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
// the provider verbatim; nothing here is estimated. The JSON tags mirror the
// provider's names so ledger records read naturally.
type Usage struct {
	Input         int `json:"input_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
	Output        int `json:"output_tokens"`
	// ThinkingTokens is the share of Output spent on reasoning. The provider
	// replays those tokens when the thinking block is sent back, so they are
	// the block's measured input size.
	ThinkingTokens int `json:"thinking_tokens,omitempty"`
	// Create5m and Create1h split CacheCreation by the TTL it was written with.
	// Both are zero when the client did not report the breakdown.
	Create5m int `json:"ephemeral_5m_input_tokens,omitempty"`
	Create1h int `json:"ephemeral_1h_input_tokens,omitempty"`
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
	// AppliedEdits and ClearedTokens are the provider's own context edits
	// on this request's response, known only from the ledger.
	AppliedEdits  int
	ClearedTokens int
	// Tools are the tool definitions the request carried, by name and
	// size, known only from the ledger. Transcripts do not show them.
	Tools []ToolDef
}

// ToolDef is one tool definition's name and decoded size. Names are the
// same identifiers tool calls already carry; descriptions and schemas are
// never kept.
type ToolDef struct {
	Name  string `json:"name"`
	Bytes int    `json:"bytes"`
}

// Lane is one linear sequence of requests sharing a conversation root.
// The main loop is one lane; each sub-agent conversation is another.
type Lane struct {
	ID        string
	Sidechain bool
	Requests  []*Request
}

// Source says where a session's data came from. It decides the truth tier
// of every figure derived from the session.
type Source string

// Sources.
const (
	SourceTranscript Source = "claude-code-transcript"
	SourceLedger     Source = "replay-ledger"
)

// PrefixVisible reports whether the system prompt and tool definitions are
// present in each request's context, so nothing ahead of the first message
// needs estimating. Only the proxy sees them.
func (s Source) PrefixVisible() bool {
	return s == SourceLedger
}

// Tier is the provenance label every report carries.
func (s Source) Tier() string {
	if s == SourceLedger {
		return "measured (proxy-recorded)"
	}
	return "estimated (transcripts only)"
}

// Session is a parsed transcript or ledger.
type Session struct {
	ID            string
	Path          string
	ClientVersion string
	Source        Source
	Lanes         []*Lane
	// Skipped counts lines the parser could not interpret. Non-zero is not an
	// error, but it is reported so a format change does not pass silently.
	Skipped int
	// Policy names the request-parameter policy the proxy applied to this
	// session's requests, and Trial the arm of the live trial it was in:
	// "treated", "control", or empty. Only the ledger knows either.
	Policy string
	Trial  string
	// Compactions are the history rewrites this session recorded, in order.
	//
	// Claude Code writes the sizes it dropped, so this is measured rather than
	// inferred from a prompt that shrank. It matters because the transcripts
	// containing a compaction hold a disproportionate share of re-billed
	// tokens, so they are exactly the sessions worth quantifying rather than
	// waving at.
	Compactions []Compaction
}

// Compaction is one recorded history rewrite.
type Compaction struct {
	Trigger    string
	PreTokens  int
	PostTokens int
	// CumulativeDropped is the client's own running total for the session.
	CumulativeDropped int
	DurationMS        int
}

// Sized reports whether the client recorded the sizes. A compaction with no
// sizes is still a compaction; it just cannot say how much left, and saying so
// is different from saying nothing left.
func (c Compaction) Sized() bool { return c.PreTokens > 0 }

// Dropped is how much context the rewrite removed, never negative.
//
// One record in the measured corpus reports postTokens ABOVE preTokens
// (22,303 -> 296,742). Whatever produced it, a negative drop would subtract
// from a total describing content that LEFT the context, and a single bad
// record would silently offset several good ones.
func (c Compaction) Dropped() int {
	if d := c.PreTokens - c.PostTokens; d > 0 {
		return d
	}
	return 0
}

// Lane finds or creates the lane with the given id, keeping lanes in order
// of first appearance.
func (s *Session) Lane(id string, sidechain bool) *Lane {
	for _, l := range s.Lanes {
		if l.ID == id {
			return l
		}
	}
	l := &Lane{ID: id, Sidechain: sidechain}
	s.Lanes = append(s.Lanes, l)
	return l
}

// RequestCount is the number of provider requests across all lanes.
func (s *Session) RequestCount() int {
	n := 0
	for _, l := range s.Lanes {
		n += len(l.Requests)
	}
	return n
}
