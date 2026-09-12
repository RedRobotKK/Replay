// Package ledger stores what the proxy observed about each request as
// derived data: block kinds, sizes, labels, timings, and provider usage.
// It never stores message text, headers, or credentials.
//
// The block and usage types are the transcript package's own, whose JSON
// tags define exactly the content-free subset that is persisted. One JSONL
// file per client session lives under the ledger directory, and the reader
// turns those files back into transcript sessions so the analysis commands
// work on measured data exactly as they do on transcripts.
package ledger

import (
	"encoding/json"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// SchemaVersion is written on every record so a future reader can tell what it
// is looking at.
//
// BUMPING IT DISCARDS EVERY EXISTING LEDGER. This comment used to say "bump it
// on any incompatible change", which is what the field looks like it is for and
// is the opposite of what the reader does: ReadRecords compares with exact
// equality and counts a mismatch as SKIPPED, which its own doc defines as data
// loss. So a bump to 3 does not migrate a schema-2 file, it makes every record
// in it unreadable on a machine that already holds months of them.
//
// Two fields already depend on that being true. Refusal says so where it is
// declared — "bumping it would discard every existing ledger rather than
// extend it" — and Epoch was added the same way. Both are optional fields that
// an old reader ignores and a new reader sees as absent, which is the only
// evolution this gate permits.
//
// So the real rule is: ADD optional fields, never rename or repurpose one. If
// a change genuinely cannot be expressed that way, changing this constant is a
// migration with a cost, not a version bump, and it needs a reader that accepts
// the older number.
const SchemaVersion = 2

// Block is the transcript block type; its Text is never serialized.
type Block = transcript.Block

// Usage is the transcript usage type, serialized with the provider's names.
type Usage = transcript.Usage

// The Correlation vocabulary, defined once in transcript and re-exported
// here so a ledger writer and a ledger reader cannot drift apart on the
// spelling of a value that is persisted to disk.
const (
	CorrelationUnmeasured  = transcript.CorrelationUnmeasured
	CorrelationLaneSerial  = transcript.CorrelationLaneSerial
	CorrelationLaneOverlap = transcript.CorrelationLaneOverlap
)

// Record is one proxied request and its response.
type Record struct {
	Schema    int       `json:"schema"`
	Timestamp time.Time `json:"ts"`
	// SessionID is the client-supplied session header, or a hash of the
	// stable prefix when the client sent none.
	SessionID string `json:"session_id"`
	// AgentID is the client-supplied sub-agent header, empty for the main loop.
	AgentID string `json:"agent_id,omitempty"`
	// RequestID is the provider's request id from the response headers.
	// Absent when the provider sent none, and never synthesised here, so a
	// reader can tell a provider id from a locally invented one.
	RequestID string `json:"request_id,omitempty"`
	// Correlation says whether this request had its lane to itself while it
	// was open. That is what decides whether the record before it in the file
	// is its predecessor or merely the one that finished first, and every
	// per-event cause downstream is only as good as that pairing.
	//
	// Omitted rather than written false when unknown: a record from a build
	// that never took this reading must not read as one that measured no
	// overlap. Absence, zero and unknown are three values (ADR-0018).
	//
	// Only the proxy can take it. The offline reader cannot recover it from
	// timings taken at the other end of the wire.
	Correlation string `json:"correlation,omitempty"`
	Path        string `json:"path"`
	// Frozen records that the freeze-prefix rewrite pinned a version string
	// in this request's body.
	//
	// Separate from Policy because Policy is one string and applyPolicy sets
	// it unconditionally: a request that was frozen and then context-edited
	// would record only the second, on a field documented as "empty when the
	// bytes went through unchanged". Two rewrites, one slot, and the reader
	// cannot tell. This is the smallest thing that keeps the record able to
	// say what happened.
	Frozen bool `json:"frozen,omitempty"`
	// Epoch is the tool-set epoch this request ran under, empty when none was
	// labelled — which is every request unless --freeze-prefix is on.
	//
	// COMPARABLE WITHIN A SESSION, NOT ACROSS TIME. It is sha256 of the tools
	// bytes exactly as forwarded, which is the right reading for the question
	// it answers — did the tools change between request N and N+1 — and the
	// wrong one for any other. A vendor rewording one tool description, a
	// client reordering keys, or a whitespace change yields a different epoch
	// for an identical tool set, so two sessions a week apart cannot be
	// grouped by it. Recorded here because a persisted key whose comparability
	// scope is undocumented will be compared outside it.
	//
	// PrefixHash on this same record is derived from a PARSE (summarize.go),
	// over an overlapping subject, with different stability. Two hashes, two
	// readings, one line of JSON — so a consumer choosing between them needs
	// to know which question each answers, and now can.
	//
	// It is OUR id for a tool set, not the provider's cache key. The kernel
	// debate wanted H(policy, epoch, tools, model, effort) to BE the provider's
	// key; it cannot be, because that key is not published and every term in it
	// would be a guess about someone else's hashing. What this labels is a fact
	// we can check: the tools JSON on the wire changed. Whether the provider's
	// cache moved is answered by usage against ExpectedRead, never by this
	// label — a self-report is not evidence about the thing reporting it.
	Epoch string `json:"epoch,omitempty"`
	RequestSummary
	// Policy names the request-parameter policy the proxy applied to this
	// request, empty when the bytes went through unchanged.
	Policy string `json:"policy,omitempty"`
	Status int    `json:"status"`
	// Refusal names the guard that answered this request locally, when one
	// did. Counts and thresholds only, never content. Added as an optional
	// field on purpose: SchemaVersion gates which records a reader accepts, so
	// bumping it would discard every existing ledger rather than extend it.
	Refusal string `json:"refusal,omitempty"`
	// RefusalReason is the guard message: counts and thresholds, never content.
	RefusalReason string `json:"refusal_reason,omitempty"`
	// Trimmed counts blocks a destructive transform removed content from, by
	// tool name. Nothing writes it yet: the live trimmer does not ship, and
	// this is the landing place so that when one is built its effect is on
	// the record from the first request rather than added afterwards.
	//
	// BodyHashBefore and BodyHashAfter bracket any such transform. They are
	// what makes "the proxy forwards bytes unchanged" checkable rather than
	// promised: equal hashes are the proof, and a live trimmer would be the
	// first policy that could not produce them.
	Trimmed        map[string]int `json:"trimmed,omitempty"`
	BodyHashBefore string         `json:"body_hash_before,omitempty"`
	BodyHashAfter  string         `json:"body_hash_after,omitempty"`
	// Masked counts the secrets the proxy replaced with placeholders in
	// this request, by pattern name. Never a secret or a placeholder.
	Masked map[string]int `json:"masked,omitempty"`

	// MaskDegraded records that a secret was positively identified and then
	// blind-scrubbed because the vault could not store the mapping. The
	// request still went out and the credential did not, but the placeholder
	// carries no vault entry and cannot be rehydrated, so a later reader needs
	// to know this request is not like the others.
	MaskDegraded bool `json:"mask_degraded,omitempty"`
	// Rehydrated counts the placeholders the proxy restored in this
	// response, by destination: text, edit:<tool>, or tool:<tool>.
	// RehydrationDenied counts those left in place, by destination and
	// reason. Neither ever holds a secret, a placeholder, or a path.
	Rehydrated        map[string]int `json:"rehydrated,omitempty"`
	RehydrationDenied map[string]int `json:"rehydration_denied,omitempty"`
	// Retries is how many times the proxy resent this request before the
	// response it recorded.
	Retries int `json:"retries,omitempty"`
	// HeldMS is how long the proxy held this request behind a sibling
	// with the same prefix before forwarding it.
	HeldMS    int64 `json:"held_ms,omitempty"`
	LatencyMS int64 `json:"latency_ms"`
	// Quota is what the provider said this request spent against a rate
	// limit, taken verbatim from its own response headers and keyed by
	// header name. Every other figure on this record is denominated in
	// dollars, which a flat-seat subscriber does not spend; this is the only
	// place their actual budget is visible. Values are unparsed on purpose -
	// see quotaFrom in internal/proxy.
	Quota map[string]string `json:"quota,omitempty"`
	// Response is the structure of what the provider returned. Usage is
	// absent on error responses and on endpoints that report none.
	Response Response `json:"response"`
	// Cache is the proxy's live classification of this response's cache
	// read against the previous request in the session, when known.
	Cache *CacheOutcome `json:"cache,omitempty"`
}

// RequestSummary is what the proxy learns from a request body: its
// structure and the attributes the analysis keys on. Embedded in Record so
// the fields serialize flat.
type RequestSummary struct {
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
	Stream bool   `json:"stream"`
	Prompt Prompt `json:"prompt"`
	// PrefixHash is a hash of the tool definitions and system prompt as
	// sent. Two requests with the same hash rendered the same cacheable
	// prefix; a change between requests is a break cause the proxy can
	// name with certainty. It contains no content.
	PrefixHash string `json:"prefix_hash,omitempty"`
	// SessionHash identifies the session by its stable prefix and first
	// message when the client sends no session header. Not persisted.
	SessionHash string `json:"-"`
}

// Prompt is the request body reduced to structure.
type Prompt struct {
	SystemBytes int `json:"system_bytes"`
	// ToolBytes is the decoded size of the tool definitions; ToolCount how
	// many; Tools their names and sizes, so an advisor can tell which of
	// them the session never called.
	ToolBytes int                  `json:"tool_bytes"`
	ToolCount int                  `json:"tool_count"`
	Tools     []transcript.ToolDef `json:"tools,omitempty"`
	// CacheControlCount is how many cache markers the client placed.
	CacheControlCount int       `json:"cache_control"`
	Messages          []Message `json:"messages"`
	// ContextEdits is true when the client sent its own context editing.
	ContextEdits bool `json:"context_edits,omitempty"`
}

// Message is one message reduced to its blocks.
type Message struct {
	Role   string  `json:"role"`
	Blocks []Block `json:"blocks"`
}

// Response is the reply reduced to structure and usage.
type Response struct {
	Blocks []Block `json:"blocks,omitempty"`
	Usage  *Usage  `json:"usage,omitempty"`
	// RawUsage is the provider's own usage object, verbatim and unparsed.
	//
	// Usage above keeps the fields this build knows are load-bearing, which
	// makes it lossy by construction. A field nobody knew mattered is exactly
	// what a later calibration needs, and it can only come from a payload
	// stored before anyone knew to ask. See internal/usage.
	RawUsage json.RawMessage `json:"raw_usage,omitempty"`
	// AppliedEdits and ClearedInputTokens report the provider's own
	// context edits on this response: how many it applied and how many
	// prompt tokens they removed. They are the applied policy's measured
	// side.
	AppliedEdits       int `json:"applied_edits,omitempty"`
	ClearedInputTokens int `json:"cleared_input_tokens,omitempty"`
}

// CacheOutcome is the live classification of one response's cache read.
type CacheOutcome struct {
	Outcome  string                `json:"outcome"`
	Expected int                   `json:"expected,omitempty"`
	Deficit  int                   `json:"deficit,omitempty"`
	Cause    cachemodel.BreakCause `json:"cause,omitempty"`
	// CauseDetail names what actually changed, in words, for a person. It is
	// deliberately free text and deliberately NOT a metrics label: Cause is
	// emitted as replay_cache_break_total{cause=...} and must stay a bounded
	// vocabulary, while this can name the thirty-four tools that arrived.
	CauseDetail string `json:"cause_detail,omitempty"`
}
