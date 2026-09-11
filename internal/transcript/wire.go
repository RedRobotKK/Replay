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
	"unicode"
	"unicode/utf16"
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
	Role string `json:"role"`
	// ID is the provider's identifier for the response this line belongs to.
	// It is the only per-request identifier a transcript from a non-`cli`
	// entrypoint carries, and it groups the lines of one response exactly as
	// the top-level requestId does.
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *WireUsage      `json:"usage"`
}

// RawBlock is a content block as it appears on the wire. Unknown fields are
// ignored so a provider addition never breaks decoding.
type RawBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	// Source carries an image or document payload on the Anthropic shape,
	// where OpenAI-derived clients use Content.
	//
	// It was absent, so DecodeBlock's image arm read Content and every image
	// in a Claude Code transcript measured zero. The provider billed for the
	// bytes; the fit was told they weighed nothing, which drags the ratio for
	// every other block in the same session.
	Source       json.RawMessage `json:"source"`
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
		// The name computed here goes into toolNames and nowhere else: the
		// block below labels itself from rb.Name. So there is nothing to
		// compute when there is nowhere to put it, and nothing to recompute
		// when the id is already there.
		//
		// That second case is the common one. collectToolNames walks every
		// assistant tool_use block and labels it before the decoder builds a
		// single message, and then DecodeBlock walks the same blocks again —
		// twice more, once through the memoised context and once through the
		// directly decoded output — with the same map and the same label
		// function. The label is a pure function of its two arguments, so
		// each of those recomputed a value that was already in the map, and
		// labelling reads the whole tool input to do it.
		//
		// An entry already in the map is therefore trusted, which makes
		// toolNames an input as well as an output: the map and the label
		// function have to come from the same caller. Every caller today
		// either passes a map it filled itself with this label function
		// (claudecode) or a fresh empty one (the ledger, whose labels are
		// keyed and must never be replaced by plain ones).
		if toolNames != nil && rb.ID != "" {
			if _, named := toolNames[rb.ID]; !named {
				name := rb.Name
				if label != nil {
					name = label(rb.Name, rb.Input)
				}
				toolNames[rb.ID] = name
			}
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
		// Whichever shape carries the payload. Anthropic nests it under
		// source; OpenAI-derived clients put it in content. Measured as the
		// bytes that crossed the wire, so base64 counts as encoded rather than
		// decoded: the provider was handed the encoding.
		//
		// A url-referenced image has neither, and zero is the honest answer
		// there — the payload is not in the transcript to measure. BI4 pins
		// that distinction so a later change cannot let one stand for the
		// other.
		return Block{Kind: rb.Type, Label: rb.Type,
			Bytes: ContentBytes(rb.Content) + ContentBytes(rb.Source)}
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
//
// It reads the bytes without decoding them. The obvious implementation —
// a json.Decoder and a loop over Token() — costs far more than it looks:
// Token calls Decode once per scalar, which boxes every string and number
// on the heap and constructs a syntax error at the end of each value that
// it then discards. Measuring one small object allocated 98 times. Across a
// parse it was 26% of every allocation the parser made, for a function whose
// whole output is one int.
//
// A malformed value measures as its raw length. That is a different measure
// from the one above, and it is deliberate: something crossed the wire and
// was paid for, so the honest floor is the bytes that were sent. The scanner
// reproduces it exactly, including discarding whatever it had counted before
// the error.
func ContentBytes(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	n, ok := measureContent(raw)
	if !ok {
		return len(raw)
	}
	return n
}

// How a scan ended. The distinction between the last two is not a detail:
// running out of bytes is where the decoder this replaced returned io.EOF,
// which its caller read as a clean finish and kept the partial count for.
// `{"a":1` measures 2, not 6. Only a byte that cannot be there is an error.
type scanEnd int

const (
	scanComplete  scanEnd = iota // a whole value was read
	scanTruncated                // the input ran out at a boundary
	scanBad                      // a byte that cannot be there
)

// measureContent sums every top-level value in raw. There may be more than
// one — the decoder this replaced did not stop at the first, and `1 2`
// measured 2 — and trailing space after the last is not an error.
func measureContent(b []byte) (int, bool) {
	n, i := 0, 0
	for {
		// No end-of-input check here. measureJSONValue makes the same one
		// and reports scanTruncated, which the switch below already turns
		// into the same answer, so a check here can never change a result.
		m, next, how := measureJSONValue(b, i)
		n += m
		switch how {
		case scanComplete:
			i = next
		case scanTruncated:
			return n, true
		default:
			return 0, false
		}
	}
}

func skipJSONSpace(b []byte, i int) int {
	for i < len(b) {
		switch b[i] {
		case ' ', '\t', '\n', '\r':
			i++
		default:
			return i
		}
	}
	return i
}

// measureJSONValue measures one complete value starting at i and returns the
// index just past it. It walks containers with an explicit stack rather than
// recursion: this path has no nesting limit — twenty thousand open brackets
// are measured, not refused — and a recursive scanner would make the depth of
// a transcript's tool input a property of the goroutine stack.
func measureJSONValue(b []byte, i int) (n, end int, how scanEnd) {
	var inline [32]byte
	stack := inline[:0]
	wantValue := true

	for {
		if wantValue {
			i = skipJSONSpace(b, i)
			if i >= len(b) {
				return n, i, scanTruncated
			}
			switch b[i] {
			case '{':
				stack = append(stack, '{')
				i++
				i = skipJSONSpace(b, i)
				if i >= len(b) {
					return n, i, scanTruncated
				}
				if b[i] == '}' {
					i++
					stack = stack[:len(stack)-1]
					wantValue = false
					break
				}
				m, next, k := measureJSONKey(b, i)
				n += m
				if k != scanComplete {
					return n, next, k
				}
				i = next
				continue
			case '[':
				stack = append(stack, '[')
				i++
				i = skipJSONSpace(b, i)
				if i >= len(b) {
					return n, i, scanTruncated
				}
				if b[i] == ']' {
					i++
					stack = stack[:len(stack)-1]
					wantValue = false
					break
				}
				continue
			case '"':
				m, next, ok := measureJSONString(b, i)
				if !ok {
					return 0, 0, scanBad
				}
				n += m
				i = next
				wantValue = false
			case 't':
				if !jsonWord(b, i, "true") {
					return 0, 0, scanBad
				}
				n += len("true")
				i += len("true")
				wantValue = false
			case 'f':
				if !jsonWord(b, i, "false") {
					return 0, 0, scanBad
				}
				n += len("false")
				i += len("false")
				wantValue = false
			case 'n':
				if !jsonWord(b, i, "null") {
					return 0, 0, scanBad
				}
				n += len("null")
				i += len("null")
				wantValue = false
			default:
				m, next, ok := measureJSONNumber(b, i)
				if !ok {
					return 0, 0, scanBad
				}
				n += m
				i = next
				wantValue = false
			}
		}

		if len(stack) == 0 {
			return n, i, scanComplete
		}
		i = skipJSONSpace(b, i)
		if i >= len(b) {
			return n, i, scanTruncated
		}
		top := stack[len(stack)-1]
		switch b[i] {
		case ',':
			i++
			if top == '{' {
				i = skipJSONSpace(b, i)
				if i >= len(b) {
					return n, i, scanTruncated
				}
				m, next, k := measureJSONKey(b, i)
				n += m
				if k != scanComplete {
					return n, next, k
				}
				i = next
			}
			wantValue = true
		case '}':
			if top != '{' {
				return 0, 0, scanBad
			}
			stack = stack[:len(stack)-1]
			i++
		case ']':
			if top != '[' {
				return 0, 0, scanBad
			}
			stack = stack[:len(stack)-1]
			i++
		default:
			return 0, 0, scanBad
		}
	}
}

// measureJSONKey measures an object key and consumes the colon after it.
// Keys count toward the total, and a repeated key counts each time.
func measureJSONKey(b []byte, i int) (n, end int, how scanEnd) {
	n, i, ok := measureJSONString(b, i)
	if !ok {
		return 0, 0, scanBad
	}
	i = skipJSONSpace(b, i)
	if i >= len(b) {
		// The key was read whole and counted; the colon after it never
		// arrived. The decoder counted the key and stopped.
		return n, i, scanTruncated
	}
	if b[i] != ':' {
		return 0, 0, scanBad
	}
	return n, i + 1, scanComplete
}

func jsonWord(b []byte, i int, word string) bool {
	return i+len(word) <= len(b) && string(b[i:i+len(word)]) == word
}

// measureJSONString reports the length of the string literal at i once
// decoded, without decoding it.
//
// Failure returns the index it had reached, not zero. Zero sent a caller
// that ignored ok back to the start of the buffer, where the rescan reached
// the same answer by another route and the caller's own check became
// unobservable — the same trap readHex4 carried.
func measureJSONString(b []byte, i int) (n, end int, ok bool) {
	if i >= len(b) || b[i] != '"' {
		return 0, i, false
	}
	i++
	for i < len(b) {
		c := b[i]
		switch {
		case c == '"':
			return n, i + 1, true
		case c == '\\':
			i++
			if i >= len(b) {
				return 0, i, false
			}
			switch b[i] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				n++
				i++
			case 'u':
				r, next, good := readHex4(b, i+1)
				if !good {
					return 0, i, false
				}
				i = next
				if utf16.IsSurrogate(r) {
					// A high surrogate followed by a valid low one is one
					// rune and six more bytes consumed. Anything else is
					// the replacement character, and the escape that
					// failed to pair is left where it is to be read again.
					if r2, next2, good2 := readHex4Escape(b, i); good2 {
						if dec := utf16.DecodeRune(r, r2); dec != unicode.ReplacementChar {
							n += utf8.RuneLen(dec)
							i = next2
							break
						}
					}
					n += utf8.RuneLen(unicode.ReplacementChar)
					break
				}
				n += utf8.RuneLen(r)
			default:
				return 0, i, false
			}
		case c < 0x20:
			// A raw control character in a string literal is a syntax
			// error to the scanner this replaced, not a character.
			return 0, i, false
		default:
			r, size := utf8.DecodeRune(b[i:])
			if r == utf8.RuneError && size == 1 {
				// Not valid UTF-8. The decoder substituted the replacement
				// character, so one bad byte measures three.
				n += utf8.RuneLen(unicode.ReplacementChar)
				i++
				continue
			}
			n += size
			i += size
		}
	}
	return 0, i, false
}

// readHex4 reads four hex digits at i.
func readHex4(b []byte, i int) (rune, int, bool) {
	// Failure returns the index it was given, not zero. Returning zero sent
	// a caller that ignored ok back to the start of the buffer, where the
	// rescan happened to reach the same answer by a different route — which
	// made the caller's own check unobservable and hid it from the reviewer.
	if i+4 > len(b) {
		return 0, i, false
	}
	var r rune
	for k := 0; k < 4; k++ {
		c := b[i+k]
		switch {
		case '0' <= c && c <= '9':
			r = r*16 + rune(c-'0')
		case 'a' <= c && c <= 'f':
			r = r*16 + rune(c-'a'+10)
		case 'A' <= c && c <= 'F':
			r = r*16 + rune(c-'A'+10)
		default:
			return 0, i, false
		}
	}
	return r, i + 4, true
}

// readHex4Escape reads a full \uXXXX escape at i, for the low half of a
// surrogate pair.
func readHex4Escape(b []byte, i int) (rune, int, bool) {
	if i+2 > len(b) || b[i] != '\\' || b[i+1] != 'u' {
		return 0, i, false
	}
	return readHex4(b, i+2)
}

// measureJSONNumber validates a JSON number and reports the length of its
// literal, which is what json.Number.String returned for it.
func measureJSONNumber(b []byte, i int) (n, end int, ok bool) {
	start := i
	if i < len(b) && b[i] == '-' {
		i++
	}
	switch {
	case i >= len(b):
		return 0, 0, false
	case b[i] == '0':
		i++
	case '1' <= b[i] && b[i] <= '9':
		for i < len(b) && '0' <= b[i] && b[i] <= '9' {
			i++
		}
	default:
		return 0, 0, false
	}
	if i < len(b) && b[i] == '.' {
		i++
		if i >= len(b) || b[i] < '0' || b[i] > '9' {
			return 0, 0, false
		}
		for i < len(b) && '0' <= b[i] && b[i] <= '9' {
			i++
		}
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		if i >= len(b) || b[i] < '0' || b[i] > '9' {
			return 0, 0, false
		}
		for i < len(b) && '0' <= b[i] && b[i] <= '9' {
			i++
		}
	}
	return i - start, i, true
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
//
// It walks byte offsets rather than building a []rune. The labels reaching it
// are tool-call parameter values, which run to whole file contents, and
// converting one of those to runes to keep sixty of them allocated four bytes
// for every byte of the part being discarded: 43 MB of the 78 MB transcript
// benchmark. The walk stops at the first rune past the limit, so the cost is
// the width, not the label.
func TruncateLabel(s string, n int) string {
	if n <= 0 {
		return s
	}
	count, cut := 0, 0
	for i := range s {
		if count == n-1 {
			cut = i
		}
		count++
		if count > n {
			return s[:cut] + "…"
		}
	}
	// Fewer runes than the limit, or exactly the limit: nothing to cut.
	return s
}
