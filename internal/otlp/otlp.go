// Package otlp writes Replay's findings as OpenTelemetry spans, to a file.
//
// To a FILE, and never to a network. requirements.md says "All data stays on
// the machine. There is no telemetry", and §10.3 lists "Sends anything
// anywhere except the provider request the client asked for" under What Replay
// Never Does. A network exporter would require editing both, and it would
// inherit http.ProxyFromEnvironment from the proxy's transport - so a
// plain-http collector, plus the auth header every real collector wants, puts
// the spans AND that collector's key in cleartext through whatever HTTP_PROXY
// names, a variable the user never set.
//
// A file needs no endpoint, no credential, no consent gesture and no new
// outbound surface. Every collector has a file receiver, the human moves the
// file, and the guarantee in the README survives intact.
//
// Two defaults in that receiver will silently drop this file, so the config
// has to ship with it rather than be left to the reader:
//
//	filelog:
//	  include: [ ~/.replay/otlp/*.jsonl ]
//	  start_at: beginning      # the default is `end`: pre-existing lines are
//	                           # never read, and a handed-over file is entirely
//	                           # pre-existing
//	  max_log_size: 4MiB       # the default 1MiB SPLITS a longer line into
//	                           # fragments that then fail to unmarshal
//
// maxLineBytes below keeps the second one from mattering even at the default,
// but a reader who sets neither gets an empty ingest and no error, which is the
// failure mode this project spends its time refusing.
package otlp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// maxLineBytes keeps every emitted line under the OpenTelemetry Collector's
// filelog receiver default of max_log_size 1MiB.
//
// Beyond that default the receiver SPLITS the line, and the fragments fail to
// unmarshal - a silent drop, on someone else's machine, long after these tests
// went green. So a batch is written as several lines rather than one.
const maxLineBytes = 900_000

// Turn is one request's findings, as the exporter needs them.
//
// Deliberately not a ledger.Record. Building from a narrow struct means a
// field added upstream cannot become a disclosure by accident, which is the
// rule internal/observation states and the reason it exists.
type Turn struct {
	Start, End                time.Time
	Model                     string
	InputTokens, OutputTokens int
	CacheRead, CacheWrite     int
	Outcome                   string
	Tier                      string

	// PrefixHash is the ledger's unkeyed hash. It is NEVER emitted as-is; see
	// Builder.Turn.
	PrefixHash string

	// Present so the allowlist test can poison them and prove they do not
	// reach the output. Nothing reads them.
	SessionID string
	Path      string
	Tools     []string
}

type Value struct {
	String string `json:"stringValue,omitempty"`
	Int    string `json:"intValue,omitempty"`
}

type Attr struct {
	Key   string `json:"key"`
	Value Value  `json:"value"`
}

// Span is an OTLP span in the JSON encoding.
type Span struct {
	TraceID           string `json:"traceId"`
	SpanID            string `json:"spanId"`
	Name              string `json:"name"`
	Kind              int    `json:"kind"`
	StartTimeUnixNano string `json:"startTimeUnixNano"`
	EndTimeUnixNano   string `json:"endTimeUnixNano"`
	Attributes        []Attr `json:"attributes"`
}

// Builder derives ids and keyed labels for one installation.
type Builder struct{ key []byte }

func New(key string) *Builder { return &Builder{key: []byte(key)} }

// tag is a keyed, truncated label.
//
// Keyed because ledger/summarize.go hashes the prefix with UNKEYED SHA-256,
// while session labels ARE keyed from .label-key. On the machine an unkeyed
// hash is just an equality test. In a file that leaves the machine it is a
// confirmation oracle: anyone holding a candidate system prompt can test it
// against the hash. The same objection as the open call_key finding, and the
// same fix the codebase already applies elsewhere.
func (b *Builder) tag(parts ...string) string {
	m := hmac.New(sha256.New, b.key)
	for _, p := range parts {
		m.Write([]byte(p))
		m.Write([]byte{0})
	}
	return hex.EncodeToString(m.Sum(nil))
}

// Turn builds the span for one request.
//
// Ids are written as lowercase hex by hand. A generic protobuf-JSON serializer
// base64-encodes them and every compliant receiver rejects that: the Collector
// length-checks before decoding and fails with "invalid length for ID". Its own
// exporter shipped exactly this bug once.
func (b *Builder) Turn(t Turn) Span {
	trace := b.tag("trace", t.SessionID)[:32]
	span := b.tag("span", t.SessionID, t.Start.String(), t.PrefixHash)[:16]
	return Span{
		TraceID: trace, SpanID: span,
		Name: "replay.turn", Kind: 2, // SPAN_KIND_SERVER
		StartTimeUnixNano: fmt.Sprintf("%d", t.Start.UnixNano()),
		EndTimeUnixNano:   fmt.Sprintf("%d", t.End.UnixNano()),
		Attributes: []Attr{
			{Key: "gen_ai.system", Value: Value{String: "anthropic"}},
			{Key: "gen_ai.operation.name", Value: Value{String: "chat"}},
			{Key: "gen_ai.request.model", Value: Value{String: t.Model}},
			{Key: "gen_ai.usage.input_tokens", Value: Value{Int: itoa(t.InputTokens)}},
			{Key: "gen_ai.usage.output_tokens", Value: Value{Int: itoa(t.OutputTokens)}},
			// No GenAI convention covers cache economics, so these are
			// replay.* rather than a convention name they do not have.
			{Key: "replay.cache.read_tokens", Value: Value{Int: itoa(t.CacheRead)}},
			{Key: "replay.cache.write_tokens", Value: Value{Int: itoa(t.CacheWrite)}},
			{Key: "replay.cache.outcome", Value: Value{String: t.Outcome}},
			{Key: "replay.cache.prefix_id", Value: Value{String: b.tag("prefix", t.PrefixHash)[:16]}},
			{Key: "replay.calibration.tier", Value: Value{String: t.Tier}},
		},
	}
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

// Write emits spans to a new file under dir, one OTLP payload per line.
//
// Same discipline as WriteObservation: owner-only, never through a symlink,
// never over an existing file. A human reads it and moves it.
func Write(dir string, spans []Span) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	name := fmt.Sprintf("otlp-%d.jsonl", time.Now().UTC().Unix())
	return writeTo(filepath.Join(dir, name), spans)
}

func writeTo(path string, spans []Span) (string, error) {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s is a symlink; refusing to write through a redirected path", path)
		}
		return "", fmt.Errorf("%s already exists; move or delete it rather than replacing an export", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // the Sync below reports the write error

	// Batched so no single line reaches the receiver's split threshold.
	batch := []Span{}
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		body, err := json.Marshal(payload(batch))
		if err != nil {
			return err
		}
		_, err = f.Write(append(body, '\n'))
		batch = batch[:0]
		return err
	}
	for _, s := range spans {
		batch = append(batch, s)
		if b, err := json.Marshal(payload(batch)); err == nil && len(b) > maxLineBytes/2 {
			if err := flush(); err != nil {
				return "", err
			}
		}
	}
	if err := flush(); err != nil {
		return "", err
	}
	return path, f.Sync()
}

func payload(spans []Span) map[string]any {
	return map[string]any{"resourceSpans": []any{map[string]any{
		"resource": map[string]any{"attributes": []Attr{
			{Key: "service.name", Value: Value{String: "replay"}},
		}},
		"scopeSpans": []any{map[string]any{
			"scope": map[string]any{"name": "replay"},
			"spans": spans,
		}},
	}}}
}
