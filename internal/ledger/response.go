package ledger

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// ParseResponse reduces a non-streaming Messages API response to structure
// and usage. A body that is not a message (an error object) yields an empty
// response and no error; the status code carries the outcome.
func ParseResponse(body []byte) Response {
	var msg struct {
		Type    string                `json:"type"`
		Content []transcript.RawBlock `json:"content"`
		Usage   *transcript.WireUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &msg); err != nil || msg.Type != "message" {
		return Response{}
	}
	resp := Response{Blocks: stripText(transcript.DecodeBlocks(msg.Content, transcript.RoleAssistant, nil, nil))}
	if msg.Usage != nil {
		u := msg.Usage.Usage()
		resp.Usage = &u
	}
	return resp
}

// StreamParser accumulates the structure and usage of a server-sent event
// stream as bytes pass through. It is fed the response bytes in whatever
// chunks arrive and never blocks the stream.
type StreamParser struct {
	pending bytes.Buffer
	blocks  []Block
	usage   transcript.WireUsage
	seen    bool
}

// Write feeds response bytes. It never returns an error: the stream must
// reach the client whatever the parser makes of it.
func (s *StreamParser) Write(p []byte) (int, error) {
	s.pending.Write(p)
	for {
		i := bytes.IndexByte(s.pending.Bytes(), '\n')
		if i < 0 {
			return len(p), nil
		}
		line := s.pending.Next(i + 1)
		s.line(bytes.TrimRight(line, "\r\n"))
	}
}

// dataPrefix introduces an SSE data line. Event names are repeated inside
// the JSON as "type", so only data lines matter.
var dataPrefix = []byte("data: ")

func (s *StreamParser) line(l []byte) {
	if !bytes.HasPrefix(l, dataPrefix) {
		return
	}
	var ev struct {
		Type    string `json:"type"`
		Index   int    `json:"index"`
		Message *struct {
			Usage *transcript.WireUsage `json:"usage"`
		} `json:"message"`
		ContentBlock *transcript.RawBlock `json:"content_block"`
		Delta        *struct {
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
			Thinking    string `json:"thinking"`
		} `json:"delta"`
		Usage *transcript.WireUsage `json:"usage"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(l, dataPrefix), &ev); err != nil {
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
			b := transcript.DecodeBlock(*ev.ContentBlock, transcript.RoleAssistant, nil, nil)
			b.Text = ""
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

func merge(dst, src *transcript.WireUsage) {
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
		u := s.usage.Usage()
		resp.Usage = &u
	}
	return resp
}

// IsEventStream reports whether a content type denotes server-sent events.
func IsEventStream(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "text/event-stream")
}
