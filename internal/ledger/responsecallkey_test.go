package ledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Finding 4 of the 2026-09-04 adversarial security review, closed here.
//
// Verbatim, from docs/evidence/security-review-2026-09-04.md:
//
//	Response-side `call_key` is an unkeyed SHA-256 of the full tool input, so
//	a ledger holder can confirm guessed tool calls offline. The request side
//	is HMAC'd; the response side is not
//
// and from RELEASE-CRITERIA.md:
//
//	The two halves of one ledger must not have different properties.
//
// What was wrong. transcript.CallKey is SHA-256 over name and input with no
// key in it. The request half is re-keyed by Labeler.keyCalls before the
// summary is built (summarize.go), so a ledger holder cannot confirm a
// guessed request-side call. The response half was written to disk exactly as
// transcript.DecodeBlock produced it, so `sha256("Bash\x00" + guess)[:16]`
// confirmed or refuted any guess offline, with no key and no interaction.
// A tool input is a shell command, a file path, a search string — confirming
// one is confirming content, which is the property the whole ledger design
// exists to deny.
//
// What closes it. Store.Append re-keys the response half under the same
// ledger secret the request half already uses, so the file on disk carries
// two keyed halves. The re-key is HMAC over the digest rather than over the
// input, because the input is already gone by then: ParseResponse strips
// Text before the record is built, and the streaming parser never
// accumulates input_json_delta at all. HMAC over the digest is enough for
// what the finding names — an offline guesser can compute the digest and
// still cannot compute the HMAC — and it keeps the equality that the loop
// detector and the repeated-command classifier rely on.
//
// The two halves are keyed under one secret but are NOT interchangeable
// values, and nothing compares them: the loop detector reads request-side
// prompt blocks only (proxy/guards.go), and the repeated-command classifier
// reads req.Context only (analysis/errors.go). RC5 below pins that no
// consumer starts depending on cross-half equality by accident.

const rcToolInput = `{"command":"cat /etc/shadow"}`

// rcResponse builds a record whose response half carries two identical tool
// calls and one different one, as an assistant turn actually would.
func rcResponse() Response {
	body := `{"id":"msg_1","type":"message","role":"assistant","content":[` +
		`{"type":"tool_use","id":"t1","name":"Bash","input":` + rcToolInput + `},` +
		`{"type":"tool_use","id":"t2","name":"Bash","input":` + rcToolInput + `},` +
		`{"type":"tool_use","id":"t3","name":"Bash","input":{"command":"ls"}}],` +
		`"usage":{"input_tokens":1,"output_tokens":1}}`
	return ParseResponse([]byte(body))
}

// rcPersisted appends one record to a store rooted at dir and returns the
// response-side call keys as they were actually written to the file.
func rcPersisted(t *testing.T, dir string) []string {
	t.Helper()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	rec := Record{
		Timestamp:      time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		SessionID:      "rc-session",
		Path:           "/v1/messages",
		RequestSummary: RequestSummary{Model: "claude-opus-5"},
		Response:       rcResponse(),
	}
	if err := store.Append(rec); err != nil {
		t.Fatal(err)
	}
	// Read the bytes on disk, not the in-memory record. The finding is about
	// what a ledger holder has, and a ledger holder has the file.
	raw, err := os.ReadFile(filepath.Join(dir, "rc-session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Response struct {
			Blocks []struct {
				Kind    string `json:"kind"`
				CallKey string `json:"call_key"`
			} `json:"blocks"`
		} `json:"response"`
	}
	line := strings.TrimSpace(string(raw))
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("decode ledger line: %v", err)
	}
	keys := make([]string, 0, 3)
	for _, b := range got.Response.Blocks {
		if b.Kind == transcript.KindToolUse {
			keys = append(keys, b.CallKey)
		}
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 response tool_use blocks on disk, got %d (line: %s)", len(keys), line)
	}
	return keys
}

// RC1: a guessed tool call cannot be confirmed against the ledger file.
//
// This is the finding itself. The attacker's guess is the whole input, and
// the unkeyed digest of it is what used to sit in the file.
//
// PASS: no persisted response call key equals the unkeyed digest of the call
// that produced it.
// FAIL: it does, which is offline confirmation of content.
func TestRC1_ResponseCallKeysAreNotTheUnkeyedDigest(t *testing.T) {
	guess := transcript.CallKey("Bash", json.RawMessage(rcToolInput))
	if guess == "" {
		t.Fatal("the guess is empty; this test would pass vacuously")
	}
	for i, got := range rcPersisted(t, t.TempDir()) {
		if got == guess {
			t.Errorf("response block %d persisted the unkeyed digest %q: a ledger holder can confirm a guessed tool call offline", i, got)
		}
	}
}

// RC2: the key depends on the ledger's own secret.
//
// The complement of RC1, and the one that cannot be satisfied by any
// keyless transform: two ledgers holding the same response must not agree on
// the value, or the value is still a function of the content alone.
//
// PASS: two stores with independently generated label keys disagree.
// FAIL: they agree, which means whatever replaced the digest is still
// computable by anyone.
func TestRC2_ResponseCallKeysDependOnTheLedgerSecret(t *testing.T) {
	one := rcPersisted(t, t.TempDir())
	two := rcPersisted(t, t.TempDir())
	for i := range one {
		if one[i] == two[i] {
			t.Errorf("response block %d has the same key %q in two different ledgers; the key does not depend on the secret", i, one[i])
		}
	}
}

// RC3: equality inside one ledger survives, because two consumers depend on
// it — the loop detector (proxy/guards.go) and the repeated-command
// classifier (analysis/errors.go). A key that broke it would turn this fix
// into a silent regression of the guard that stops an agent looping.
//
// PASS: the two identical calls share a key; the third does not.
// FAIL: either, which is a defeated consumer rather than a defeated attacker.
func TestRC3_EqualCallsStillShareAKeyWithinOneLedger(t *testing.T) {
	keys := rcPersisted(t, t.TempDir())
	if keys[0] == "" {
		t.Fatal("an empty key would make every comparison below vacuous")
	}
	if keys[0] != keys[1] {
		t.Errorf("identical calls got different keys %q and %q; the loop detector counts on equality", keys[0], keys[1])
	}
	if keys[0] == keys[2] {
		t.Errorf("a different call got the same key %q; the loop detector would over-count", keys[0])
	}
}

// RC4: the streaming half is keyed too.
//
// The response arrives as server-sent events far more often than as one JSON
// body, and it is parsed by a different function. A fix applied to
// ParseResponse alone would leave the common path open, and nothing else
// would notice.
//
// PASS: the streamed record's persisted key is not the unkeyed digest.
// FAIL: it is, which is the finding still open on the path most traffic uses.
func TestRC4_StreamedResponseCallKeysAreKeyedToo(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	sp := &StreamParser{}
	events := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"t1\",\"name\":\"Bash\",\"input\":" + rcToolInput + "}}\n\n"
	if _, err := sp.Write([]byte(events)); err != nil {
		t.Fatal(err)
	}
	resp := sp.Result()
	var live string
	for _, b := range resp.Blocks {
		if b.Kind == transcript.KindToolUse {
			live = b.CallKey
		}
	}
	if live == "" {
		t.Fatal("the stream parser produced no tool_use call key; this test would pass vacuously")
	}
	if err := store.Append(Record{
		Timestamp: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		SessionID: "rc-stream", Path: "/v1/messages",
		RequestSummary: RequestSummary{Model: "claude-opus-5"}, Response: resp,
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "rc-stream.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), live) {
		t.Errorf("the streamed response persisted its unkeyed call key %q verbatim", live)
	}
}

// RC5: appending must not mutate the caller's record.
//
// Append takes a Record by value but Blocks is a slice, so a re-key written
// in place would reach back into the proxy's live record — which is still
// read after the write for spend attribution and the guard counters. The bug
// this pins is subtle and would look like a guard misfiring, not like a
// keying change.
//
// PASS: the in-memory blocks are untouched after Append.
// FAIL: they changed, which is Append reaching into its caller.
func TestRC5_AppendDoesNotRewriteTheCallersBlocks(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	resp := rcResponse()
	before := make([]string, len(resp.Blocks))
	for i, b := range resp.Blocks {
		before[i] = b.CallKey
	}
	if err := store.Append(Record{
		Timestamp: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		SessionID: "rc-mutate", Path: "/v1/messages",
		RequestSummary: RequestSummary{Model: "claude-opus-5"}, Response: resp,
	}); err != nil {
		t.Fatal(err)
	}
	for i, b := range resp.Blocks {
		if b.CallKey != before[i] {
			t.Errorf("block %d call key changed under the caller: %q -> %q", i, before[i], b.CallKey)
		}
	}
}

// RC6: only tool calls are re-keyed, and only ones that already have an
// identity.
//
// The filter in KeyResponseCalls is load-bearing in a way that is easy to
// miss: HMAC of the empty string is a perfectly good-looking hex value, so
// dropping the filter would stamp a call key onto every text and thinking
// block in every response. The loop detector and the repeated-command
// classifier both key off "this block has a CallKey", so that is not a
// cosmetic difference — it is a text block being counted as a repeated tool
// call.
//
// PASS: a text block keeps its empty key; a tool_use block gets a new one.
// FAIL: a key appears where there was none.
func TestRC6_OnlyToolCallsWithAnIdentityAreReKeyed(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	blocks := []Block{
		{Kind: transcript.KindText, Label: "assistant text", Bytes: 5},
		{Kind: transcript.KindToolUse, Label: "call Bash", ToolName: "Bash", CallKey: "abc123"},
		// A tool_use the wire parser could not identify: no key in, no key out.
		{Kind: transcript.KindToolUse, Label: "call Bash", ToolName: "Bash"},
	}
	out := store.Labeler().KeyResponseCalls(blocks)
	if out[0].CallKey != "" {
		t.Errorf("a text block was given a call key %q", out[0].CallKey)
	}
	if out[1].CallKey == "" || out[1].CallKey == "abc123" {
		t.Errorf("the tool call was not re-keyed: %q", out[1].CallKey)
	}
	if out[2].CallKey != "" {
		t.Errorf("a tool call with no identity was given one: %q", out[2].CallKey)
	}
}
