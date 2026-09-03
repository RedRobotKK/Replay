package ledger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// rawRequest is the subset of a Messages API request the summarizer reads.
// Everything else is ignored, which is also why unknown fields never break
// the proxy.
type rawRequest struct {
	Model             string                   `json:"model"`
	Stream            bool                     `json:"stream"`
	System            json.RawMessage          `json:"system"`
	Tools             []json.RawMessage        `json:"tools"`
	Messages          []json.RawMessage        `json:"messages"`
	OutputConfig      *struct{ Effort string } `json:"output_config"`
	ContextManagement json.RawMessage          `json:"context_management"`
}

// Labeler turns tool calls into labels that carry no content. Paths are
// replaced by a keyed hash that keeps the extension, so the same file
// attributes consistently within one ledger while the ledger never holds a
// path; every other argument is dropped and only the tool name remains.
type Labeler struct {
	key []byte
}

// NewLabeler builds a labeler over a secret key. The key never leaves the
// ledger directory; without it the hashes cannot be matched to a path.
func NewLabeler(key []byte) *Labeler {
	return &Labeler{key: key}
}

// Label renders "Read r/3f9a…go" for path arguments and just the tool name
// otherwise.
func (l *Labeler) Label(name string, input json.RawMessage) string {
	_, value, ok := transcript.LabelArg(input, transcript.PathArgs)
	if !ok {
		return name
	}
	mac := hmac.New(sha256.New, l.key)
	mac.Write([]byte(value))
	return name + " " + transcript.HashedPathLabel(hex.EncodeToString(mac.Sum(nil)), value)
}

// keyCalls replaces each tool call's identity with one keyed by the
// ledger's secret, so equal calls still share a key inside one ledger
// while nobody holding the file can confirm a guessed call.
func (l *Labeler) keyCalls(blocks []Block) []Block {
	for i := range blocks {
		if blocks[i].Kind == transcript.KindToolUse {
			mac := hmac.New(sha256.New, l.key)
			mac.Write([]byte(blocks[i].ToolName))
			mac.Write([]byte{0})
			mac.Write([]byte(blocks[i].Text))
			blocks[i].CallKey = hex.EncodeToString(mac.Sum(nil))[:hashLabelBytes]
		}
	}
	return blocks
}

// SummarizeRequest reduces a Messages API request body to its structure
// and attributes. Labels come from the labeler and carry no content; block
// text is dropped before the summary leaves this function.
func SummarizeRequest(body []byte, labeler *Labeler) (RequestSummary, error) {
	var req rawRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return RequestSummary{}, err
	}
	sum := RequestSummary{Model: req.Model, Stream: req.Stream}
	if req.OutputConfig != nil {
		sum.Effort = req.OutputConfig.Effort
	}
	p := &sum.Prompt
	p.ToolCount = len(req.Tools)
	p.ContextEdits = len(req.ContextManagement) > 0
	p.SystemBytes, p.CacheControlCount = systemSize(req.System)
	for _, t := range req.Tools {
		def := transcript.ToolDef{Name: toolName(t), Bytes: transcript.ContentBytes(t)}
		p.Tools = append(p.Tools, def)
		p.ToolBytes += def.Bytes
		if hasCacheControl(t) {
			p.CacheControlCount++
		}
	}
	sum.PrefixHash = hashOf("prefix-", append(req.Tools, req.System)...)
	if len(req.Messages) > 0 {
		sum.SessionHash = hashOf("session-", req.System, req.Messages[0])
	}
	toolNames := map[string]string{}
	for _, raw := range req.Messages {
		var m transcript.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return RequestSummary{}, err
		}
		msg := Message{Role: m.Role}
		text, blocks, isText, err := transcript.DecodeContent(m.Content)
		if err != nil {
			return RequestSummary{}, err
		}
		if isText {
			msg.Blocks = []Block{{Kind: transcript.KindText, Label: transcript.TextLabel(m.Role), Bytes: len(text)}}
		} else {
			for _, b := range blocks {
				if len(b.CacheControl) > 0 {
					p.CacheControlCount++
				}
			}
			msg.Blocks = stripText(labeler.keyCalls(transcript.DecodeBlocks(blocks, m.Role, toolNames, labeler.Label)))
		}
		p.Messages = append(p.Messages, msg)
	}
	return sum, nil
}

// stripText removes block text so nothing readable reaches the ledger.
func stripText(blocks []Block) []Block {
	for i := range blocks {
		blocks[i].Text = ""
	}
	return blocks
}

// hashLabelBytes is how much of a hash the prefix and session labels keep.
const hashLabelBytes = 16

// hashOf hashes raw JSON values as sent, so a byte-identical prefix hashes
// identically and any change, including whitespace, does not.
func hashOf(prefix string, parts ...json.RawMessage) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
		h.Write([]byte{0})
	}
	return prefix + hex.EncodeToString(h.Sum(nil))[:hashLabelBytes]
}

// toolName reads a tool definition's name; built-in tools carry a type
// and a name, custom tools a name. Nothing else is decoded.
func toolName(raw json.RawMessage) string {
	var t struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &t); err != nil || t.Name == "" {
		return transcript.LabelUnknownTool
	}
	return transcript.SanitizeLabel(t.Name)
}

// systemSize handles both the string and the block-list form of system.
func systemSize(raw json.RawMessage) (int, int) {
	text, blocks, isText, err := transcript.DecodeContent(raw)
	if err != nil {
		return len(raw), 0
	}
	if isText {
		return len(text), 0
	}
	size, markers := 0, 0
	for _, b := range blocks {
		size += len(b.Text)
		if len(b.CacheControl) > 0 {
			markers++
		}
	}
	return size, markers
}

func hasCacheControl(raw json.RawMessage) bool {
	var probe struct {
		CacheControl json.RawMessage `json:"cache_control"`
	}
	return json.Unmarshal(raw, &probe) == nil && len(probe.CacheControl) > 0
}
