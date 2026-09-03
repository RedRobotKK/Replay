package ledger

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// rawUsage is the provider's usage object.
type rawUsage struct {
	Input         int `json:"input_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
	Output        int `json:"output_tokens"`
	OutputDetails *struct {
		Thinking int `json:"thinking_tokens"`
	} `json:"output_tokens_details"`
	CacheBreak *struct {
		Short int `json:"ephemeral_5m_input_tokens"`
		Long  int `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
}

func (u *rawUsage) toUsage() *Usage {
	if u == nil {
		return nil
	}
	out := &Usage{Input: u.Input, CacheCreation: u.CacheCreation, CacheRead: u.CacheRead, Output: u.Output}
	if u.OutputDetails != nil {
		out.ThinkingTokens = u.OutputDetails.Thinking
	}
	if u.CacheBreak != nil {
		out.Create5m = u.CacheBreak.Short
		out.Create1h = u.CacheBreak.Long
	}
	return out
}

// ParseResponse reduces a non-streaming Messages API response to structure
// and usage. A body that is not a message (an error object) yields an empty
// response and no error; the status code carries the outcome.
func ParseResponse(body []byte) Response {
	var msg struct {
		Type    string     `json:"type"`
		Content []rawBlock `json:"content"`
		Usage   *rawUsage  `json:"usage"`
	}
	if err := json.Unmarshal(body, &msg); err != nil || msg.Type != "message" {
		return Response{}
	}
	resp := Response{Usage: msg.Usage.toUsage()}
	names := map[string]string{}
	for _, b := range msg.Content {
		resp.Blocks = append(resp.Blocks, summarizeBlock(b, transcript.RoleAssistant, names, nil))
	}
	return resp
}

// StreamParser accumulates the structure and usage of a server-sent event
// stream as bytes pass through. It is fed the response bytes in whatever
// chunks arrive and never blocks the stream.
type StreamParser struct {
	pending bytes.Buffer
	blocks  []Block
	usage   rawUsage
	seen    bool
}

// Write feeds response bytes. It never returns an error: the stream must
// reach the client whatever the parser makes of it.
func (s *StreamParser) Write(p []byte) (int, error) {
	s.pending.Write(p)
	for {
		line, err := s.pending.ReadString('\n')
		if err != nil {
			// Incomplete line: keep it for the next chunk.
			s.pending.Reset()
			s.pending.WriteString(line)
			return len(p), nil
		}
		s.line(strings.TrimRight(line, "\r\n"))
	}
}

// line handles one SSE line. Only data lines matter; event names are
// repeated inside the JSON as "type".
func (s *StreamParser) line(l string) {
	const dataPrefix = "data: "
	if !strings.HasPrefix(l, dataPrefix) {
		return
	}
	var ev struct {
		Type    string `json:"type"`
		Index   int    `json:"index"`
		Message *struct {
			Usage *rawUsage `json:"usage"`
		} `json:"message"`
		ContentBlock *rawBlock `json:"content_block"`
		Delta        *struct {
			Type        string `json:"type"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
			Thinking    string `json:"thinking"`
		} `json:"delta"`
		Usage *rawUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(l, dataPrefix)), &ev); err != nil {
		return
	}
	switch ev.Type {
	case "message_start":
		if ev.Message != nil && ev.Message.Usage != nil {
			s.usage = *ev.Message.Usage
			s.seen = true
		}
	case "content_block_start":
		if ev.ContentBlock != nil {
			b := summarizeBlock(*ev.ContentBlock, transcript.RoleAssistant, map[string]string{}, nil)
			s.blocks = append(s.blocks, b)
		}
	case "content_block_delta":
		if ev.Delta != nil && ev.Index >= 0 && ev.Index < len(s.blocks) {
			s.blocks[ev.Index].Bytes += len(ev.Delta.Text) + len(ev.Delta.PartialJSON) + len(ev.Delta.Thinking)
		}
	case "message_delta":
		if ev.Usage != nil {
			// The final usage carries output tokens and, on some models,
			// refreshed input accounting; take every non-zero field.
			merge(&s.usage, ev.Usage)
			s.seen = true
		}
	}
}

func merge(dst, src *rawUsage) {
	if src.Input > 0 {
		dst.Input = src.Input
	}
	if src.CacheCreation > 0 {
		dst.CacheCreation = src.CacheCreation
	}
	if src.CacheRead > 0 {
		dst.CacheRead = src.CacheRead
	}
	if src.Output > 0 {
		dst.Output = src.Output
	}
	if src.OutputDetails != nil {
		dst.OutputDetails = src.OutputDetails
	}
	if src.CacheBreak != nil {
		dst.CacheBreak = src.CacheBreak
	}
}

// Result returns what the stream contained. Usage is nil when no
// message_start or message_delta carried one.
func (s *StreamParser) Result() Response {
	resp := Response{Blocks: s.blocks}
	if s.seen {
		resp.Usage = s.usage.toUsage()
	}
	return resp
}

// IsEventStream reports whether a content type denotes server-sent events.
func IsEventStream(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "text/event-stream")
}
