package ledger

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/ownerdir"
	"github.com/RedRobotKK/Replay/internal/transcript"
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
	dir      string
	labeler  *Labeler
	mu       sync.Mutex
	pins     map[string]Pin
	revert   Revert
	reverted bool
}

// Open creates the ledger directory and its label key if needed.
func Open(dir string) (*Store, error) {
	// Create it if it is missing and VERIFY it if it is not: os.MkdirAll
	// applies its mode only when it creates, so a pre-existing world-writable
	// ~/.replay/ledger used to be accepted in silence. Finding 7.
	if err := ownerdir.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("ledger directory: %w", err)
	}
	keyPath := filepath.Join(dir, labelKeyFile)
	// The label key is what makes the hashed path labels and the tool call
	// keys anonymous, so it is checked by name rather than left to the
	// directory: os.WriteFile does not change the mode of a file that is
	// already there.
	if err := ownerdir.EnsureFile(keyPath); err != nil {
		return nil, fmt.Errorf("ledger label key: %w", err)
	}
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	pins, err := loadPins(filepath.Join(dir, pinsFile))
	if err != nil {
		return nil, err
	}
	revert, reverted := loadRevert(filepath.Join(dir, revertFile))
	return &Store{dir: dir, labeler: NewLabeler(key), pins: pins, revert: revert, reverted: reverted}, nil
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
	// The response half of the record arrives with unkeyed call identities,
	// because nothing downstream of the wire has the ledger secret. Key them
	// here, at the one place a record becomes a file, so both halves of what
	// a ledger holder can read have the same property. Finding 4.
	rec.Response.Blocks = s.labeler.KeyResponseCalls(rec.Response.Blocks)
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode ledger record: %w", err)
	}
	path := filepath.Join(s.dir, sessionFileName(rec.SessionID)+".jsonl")

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

// sessionFileName maps a session id to a file name. Ids that need
// characters replaced also get a short hash of the original, so two ids
// that sanitize alike never share a file.
func sessionFileName(id string) string {
	name := safeName.ReplaceAllString(id, "_")
	if name == "" {
		return "unknown"
	}
	if name == id {
		return name
	}
	sum := sha256.Sum256([]byte(id))
	return name + "-" + hex.EncodeToString(sum[:])[:8]
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
// ReadRecords reads a ledger file.
//
// Three return values because there are three different facts, and they used
// to be two. `skipped` counts complete lines that are not records of the
// current schema — data loss, or an upgrade. `incomplete` says the file does
// not end in a newline, which means the last record had not finished being
// written when it was read.
//
// Collapsing the second into the first told a reader of a LIVE ledger that
// records had been skipped. Store.Append writes one record per os.File.Write,
// and Go loops on a short write, so any reader polling a ledger that
// `replay serve` is still writing can land inside that loop. Nothing is lost
// there; the record arrives a moment later. Reporting it as skipped is the
// difference between "your data is fine" and "your data is gone".
func ReadRecords(path string) (records []Record, skipped int, incomplete bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, false, fmt.Errorf("open ledger: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file; a close error carries no information we can act on
	// Whether the file ends on a newline is what separates a torn write from a
	// corrupt record, and the scanner cannot answer it: it strips the
	// terminator, so the last line looks identical either way. Asked of the
	// file directly, before reading.
	// The shape of the file, asked before its content is read.
	//
	// tailKnown is carried rather than folded away. A file whose shape could
	// not be established is not a file with a torn tail, and reporting one as
	// the other is the same collapse this change exists to undo one level up.
	endsClean, tailKnown := endsWithNewline(f)
	scanner := transcript.NewLineScanner(f)
	var lastWasSkip bool
	for scanner.Scan() {
		lastWasSkip = false
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
			lastWasSkip = true
			continue
		}
		records = append(records, rec)
	}
	// A line the parser rejected, sitting last in a file with no terminating
	// newline, is a record still being written rather than a broken one.
	// Unknown shape means no claim: incomplete stays false, and whatever the
	// scanner found is reported on its own terms.
	//
	// tailKnown is defence in depth and no current input reaches it. Every way
	// the shape goes unknown — a directory, a handle closed underneath us — is
	// also a way the scan fails, so control never arrives here with tailKnown
	// false. A hand-written mutant of the form `(tailKnown || true)` therefore
	// survives the suite, and is recorded here rather than left for the next
	// reader to rediscover: guard-reachability does not catch it because it
	// neutralises the whole condition, which TL1 does catch.
	//
	// It stays because the term is what the sentence means. Dropping it would
	// make the line read "an unknown tail is a torn tail", which is the
	// collapse this change exists to undo, and it would be correct only for as
	// long as the two failure sets keep coinciding.
	if lastWasSkip && tailKnown && !endsClean {
		skipped--
		incomplete = true
	}
	if err := scanner.Err(); err != nil {
		return nil, skipped, incomplete, fmt.Errorf("read ledger: %w", err)
	}
	return records, skipped, incomplete, nil
}

// ReadFile turns one ledger file into a Session at the measured tier.
func ReadFile(path string) (*transcript.Session, error) {
	// The incomplete flag is deliberately not folded into Session.Skipped.
	// A record still in flight is not a record the reader lost, and the count
	// they see must mean only the second thing.
	records, skipped, _, err := ReadRecords(path)
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
		// A refusal explains itself; an empty record does not.
		//
		// recordRefusal writes exactly this shape on purpose: the proxy
		// answered locally, so there is no provider usage and no prompt to
		// carry. Counting it as skipped reported a working spend cap as a
		// ledger the reader could not read.
		if rec.Refusal != "" {
			b.session.Refusals++
			return
		}
		b.session.Skipped++
		return
	}
	lane := b.session.Lane(rec.AgentID, rec.AgentID != "")
	lane.Requests = append(lane.Requests, requestFromRecord(rec, b.added-1, b.memo))
	if rec.Policy != "" {
		b.session.Policy, b.session.Trial = rec.Policy, TrialTreated
	}
}

// Trial arms as the pins and the sessions built from the ledger name them.
const (
	TrialTreated = "treated"
	TrialControl = "control"
)

// MarkControl marks a session the pins say was held out of a trial, when
// its records show no policy.
func (s *Store) MarkControl(session *transcript.Session) {
	if session.Trial != "" {
		return
	}
	if p, ok := s.Pin(session.ID); ok && p.Trial == TrialControl {
		session.Trial = TrialControl
	}
}

// Session is the session built so far. Lanes are shared with the builder;
// callers analyze, they do not modify.
func (b *SessionBuilder) Session() *transcript.Session { return b.session }

func requestFromRecord(rec Record, index int, memo map[string]*transcript.Message) *transcript.Request {
	req := &transcript.Request{
		ID: rec.RequestID,
		// The provider sent an id, or it did not. Recorded rather than
		// inferred from the string below, which is deliberately id-shaped and
		// would otherwise be indistinguishable from one.
		IDMeasured:    rec.RequestID != "",
		Correlation:   rec.Correlation,
		Model:         rec.Model,
		Effort:        rec.Effort,
		Timestamp:     rec.Timestamp,
		Usage:         *rec.Response.Usage,
		AppliedEdits:  rec.Response.AppliedEdits,
		ClearedTokens: rec.Response.ClearedInputTokens,
		Tools:         rec.Prompt.Tools,
		Epoch:         rec.Epoch,
	}
	if req.ID == "" {
		// A name for this record within this file, and nothing more. It is
		// the record's arrival position, so the same string names a different
		// request in every other ledger file; IDMeasured above is what stops
		// a consumer joining on it across files.
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

// pinsFile records, per session, the policy decision made at its first
// request, one JSON line each. It lives next to the session files under
// a name no ledger glob matches and holds names and numbers only.
const pinsFile = ".pins"

// Pin is a session's policy decision, made once and kept for the
// session's life across policy-file rewrites and proxy restarts (PX-8).
type Pin struct {
	SessionID string `json:"session_id"`
	// Policy names the pinned policy, empty when the session runs with
	// none. Trigger and Keep are its parameters.
	Policy   string    `json:"policy,omitempty"`
	Trigger  int       `json:"trigger,omitempty"`
	Keep     int       `json:"keep,omitempty"`
	Decision string    `json:"decision"`
	At       time.Time `json:"at"`
	// Trial says which arm of a live trial the session is in: "treated"
	// or "control"; empty when no trial was running.
	Trial string `json:"trial,omitempty"`
	// Type is the session type the selection was looked up for.
	Type string `json:"type,omitempty"`
}

// revertFile holds the one revert record, if a trial was reverted.
const revertFile = ".revert"

// Revert records that a live trial tripped its guardrail: from then on
// new sessions run without the policy until a newer learning result
// replaces it (LN-5).
type Revert struct {
	Policy  string `json:"policy"`
	Trigger int    `json:"trigger,omitempty"`
	Keep    int    `json:"keep,omitempty"`
	Reason  string `json:"reason"`
	// Breached counts the sessions whose guardrail tripped.
	Breached int       `json:"breached_sessions"`
	At       time.Time `json:"at"`
	// PolicyGenerated is the generation time of the policy file the
	// reverted policy came from; a newer file lifts the revert.
	PolicyGenerated time.Time `json:"policy_generated"`
}

// Revert returns the persisted revert, if any.
func (s *Store) Revert() (Revert, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revert, s.reverted
}

// SetRevert persists a revert, replacing any earlier one.
func (s *Store) SetRevert(r Revert) error {
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode revert: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.WriteFile(filepath.Join(s.dir, revertFile), append(data, '\n'), filePerm); err != nil {
		return fmt.Errorf("write revert: %w", err)
	}
	s.revert, s.reverted = r, true
	return nil
}

// loadRevert reads the revert record; a missing or unreadable one is no
// revert, since a record the proxy cannot read must not silence a policy
// it cannot name.
func loadRevert(path string) (Revert, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Revert{}, false
	}
	var r Revert
	if err := json.Unmarshal(data, &r); err != nil || r.Policy == "" {
		return Revert{}, false
	}
	return r, true
}

// Pin returns the persisted decision for a session, if one was made.
func (s *Store) Pin(sessionID string) (Pin, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pins[sessionID]
	return p, ok
}

// SetPin persists a session's decision. The file is append-only; the
// last line for a session wins on reload, and the first decision is the
// only one the proxy ever writes.
func (s *Store) SetPin(p Pin) error {
	line, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode pin: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(s.dir, pinsFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, filePerm)
	if err != nil {
		return fmt.Errorf("open pins file: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close() // the write error is the one worth reporting
		return fmt.Errorf("write pin: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close pins file: %w", err)
	}
	s.pins[p.SessionID] = p
	return nil
}

// loadPins reads the pins file; a missing file is an empty map and a
// line that does not parse is skipped, since a pin the proxy cannot read
// is a decision it must make again rather than a reason to refuse to start.
func loadPins(path string) (map[string]Pin, error) {
	pins := map[string]Pin{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return pins, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open pins file: %w", err)
	}
	defer f.Close() //nolint:errcheck // read-only file; a close error carries no information we can act on
	scanner := transcript.NewLineScanner(f)
	for scanner.Scan() {
		var p Pin
		if err := json.Unmarshal(scanner.Bytes(), &p); err != nil || p.SessionID == "" {
			continue
		}
		pins[p.SessionID] = p
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read pins file: %w", err)
	}
	return pins, nil
}

// endsWithNewline reports whether the file's final byte is a newline, and
// whether that could be established at all.
//
// Two returns rather than one error, because there are three outcomes and an
// error return collapsed them. The file ends clean; it does not; or the
// question could not be asked — a directory opens and stats successfully and
// then refuses ReadAt, and a handle closed underneath us fails at Stat. The
// third is not a kind of "torn", and defaulting it to false said it was.
//
// An empty file ends clean and KNOWN: it has no unterminated last line because
// it has no last line. Without that case the last byte is read at offset -1,
// which fails, and every ledger would be unknown for the window between
// creation and its first record.
//
// The caller does not need an error from here. Every input that makes this
// unknown also fails the scan that follows, so the failure reaches the caller
// as a scanner error with the content-level reason attached, which is the more
// useful of the two.
func endsWithNewline(f *os.File) (clean, known bool) {
	info, err := f.Stat()
	if err != nil {
		return false, false
	}
	if info.Size() == 0 {
		return true, true
	}
	// ReadAt, so the scanner that follows still starts at zero. A Seek back to
	// the start stood here to restore the offset; ReadAt reads at an absolute
	// offset and does not move the file position, so it restored something
	// nothing had moved, and its error branch was one no input could reach.
	var last [1]byte
	if _, err := f.ReadAt(last[:], info.Size()-1); err != nil {
		return false, false
	}
	return last[0] == '\n', true
}
