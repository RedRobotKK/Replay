package ledger

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// File permissions: the ledger is the user's own data and nobody else's.
const (
	dirPerm  = 0o700
	filePerm = 0o600
)

// safeName restricts session ids to characters that are safe in a file name.
var safeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// labelKeyFile holds the per-ledger key that keys path hashes in labels.
const (
	labelKeyFile  = ".label-key"
	labelKeyBytes = 32
)

// Store appends records to one file per session.
type Store struct {
	dir     string
	labeler *Labeler
	mu      sync.Mutex
}

// Open creates the ledger directory and its label key if needed.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("create ledger directory: %w", err)
	}
	key, err := loadOrCreateKey(filepath.Join(dir, labelKeyFile))
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir, labeler: NewLabeler(key)}, nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil && len(key) == labelKeyBytes {
		return key, nil
	}
	key = make([]byte, labelKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate label key: %w", err)
	}
	if err := os.WriteFile(path, key, filePerm); err != nil {
		return nil, fmt.Errorf("write label key: %w", err)
	}
	return key, nil
}

// Dir is the ledger directory.
func (s *Store) Dir() string { return s.dir }

// Labeler is the store's content-free labeler.
func (s *Store) Labeler() *Labeler { return s.labeler }

// Append writes one record to its session file.
func (s *Store) Append(rec Record) error {
	rec.Schema = SchemaVersion
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode ledger record: %w", err)
	}
	name := safeName.ReplaceAllString(rec.SessionID, "_")
	if name == "" {
		name = "unknown"
	}
	path := filepath.Join(s.dir, name+".jsonl")

	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("open ledger file: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close() // the write error is the one worth reporting
		return fmt.Errorf("write ledger record: %w", err)
	}
	return f.Close()
}

// IsLedgerFile reports whether a file starts with a ledger record. It reads
// only the first line.
func IsLedgerFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // read-only probe
	var probe struct {
		Schema int `json:"schema"`
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)
	if !scanner.Scan() {
		return false
	}
	return json.Unmarshal(scanner.Bytes(), &probe) == nil && probe.Schema > 0
}

// ReadFile turns one ledger file into a Session at the measured tier.
func ReadFile(path string) (*transcript.Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ledger: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file; a close error carries no information we can act on

	var records []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 64<<20)
	skipped := 0
	for scanner.Scan() {
		var rec Record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			skipped++
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no records in %s", filepath.Base(path))
	}
	return sessionFromRecords(records, path, skipped), nil
}

// sessionFromRecords builds lanes from records: one lane per agent id, in
// order of first appearance. Only records that carry usage become
// requests; the rest (count_tokens, errors) are counted as skipped.
func sessionFromRecords(records []Record, path string, skipped int) *transcript.Session {
	sort.SliceStable(records, func(i, j int) bool { return records[i].Timestamp.Before(records[j].Timestamp) })
	session := &transcript.Session{ID: records[0].SessionID, Path: path, Source: transcript.SourceLedger, PrefixVisible: true, Skipped: skipped}
	lanes := map[string]*transcript.Lane{}
	var order []string
	for i, rec := range records {
		if rec.Response.Usage == nil || len(rec.Prompt.Messages) == 0 {
			session.Skipped++
			continue
		}
		lane, ok := lanes[rec.AgentID]
		if !ok {
			lane = &transcript.Lane{ID: rec.AgentID, Sidechain: rec.AgentID != ""}
			lanes[rec.AgentID] = lane
			order = append(order, rec.AgentID)
		}
		lane.Requests = append(lane.Requests, requestFromRecord(rec, i))
	}
	for _, id := range order {
		session.Lanes = append(session.Lanes, lanes[id])
	}
	return session
}

func requestFromRecord(rec Record, index int) *transcript.Request {
	u := rec.Response.Usage
	req := &transcript.Request{
		ID:        rec.RequestID,
		Model:     rec.Model,
		Effort:    rec.Effort,
		Timestamp: rec.Timestamp,
		Sidechain: rec.AgentID != "",
		Usage: transcript.Usage{
			Input: u.Input, CacheCreation: u.CacheCreation, CacheRead: u.CacheRead, Output: u.Output,
			ThinkingTokens: u.ThinkingTokens, Create5m: u.Create5m, Create1h: u.Create1h,
		},
	}
	if req.ID == "" {
		req.ID = fmt.Sprintf("ledger-%d", index)
	}
	// The prefix ahead of the messages is one synthetic system message so
	// a change in it shows up as a divergence at position zero.
	prefix := &transcript.Message{UUID: "prefix", Role: transcript.RoleSystem, Timestamp: rec.Timestamp, Blocks: []transcript.Block{
		{Kind: transcript.KindText, Label: "system prompt", Bytes: rec.Prompt.SystemBytes},
		{Kind: transcript.KindOther, Label: fmt.Sprintf("tool definitions (%d tools)", rec.Prompt.ToolCount), Bytes: rec.Prompt.ToolBytes},
	}}
	prefix.UUID = prefixUUID(rec.Prompt)
	req.Context = append(req.Context, prefix)
	for i, m := range rec.Prompt.Messages {
		msg := &transcript.Message{UUID: messageUUID(i, m), Role: m.Role, Timestamp: rec.Timestamp}
		for _, b := range m.Blocks {
			// Ledger files are data any local process could have written;
			// labels are sanitized again on the way to a terminal.
			msg.Blocks = append(msg.Blocks, transcript.Block{Kind: b.Kind, Label: transcript.SanitizeLabel(b.Label), Bytes: b.Bytes, ToolUseID: b.ToolUseID, IsError: b.IsError})
		}
		req.Context = append(req.Context, msg)
	}
	req.Output = &transcript.Message{UUID: "out-" + req.ID, Role: transcript.RoleAssistant, Timestamp: rec.Timestamp.Add(time.Duration(rec.LatencyMS) * time.Millisecond)}
	for _, b := range rec.Response.Blocks {
		req.Output.Blocks = append(req.Output.Blocks, transcript.Block{Kind: b.Kind, Label: transcript.SanitizeLabel(b.Label), Bytes: b.Bytes, ToolUseID: b.ToolUseID})
	}
	return req
}

// prefixUUID identifies the system prefix by its sizes, so an unchanged
// prefix keeps one identity across requests and a changed one does not.
func prefixUUID(p Prompt) string {
	return fmt.Sprintf("prefix-%d-%d-%d", p.SystemBytes, p.ToolBytes, p.ToolCount)
}

// messageUUID identifies a message by position and shape. Ledger records
// do not carry client message ids, so shape is the identity: a message
// whose blocks changed size is a different message, which is exactly the
// history-edit signal the diff looks for.
func messageUUID(i int, m Message) string {
	total := 0
	for _, b := range m.Blocks {
		total += b.Bytes
	}
	return fmt.Sprintf("m%d-%s-%d-%d", i, m.Role, len(m.Blocks), total)
}
