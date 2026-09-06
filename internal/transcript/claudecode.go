package transcript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
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
	Type          string `json:"type"`
	UUID          string `json:"uuid"`
	ParentUUID    string `json:"parentUuid"`
	SessionID     string `json:"sessionId"`
	Version       string `json:"version"`
	Timestamp     string `json:"timestamp"`
	RequestID     string `json:"requestId"`
	APIBlockIndex int    `json:"apiBlockIndex"`
	IsSidechain   bool   `json:"isSidechain"`
	Effort        string `json:"effort"`
	// IsCompactSummary marks the record that replaced the history.
	IsCompactSummary bool `json:"isCompactSummary"`
	// CompactMetadata carries the sizes the client dropped. Its absence on a
	// compaction record is a client that stopped reporting them, not a
	// compaction that dropped nothing.
	CompactMetadata *rawCompaction `json:"compactMetadata"`
	// Message is decoded once, at read time, for the lines that carry one.
	Message *RawMessage `json:"message"`
}

type rawCompaction struct {
	Trigger                 string `json:"trigger"`
	PreTokens               int    `json:"preTokens"`
	PostTokens              int    `json:"postTokens"`
	CumulativeDroppedTokens int    `json:"cumulativeDroppedTokens"`
	DurationMS              int    `json:"durationMs"`
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
		byUUID[l.UUID] = l
	}

	session := &Session{Skipped: skipped, Source: SourceTranscript}
	// sawBoundary pairs a compact_boundary with the summary that follows it.
	sawBoundary := false
	for _, l := range lines {
		if l.SessionID != "" && session.ID == "" {
			session.ID = l.SessionID
		}
		if l.Version != "" && session.ClientVersion == "" {
			session.ClientVersion = l.Version
		}
		// A compaction is a PAIR of records, not one. The client writes a
		// boundary - type "system", subtype "compact_boundary", carrying
		// compactMetadata and no message - and then the summary on the next
		// line, type "user" with isCompactSummary set and the text in
		// message.content.
		//
		// Accepting either marker independently counted every compaction
		// twice, and the overstatement note derived from that count was
		// doubled with it. The boundary is the record that carries the sizes,
		// so it is the one that counts; a summary is only counted when it
		// follows no boundary, which is a client that stopped writing them
		// rather than a compaction that did not happen.
		switch {
		case l.CompactMetadata != nil:
			m := l.CompactMetadata
			session.Compactions = append(session.Compactions, Compaction{
				Trigger:           m.Trigger,
				PreTokens:         m.PreTokens,
				PostTokens:        m.PostTokens,
				CumulativeDropped: m.CumulativeDroppedTokens,
				DurationMS:        m.DurationMS,
			})
			sawBoundary = true
		case l.IsCompactSummary && !sawBoundary:
			session.Compactions = append(session.Compactions, Compaction{})
		case l.IsCompactSummary:
			// Paired with the boundary just seen; already counted.
			sawBoundary = false
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
		lane := session.Lane(laneID, group[0].IsSidechain)
		lane.Requests = append(lane.Requests, req)
	}
	if len(session.Lanes) == 0 {
		return nil, fmt.Errorf("no provider requests found")
	}
	return session, nil
}

func readLines(r io.Reader) ([]*rawLine, int, error) {
	scanner := NewLineScanner(r)
	var lines []*rawLine
	skipped := 0
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var l rawLine
		if err := json.Unmarshal(raw, &l); err != nil || l.UUID == "" {
			// Unparseable lines, and housekeeping lines (queue operations,
			// mode changes) that carry no uuid and cannot sit in a parent
			// chain, are counted and skipped.
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

// collectToolNames maps tool_use ids to labels so tool results can be
// attributed to what produced them.
func collectToolNames(lines []*rawLine) map[string]string {
	names := make(map[string]string)
	for _, l := range lines {
		if l.Type != lineTypeAssistant || l.Message == nil {
			continue
		}
		_, blocks, _, err := DecodeContent(l.Message.Content)
		if err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type == KindToolUse && b.ID != "" {
				names[b.ID] = ToolLabel(b.Name, b.Input)
			}
		}
	}
	return names
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
	first := group[0]
	if first.Message == nil || first.Message.Usage == nil {
		return nil, "", fmt.Errorf("assistant line has no usage")
	}
	ts, err := parseTime(first.Timestamp)
	if err != nil {
		return nil, "", err
	}
	// The output is decoded directly, never through the memo: the same
	// lines reappear in later contexts as runs of the parent chain, and a
	// parallel tool call's lines are interleaved there with their results,
	// so a run holds fewer lines than the whole group.
	out, err := decodeAssistantRun(group, d.toolNames)
	if err != nil {
		return nil, "", err
	}
	req := &Request{
		ID:        first.RequestID,
		Model:     first.Message.Model,
		Effort:    first.Effort,
		Timestamp: ts,
		Usage:     first.Message.Usage.Usage(),
		Output:    out,
	}

	// Context: walk the parent chain from the first output line back to the
	// root, collecting conversation messages. Consecutive assistant lines
	// with one request id collapse into one message.
	var chain []*rawLine
	for cur := d.byUUID[first.ParentUUID]; cur != nil; cur = d.byUUID[cur.ParentUUID] {
		chain = append(chain, cur)
		if len(chain) > len(d.byUUID) {
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

// memo returns the cached message for key or builds and caches it.
func (d *decoder) memo(key string, build func() (*Message, error)) (*Message, error) {
	if msg, ok := d.messages[key]; ok {
		return msg, nil
	}
	msg, err := build()
	if err != nil {
		return nil, err
	}
	d.messages[key] = msg
	return msg, nil
}

func (d *decoder) userMessage(l *rawLine) (*Message, error) {
	return d.memo(l.UUID, func() (*Message, error) {
		if l.Message == nil {
			return nil, fmt.Errorf("user line %s has no message", l.UUID)
		}
		ts, err := parseTime(l.Timestamp)
		if err != nil {
			return nil, err
		}
		msg := &Message{UUID: l.UUID, Role: RoleUser, Timestamp: ts}
		text, blocks, isText, err := DecodeContent(l.Message.Content)
		if err != nil {
			return nil, fmt.Errorf("decode user content: %w", err)
		}
		if isText {
			msg.Blocks = []Block{{Kind: KindText, Label: LabelUserText, Bytes: len(text), Text: text}}
			return msg, nil
		}
		msg.Blocks = DecodeBlocks(blocks, RoleUser, d.toolNames, ToolLabel)
		return msg, nil
	})
}

// assistantMessage merges a run of assistant lines that share a request id
// into one message, in API block order.
func (d *decoder) assistantMessage(run []*rawLine) (*Message, error) {
	return d.memo(run[0].UUID, func() (*Message, error) { return decodeAssistantRun(run, d.toolNames) })
}

// decodeAssistantRun merges the lines of one assistant turn into one
// message, in API block order.
func decodeAssistantRun(run []*rawLine, toolNames map[string]string) (*Message, error) {
	sort.SliceStable(run, func(i, j int) bool { return run[i].APIBlockIndex < run[j].APIBlockIndex })
	ts, err := parseTime(run[0].Timestamp)
	if err != nil {
		return nil, err
	}
	msg := &Message{UUID: run[0].UUID, Role: RoleAssistant, Timestamp: ts}
	for _, l := range run {
		if l.Message == nil {
			return nil, fmt.Errorf("assistant line %s has no message", l.UUID)
		}
		_, blocks, _, err := DecodeContent(l.Message.Content)
		if err != nil {
			return nil, fmt.Errorf("decode assistant content: %w", err)
		}
		msg.Blocks = append(msg.Blocks, DecodeBlocks(blocks, RoleAssistant, toolNames, ToolLabel)...)
	}
	return msg, nil
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
