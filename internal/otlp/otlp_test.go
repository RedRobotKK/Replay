package otlp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// OTLP spans written to a FILE, never pushed to a network.
//
// The design debate settled here from both directions. requirements.md:371
// states "All data stays on the machine. There is no telemetry", and §10.3
// lists "Sends anything anywhere except the provider request the client asked
// for" under What Replay Never Does. A network exporter would require editing
// both, and it inherits http.ProxyFromEnvironment (server.go:182) - so a
// plain-http collector plus the auth header every real collector wants puts
// spans AND that collector's key in cleartext through whatever HTTP_PROXY
// names, a variable the user never set.
//
// A file needs no endpoint, no credential, no consent gesture and no new
// outbound surface. Every collector has a file receiver. The human moves it.

// OT-1: ids are lowercase hex of the exact length, hand-encoded.
//
// A generic protobuf-JSON serializer base64s ids, and every compliant receiver
// rejects them: the Collector length-checks before decoding and fails with
// "invalid length for ID". Its OWN exporter shipped this bug once
// (open-telemetry/opentelemetry-collector#4221). So ids are written by hand
// and asserted, because the failure is a 400 from someone else's server long
// after the tests went green.
//
// PASS: 32 hex for a trace id, 16 for a span id.
// FAIL: base64, uppercase, or the wrong length.
func TestOT1_IdsAreHexAndCorrectlySized(t *testing.T) {
	b := New("k")
	sp := b.Turn(sample())
	hex32 := regexp.MustCompile(`^[0-9a-f]{32}$`)
	hex16 := regexp.MustCompile(`^[0-9a-f]{16}$`)
	if !hex32.MatchString(sp.TraceID) {
		t.Errorf("traceId %q is not 32 lowercase hex characters", sp.TraceID)
	}
	if !hex16.MatchString(sp.SpanID) {
		t.Errorf("spanId %q is not 16 lowercase hex characters", sp.SpanID)
	}
}

// OT-2: the attribute set is a frozen allowlist, built field by field.
//
// PASS: exactly the expected keys, and no value carries a poisoned identifier.
// FAIL: any repository name, filesystem path, session id or tool name - the
// classes internal/observation was written to keep out and metrics_listener
// refuses to put on a network.
func TestOT2_AttributesAreAFrozenAllowlist(t *testing.T) {
	in := sample()
	in.Model = "claude-opus-5"
	in.SessionID = "sess-SECRETSESSION"
	in.Path = "/Users/daniel/Development/SECRETREPO/x.go"
	in.Tools = []string{"mcp__SECRETTOOL__run"}
	in.PrefixHash = "prefix-SECRETHASH"

	sp := New("k").Turn(in)
	got := map[string]bool{}
	for _, a := range sp.Attributes {
		got[a.Key] = true
	}
	want := []string{
		"gen_ai.system", "gen_ai.operation.name", "gen_ai.request.model",
		"gen_ai.usage.input_tokens", "gen_ai.usage.output_tokens",
		"gen_ai.usage.cache_read.input_tokens", "gen_ai.usage.cache_write.input_tokens",
		"replay.cache.outcome", "replay.cache.prefix_id", "replay.calibration.tier",
	}
	for _, k := range want {
		if !got[k] {
			t.Errorf("attribute %q is missing", k)
		}
	}
	if len(got) != len(want) {
		t.Errorf("attribute set has %d keys, want exactly %d: %v", len(got), len(want), got)
	}
	blob, _ := json.Marshal(sp)
	for _, poison := range []string{"SECRETSESSION", "SECRETREPO", "SECRETTOOL", "SECRETHASH", "/Users/"} {
		if strings.Contains(string(blob), poison) {
			t.Errorf("the span leaked %q", poison)
		}
	}
}

// OT-3: the prefix id is keyed, not the raw hash.
//
// ledger/summarize.go hashes the prefix with UNKEYED truncated SHA-256, while
// session labels ARE keyed from .label-key. Locally an unkeyed hash is an
// equality test. Off the machine it is a confirmation oracle: anyone holding a
// candidate system prompt can test it. Same shape as the open call_key
// finding.
//
// PASS: two installs with different keys produce different ids for one prefix,
// and one install is stable.
// FAIL: the raw hash passed through.
func TestOT3_PrefixIDIsKeyed(t *testing.T) {
	in := sample()
	in.PrefixHash = "prefix-abc123"
	a := attr(New("key-a").Turn(in), "replay.cache.prefix_id")
	b := attr(New("key-b").Turn(in), "replay.cache.prefix_id")
	if a == "" || a == in.PrefixHash {
		t.Fatalf("prefix_id %q is the raw hash", a)
	}
	if a == b {
		t.Error("two installs produced the same prefix id; the key is not in the derivation")
	}
	if a != attr(New("key-a").Turn(in), "replay.cache.prefix_id") {
		t.Error("the same key must produce a stable id, or nothing can be correlated locally")
	}
}

// OT-4: a written line never exceeds the file receiver's default limit.
//
// The OpenTelemetry Collector's filelog receiver defaults to max_log_size 1MiB
// and SPLITS beyond it, so an oversized line arrives as fragments that fail to
// unmarshal. Silent drop, on someone else's machine.
//
// PASS: every emitted line is under the cap even with a large batch.
// FAIL: one long line, which is what a naive "marshal the batch" produces.
func TestOT4_LinesStayUnderTheReceiverLimit(t *testing.T) {
	dir := t.TempDir()
	b := New("k")
	var spans []Span
	for i := 0; i < 5000; i++ {
		spans = append(spans, b.Turn(sample()))
	}
	path, err := Write(dir, spans)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
		if len(line) >= maxLineBytes {
			t.Fatalf("line %d is %d bytes, at or over the %d cap the collector splits at",
				i, len(line), maxLineBytes)
		}
	}
}

// OT-5: writing refuses a symlink and refuses to overwrite.
func TestOT5_WriteRefusesSymlinkAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	sp := []Span{New("k").Turn(sample())}
	path, err := Write(dir, sp)
	if err != nil {
		t.Fatal(err)
	}
	// The guarantee is that a write never lands on a file that already exists,
	// and it is tested against the same path rather than by calling Write
	// twice. Write names files otlp-<unix seconds>.jsonl, so two calls only
	// collide when both fall inside the same second: on a slow runner they do
	// not, the second call gets its own name, and it is right to succeed. This
	// test spent that whole time asserting the clock rather than the code, and
	// went red the first time a runner was slow enough to separate them.
	// Asserting the message, not just the refusal. Two layers refuse an
	// existing path: the Lstat above the open, and O_EXCL on the open itself
	// as a backstop for the race between them. Either alone still refuses, so
	// a bare err != nil check cannot tell which is working, and removing the
	// first one leaves the user with a raw OS error instead of a sentence
	// telling them what to do about it.
	_, err = writeTo(path, sp)
	if err == nil {
		t.Fatal("a write onto an existing file must refuse")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("refused, but not by the check that explains itself: %v", err)
	}
	link := filepath.Join(dir, "link.jsonl")
	if err := os.Symlink(path, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err = writeTo(link, sp)
	if err == nil {
		t.Fatal("writing through a symlink must refuse")
	}
	// The reasoning three lines up, applied to the branch it skipped. The
	// symlink check and the exists check share one `if info, err :=
	// os.Lstat(path)`, and Lstat succeeds on a symlink, so removing the symlink
	// check leaves the exists check refusing instead — non-nil, and the bare
	// `err == nil` this replaces stayed green over it. Reported SURVIVED by
	// scripts/refusal-reachability.
	if !strings.Contains(err.Error(), "refusing to write through a redirected path") {
		t.Errorf("refused, but not by the symlink guard: %v", err)
	}
}

func attr(s Span, key string) string {
	for _, a := range s.Attributes {
		if a.Key == key {
			return a.Value.String
		}
	}
	return ""
}

func sample() Turn {
	return Turn{
		Start: time.Unix(1757000000, 0), End: time.Unix(1757000001, 0),
		Model: "claude-opus-5", InputTokens: 10, OutputTokens: 5,
		CacheRead: 1000, CacheWrite: 0, Outcome: "read_first",
		PrefixHash: "prefix-abc", Tier: "measured",
	}
}

// OT-6: this package cannot send.
//
// The whole design rests on the exporter writing a file and nothing else. A
// comment saying so is not a guarantee; an import allowlist is. The same shape
// internal/observation uses, and the reason it uses it: the promise is
// structural or it is decorative.
//
// PASS: no network package is reachable from here.
// FAIL: net/http appears, which is the first line of the version this debate
// rejected.
func TestOT6_ThisPackageCannotSend(t *testing.T) {
	src, err := os.ReadFile("otlp.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{`"net/http"`, `"net"`, `"net/url"`, `net.Dial`} {
		if strings.Contains(string(src), banned) {
			t.Errorf("the exporter imports %s; it writes a file and must not be able to send", banned)
		}
	}
}

// Cache tokens use the convention's names, and input_tokens includes them.
//
// The GenAI semantic conventions specified cache accounting on 2026-08-20
// (semantic-conventions-genai PR #440): gen_ai.usage.cache_read.input_tokens
// and gen_ai.usage.cache_write.input_tokens, both experimental, and both
// defined as SUBSETS of gen_ai.usage.input_tokens.
//
// That subset rule is the trap. This engine reads the Anthropic wire, where
// cache_read_input_tokens and cache_creation_input_tokens are DISJOINT from
// input_tokens, so emitting the field straight through would report an input
// total that omits every cached token. The two models disagree and the
// convention has to win, because the whole point of a convention is that a
// reader can add up figures from two vendors.
func TestOTLPCacheUsesTheConventionAndInputIncludesIt(t *testing.T) {
	b := &Builder{}
	sp := b.Turn(Turn{Model: "claude-opus-5", InputTokens: 1000, OutputTokens: 50,
		CacheRead: 4000, CacheWrite: 500, Outcome: "reproduced", Tier: "measured"})

	got := map[string]string{}
	for _, a := range sp.Attributes {
		v := a.Value.String
		if a.Value.Int != "" {
			v = a.Value.Int
		}
		got[a.Key] = v
	}
	if got["gen_ai.usage.cache_read.input_tokens"] != "4000" {
		t.Errorf("cache read is not on the convention attribute: %v", got)
	}
	if got["gen_ai.usage.cache_write.input_tokens"] != "500" {
		t.Errorf("cache write is not on the convention attribute: %v", got)
	}
	// 1000 uncached + 4000 read + 500 written. The convention's input total is
	// everything the prompt was made of, not the part that missed the cache.
	if got["gen_ai.usage.input_tokens"] != "5500" {
		t.Errorf("input_tokens = %q, want 5500: the convention says cache tokens are "+
			"included in it, and Anthropic reports them disjoint, so the sum has to be "+
			"made here or every cached token vanishes from the total",
			got["gen_ai.usage.input_tokens"])
	}
	for _, dead := range []string{"replay.cache.read_tokens", "replay.cache.write_tokens"} {
		if _, ok := got[dead]; ok {
			t.Errorf("%s is still emitted; a private name for a specified attribute "+
				"splits the ecosystem for no gain", dead)
		}
	}
}
