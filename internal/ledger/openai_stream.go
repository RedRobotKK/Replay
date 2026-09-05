package ledger

import (
	"bytes"
	"encoding/json"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// OpenAIStreamParser reads an OpenAI-compatible SSE response.
//
// The shape is not Anthropic's. There are no event: lines and no message_start;
// every frame is a data: line carrying choice deltas, and usage arrives in a
// final frame with an empty choices array. Feeding one to the other's parser
// yields nothing, which matters because this family's clients stream by
// default: Cursor does, so a chat/completions path that only reads whole-body
// responses reads nothing in practice.
//
// Usage is optional on a stream and is only sent when the client set
// stream_options.include_usage. When it is absent this reports no usage at all
// rather than zero, because zero would tell the spend cap the request was free.
type OpenAIStreamParser struct {
	buf   bytes.Buffer
	usage *transcript.OpenAIUsage
	raw   json.RawMessage
	bytes int
}

// Write consumes stream bytes. It never fails; a frame it cannot parse is
// skipped rather than aborting the read, because the response has already been
// sent to the client by then and the record is the only thing at stake.
func (p *OpenAIStreamParser) Write(b []byte) (int, error) {
	p.buf.Write(b)
	for {
		i := bytes.IndexByte(p.buf.Bytes(), '\n')
		if i < 0 {
			return len(b), nil
		}
		line := make([]byte, i)
		copy(line, p.buf.Bytes()[:i])
		p.buf.Next(i + 1)
		p.line(bytes.TrimSpace(line))
	}
}

func (p *OpenAIStreamParser) line(line []byte) {
	const prefix = "data:"
	if !bytes.HasPrefix(line, []byte(prefix)) {
		return
	}
	payload := bytes.TrimSpace(line[len(prefix):])
	if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
		return
	}
	var frame struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *transcript.OpenAIUsage `json:"usage"`
	}
	if json.Unmarshal(payload, &frame) != nil {
		return
	}
	for _, c := range frame.Choices {
		p.bytes += len(c.Delta.Content)
	}
	if frame.Usage != nil {
		// A later frame wins: the final one carries the totals.
		p.usage = frame.Usage
		var probe struct {
			Usage json.RawMessage `json:"usage"`
		}
		if json.Unmarshal(payload, &probe) == nil {
			p.raw = probe.Usage
		}
	}
}

// Result is the reply reduced to structure and usage.
func (p *OpenAIStreamParser) Result() Response {
	var out Response
	if p.bytes > 0 {
		out.Blocks = append(out.Blocks, Block{Kind: transcript.KindText, Label: transcript.LabelAssistantText, Bytes: p.bytes})
	}
	if p.usage != nil {
		u := p.usage.Usage()
		out.Usage = &u
		out.RawUsage = p.raw
	}
	return out
}
