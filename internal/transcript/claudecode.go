package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Scanner sizing. Tool results can be large; the maximum is generous while
// still refusing pathological input.
const (
	scannerInitialBytes = 1 << 20
	maxLineBytes        = 64 << 20
)

// Line types Claude Code writes that carry conversation content.
const (
	lineTypeUser      = "user"
	lineTypeAssistant = "assistant"
)

// rawLine is the subset of a Claude Code JSONL line this parser reads.
// Unknown fields are ignored on purpose so a client update does not break
// parsing; unknown line types are counted in Session.Skipped.
type rawLine struct {
	Type          string          `json:"type"`
	UUID          string          `json:"uuid"`
	ParentUUID    string          `json:"parentUuid"`
	SessionID     string          `json:"sessionId"`
	Version       string          `json:"version"`
	Timestamp     string          `json:"timestamp"`
	RequestID     string          `json:"requestId"`
	APIBlockIndex int             `json:"apiBlockIndex"`
	IsSidechain   bool            `json:"isSidechain"`
	Effort        string          `json:"effort"`
	Message       json.RawMessage `json:"message"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *rawUsage       `json:"usage"`
}

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

type rawBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	Signature string          `json:"signature"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// ParseClaudeCodeFile parses one Claude Code transcript file.
func ParseClaudeCodeFile(path string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file; a close error carries no information we can act on

	s, err := ParseClaudeCode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	s.Path = path
	return s, nil
}

// ParseClaudeCode parses Claude Code JSONL from a reader.
func ParseClaudeCode(r io.Reader) (*Session, error) {
	lines, skipped, err := readLines(r)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("no conversation lines found")
	}

	byUUID := make(map[string]*rawLine, len(lines))
	for _, l := range lines {
		if l.UUID != "" {
			byUUID[l.UUID] = l
		}
	}

	session := &Session{Skipped: skipped}
	for _, l := range lines {
		if l.SessionID != "" && session.ID == "" {
			session.ID = l.SessionID
		}
		if l.Version != "" && session.ClientVersion == "" {
			session.ClientVersion = l.Version
		}
	}

	// Group assistant lines by request, preserving first-seen order.
	var order []string
	groups := make(map[string][]*rawLine)
	for _, l := range lines {
		if l.Type != lineTypeAssistant || l.RequestID == "" {
			continue
		}
		if _, seen := groups[l.RequestID]; !seen {
			order = append(order, l.RequestID)
		}
		groups[l.RequestID] = append(groups[l.RequestID], l)
	}

	dec := &decoder{toolNames: collectToolNames(lines), byUUID: byUUID, messages: make(map[string]*Message)}
	lanes := make(map[string]*Lane)
	var laneOrder []string

	for _, id := range order {
		group := groups[id]
		sort.SliceStable(group, func(i, j int) bool { return group[i].APIBlockIndex < group[j].APIBlockIndex })
		req, laneID, err := dec.buildRequest(group)
		if err != nil {
			// One malformed request (no usage on an interrupted call, a bad
			// timestamp) must not hide the rest of the session.
			session.Skipped++
			continue
		}
		lane, ok := lanes[laneID]
		if !ok {
			lane = &Lane{ID: laneID, Sidechain: req.Sidechain}
			lanes[laneID] = lane
			laneOrder = append(laneOrder, laneID)
		}
		lane.Requests = append(lane.Requests, req)
	}
	for _, id := range laneOrder {
		session.Lanes = append(session.Lanes, lanes[id])
	}
	if len(session.Lanes) == 0 {
		return nil, fmt.Errorf("no provider requests found")
	}
	return session, nil
}

func readLines(r io.Reader) ([]*rawLine, int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scannerInitialBytes), maxLineBytes)
	var lines []*rawLine
	skipped := 0
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var l rawLine
		if err := json.Unmarshal(raw, &l); err != nil {
			skipped++
			continue
		}
		if l.UUID == "" {
			// Housekeeping lines (queue operations, mode changes) carry no
			// conversation content and cannot appear in a parent chain.
			skipped++
			continue
		}
		lines = append(lines, &l)
	}
	if err := scanner.Err(); err != nil {
		return nil, skipped, fmt.Errorf("read transcript: %w", err)
	}
	return lines, skipped, nil
}

// collectToolNames maps tool_use ids to the tool name and a short label of
// its input so tool results can be attributed to what produced them.
func collectToolNames(lines []*rawLine) map[string]string {
	names := make(map[string]string)
	for _, l := range lines {
		if l.Type != lineTypeAssistant {
			continue
		}
		var m rawMessage
		if err := json.Unmarshal(l.Message, &m); err != nil {
			continue
		}
		var blocks []rawBlock
		if err := json.Unmarshal(m.Content, &blocks); err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type == KindToolUse && b.ID != "" {
				names[b.ID] = toolLabel(b.Name, b.Input)
			}
		}
	}
	return names
}

// toolLabel renders "Read path/to/file" style labels from a tool call. Only
// well-known argument names are used; everything else is just the tool name.
func toolLabel(name string, input json.RawMessage) string {
	var args map[string]any
	if err := json.Unmarshal(input, &args); err != nil || len(args) == 0 {
		return name
	}
	for _, key := range []string{"file_path", "path", "pattern", "command", "url", "query"} {
		if v, ok := args[key].(string); ok && v != "" {
			return name + " " + truncateLabel(v)
		}
	}
	return name
}

// labelMaxLen bounds stored labels. It is far wider than any table column
// so that distinct calls keep distinct labels; reports truncate for display.
const labelMaxLen = 400

func truncateLabel(s string) string {
	return TruncateLabel(strings.ReplaceAll(s, "\n", " "), labelMaxLen)
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

// decoder memoizes decoded messages by line uuid. Every request's context
// is a prefix of the next one's, so without this each line would be
// decoded once per request that carries it.
type decoder struct {
	toolNames map[string]string
	byUUID    map[string]*rawLine
	messages  map[string]*Message
}

func (d *decoder) buildRequest(group []*rawLine) (*Request, string, error) {
	byUUID, toolNames := d.byUUID, d.toolNames
	first := group[0]
	var firstMsg rawMessage
	if err := json.Unmarshal(first.Message, &firstMsg); err != nil {
		return nil, "", fmt.Errorf("decode assistant message: %w", err)
	}
	if firstMsg.Usage == nil {
		return nil, "", fmt.Errorf("assistant line has no usage")
	}
	ts, err := parseTime(first.Timestamp)
	if err != nil {
		return nil, "", err
	}

	req := &Request{
		ID:        first.RequestID,
		Model:     firstMsg.Model,
		Effort:    first.Effort,
		Timestamp: ts,
		Sidechain: first.IsSidechain,
		Usage:     convertUsage(firstMsg.Usage),
	}

	// Output: every block across the group's lines, in API order.
	out := &Message{UUID: first.UUID, Role: RoleAssistant, Timestamp: ts}
	for _, l := range group {
		var m rawMessage
		if err := json.Unmarshal(l.Message, &m); err != nil {
			return nil, "", fmt.Errorf("decode assistant line: %w", err)
		}
		blocks, err := decodeBlocks(m.Content, RoleAssistant, toolNames)
		if err != nil {
			return nil, "", err
		}
		out.Blocks = append(out.Blocks, blocks...)
	}
	req.Output = out

	// Context: walk the parent chain from the first output line back to the
	// root, collecting conversation messages. Consecutive assistant lines
	// with one request id collapse into one message.
	var chain []*rawLine
	for cur := byUUID[first.ParentUUID]; cur != nil; cur = byUUID[cur.ParentUUID] {
		chain = append(chain, cur)
		if len(chain) > len(byUUID) {
			return nil, "", fmt.Errorf("parent chain cycle at %s", cur.UUID)
		}
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}

	laneID := ""
	for i := 0; i < len(chain); i++ {
		l := chain[i]
		if laneID == "" && (l.Type == lineTypeUser || l.Type == lineTypeAssistant) {
			laneID = l.UUID
		}
		switch l.Type {
		case lineTypeUser:
			msg, err := d.userMessage(l)
			if err != nil {
				return nil, "", err
			}
			req.Context = append(req.Context, msg)
		case lineTypeAssistant:
			// Merge the run of lines that belong to the same request.
			runEnd := i
			for runEnd+1 < len(chain) && chain[runEnd+1].Type == lineTypeAssistant && chain[runEnd+1].RequestID == l.RequestID {
				runEnd++
			}
			msg, err := d.assistantMessage(chain[i : runEnd+1])
			if err != nil {
				return nil, "", err
			}
			req.Context = append(req.Context, msg)
			i = runEnd
		default:
			// Hook summaries and similar lines sit in the chain but are not
			// sent to the provider.
		}
	}
	if laneID == "" {
		laneID = first.UUID
	}
	return req, laneID, nil
}

func (d *decoder) userMessage(l *rawLine) (*Message, error) {
	if msg, ok := d.messages[l.UUID]; ok {
		return msg, nil
	}
	msg, err := decodeUserLine(l, d.toolNames)
	if err != nil {
		return nil, err
	}
	d.messages[l.UUID] = msg
	return msg, nil
}

func (d *decoder) assistantMessage(run []*rawLine) (*Message, error) {
	key := run[0].UUID
	if msg, ok := d.messages[key]; ok {
		return msg, nil
	}
	msg, err := decodeAssistantRun(run, d.toolNames)
	if err != nil {
		return nil, err
	}
	d.messages[key] = msg
	return msg, nil
}

func decodeUserLine(l *rawLine, toolNames map[string]string) (*Message, error) {
	var m rawMessage
	if err := json.Unmarshal(l.Message, &m); err != nil {
		return nil, fmt.Errorf("decode user message: %w", err)
	}
	ts, err := parseTime(l.Timestamp)
	if err != nil {
		return nil, err
	}
	msg := &Message{UUID: l.UUID, Role: RoleUser, Timestamp: ts}
	// User content is either a plain string or a list of blocks.
	var text string
	if err := json.Unmarshal(m.Content, &text); err == nil {
		msg.Blocks = []Block{{Kind: KindText, Label: "user text", Bytes: len(text), Text: text}}
		return msg, nil
	}
	blocks, err := decodeBlocks(m.Content, RoleUser, toolNames)
	if err != nil {
		return nil, err
	}
	msg.Blocks = blocks
	return msg, nil
}

func decodeAssistantRun(run []*rawLine, toolNames map[string]string) (*Message, error) {
	sort.SliceStable(run, func(i, j int) bool { return run[i].APIBlockIndex < run[j].APIBlockIndex })
	ts, err := parseTime(run[0].Timestamp)
	if err != nil {
		return nil, err
	}
	msg := &Message{UUID: run[0].UUID, Role: RoleAssistant, Timestamp: ts}
	for _, l := range run {
		var m rawMessage
		if err := json.Unmarshal(l.Message, &m); err != nil {
			return nil, fmt.Errorf("decode assistant message: %w", err)
		}
		blocks, err := decodeBlocks(m.Content, RoleAssistant, toolNames)
		if err != nil {
			return nil, err
		}
		msg.Blocks = append(msg.Blocks, blocks...)
	}
	return msg, nil
}

func decodeBlocks(content json.RawMessage, role string, toolNames map[string]string) ([]Block, error) {
	var raws []rawBlock
	if err := json.Unmarshal(content, &raws); err != nil {
		return nil, fmt.Errorf("decode %s content blocks: %w", role, err)
	}
	blocks := make([]Block, 0, len(raws))
	for _, rb := range raws {
		blocks = append(blocks, convertBlock(rb, role, toolNames))
	}
	return blocks, nil
}

func convertBlock(rb rawBlock, role string, toolNames map[string]string) Block {
	switch rb.Type {
	case KindText:
		label := "user text"
		if role == RoleAssistant {
			label = "assistant text"
		}
		return Block{Kind: KindText, Label: label, Bytes: len(rb.Text), Text: rb.Text}
	case KindThinking:
		// The signature is what travels on the wire when reasoning is
		// omitted; the thinking text may be empty.
		return Block{Kind: KindThinking, Label: "assistant thinking", Bytes: len(rb.Thinking), Text: rb.Thinking}
	case KindToolUse:
		return Block{Kind: KindToolUse, Label: "tool call: " + rb.Name, Bytes: len(rb.Name) + contentBytes(rb.Input), Text: string(rb.Input), ToolUseID: rb.ID, ToolName: rb.Name}
	case KindToolResult:
		text := toolResultText(rb.Content)
		name := toolNames[rb.ToolUseID]
		if name == "" {
			name = "unknown tool"
		}
		return Block{Kind: KindToolResult, Label: "tool result: " + name, Bytes: len(text), Text: text, ToolUseID: rb.ToolUseID, ToolName: name, IsError: rb.IsError}
	case KindImage, KindDocument:
		return Block{Kind: rb.Type, Label: rb.Type, Bytes: len(rb.Content)}
	default:
		return Block{Kind: KindOther, Label: "other: " + rb.Type, Bytes: len(rb.Content) + len(rb.Text)}
	}
}

// contentBytes measures a JSON value by its decoded content: string values
// and object keys by their length, numbers by their literal, booleans and
// null by their keyword. It is independent of escaping and key order, so a
// redacted transcript measures exactly like the original.
func contentBytes(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return len(raw)
	}
	return decodedBytes(v)
}

func decodedBytes(v any) int {
	switch val := v.(type) {
	case string:
		return len(val)
	case json.Number:
		return len(val.String())
	case bool:
		if val {
			return len("true")
		}
		return len("false")
	case nil:
		return len("null")
	case []any:
		n := 0
		for _, item := range val {
			n += decodedBytes(item)
		}
		return n
	case map[string]any:
		n := 0
		for k, item := range val {
			n += len(k) + decodedBytes(item)
		}
		return n
	default:
		return 0
	}
}

// toolResultText extracts the textual content of a tool result, which the
// client stores either as a string or as a list of text blocks.
func toolResultText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(content, &s); err == nil {
		return s
	}
	var parts []rawBlock
	if err := json.Unmarshal(content, &parts); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p.Text)
	}
	return sb.String()
}

func convertUsage(u *rawUsage) Usage {
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

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("line has no timestamp")
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", s, err)
	}
	return t, nil
}
