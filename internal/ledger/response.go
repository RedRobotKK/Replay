package ledger

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// ParseResponse reduces a non-streaming Messages API response to structure
// and usage. A body that is not a message (an error object) yields an empty
// response and no error; the status code carries the outcome.
func ParseResponse(body []byte) Response {
	var msg struct {
		Type              string                `json:"type"`
		Content           []transcript.RawBlock `json:"content"`
		Usage             *transcript.WireUsage `json:"usage"`
		ContextManagement *contextManagement    `json:"context_management"`
	}
	if err := json.Unmarshal(body, &msg); err != nil || msg.Type != "message" {
		return Response{}
	}
	resp := Response{Blocks: stripText(transcript.DecodeBlocks(msg.Content, transcript.RoleAssistant, nil, nil))}
	if msg.Usage != nil {
		u := msg.Usage.Usage()
		resp.Usage = &u
	}
	resp.AppliedEdits, resp.ClearedInputTokens = msg.ContextManagement.totals()
	return resp
}

// contextManagement is the provider's report of the edits it applied.
type contextManagement struct {
	AppliedEdits []struct {
		ClearedInputTokens int `json:"cleared_input_tokens"`
	} `json:"applied_edits"`
}

func (c *contextManagement) totals() (edits, cleared int) {
	if c == nil {
		return 0, 0
	}
	for _, e := range c.AppliedEdits {
		cleared += e.ClearedInputTokens
	}
	return len(c.AppliedEdits), cleared
}

// maxPendingLine bounds one unfinished event line the stream parser holds.
const maxPendingLine = 1 << 20

// StreamParser accumulates the structure and usage of a server-sent event
// stream as bytes pass through. It is fed the response bytes in whatever
// chunks arrive and never blocks the stream.
type StreamParser struct {
	dropped bool
	pending bytes.Buffer
	blocks  []Block
	usage   transcript.WireUsage
	seen    bool
	edits   int
	cleared int
}

// Write feeds response bytes. It never returns an error: the stream must
// reach the client whatever the parser makes of it.
func (s *StreamParser) Write(p []byte) (int, error) {
	if s.dropped {
		return len(p), nil
	}
	s.pending.Write(p)
	if s.pending.Len() > maxPendingLine {
		// A stream with no line breaks is not one the parser understands;
		// stop keeping it rather than grow without bound.
		s.dropped = true
		s.pending.Reset()
		return len(p), nil
	}
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
			Usage             *transcript.WireUsage `json:"usage"`
			ContextManagement *contextManagement    `json:"context_management"`
		} `json:"message"`
		ContentBlock *transcript.RawBlock `json:"content_block"`
		Delta        *struct {
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
			Thinking    string `json:"thinking"`
		} `json:"delta"`
		Usage             *transcript.WireUsage `json:"usage"`
		ContextManagement *contextManagement    `json:"context_management"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(l, dataPrefix), &ev); err != nil {
		return
	}
	// The provider reports applied edits on the message; the stream may
	// carry them at the start or in the final delta.
	if ev.Message != nil {
		s.noteEdits(ev.Message.ContextManagement)
	}
	s.noteEdits(ev.ContextManagement)
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

func (s *StreamParser) noteEdits(c *contextManagement) {
	if c == nil {
		return
	}
	s.edits, s.cleared = c.totals()
}

// Result returns what the stream contained. Usage is nil when no
// message_start or message_delta carried one.
func (s *StreamParser) Result() Response {
	resp := Response{Blocks: s.blocks, AppliedEdits: s.edits, ClearedInputTokens: s.cleared}
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
