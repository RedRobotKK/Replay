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

	// PrefixHash and SessionHash are not decoration. The proxy falls back to
	// SessionHash when the client sends no session header, which every
	// OpenAI-compatible client does because the header it looks for is Claude
	// Code's own; leaving it empty meant the ledger record was dropped and the
	// log line still looked correct. The sibling gate keys on PrefixHash, and
	// an empty key makes unrelated requests queue behind each other.
	//
	// Hashed from block structure rather than text, because this parser never
	// holds message content and is not going to start. Two sessions whose
	// system prompt and first message match in kind and size to the byte will
	// collide; that is a real limit of hashing shape instead of content, and a
	// far smaller error than writing nothing at all.
	if len(req.Blocks) > 0 {
		sum.PrefixHash = hashOf("prefix-", blockIdentity(req.Blocks[0]))
		if len(req.Blocks) > 1 {
			sum.SessionHash = hashOf("session-",
				blockIdentity(req.Blocks[0]), blockIdentity(req.Blocks[1]))
		} else {
			sum.SessionHash = hashOf("session-", blockIdentity(req.Blocks[0]))
		}
	}
	return sum, nil
}

// blockIdentity renders a block's structure, never its text, as the bytes a
// hash is taken over.
func blockIdentity(b transcript.Block) json.RawMessage {
	id := struct {
		Kind  string `json:"kind"`
		Bytes int    `json:"bytes"`
		Name  string `json:"name,omitempty"`
	}{Kind: b.Kind, Bytes: b.Bytes, Name: b.ToolName}
	out, err := json.Marshal(id)
	if err != nil {
		// The struct above has no unmarshalable field, so this cannot fire;
		// returning empty keeps the hash defined rather than panicking in a
		// request path.
		return json.RawMessage("{}")
	}
	return out
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
		// Keep the provider's bytes, not a re-marshalling of our own struct.
		// Round-tripping through a typed value silently drops every field the
		// type does not declare, which is exactly the set worth keeping: live
		// DeepSeek reports prompt_cache_hit_tokens and prompt_cache_miss_tokens
		// beside the OpenAI-shaped prompt_tokens_details, and both were
		// discarded here until 2026-09-05. RawUsage is documented as verbatim
		// and unparsed, and the design note gives the reason — a field we did
		// not know mattered is what tomorrow's calibration needs.
		out.RawUsage = rawUsageBytes(body)
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

// rawUsageBytes lifts the "usage" object out of a response body without
// interpreting it. It returns nil when the body is not an object with a usage
// member, in which case the caller simply has no raw copy.
func rawUsageBytes(body []byte) json.RawMessage {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	u, ok := envelope["usage"]
	if !ok || len(u) == 0 || string(u) == "null" {
		return nil
	}
	return u
}
