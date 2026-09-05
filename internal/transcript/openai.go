package transcript

import "encoding/json"

// The OpenAI-compatible wire shape, which Cursor, DeepSeek, Grok and OpenAI
// itself all speak.
//
// One difference matters more than all the others put together. This provider
// counts INCLUSIVELY: prompt_tokens already contains the cached tokens.
// Anthropic counts EXCLUSIVELY: input_tokens is the uncached remainder with the
// cache reported beside it. The same 150 tokens are prompt=150,cached=50 to one
// and input=100,cache_read=50 to the other.
//
// This package's Usage is exclusive, so the adapter subtracts. Copying the
// provider's prompt figure into Input instead would count the cache twice, and
// the error grows with the cache hit rate, which means it is largest on exactly
// the sessions anyone runs this tool for. Langfuse shipped that bug.

// OpenAIUsage is the usage object on a chat/completions response.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptDetails    *struct {
		// CachedTokens is the part of PromptTokens served from cache. There
		// is no counterpart for a cache write: this family caches
		// automatically and reports no separate write, which is why
		// CacheCreation comes out zero and why the trimming advice built on
		// a write penalty does not transfer.
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

// Usage converts the inclusive wire shape to this package's exclusive one.
//
// A nil receiver yields the zero value, matching WireUsage.
func (u *OpenAIUsage) Usage() Usage {
	if u == nil {
		return Usage{}
	}
	cached := 0
	if u.PromptDetails != nil {
		cached = u.PromptDetails.CachedTokens
	}
	// More cached than prompt is a provider bug or a shape this build does not
	// understand. Clamping keeps the total honest and stops a negative fresh
	// count flowing into every cost figure downstream, where it would read as
	// a saving.
	if cached > u.PromptTokens {
		cached = u.PromptTokens
	}
	if cached < 0 {
		cached = 0
	}
	out := Usage{
		Input:     u.PromptTokens - cached,
		CacheRead: cached,
		Output:    u.CompletionTokens,
	}
	if u.CompletionDetails != nil {
		out.ThinkingTokens = u.CompletionDetails.ReasoningTokens
	}
	return out
}

// OpenAIRequest is what the proxy needs from a chat/completions body: enough to
// guard and to attribute, and no message text.
type OpenAIRequest struct {
	Model  string
	Stream bool
	// Bytes is the body size, which is what the spend cap and the byte-to-token
	// fit act on.
	Bytes int
	// Blocks is one block per message, carrying kind, size and tool name.
	// Text is deliberately never set: the ledger's promise is that message
	// content is not written, and the cheapest way to keep a promise is to
	// never hold the thing.
	Blocks []Block
}

// oaiMessage is one message, decoded only as far as the guards need.
type oaiMessage struct {
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Name      string          `json:"name"`
	ToolCalls []struct {
		ID       string `json:"id"`
		Function struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"function"`
	} `json:"tool_calls"`
	ToolCallID string `json:"tool_call_id"`
}

// ParseOpenAIRequest reads a chat/completions body into a summary.
//
// Unknown fields are ignored so a provider addition never breaks decoding,
// which is the same rule the Anthropic decoder follows.
func ParseOpenAIRequest(body []byte) (*OpenAIRequest, error) {
	var raw struct {
		Model    string       `json:"model"`
		Stream   bool         `json:"stream"`
		Messages []oaiMessage `json:"messages"`
		Tools    []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := &OpenAIRequest{Model: raw.Model, Stream: raw.Stream, Bytes: len(body)}
	for _, m := range raw.Messages {
		b := Block{Kind: kindForRole(m.Role), Bytes: ContentBytes(m.Content)}
		switch {
		case len(m.ToolCalls) > 0:
			// A message can carry several calls; the first names the block,
			// and each contributes a CallKey so a repeated call is visible to
			// the loop detector.
			c := m.ToolCalls[0]
			b.Kind = KindToolUse
			b.ToolName = c.Function.Name
			b.ToolUseID = c.ID
			b.Label = LabelToolCallPrefix + c.Function.Name
			b.CallKey = CallKey(c.Function.Name, c.Function.Arguments)
			b.Bytes += ContentBytes(c.Function.Arguments)
		case m.Role == "tool":
			b.Kind = KindToolResult
			b.ToolUseID = m.ToolCallID
			b.Label = LabelToolResultPrefix + "tool"
		default:
			b.Label = TextLabel(m.Role)
		}
		out.Blocks = append(out.Blocks, b)
	}
	return out, nil
}

func kindForRole(role string) string {
	if role == "tool" {
		return KindToolResult
	}
	return KindText
}
