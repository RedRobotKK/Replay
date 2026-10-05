package transcript

import "encoding/json"

// The OpenAI Responses API wire shape: what Codex CLI and GPT-6 Astra speak.
//
// Read into this package on 2026-10-04, after a local probe showed Codex CLI
// 0.154.0 reaches an ordinary HTTP proxy over SSE when given a custom base
// URL. The field names are the ones Codex's own parser deserialises
// (codex-rs/codex-api/src/sse/responses.rs), which is the consumer this
// adapter exists for.
//
// Counting is INCLUSIVE, as on chat completions: input_tokens already holds
// the cached share, and on GPT-5.6 and later the written share as well,
// reported beside it as input_tokens_details.cache_write_tokens. This
// package's Usage is exclusive, so both shares are subtracted out of Input.
// Where the provider sends no write figure, CacheCreation is zero: it is
// never reconstructed from the uncached remainder, which would turn an
// unobserved quantity into a number somebody is billed against. The Codex
// rollout reader applies the same rule to the same fields one layer up.

// ResponsesUsage is the usage object on a Responses reply, non-streaming or
// on the response.completed and response.incomplete events of a stream.
type ResponsesUsage struct {
	InputTokens   int                     `json:"input_tokens"`
	OutputTokens  int                     `json:"output_tokens"`
	TotalTokens   int                     `json:"total_tokens"`
	InputDetails  *ResponsesInputDetails  `json:"input_tokens_details"`
	OutputDetails *ResponsesOutputDetails `json:"output_tokens_details"`
}

// ResponsesInputDetails splits the input figure.
type ResponsesInputDetails struct {
	// CachedTokens is the share of InputTokens served from cache: the READ.
	CachedTokens int `json:"cached_tokens"`
	// CacheWriteTokens is the WRITE, sent by GPT-5.6 and later and absent on
	// older models and on third parties that normalise it away. A pointer so
	// a reported zero is told from a field that is not there.
	CacheWriteTokens *int `json:"cache_write_tokens"`
}

// ResponsesOutputDetails splits the output figure.
type ResponsesOutputDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

// Empty reports a usage object that measures nothing: absent, or `{}`, or
// all zero. A real completion always has input tokens, so an all-zero
// object is the absence of a measurement rather than a measurement of zero.
func (u *ResponsesUsage) Empty() bool {
	return u == nil || (u.InputTokens == 0 && u.OutputTokens == 0)
}

// Usage converts the inclusive wire shape to this package's exclusive one.
// A nil receiver yields the zero value, matching the other adapters.
func (u *ResponsesUsage) Usage() Usage {
	if u == nil {
		return Usage{}
	}
	cached, write := 0, 0
	if u.InputDetails != nil {
		cached = u.InputDetails.CachedTokens
		if u.InputDetails.CacheWriteTokens != nil {
			write = *u.InputDetails.CacheWriteTokens
		}
	}
	// A share above its parent is a provider bug or a shape this build does
	// not understand. Clamping keeps the total honest and stops a negative
	// fresh count flowing into every cost figure as a saving.
	if cached < 0 {
		cached = 0
	}
	if write < 0 {
		write = 0
	}
	if cached > u.InputTokens {
		cached = u.InputTokens
	}
	if cached+write > u.InputTokens {
		write = u.InputTokens - cached
	}
	out := Usage{
		Input:         u.InputTokens - cached - write,
		CacheCreation: write,
		CacheRead:     cached,
		Output:        u.OutputTokens,
	}
	if u.OutputDetails != nil {
		out.ThinkingTokens = u.OutputDetails.ReasoningTokens
	}
	return out
}

// ResponsesRequest is what the proxy needs from a Responses body: enough to
// guard and attribute, and no message text.
type ResponsesRequest struct {
	Model  string
	Stream bool
	// PromptCacheKey is the client's own cache key, which Codex sets to its
	// conversation id. Empty when the client sent none.
	PromptCacheKey string
	// Bytes is the body size; Instructions the size of the instructions
	// string, which is this shape's system prefix.
	Bytes        int
	Instructions int
	// Items is one block per input item, with the role it carries.
	Items []ResponsesItem
	Tools []ToolDef
}

// ResponsesItem is one input item reduced to a role and a block.
type ResponsesItem struct {
	Role  string
	Block Block
}

// responsesInputItem is one input item, decoded only as far as the guards
// need. Codex sends typed items; the API also accepts untyped messages, so
// Type may be empty when Role is not.
type responsesInputItem struct {
	Type             string          `json:"type"`
	Role             string          `json:"role"`
	Content          json.RawMessage `json:"content"`
	CallID           string          `json:"call_id"`
	Name             string          `json:"name"`
	Arguments        json.RawMessage `json:"arguments"`
	Output           json.RawMessage `json:"output"`
	Summary          json.RawMessage `json:"summary"`
	EncryptedContent string          `json:"encrypted_content"`
}

// ParseResponsesRequest reads a Responses body into a summary. Unknown
// fields and unknown item kinds are kept as size only, so a provider
// addition never breaks decoding.
func ParseResponsesRequest(body []byte) (*ResponsesRequest, error) {
	var raw struct {
		Model          string            `json:"model"`
		Stream         bool              `json:"stream"`
		Instructions   string            `json:"instructions"`
		PromptCacheKey string            `json:"prompt_cache_key"`
		Input          json.RawMessage   `json:"input"`
		Tools          []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	out := &ResponsesRequest{Model: raw.Model, Stream: raw.Stream, PromptCacheKey: raw.PromptCacheKey, Bytes: len(body), Instructions: len(raw.Instructions)}
	for _, t := range raw.Tools {
		var tool struct {
			Type string `json:"type"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(t, &tool)
		name := tool.Name
		if name == "" {
			name = tool.Type
		}
		out.Tools = append(out.Tools, ToolDef{Name: name, Bytes: len(t)})
	}
	// input is a string or a list of items.
	var text string
	if len(raw.Input) > 0 && json.Unmarshal(raw.Input, &text) == nil {
		out.Items = append(out.Items, ResponsesItem{Role: RoleUser, Block: Block{Kind: KindText, Label: LabelUserText, Bytes: len(text)}})
		return out, nil
	}
	var items []json.RawMessage
	if len(raw.Input) > 0 {
		if err := json.Unmarshal(raw.Input, &items); err != nil {
			return nil, err
		}
	}
	for _, it := range items {
		var item responsesInputItem
		if err := json.Unmarshal(it, &item); err != nil {
			return nil, err
		}
		out.Items = append(out.Items, responsesItemBlock(item, len(it)))
	}
	return out, nil
}

// responsesItemBlock reduces one input item. Text is kept ONLY on a tool
// call, as the arguments, so a labeler can key the call; the ledger strips
// it before anything is written.
func responsesItemBlock(item responsesInputItem, rawLen int) ResponsesItem {
	switch {
	case item.Type == "function_call":
		return ResponsesItem{Role: RoleAssistant, Block: Block{
			Kind: KindToolUse, ToolName: item.Name, ToolUseID: item.CallID,
			Label:   LabelToolCallPrefix + item.Name,
			Bytes:   len(item.Name) + ContentBytes(item.Arguments),
			Text:    string(item.Arguments),
			CallKey: CallKey(item.Name, item.Arguments),
		}}
	case item.Type == "function_call_output":
		return ResponsesItem{Role: "tool", Block: Block{
			Kind: KindToolResult, ToolUseID: item.CallID,
			Label: LabelToolResultPrefix + "tool", Bytes: ResponsesContentBytes(item.Output),
		}}
	case item.Type == "reasoning":
		return ResponsesItem{Role: RoleAssistant, Block: Block{
			Kind: KindThinking, Label: LabelAssistantThinking,
			Bytes: ResponsesContentBytes(item.Summary) + len(item.EncryptedContent),
		}}
	case item.Type == "message" || (item.Type == "" && item.Role != ""):
		role := responsesRole(item.Role)
		return ResponsesItem{Role: role, Block: Block{Kind: KindText, Label: TextLabel(role), Bytes: ResponsesContentBytes(item.Content)}}
	}
	// Something this build does not know: counted by size, under the role
	// the API would attribute it to, so the prompt total still reconciles.
	return ResponsesItem{Role: RoleUser, Block: Block{Kind: KindOther, Label: "other", Bytes: rawLen}}
}

// ResponsesContentBytes measures a Responses content value by its text: a
// list of typed parts by the text each carries, and a part with no text (an
// image, a file) by its whole size. ContentBytes would count the parts'
// keys and type names too, which are framing the user did not write and the
// model does not read as prompt. Anything that is not a list, a plain
// string or an absent value included, is measured by ContentBytes, which
// already gives a string its decoded length and an absent value zero.
func ResponsesContentBytes(raw json.RawMessage) int {
	var parts []json.RawMessage
	if json.Unmarshal(raw, &parts) != nil {
		return ContentBytes(raw)
	}
	n := 0
	for _, p := range parts {
		var part struct {
			Text    string `json:"text"`
			Refusal string `json:"refusal"`
		}
		if json.Unmarshal(p, &part) == nil && (part.Text != "" || part.Refusal != "") {
			n += len(part.Text) + len(part.Refusal)
			continue
		}
		n += ContentBytes(p)
	}
	return n
}

// responsesRole maps the wire roles onto this package's three. The API's
// developer role is the system prefix by another name.
func responsesRole(role string) string {
	switch role {
	case RoleAssistant:
		return RoleAssistant
	case RoleSystem, "developer":
		return RoleSystem
	}
	return RoleUser
}
