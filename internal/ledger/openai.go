package ledger

import (
	"encoding/json"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The OpenAI-compatible request and response shapes, reduced to the same
// structure the Anthropic path produces so everything downstream is unchanged.
//
// The counting difference is handled in transcript.OpenAIUsage.Usage(), which
// subtracts the cached tokens out of the inclusive prompt figure. Everything
// here is structure and size; no message text is kept, which is the ledger's
// standing promise and not a property of one provider's parser.

// SummarizeOpenAIRequest reduces a chat/completions body to what the guards and
// the analysis need.
//
// The labeler is accepted and unused: it hashes file paths out of tool inputs,
// and this parser never reads a tool argument beyond hashing it into a CallKey
// for loop detection. It stays in the signature so the two summarisers are
// interchangeable at the call site rather than the caller having to know which
// provider it is talking to.
func SummarizeOpenAIRequest(body []byte, _ *Labeler) (RequestSummary, error) {
	req, err := transcript.ParseOpenAIRequest(body)
	if err != nil {
		return RequestSummary{}, err
	}
	sum := RequestSummary{Model: req.Model, Stream: req.Stream}

	// This family caches automatically with no client markers, so there is no
	// cache_control to count and no prefix the client chose. The system
	// message is the closest thing to a stable prefix, and it is the first
	// message when one is present.
	msgs := make([]Message, 0, len(req.Blocks))
	for _, b := range req.Blocks {
		role := "user"
		switch b.Kind {
		case transcript.KindToolResult:
			role = "tool"
		case transcript.KindToolUse:
			role = "assistant"
		}
		msgs = append(msgs, Message{Role: role, Blocks: []Block{b}})
	}
	sum.Prompt = Prompt{Messages: msgs}
	if len(req.Blocks) > 0 {
		sum.Prompt.SystemBytes = req.Blocks[0].Bytes
	}
	return sum, nil
}

// openAIResponse is the reply, decoded only as far as the ledger needs.
type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage *transcript.OpenAIUsage `json:"usage"`
}

// ParseOpenAIResponse reduces a chat/completions reply to structure and usage.
//
// The raw usage object is kept verbatim beside the parsed one. Normalising is
// lossy by construction and this family is the one whose fields are least
// understood here, so the payload that answers tomorrow's question has to be
// stored before anyone knows to ask it.
func ParseOpenAIResponse(body []byte) Response {
	var raw openAIResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return Response{}
	}
	var out Response
	if raw.Usage != nil {
		u := raw.Usage.Usage()
		out.Usage = &u
		if m, err := json.Marshal(raw.Usage); err == nil {
			out.RawUsage = m
		}
	}
	for _, c := range raw.Choices {
		if c.Message.Content != "" {
			out.Blocks = append(out.Blocks, Block{Kind: transcript.KindText, Label: transcript.LabelAssistantText, Bytes: len(c.Message.Content)})
		}
		for _, tc := range c.Message.ToolCalls {
			out.Blocks = append(out.Blocks, Block{Kind: transcript.KindToolUse, ToolName: tc.Function.Name, Label: transcript.LabelToolCallPrefix + tc.Function.Name})
		}
	}
	return out
}
