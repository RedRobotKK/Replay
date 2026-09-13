package transcript

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

const sdkFixture = "sdkdata/sdk-no-requestid.jsonl"

// A transcript written by a non-`cli` entrypoint carries no top-level
// requestId, and the whole file was discarded because of it.
//
// Claude Code writes `requestId` on assistant lines only from the `cli`
// entrypoint. `sdk-cli`, `sdk-ts` and `claude-desktop` write the same
// `message.usage` and the same `message.id` with no `requestId` at all, so
// the grouping loop skipped every assistant line, `Lanes` came back empty and
// ParseClaudeCode returned "no provider requests found". On the corpus this
// was found on, seven files out of 1821 were dropped whole, two of them
// carrying 8.93M cache-read and 38.8k output tokens that no figure counted.
func TestSDKTranscriptWithoutRequestIDIsRead(t *testing.T) {
	s, err := ParseClaudeCodeFile(sdkFixture)
	if err != nil {
		t.Fatalf("a transcript with no top-level requestId must still be read: %v", err)
	}
	if len(s.Lanes) != 1 {
		t.Fatalf("lanes = %d, want 1", len(s.Lanes))
	}
	reqs := s.Lanes[0].Requests
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2 - five assistant lines carrying two message ids", len(reqs))
	}
	if reqs[0].ID != "msg_A" || reqs[1].ID != "msg_B" {
		t.Fatalf("requests are identified as %q and %q, want the message ids", reqs[0].ID, reqs[1].ID)
	}

	// Usage is repeated verbatim on every line of one response, so a request
	// takes it from one line. Grouping three lines and summing them would
	// treble the bill.
	if got := reqs[0].Usage.CacheCreation; got != 53540 {
		t.Fatalf("first request cache creation = %d, want 53540 read once, not summed over its lines", got)
	}
	if got := reqs[0].Usage.Output; got != 132 {
		t.Fatalf("first request output = %d, want 132", got)
	}
	if got := reqs[1].Usage.CacheRead; got != 53540 {
		t.Fatalf("second request cache read = %d, want 53540", got)
	}

	// The three lines of one response are one assistant message, in the order
	// the API produced them.
	kinds := make([]string, 0, len(reqs[0].Output.Blocks))
	for _, b := range reqs[0].Output.Blocks {
		kinds = append(kinds, b.Kind)
	}
	if strings.Join(kinds, ",") != "thinking,text,tool_use" {
		t.Fatalf("first response blocks = %v, want the three lines merged in order", kinds)
	}

	// The second request's context holds the first turn as ONE assistant
	// message, not three. The run-merge in the parent chain keyed on the
	// top-level requestId too, and with that field absent on every line it
	// compared "" to "" and would have merged unrelated turns together.
	if n := len(reqs[1].Context); n != 3 {
		t.Fatalf("second request context = %d messages, want user, assistant, tool result", n)
	}
	if reqs[1].Context[1].Role != RoleAssistant || len(reqs[1].Context[1].Blocks) != 3 {
		t.Fatalf("the first turn is not one merged assistant message: %+v", reqs[1].Context[1])
	}
}

// The provider's own request id stays authoritative where it exists. The
// fallback is a fallback: message.id is read only when requestId is absent,
// because cmd/replay/cost.go dedupes requests across files on this value and a
// client that writes both must not be re-keyed onto the other one.
func TestTopLevelRequestIDWinsOverMessageID(t *testing.T) {
	// Two responses under one message id and two distinct request ids. If
	// message.id took precedence they would collapse into a single request
	// and one response's usage would vanish.
	in := `{"type":"user","uuid":"u1","parentUuid":null,"sessionId":"s","version":"1","timestamp":"2026-09-02T00:00:00Z","message":{"role":"user","content":"hi"}}` + "\n" +
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","sessionId":"s","requestId":"req_1","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:01Z","message":{"id":"msg_same","role":"assistant","model":"m","content":[{"type":"text","text":"one"}],"usage":{"input_tokens":1,"output_tokens":2}}}` + "\n" +
		`{"type":"assistant","uuid":"a2","parentUuid":"a1","sessionId":"s","requestId":"req_2","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:02Z","message":{"id":"msg_same","role":"assistant","model":"m","content":[{"type":"text","text":"two"}],"usage":{"input_tokens":3,"output_tokens":4}}}` + "\n"
	s, err := ParseClaudeCode(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	reqs := s.Lanes[0].Requests
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2 - the top-level requestId separates them", len(reqs))
	}
	if reqs[0].ID != "req_1" || reqs[1].ID != "req_2" {
		t.Fatalf("request ids = %q, %q; want the top-level requestIds", reqs[0].ID, reqs[1].ID)
	}
	if reqs[0].IDFromMessage || reqs[1].IDFromMessage {
		t.Fatal("a request that carried a requestId must not claim its id came from the message")
	}
}

// Absence, zero and unknown are three values (ADR-0018), and so are "this is
// the provider's request id" and "this is the provider's message id standing
// in for one". A consumer that needs the request id specifically can tell.
func TestRecoveredRequestSaysWhereItsIDCameFrom(t *testing.T) {
	s, err := ParseClaudeCodeFile(sdkFixture)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range s.Lanes[0].Requests {
		if !r.IDFromMessage {
			t.Fatalf("request %d recovered from message.id does not say so: %+v", i, r)
		}
	}
}

// Both of the ids this parser can hand back are the provider's own, so both
// are measured.
//
// IDMeasured answers a coarser question than IDFromMessage: not which of the
// provider's two identifiers this is, but whether it came off the wire at all.
// The alternative it exists to exclude is the `ledger-<n>` the ledger reader
// synthesises from a record's position in its file, which names a position and
// not a request - every ledger file has a `ledger-0`. `replay cost` joins
// across files on measured ids only, so a parser that marked its requests
// unmeasured would quietly withdraw every transcript from the overlap figure
// and report them all as unjoinable instead: a corpus-wide number moving with
// nothing failing.
//
// The message-id case is the one worth pinning. It is the newer path and the
// easy mistake is to read "recovered from the message id" as "not really the
// provider's", which it is not - Claude Code writes message.id from the
// provider's response either way.
func TestBothRecoveredAndDirectIDsAreMeasured(t *testing.T) {
	for _, tc := range []struct {
		name        string
		file        string
		wantFromMsg bool
	}{
		{"message id, no top-level requestId", sdkFixture, true},
		{"top-level requestId", "testdata/session-redacted.jsonl", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := ParseClaudeCodeFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			n := 0
			for _, lane := range s.Lanes {
				for i, r := range lane.Requests {
					n++
					if !r.IDMeasured {
						t.Fatalf("request %d carries the provider id %q and is marked "+
							"unmeasured, so `replay cost` will not join it across files", i, r.ID)
					}
					if r.IDFromMessage != tc.wantFromMsg {
						t.Fatalf("request %d: IDFromMessage = %v, want %v",
							i, r.IDFromMessage, tc.wantFromMsg)
					}
				}
			}
			if n == 0 {
				t.Fatal("the fixture produced no requests, so this asserts nothing")
			}
		})
	}
}

// Every fixture in this repository is a redacted transcript, and a user
// attaching one to a bug report about SDK transcripts would have been
// attaching the evidence with the identifier removed. Redaction keeps the
// top-level requestId verbatim; message.id is the same class of value - a
// provider-issued opaque identifier, not user content - and dropping it made
// a redacted SDK transcript unreadable by the parser that had just read the
// original.
func TestRedactedSDKTranscriptStaysReadable(t *testing.T) {
	raw, err := os.ReadFile(sdkFixture)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Redact(bytes.NewReader(raw), &out); err != nil {
		t.Fatal(err)
	}
	before, err := ParseClaudeCode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	after, err := ParseClaudeCode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatalf("a redacted SDK transcript must still be readable: %v", err)
	}
	if before.RequestCount() != after.RequestCount() {
		t.Fatalf("request count changed under redaction: %d -> %d", before.RequestCount(), after.RequestCount())
	}
	for i := range before.Lanes[0].Requests {
		b, a := before.Lanes[0].Requests[i], after.Lanes[0].Requests[i]
		if b.Usage != a.Usage {
			t.Fatalf("request %d usage changed under redaction: %+v -> %+v", i, b.Usage, a.Usage)
		}
	}
}

// An API error the client wrote down is not a request the provider answered.
//
// Claude Code records a failed call as an assistant line with
// `isApiErrorMessage: true`, `message.model` of the literal string
// "<synthetic>" and a usage object of all zeros. Forty of them sit in the 1821
// transcripts this was measured on, and counting one as a request puts a
// non-model in a model column and a zero where no measurement was ever taken.
//
// It matters beyond tidiness. cmd/replay/cost.go names a lane's model from its
// FIRST request, and prices the re-billed figure only for a model the table
// knows. One placeholder landing at the head of a lane took that lane's
// re-billed tokens from 1,586,545 to zero with its cost unchanged - the exact
// shape ADR-0018 is about, arithmetically fine and epistemically silent.
func TestAPIErrorPlaceholderIsNotARequest(t *testing.T) {
	line := func(uuid, parent, rid, ts, text string, apiErr bool) string {
		id := ""
		if rid != "" {
			id = `"requestId":"` + rid + `",`
		}
		e := ""
		model := "claude-opus-5"
		if apiErr {
			e = `"isApiErrorMessage":true,"apiErrorStatus":529,`
			model = "<synthetic>"
		}
		return `{"type":"assistant","uuid":"` + uuid + `","parentUuid":"` + parent + `","sessionId":"s",` + id + e +
			`"apiBlockIndex":0,"timestamp":"` + ts + `","message":{"id":"msg_` + uuid + `","role":"assistant","model":"` + model +
			`","content":[{"type":"text","text":"` + text + `"}],"usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0}}}`
	}
	in := `{"type":"user","uuid":"u1","parentUuid":null,"sessionId":"s","version":"1","timestamp":"2026-09-02T00:00:00Z","message":{"role":"user","content":"hi"}}` + "\n" +
		// The placeholder comes first, and carries no requestId: both the
		// path that has one and the path that falls back must refuse it.
		line("a1", "u1", "", "2026-09-02T00:00:01Z", "API Error: 529 Overloaded.", true) + "\n" +
		line("a2", "a1", "", "2026-09-02T00:00:02Z", "API Error: 529 Overloaded.", true) + "\n" +
		`{"type":"assistant","uuid":"a3","parentUuid":"a2","sessionId":"s","requestId":"req_1","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:03Z","message":{"id":"msg_real","role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"cache_creation_input_tokens":20,"cache_read_input_tokens":0,"output_tokens":5}}}` + "\n"
	s, err := ParseClaudeCode(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	reqs := s.Lanes[0].Requests
	if len(reqs) != 1 {
		models := make([]string, 0, len(reqs))
		for _, r := range reqs {
			models = append(models, r.Model)
		}
		t.Fatalf("requests = %d %v, want 1 - two of those lines are the client's own error records", len(reqs), models)
	}
	if reqs[0].Model != "claude-opus-5" {
		t.Fatalf("the lane's first request is %q; a placeholder at the head of a lane renames the whole lane's model", reqs[0].Model)
	}
}

// Two responses adjacent in the parent chain stay two messages.
//
// buildRequest reconstructs a request's context by walking the parent chain
// and merging the RUN of assistant lines that belong to one response. It
// compared the top-level requestId to decide what "one response" meant. In a
// file where no line carries that field, that comparison is "" == "", which is
// true for every pair: two responses sitting next to each other in the chain -
// a turn resumed after an interruption, an error record and the retry that
// followed it - would merge into a single assistant message, and the request
// downstream would be told the model said in one turn what it said in two.
//
// The 1821 transcripts this was measured on contain no instance of the shape:
// every assistant run in them is separated by a user line or a tool result. So
// this transcript is constructed, and it is constructed rather than dropped
// because the alternative expression agrees with this one only by accident.
// Keyed on requestKey, the merge groups lines that share an identifier; keyed
// on the absent field, it groups lines that share nothing.
func TestAdjacentResponsesWithoutRequestIDsDoNotMerge(t *testing.T) {
	asst := func(uuid, parent, mid, ts, text string) string {
		return `{"type":"assistant","uuid":"` + uuid + `","parentUuid":"` + parent + `","sessionId":"s","timestamp":"` + ts +
			`","message":{"id":"` + mid + `","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"text","text":"` + text +
			`"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":10,"cache_read_input_tokens":0,"output_tokens":2}}}`
	}
	in := `{"type":"user","uuid":"u1","parentUuid":null,"sessionId":"s","version":"1","timestamp":"2026-09-02T00:00:00Z","message":{"role":"user","content":"hi"}}` + "\n" +
		asst("a1", "u1", "msg_A", "2026-09-02T00:00:01Z", "first response") + "\n" +
		asst("a2", "a1", "msg_B", "2026-09-02T00:00:02Z", "second response") + "\n" +
		`{"type":"user","uuid":"u2","parentUuid":"a2","sessionId":"s","timestamp":"2026-09-02T00:00:03Z","message":{"role":"user","content":"go on"}}` + "\n" +
		asst("a3", "u2", "msg_C", "2026-09-02T00:00:04Z", "third response") + "\n"
	s, err := ParseClaudeCode(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	reqs := s.Lanes[0].Requests
	if len(reqs) != 3 {
		t.Fatalf("requests = %d, want 3", len(reqs))
	}
	// Context of the third request: the user turn, response A, response B,
	// the second user turn. Four messages, not three.
	ctx := reqs[2].Context
	if len(ctx) != 4 {
		roles := make([]string, 0, len(ctx))
		for _, m := range ctx {
			roles = append(roles, string(m.Role)+"/"+fmt.Sprint(len(m.Blocks)))
		}
		t.Fatalf("context = %d messages %v, want 4 - two responses that merged into one", len(ctx), roles)
	}
	if ctx[1].Role != RoleAssistant || len(ctx[1].Blocks) != 1 {
		t.Fatalf("the first response is not one message of one block: %+v", ctx[1])
	}
	if ctx[2].Role != RoleAssistant || len(ctx[2].Blocks) != 1 {
		t.Fatalf("the second response is not a message of its own: %+v", ctx[2])
	}
}
