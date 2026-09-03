package ledger

import (
	"bufio"
	"bytes"
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

// labelKeyFile holds the per-ledger key that keys path hashes in labels.
const (
	labelKeyFile  = ".label-key"
	labelKeyBytes = 32
)

// probeBytes is how much of a file IsLedgerFile reads.
const probeBytes = 4096

// safeName restricts session ids to characters that are safe in a file name.
var safeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

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
// only the first few kilobytes.
func IsLedgerFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close() //nolint:errcheck // read-only probe
	var probe struct {
		Schema int `json:"schema"`
	}
	line, err := bufio.NewReaderSize(f, probeBytes).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return false
	}
	return json.Unmarshal(line, &probe) == nil && probe.Schema > 0
}

// ReadRecords reads every record in a ledger file and counts lines it could
// not decode.
func ReadRecords(path string) (records []Record, skipped int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open ledger: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file; a close error carries no information we can act on
	scanner := transcript.NewLineScanner(f)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			// A file exists before its first record is flushed.
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil || rec.Schema != SchemaVersion {
			// An older schema names fields differently; reading it as the
			// current one would produce figures that look measured and
			// are not.
			skipped++
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, skipped, fmt.Errorf("read ledger: %w", err)
	}
	return records, skipped, nil
}

// ReadFile turns one ledger file into a Session at the measured tier.
func ReadFile(path string) (*transcript.Session, error) {
	records, skipped, err := ReadRecords(path)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no records in %s", filepath.Base(path))
	}
	return sessionFromRecords(records, path, skipped), nil
}

// sessionFromRecords builds a session from a whole file.
func sessionFromRecords(records []Record, path string, skipped int) *transcript.Session {
	sort.SliceStable(records, func(i, j int) bool { return records[i].Timestamp.Before(records[j].Timestamp) })
	b := NewSessionBuilder(records[0].SessionID, path)
	b.session.Skipped = skipped
	for _, rec := range records {
		b.Add(rec)
	}
	return b.Session()
}

// SessionBuilder turns records into a session one at a time, so the proxy
// can score a session as it grows with the same code the offline reader
// uses. One lane per agent id, in order of first appearance. Only records
// that carry usage become requests; the rest (count_tokens, errors) are
// counted as skipped. Messages are memoized by identity, since every
// request's context is a prefix of the next one's.
type SessionBuilder struct {
	session *transcript.Session
	memo    map[string]*transcript.Message
	added   int
}

// NewSessionBuilder starts an empty session at the measured tier.
func NewSessionBuilder(id, path string) *SessionBuilder {
	return &SessionBuilder{
		session: &transcript.Session{ID: id, Path: path, Source: transcript.SourceLedger},
		memo:    map[string]*transcript.Message{},
	}
}

// Add appends one record. Records must arrive in time order.
func (b *SessionBuilder) Add(rec Record) {
	b.added++
	if rec.Response.Usage == nil || len(rec.Prompt.Messages) == 0 {
		b.session.Skipped++
		return
	}
	lane := b.session.Lane(rec.AgentID, rec.AgentID != "")
	lane.Requests = append(lane.Requests, requestFromRecord(rec, b.added-1, b.memo))
}

// Session is the session built so far. Lanes are shared with the builder;
// callers analyze, they do not modify.
func (b *SessionBuilder) Session() *transcript.Session { return b.session }

func requestFromRecord(rec Record, index int, memo map[string]*transcript.Message) *transcript.Request {
	req := &transcript.Request{
		ID:            rec.RequestID,
		Model:         rec.Model,
		Effort:        rec.Effort,
		Timestamp:     rec.Timestamp,
		Usage:         *rec.Response.Usage,
		AppliedEdits:  rec.Response.AppliedEdits,
		ClearedTokens: rec.Response.ClearedInputTokens,
	}
	if req.ID == "" {
		req.ID = fmt.Sprintf("ledger-%d", index)
	}
	// The prefix ahead of the messages is one synthetic system message so
	// a change in it shows up as a divergence at position zero.
	prefixID := prefixUUID(rec)
	prefix, ok := memo[prefixID]
	if !ok {
		prefix = &transcript.Message{UUID: prefixID, Role: transcript.RoleSystem, Timestamp: rec.Timestamp, Blocks: []Block{
			{Kind: transcript.KindText, Label: "system prompt", Bytes: rec.Prompt.SystemBytes},
			{Kind: transcript.KindOther, Label: fmt.Sprintf("tool definitions (%d tools)", rec.Prompt.ToolCount), Bytes: rec.Prompt.ToolBytes},
		}}
		memo[prefixID] = prefix
	}
	req.Context = append(req.Context, prefix)
	for i, m := range rec.Prompt.Messages {
		id := messageUUID(i, m)
		msg, ok := memo[id]
		if !ok {
			msg = &transcript.Message{UUID: id, Role: m.Role, Timestamp: rec.Timestamp, Blocks: sanitized(m.Blocks)}
			memo[id] = msg
		}
		req.Context = append(req.Context, msg)
	}
	req.Output = &transcript.Message{UUID: "out-" + req.ID, Role: transcript.RoleAssistant, Timestamp: rec.Timestamp.Add(time.Duration(rec.LatencyMS) * time.Millisecond), Blocks: sanitized(rec.Response.Blocks)}
	return req
}

// sanitized copies blocks with their labels made safe to print. Ledger
// files are data any local process could have written.
func sanitized(blocks []Block) []Block {
	out := make([]Block, len(blocks))
	for i, b := range blocks {
		b.Label = transcript.SanitizeLabel(b.Label)
		out[i] = b
	}
	return out
}

// prefixUUID identifies the system prefix by its hash when the record has
// one, and by its sizes otherwise, so an unchanged prefix keeps one
// identity across requests and a changed one does not.
func prefixUUID(rec Record) string {
	if rec.PrefixHash != "" {
		return rec.PrefixHash
	}
	return fmt.Sprintf("prefix-%d-%d-%d", rec.Prompt.SystemBytes, rec.Prompt.ToolBytes, rec.Prompt.ToolCount)
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
