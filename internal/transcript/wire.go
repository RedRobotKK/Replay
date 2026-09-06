package transcript

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path"
	"strings"
	"unicode/utf8"
)

// Scanner sizing for line-oriented files (transcripts, ledgers). Tool
// results can be large; the maximum is generous while still refusing
// pathological input.
const (
	// The starting size, not the ceiling: bufio.Scanner grows on demand up to
	// maxLineBytes, so a large line still parses.
	//
	// This was 1 MB, which made it the single largest allocation site in the
	// program — 1,431 MB across a 1.35 GB corpus, 28% of every byte the
	// pipeline allocated, and larger than the median transcript, so for half
	// the corpus the buffer exceeded the whole file. The longest real line
	// measured across that corpus is 2.77 MB, far under the cap, so the only
	// effect of starting big was allocating memory most files never used.
	scannerInitialBytes = 64 << 10
	maxLineBytes        = 64 << 20
)

// NewLineScanner returns a scanner sized for transcript and ledger lines.
func NewLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scannerInitialBytes), maxLineBytes)
	return scanner
}

// RawMessage is a message as it appears on the provider wire and in
// transcripts: a role and content that is either a string or a block list.
type RawMessage struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *WireUsage      `json:"usage"`
}

// RawBlock is a content block as it appears on the wire. Unknown fields are
// ignored so a provider addition never breaks decoding.
type RawBlock struct {
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

// WireUsage is the provider's usage object as serialized on the wire.
type WireUsage struct {
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

// Usage converts the wire shape to the analysis shape. A nil receiver
// yields the zero value.
func (u *WireUsage) Usage() Usage {
	if u == nil {
		return Usage{}
	}
	out := Usage{Input: u.Input, CacheCreation: u.CacheCreation, CacheRead: u.CacheRead, Output: u.Output}
	if u.OutputDetails != nil {
		out.ThinkingTokens = u.OutputDetails.Thinking
	}
	if u.CacheBreak != nil {
		out.Create5m = u.CacheBreak.Short
		out.Create1h = u.CacheBreak.Long
	}
	return out
}

// DecodeContent decodes message or tool-result content, which the wire
// carries either as a JSON string or as a block list. It peeks at the first
// byte rather than trying both decodings.
func DecodeContent(raw json.RawMessage) (text string, blocks []RawBlock, isText bool, err error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] == 'n' {
		return "", nil, false, nil
	}
	if trimmed[0] == '"' {
		err = json.Unmarshal(trimmed, &text)
		return text, nil, true, err
	}
	err = json.Unmarshal(trimmed, &blocks)
	return "", blocks, false, err
}

// LabelFunc names a tool call for attribution from its name and input.
type LabelFunc func(name string, input json.RawMessage) string

// DecodeBlock converts a wire block to the analysis block. toolNames maps
// tool_use ids to labels so results attribute to the call that produced
// them; label names new calls and may be nil to use the bare tool name.
func DecodeBlock(rb RawBlock, role string, toolNames map[string]string, label LabelFunc) Block {
	switch rb.Type {
	case KindText:
		return Block{Kind: KindText, Label: TextLabel(role), Bytes: len(rb.Text), Text: rb.Text}
	case KindThinking:
		// The signature travels on the wire when reasoning is omitted; the
		// thinking text may be empty. Its tokens come from usage, not bytes.
		return Block{Kind: KindThinking, Label: LabelAssistantThinking, Bytes: len(rb.Thinking), Text: rb.Thinking}
	case KindToolUse:
		name := rb.Name
		if label != nil {
			name = label(rb.Name, rb.Input)
		}
		if toolNames != nil && rb.ID != "" {
			toolNames[rb.ID] = name
		}
		return Block{Kind: KindToolUse, Label: LabelToolCallPrefix + rb.Name, Bytes: len(rb.Name) + ContentBytes(rb.Input), Text: string(rb.Input), ToolUseID: rb.ID, ToolName: rb.Name, CallKey: CallKey(rb.Name, rb.Input)}
	case KindToolResult:
		text := toolResultText(rb.Content)
		name := toolNames[rb.ToolUseID]
		if name == "" {
			name = LabelUnknownTool
		}
		return Block{Kind: KindToolResult, Label: LabelToolResultPrefix + name, Bytes: len(text), Text: text, ToolUseID: rb.ToolUseID, ToolName: name, IsError: rb.IsError}
	case KindImage, KindDocument:
		return Block{Kind: rb.Type, Label: rb.Type, Bytes: len(rb.Content)}
	default:
		return Block{Kind: KindOther, Label: LabelOtherPrefix + rb.Type, Bytes: ContentBytes(rb.Content) + len(rb.Text)}
	}
}

// DecodeBlocks converts a block list.
func DecodeBlocks(raws []RawBlock, role string, toolNames map[string]string, label LabelFunc) []Block {
	blocks := make([]Block, 0, len(raws))
	for _, rb := range raws {
		blocks = append(blocks, DecodeBlock(rb, role, toolNames, label))
	}
	return blocks
}

// toolResultText extracts the textual content of a tool result, which the
// client stores either as a string or as a list of text blocks.
func toolResultText(content json.RawMessage) string {
	text, parts, isText, err := DecodeContent(content)
	if err != nil {
		return ""
	}
	if isText {
		return text
	}
	if len(parts) == 1 {
		return parts[0].Text
	}
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p.Text)
	}
	return sb.String()
}

// CallKey identifies a tool call by name and input without retaining the
// input. Identical calls share a key; the key reveals nothing about the
// arguments.
func CallKey(name string, input json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte{0})
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ContentBytes measures a JSON value by its decoded content: string values
// and object keys by their length, numbers by their literal, booleans and
// null by their keyword. It is independent of escaping and key order, so a
// redacted transcript measures exactly like the original.
func ContentBytes(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	n := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return n
			}
			return len(raw)
		}
		switch v := tok.(type) {
		case string:
			n += len(v)
		case json.Number:
			n += len(v.String())
		case bool:
			if v {
				n += len("true")
			} else {
				n += len("false")
			}
		case nil:
			n += len("null")
		}
	}
}

// Tool-call argument names, in the order they are consulted for labels.
// LabelArgs are shown in transcript-tier labels; PathArgs are the subset
// that hold file paths and keep their extension when hashed.
var (
	LabelArgs = []string{"file_path", "path", "pattern", "command", "url", "query"}
	PathArgs  = []string{"file_path", "path"}
)

// labelValues is the typed view of the arguments labels are built from, so
// a large tool input (a file write) is not decoded into a generic map.
type labelValues struct {
	FilePath string `json:"file_path"`
	Path     string `json:"path"`
	Pattern  string `json:"pattern"`
	Command  string `json:"command"`
	URL      string `json:"url"`
	Query    string `json:"query"`
}

func (v labelValues) get(arg string) string {
	switch arg {
	case "file_path":
		return v.FilePath
	case "path":
		return v.Path
	case "pattern":
		return v.Pattern
	case "command":
		return v.Command
	case "url":
		return v.URL
	case "query":
		return v.Query
	}
	return ""
}

// LabelArg returns the first label argument present in a tool input, by
// name and value, or ok=false when none is.
func LabelArg(input json.RawMessage, args []string) (arg, value string, ok bool) {
	var v labelValues
	if err := json.Unmarshal(input, &v); err != nil {
		return "", "", false
	}
	for _, a := range args {
		if s := v.get(a); s != "" {
			return a, s, true
		}
	}
	return "", "", false
}

// labelMaxLen bounds stored labels. It is far wider than any table column
// so that distinct calls keep distinct labels; reports truncate for display.
const labelMaxLen = 400

// ToolLabel renders "Read path/to/file" style labels from a tool call for
// the transcript tier, where the user's own content may be shown to them.
func ToolLabel(name string, input json.RawMessage) string {
	if _, v, ok := LabelArg(input, LabelArgs); ok {
		return name + " " + TruncateLabel(SanitizeLabel(v), labelMaxLen)
	}
	return name
}

// HashedLabelBytes is how much of a hash a hashed path label keeps.
const HashedLabelBytes = 12

// HashedPathLabel renders a content-free path label: a hash prefix plus the
// path's extension when it is a plain one. Callers supply the hash so the
// redactor (salted, per file) and the ledger (keyed, per store) keep their
// own secrets.
func HashedPathLabel(hexDigest, original string) string {
	ext := path.Ext(original)
	if !SafeExtension(ext) {
		ext = ""
	}
	return "r/" + hexDigest[:HashedLabelBytes] + ext
}

// SafeExtension accepts short, alphanumeric extensions only.
func SafeExtension(ext string) bool {
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

// SanitizeLabel makes a label safe to print: control characters (including
// escape, carriage return, and the C1 range) become spaces so content that
// an agent read from an untrusted file cannot drive the user's terminal.
func SanitizeLabel(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return ' '
		}
		return r
	}, s)
}

// TruncateLabel shortens a label to at most n runes, ending in an ellipsis
// when it was cut. It never splits a multi-byte character.
func TruncateLabel(s string, n int) string {
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}
