package ledger

import (
	"encoding/json"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The OpenAI Responses API, reduced to the structure the Anthropic and
// chat-completions paths produce, so everything downstream is unchanged:
// the same RequestSummary feeds the guards, the same Response feeds the
// ledger, the pricing and the analysis. No parallel accounting.
//
// Read since 2026-10-04 (R-1). Before that the path was forwarded unread;
// the probe that justified reading it is in
// docs/evidence/activation-path-2026-10-04.md.
//
// What this path does NOT get: cache-break classification. The provider's
// cache is addressed by the client's own prompt_cache_key and reported back
// as cached_tokens, and nothing on the wire establishes that the previous
// prompt total is the expected read, which is the Messages rule the live
// classifier applies. The proxy skips the classifier for records on this
// path and says so once on stderr.

// SummarizeResponsesRequest reduces a Responses body to what the guards and
// the analysis need. The labeler keys tool calls under the ledger secret,
// as the Messages path does; nil leaves the content-free digest.
func SummarizeResponsesRequest(body []byte, labeler *Labeler) (RequestSummary, error) {
	req, err := transcript.ParseResponsesRequest(body)
	if err != nil {
		return RequestSummary{}, err
	}
	sum := RequestSummary{Model: req.Model, Stream: req.Stream}
	sum.Prompt.SystemBytes = req.Instructions
	sum.Prompt.Tools = req.Tools
	sum.Prompt.ToolCount = len(req.Tools)
	for _, t := range req.Tools {
		sum.Prompt.ToolBytes += t.Bytes
	}
	msgs := make([]Message, 0, len(req.Items))
	for _, it := range req.Items {
		blocks := []Block{it.Block}
		if labeler != nil {
			blocks = labeler.keyCalls(blocks)
		}
		msgs = append(msgs, Message{Role: it.Role, Blocks: stripText(blocks)})
	}
	sum.Prompt.Messages = msgs

	// PrefixHash is the instructions and the tool set, by size and name,
	// never by text: the stable prefix the provider would be caching.
	prefix := responsesPrefixIdentity(req)
	sum.PrefixHash = hashOf("prefix-", prefix)

	// SessionHash. The Responses protocol carries the client's own
	// prompt_cache_key, which Codex sets to its conversation id; that is the
	// one piece of session identity this shape provides and it is used when
	// present. Hashed, not copied: a conversation id is an identifier the
	// ledger has no need to hold. Absent, the fallback is the structural
	// hash the chat-completions path uses, with the same stated limit: two
	// sessions whose prefix and first items match in kind and size collide.
	if req.PromptCacheKey != "" {
		sum.SessionHash = hashOf("session-", json.RawMessage("prompt_cache_key\x00"+req.PromptCacheKey))
	} else {
		parts := []json.RawMessage{prefix}
		for i, it := range req.Items {
			if i == 2 {
				break
			}
			parts = append(parts, blockIdentity(it.Block))
		}
		sum.SessionHash = hashOf("session-", parts...)
	}
	return sum, nil
}

// responsesPrefixIdentity renders the prefix's structure, never its text.
func responsesPrefixIdentity(req *transcript.ResponsesRequest) json.RawMessage {
	id := struct {
		Instructions int                  `json:"instructions"`
		Tools        []transcript.ToolDef `json:"tools,omitempty"`
	}{Instructions: req.Instructions, Tools: req.Tools}
	out, err := json.Marshal(id)
	if err != nil {
		return json.RawMessage("{}")
	}
	return out
}

// responsesOutputItem is one output item, decoded only as far as the ledger
// needs.
type responsesOutputItem struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	CallID  string `json:"call_id"`
	Content []struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Refusal string `json:"refusal"`
	} `json:"content"`
	Arguments        json.RawMessage `json:"arguments"`
	Summary          json.RawMessage `json:"summary"`
	EncryptedContent string          `json:"encrypted_content"`
}

// blocks reduces finished output items to blocks. Text is never kept.
func (it responsesOutputItem) blocks() []Block {
	switch it.Type {
	case "message":
		var out []Block
		for _, c := range it.Content {
			n := len(c.Text) + len(c.Refusal)
			if n > 0 {
				out = append(out, Block{Kind: transcript.KindText, Label: transcript.LabelAssistantText, Bytes: n})
			}
		}
		return out
	case "function_call":
		return []Block{{
			Kind: transcript.KindToolUse, ToolName: it.Name, ToolUseID: it.CallID,
			Label:   transcript.LabelToolCallPrefix + it.Name,
			Bytes:   len(it.Name) + transcript.ContentBytes(it.Arguments),
			CallKey: transcript.CallKey(it.Name, it.Arguments),
		}}
	case "reasoning":
		if n := transcript.ResponsesContentBytes(it.Summary) + len(it.EncryptedContent); n > 0 {
			return []Block{{Kind: transcript.KindThinking, Label: transcript.LabelAssistantThinking, Bytes: n}}
		}
	}
	return nil
}

// ParseResponsesResponse reduces a non-streaming Responses reply to
// structure and usage.
//
// Unparsed is set where the parser knows it could not read the body: not
// JSON, an object of another kind, or an error object with no response in
// it. It is never inferred from an empty result, so a reply that parsed and
// carried no usage stays "without usage" (QT-7a) and is not counted as a
// body nobody could read (QT-7c).
func ParseResponsesResponse(body []byte) Response {
	var raw struct {
		Object string                     `json:"object"`
		Error  json.RawMessage            `json:"error"`
		Output []responsesOutputItem      `json:"output"`
		Usage  *transcript.ResponsesUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Response{Unparsed: true}
	}
	if raw.Object != "" && raw.Object != "response" {
		return Response{Unparsed: true}
	}
	if errObj := string(raw.Error); errObj != "" && errObj != "null" && raw.Output == nil && raw.Usage == nil {
		return Response{Unparsed: true}
	}
	var out Response
	for _, it := range raw.Output {
		out.Blocks = append(out.Blocks, it.blocks()...)
	}
	// `{}` and all-zero are the absence of a measurement, as on the
	// chat-completions path; a zeroed record would enter every average as a
	// free request.
	if raw.Usage != nil && !raw.Usage.Empty() {
		u := raw.Usage.Usage()
		out.Usage = &u
		out.RawUsage = rawUsageBytes(body)
	}
	return out
}
