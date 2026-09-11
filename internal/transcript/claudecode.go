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
	Type       string `json:"type"`
	UUID       string `json:"uuid"`
	ParentUUID string `json:"parentUuid"`
	SessionID  string `json:"sessionId"`
	Version    string `json:"version"`
	Timestamp  string `json:"timestamp"`
	RequestID  string `json:"requestId"`
	// IsAPIErrorMessage marks a line the CLIENT wrote to record a failed
	// call, not a response the provider sent. It carries a message.model of
	// the literal string "<synthetic>" and a usage object of all zeros.
	IsAPIErrorMessage bool   `json:"isApiErrorMessage"`
	APIBlockIndex     int    `json:"apiBlockIndex"`
	IsSidechain       bool   `json:"isSidechain"`
	Effort            string `json:"effort"`
	// IsCompactSummary marks the record that replaced the history.
	IsCompactSummary bool `json:"isCompactSummary"`
	// CompactMetadata carries the sizes the client dropped. Its absence on a
	// compaction record is a client that stopped reporting them, not a
	// compaction that dropped nothing.
	CompactMetadata *rawCompaction `json:"compactMetadata"`
	// Message is decoded once, at read time, for the lines that carry one.
	Message *RawMessage `json:"message"`

	// The decoded content, kept so it is decoded once rather than once per
	// reader. Three passes over a transcript call DecodeContent on the same
	// Message.Content — collectToolNames, the request builder, and the
	// compaction scan — and each one unmarshals it into a fresh []RawBlock.
	// Every RawBlock carries four more json.RawMessage fields, each of which
	// copies its bytes, so one assistant line of twenty blocks produced
	// roughly 240 nested copies three times over, of content that cannot
	// change between the calls.
	//
	// Measured before this existed: 292 MB allocated and 745,015 allocations
	// to parse a 78 MB transcript, with json.RawMessage.UnmarshalJSON the
	// largest single item at 94 MB, and the allocator returning pages to the
	// OS accounting for nearly half the CPU.
	//
	// Not exported and not concurrent: forEachSession parallelises across
	// files, never within one, so a line's blocks are decoded and read on one
	// goroutine.
	decoded   bool
	decText   string
	decBlocks []RawBlock
	decIsText bool
	decErr    error
}

// content decodes this line's message content, once.
//
// The error is memoised with the result. A line whose content will not decode
// fails the same way for every reader, and re-attempting it per call site was
// three failures where the transcript has one defect.
//
// The returned slice is shared between callers, which is safe only because no
// caller writes to it. That is a property of the three call sites today rather
// than a guarantee of the type, and it is why this is unexported.
func (l *rawLine) content() (text string, blocks []RawBlock, isText bool, err error) {
	if l.decoded {
		return l.decText, l.decBlocks, l.decIsText, l.decErr
	}
	l.decoded = true
	if l.Message == nil {
		return "", nil, false, nil
	}
	l.decText, l.decBlocks, l.decIsText, l.decErr = DecodeContent(l.Message.Content)
	return l.decText, l.decBlocks, l.decIsText, l.decErr
}

// requestKey identifies the provider request an assistant line belongs to.
//
// Claude Code writes the top-level `requestId` only from the `cli`
// entrypoint. `sdk-cli`, `sdk-ts` and `claude-desktop` write the same
// `message.usage` and the same `message.id` and no `requestId` at all, so a
// parser keyed on `requestId` alone grouped nothing, `Lanes` came back empty
// and the whole file was discarded as unreadable.
//
// The message id is the provider's identifier for one response, so it groups
// the lines of one response exactly as the request id does. Measured over the
// 1821 transcripts on the machine this was found on: 28,665 request ids across
// 55,415 assistant lines, every one of them carrying exactly one message id,
// and every message id appearing under exactly one request id.
//
// The two are not interchangeable and this does not treat them as such. The
// request id wins wherever it exists, and a request that fell back says so in
// Request.IDFromMessage.
func (l *rawLine) requestKey() string {
	if l.RequestID != "" {
		return l.RequestID
	}
	if l.Message != nil {
		return l.Message.ID
	}
	return ""
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
		key := l.requestKey()
		// A client-written API error is not a provider request. Forty of them
		// sit in the 1821 transcripts this was measured on, twenty-four
		// carrying a requestId and counted as requests long before the
		// message-id fallback existed. Their model is the literal string
		// "<synthetic>" and their usage is all zeros, and cmd/replay/cost.go
		// names a lane's model from its FIRST request: one placeholder at the
		// head of a lane took that lane from claude-opus-5 to "<synthetic>",
		// which is not in any price table, and its avoidable figure from
		// 1,586,545 tokens to zero with the cost unchanged.
		//
		// They stay in the parent chain, because the next turn genuinely saw
		// the error text. They are only refused the status of a request.
		if l.Type != lineTypeAssistant || key == "" || l.IsAPIErrorMessage {
			continue
		}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], l)
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
		_, blocks, _, err := l.content()
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

	// chain is scratch for the parent walk in buildRequest, reused across
	// requests. Every request walks its whole ancestry from scratch, so a
	// fresh slice per request grew by doubling once per request and the
	// total was quadratic in the depth of the chain: 225 MB for a chain of
	// 2000 turns, most of it slice headers thrown away immediately.
	//
	// Nothing may retain it. buildRequest hands sub-slices of it to
	// assistantMessage, which reads them and returns a Message built from
	// its own storage; decodeAssistantRun reorders the sub-slice in place
	// but keeps no reference to it.
	chain []*rawLine
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
		ID:            first.requestKey(),
		IDFromMessage: first.RequestID == "",
		Model:         first.Message.Model,
		Effort:        first.Effort,
		Timestamp:     ts,
		Usage:         first.Message.Usage.Usage(),
		Output:        out,
	}

	// Context: walk the parent chain from the first output line back to the
	// root, collecting conversation messages. Consecutive assistant lines
	// with one request id collapse into one message.
	chain := d.chain[:0]
	for cur := d.byUUID[first.ParentUUID]; cur != nil; cur = d.byUUID[cur.ParentUUID] {
		chain = append(chain, cur)
		if len(chain) > len(d.byUUID) {
			return nil, "", fmt.Errorf("parent chain cycle at %s", cur.UUID)
		}
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	d.chain = chain

	// One context message per chain line at most: user lines contribute
	// one, a run of assistant lines contributes one between them, and hook
	// summaries contribute none. Sizing it here replaces the doubling
	// growth of a slice that is appended to once per ancestor.
	req.Context = make([]*Message, 0, len(chain))

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
			// Merge the run of lines that belong to the same request. Keyed
			// on requestKey rather than the raw requestId: where no line in
			// the file carries one, comparing the absent field to itself is
			// "" == "", which merges every consecutive assistant line in the
			// chain into a single message regardless of which response wrote
			// it.
			runEnd := i
			for runEnd+1 < len(chain) && chain[runEnd+1].Type == lineTypeAssistant && chain[runEnd+1].requestKey() == l.requestKey() {
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
		text, blocks, isText, err := l.content()
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
		_, blocks, _, err := l.content()
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
