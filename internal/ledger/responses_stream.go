package ledger

import (
	"bytes"
	"encoding/json"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// ResponsesStreamParser reads a Responses API SSE stream.
//
// The framing is `event: <kind>` then `data: <json>`, and the kind is
// repeated inside the JSON as "type", which is what Codex CLI's own parser
// matches on; so do we, and the event: line is ignored. Usage arrives once,
// on the terminal event: response.completed, or response.incomplete when the
// turn was cut at a limit, which Codex reads the same way because the
// provider billed it. A stream that ends before either reports no usage
// rather than zero.
//
// Text and reasoning are counted from their deltas as they pass; the tool
// call is taken from the finished item. Nothing is accumulated as text.
type ResponsesStreamParser struct {
	pending       bytes.Buffer
	dropped       bool
	usage         *transcript.ResponsesUsage
	raw           json.RawMessage
	textBytes     int
	thinkingBytes int
	tools         []Block
}

// Write consumes stream bytes in whatever chunks arrive. It never fails: the
// response has already been sent to the client, and the record is the only
// thing at stake.
func (p *ResponsesStreamParser) Write(b []byte) (int, error) {
	if p.dropped {
		return len(b), nil
	}
	p.pending.Write(b)
	if p.pending.Len() > maxPendingLine {
		p.dropped = true
		p.pending.Reset()
		return len(b), nil
	}
	for {
		i := bytes.IndexByte(p.pending.Bytes(), '\n')
		if i < 0 {
			return len(b), nil
		}
		line := make([]byte, i)
		copy(line, p.pending.Bytes()[:i])
		p.pending.Next(i + 1)
		p.line(bytes.TrimSpace(line))
	}
}

func (p *ResponsesStreamParser) line(line []byte) {
	const prefix = "data:"
	if !bytes.HasPrefix(line, []byte(prefix)) {
		return
	}
	// An empty payload (a keepalive line) fails to decode below and is
	// skipped there; it needs no check of its own.
	payload := bytes.TrimSpace(line[len(prefix):])
	var ev struct {
		Type     string               `json:"type"`
		Delta    string               `json:"delta"`
		Item     *responsesOutputItem `json:"item"`
		Response *struct {
			Usage json.RawMessage `json:"usage"`
		} `json:"response"`
	}
	if json.Unmarshal(payload, &ev) != nil {
		return
	}
	switch ev.Type {
	case "response.output_text.delta", "response.refusal.delta":
		p.textBytes += len(ev.Delta)
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		p.thinkingBytes += len(ev.Delta)
	case "response.output_item.done":
		if ev.Item != nil && ev.Item.Type == "function_call" {
			p.tools = append(p.tools, ev.Item.blocks()...)
		}
	case "response.completed", "response.incomplete":
		// A terminal event with no response object is malformed; without
		// this it would be a nil dereference in the tap. A usage that is
		// absent fails to decode, and one that is null or empty decodes to a
		// zero value that Result treats as no measurement, so neither needs
		// a check here.
		if ev.Response == nil {
			return
		}
		var u transcript.ResponsesUsage
		if json.Unmarshal(ev.Response.Usage, &u) != nil {
			return
		}
		// A later terminal event wins; there is normally exactly one.
		p.usage, p.raw = &u, ev.Response.Usage
	}
}

// Result is the stream reduced to structure and usage. Usage is nil when no
// terminal event carried one, or the one it carried measured nothing.
func (p *ResponsesStreamParser) Result() Response {
	var out Response
	if p.thinkingBytes > 0 {
		out.Blocks = append(out.Blocks, Block{Kind: transcript.KindThinking, Label: transcript.LabelAssistantThinking, Bytes: p.thinkingBytes})
	}
	if p.textBytes > 0 {
		out.Blocks = append(out.Blocks, Block{Kind: transcript.KindText, Label: transcript.LabelAssistantText, Bytes: p.textBytes})
	}
	out.Blocks = append(out.Blocks, p.tools...)
	if p.usage != nil && !p.usage.Empty() {
		u := p.usage.Usage()
		out.Usage = &u
		out.RawUsage = p.raw
	}
	return out
}
