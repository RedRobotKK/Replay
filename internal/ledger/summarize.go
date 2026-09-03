package ledger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"

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
	Messages          []rawMessage             `json:"messages"`
	OutputConfig      *struct{ Effort string } `json:"output_config"`
	ContextManagement json.RawMessage          `json:"context_management"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type rawBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text"`
	Thinking     string          `json:"thinking"`
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Input        json.RawMessage `json:"input"`
	ToolUseID    string          `json:"tool_use_id"`
	Content      json.RawMessage `json:"content"`
	IsError      bool            `json:"is_error"`
	CacheControl json.RawMessage `json:"cache_control"`
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

// Argument names that hold file paths. Everything else is content.
var pathArgs = map[string]bool{"file_path": true, "path": true}

// hashedPathBytes is how much of the hash the label keeps.
const hashedPathBytes = 12

// Label renders "Read r/3f9a…go" for path arguments and just the tool name
// otherwise.
func (l *Labeler) Label(name string, input json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil {
		return name
	}
	for arg := range pathArgs {
		v, ok := args[arg].(string)
		if !ok || v == "" {
			continue
		}
		mac := hmac.New(sha256.New, l.key)
		mac.Write([]byte(v))
		ext := path.Ext(v)
		if !safeExtension(ext) {
			ext = ""
		}
		return name + " r/" + hex.EncodeToString(mac.Sum(nil))[:hashedPathBytes] + ext
	}
	return name
}

// safeExtension accepts short, alphanumeric extensions only.
func safeExtension(ext string) bool {
	if len(ext) < 2 || len(ext) > 8 || ext[0] != '.' {
		return false
	}
	for _, r := range ext[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// SummarizeRequest reduces a Messages API request body to its structure.
// It returns the model and stream flag alongside so the caller does not
// parse the body twice. Labels come from the labeler and carry no content.
func SummarizeRequest(body []byte, labeler *Labeler) (Prompt, string, bool, string, error) {
	var req rawRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return Prompt{}, "", false, "", err
	}
	p := Prompt{ToolCount: len(req.Tools), ContextEdits: len(req.ContextManagement) > 0}
	p.SystemBytes, p.CacheControlCount = systemSize(req.System)
	for _, t := range req.Tools {
		p.ToolBytes += transcript.ContentBytes(t)
		if hasCacheControl(t) {
			p.CacheControlCount++
		}
	}
	toolNames := map[string]string{}
	for _, m := range req.Messages {
		msg := Message{Role: m.Role}
		var text string
		if err := json.Unmarshal(m.Content, &text); err == nil {
			msg.Blocks = []Block{{Kind: transcript.KindText, Bytes: len(text), Label: "user text"}}
			p.Messages = append(p.Messages, msg)
			continue
		}
		var blocks []rawBlock
		if err := json.Unmarshal(m.Content, &blocks); err != nil {
			return Prompt{}, "", false, "", err
		}
		for _, b := range blocks {
			if len(b.CacheControl) > 0 {
				p.CacheControlCount++
			}
			msg.Blocks = append(msg.Blocks, summarizeBlock(b, m.Role, toolNames, labeler))
		}
		p.Messages = append(p.Messages, msg)
	}
	effort := ""
	if req.OutputConfig != nil {
		effort = req.OutputConfig.Effort
	}
	return p, req.Model, req.Stream, effort, nil
}

// systemSize handles both the string and the block-list form of system.
func systemSize(raw json.RawMessage) (int, int) {
	if len(raw) == 0 {
		return 0, 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return len(s), 0
	}
	var blocks []rawBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return len(raw), 0
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

func summarizeBlock(b rawBlock, role string, toolNames map[string]string, labeler *Labeler) Block {
	switch b.Type {
	case transcript.KindText:
		label := "user text"
		if role == transcript.RoleAssistant {
			label = "assistant text"
		}
		return Block{Kind: b.Type, Bytes: len(b.Text), Label: label}
	case transcript.KindThinking:
		return Block{Kind: b.Type, Bytes: len(b.Thinking), Label: "assistant thinking"}
	case transcript.KindToolUse:
		if labeler != nil {
			toolNames[b.ID] = labeler.Label(b.Name, b.Input)
		} else {
			toolNames[b.ID] = b.Name
		}
		return Block{Kind: b.Type, Bytes: len(b.Name) + transcript.ContentBytes(b.Input), Label: "tool call: " + b.Name, ToolUseID: b.ID}
	case transcript.KindToolResult:
		name := toolNames[b.ToolUseID]
		if name == "" {
			name = "unknown tool"
		}
		return Block{Kind: b.Type, Bytes: transcript.ContentBytes(b.Content), Label: "tool result: " + name, ToolUseID: b.ToolUseID, IsError: b.IsError}
	default:
		return Block{Kind: transcript.KindOther, Bytes: transcript.ContentBytes(b.Content) + len(b.Text), Label: "other: " + b.Type}
	}
}
