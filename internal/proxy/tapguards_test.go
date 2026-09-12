package proxy

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The tap's refusals.
//
// The tap watches a response on its way to the client and records what the
// provider said it cost. It has one rule above all others, stated in its own
// comment: it must never affect delivery. The conditionals below are the
// second rule, which nothing tested — IT MUST NEVER REPORT A FIGURE IT DID
// NOT MEASURE.
//
// Every one of them resolves to an empty ledger.Response rather than a partial
// one. That is the whole point: a partial parse is a number that looks
// measured, and a number that looks measured is what this project exists not
// to produce. Absent is recoverable; wrong is not.
//
// Five of the ten conditionals #234 lists are already covered — by the OpenAI
// stream tests and by #220's gzip fix, all of which landed after that issue was
// written. Measured by neutralising each and running the package: 2, 4, 5, 9
// and 10 die. These are the five that did not.

// A gzipped event stream gets no incremental parser.
//
// The incremental parsers read SSE frames as they pass. Compressed bytes are
// not frames, so handing them to a parser would record nothing and call it a
// measurement. The tap buffers instead and decompresses in result().
func TestTap_AGzipEventStreamGetsNoIncrementalParser(t *testing.T) {
	rec := httptest.NewRecorder()
	tap := &responseTap{ResponseWriter: rec}
	tap.Header().Set("Content-Type", "text/event-stream")
	tap.Header().Set("Content-Encoding", "gzip")
	tap.WriteHeader(http.StatusOK)

	if tap.stream != nil || tap.ostream != nil {
		t.Fatal("a gzipped event stream was given an incremental parser. Compressed bytes are " +
			"not SSE frames, so it would parse nothing and report that as the turn's usage")
	}
	if !tap.gz {
		t.Error("the response was not recognised as gzipped, so result() will not decompress it")
	}
}

// A Write before any WriteHeader still records a status.
//
// net/http defaults an unset status to 200 on the wire, so the client is
// unaffected either way. The tap has to agree, or the ledger records status 0
// for a request the provider answered — a value no HTTP response ever has, in
// the field a reader uses to tell success from failure.
func TestTap_AWriteBeforeWriteHeaderStillRecordsAStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	tap := &responseTap{ResponseWriter: rec}
	tap.Header().Set("Content-Type", "application/json")

	if _, err := tap.Write([]byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	if tap.status != http.StatusOK {
		t.Fatalf("status recorded as %d; a response the provider answered must not be "+
			"ledgered with a status no HTTP response has", tap.status)
	}
}

// A response over the cap records nothing rather than a truncated something.
func TestTap_AnOversizeResponseRecordsNothingRatherThanAPrefix(t *testing.T) {
	rec := httptest.NewRecorder()
	tap := &responseTap{ResponseWriter: rec}
	tap.Header().Set("Content-Type", "application/json")
	tap.WriteHeader(http.StatusOK)

	// TWO writes, and that is the whole test. A single oversize write leaves
	// the buffer EMPTY, so deleting the dropped guard changes nothing — the
	// empty buffer parses to an empty response either way, and the mutant
	// survives. My first fixture did exactly that and proved nothing.
	//
	// A first write under the cap BUFFERS. The second pushes past it and sets
	// dropped, leaving a PARTIAL body behind — and that partial is what the
	// guard exists to stop result() from parsing and reporting as the turn.
	// A COMPLETE document, not a truncated one. ParseResponse rejects
	// truncated JSON, so an unparseable partial gives an empty result with or
	// without the guard and the mutant survives — my second fixture made that
	// mistake after the first one made a different version of it. What the
	// guard actually stops is a buffer holding a WHOLE document followed by
	// writes that overflowed: parse it and the ledger reports 4242 input
	// tokens for a response the client received megabytes of.
	//
	// It has to be a shape ParseResponse actually parses — "type":"message" —
	// or the guard is indistinguishable again. A bare usage object returns nil
	// usage, which is what the third version of this fixture did.
	first := []byte(`{"id":"m","type":"message","content":[],"usage":{"input_tokens":4242}}`)
	if _, err := tap.Write(first); err != nil {
		t.Fatal(err)
	}
	if tap.buffer.Len() != len(first) {
		t.Fatalf("the first write did not buffer: %d bytes held", tap.buffer.Len())
	}
	big := bytes.Repeat([]byte("x"), MaxResponseBytes+1024)
	n, err := tap.Write(big)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(big) {
		t.Fatalf("the tap short-wrote the client: %d of %d. It may record nothing, and it may "+
			"never affect delivery", n, len(big))
	}
	if !tap.dropped {
		t.Fatal("an oversize body was not marked dropped")
	}
	if got := tap.result(); got.Usage != nil || len(got.Blocks) > 0 {
		t.Fatalf("a dropped response still reported a result: %+v. A partial parse is a figure "+
			"that looks measured, which is worse than an absent one", got)
	}
	// The client got every byte.
	if rec.Body.Len() != len(first)+len(big) {
		t.Fatalf("the client received %d bytes of %d", rec.Body.Len(), len(first)+len(big))
	}
}

// Gzip that cannot be opened records nothing.
func TestTap_UnopenableGzipRecordsNothing(t *testing.T) {
	rec := httptest.NewRecorder()
	tap := &responseTap{ResponseWriter: rec}
	tap.Header().Set("Content-Type", "application/json")
	tap.Header().Set("Content-Encoding", "gzip")
	tap.WriteHeader(http.StatusOK)

	// Not gzip at all: NewReader rejects it on the header.
	if _, err := tap.Write([]byte("this is not gzip")); err != nil {
		t.Fatal(err)
	}
	if got := tap.result(); got.Usage != nil || len(got.Blocks) > 0 {
		t.Fatalf("a body announced as gzip and not gzip reported a result: %+v", got)
	}
}

// Gzip that opens and then ends early records nothing.
//
// Distinct from the case above: the header is valid, so NewReader succeeds and
// the failure lands one guard later, in ReadAll.
//
// That second guard is INDISTINGUISHABLE BY OUTCOME and this test does not
// pretend otherwise. A truncated stream decompresses its prefix, the prefix
// carries the padding that followed the document, and ParseResponse rejects it
// — so deleting the guard yields the same empty Response. I measured it across
// six truncation points at two padding sizes before concluding that rather
// than iterating fixtures until one appeared to prove something.
//
// What this test does hold is the OUTCOME: a truncated gzip body records
// nothing. The guard's own reason for existing is written at the site.
func TestTap_TruncatedGzipRecordsNothing(t *testing.T) {
	var full bytes.Buffer
	zw := gzip.NewWriter(&full)
	// Usage FIRST, then padding. A truncated stream decompresses its prefix,
	// so the prefix has to be something that parses — otherwise deleting the
	// ReadAll guard yields an unparseable partial, the result is empty either
	// way, and the mutant survives. My first fixture repeated the usage object
	// and the partial decoded to nothing parseable.
	content := `{"usage":{"input_tokens":4242}}` + strings.Repeat("x", 200000)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	truncated := full.Bytes()[:full.Len()/2]

	rec := httptest.NewRecorder()
	tap := &responseTap{ResponseWriter: rec}
	tap.Header().Set("Content-Type", "application/json")
	tap.Header().Set("Content-Encoding", "gzip")
	tap.WriteHeader(http.StatusOK)
	if _, err := tap.Write(truncated); err != nil {
		t.Fatal(err)
	}

	// The premise: this really does open and then fail, or the test is the
	// one above wearing a different name.
	if _, err := gzip.NewReader(bytes.NewReader(truncated)); err != nil {
		t.Fatalf("the fixture fails at NewReader, not at ReadAll: %v", err)
	}
	if got := tap.result(); got.Usage != nil || len(got.Blocks) > 0 {
		t.Fatalf("a truncated gzip body reported a result: %+v", got)
	}
	var empty ledger.Response
	if got := tap.result(); got.Usage != empty.Usage {
		t.Fatalf("expected an empty response, got %+v", got)
	}
}

// An event stream longer than the buffer cap is still measured.
//
// WriteHeader gives a plain event stream an incremental parser, and that is
// the only reason a long stream is measured at all. Delete the guard and the
// stream takes the same route as a JSON body: Write buffers it until
// MaxResponseBytes, sets dropped, and result() reports no observation. A
// sixteen-megabyte conversation is not exotic — it is one long agent session
// with a large tool result in it — and the ledger would carry that turn with
// no usage, which `replay cost` prices as free.
//
// Neutralising the guard leaves the rest of the suite green, because every
// other stream fixture fits in the buffer and result()'s own event-stream
// branch reparses it to the same answer. The cap is where the two paths stop
// agreeing, so the cap is where this test sits.
//
// The padding is SSE comment lines. Their shape does not matter — only that
// the body crosses the cap — and comments keep the fixture cheap by not
// putting eight hundred thousand JSON frames through the parser.
func TestTap_AnEventStreamPastTheBufferCapIsStillMeasured(t *testing.T) {
	const (
		wantInput  = 4242
		wantOutput = 77
	)
	rec := httptest.NewRecorder()
	// The recorder keeps no copy. The fixture is deliberately bigger than
	// MaxResponseBytes and a second sixteen megabytes in the test process
	// proves nothing the returned byte counts do not.
	rec.Body = nil
	tap := &responseTap{ResponseWriter: rec}
	tap.Header().Set("Content-Type", "text/event-stream")
	tap.WriteHeader(http.StatusOK)

	delivered := 0
	write := func(s string) {
		n, err := tap.Write([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		if n != len(s) {
			t.Fatalf("the tap short-wrote the client: %d of %d bytes. It may record "+
				"nothing, and it may never affect delivery", n, len(s))
		}
		delivered += n
	}

	write("data: {\"type\":\"message_start\",\"message\":{\"usage\":" +
		"{\"input_tokens\":4242}}}\n\n")
	pad := ": " + strings.Repeat("keep-alive ", 32) + "\n"
	chunk := strings.Repeat(pad, 64)
	for sent := 0; sent <= MaxResponseBytes; sent += len(chunk) {
		write(chunk)
	}
	write("data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":77}}\n\n")

	if delivered <= MaxResponseBytes {
		t.Fatalf("the fixture is %d bytes and the cap is %d: it does not cross the "+
			"boundary the guard exists for", delivered, MaxResponseBytes)
	}
	// t.Error, not t.Fatal: dropped is the mechanism and the usage below is the
	// consequence, and a run that lost the guard should print both.
	if tap.dropped {
		t.Error("a long event stream was buffered and then dropped. The incremental " +
			"parser exists so that stream length is not a reason to stop measuring")
	}
	got := tap.result()
	if got.Usage == nil {
		t.Fatalf("a %d-byte event stream carrying usage in both frames recorded none. "+
			"Buffered instead of parsed as it passed, it overran the cap and was "+
			"discarded, and the ledger prices that turn as free", delivered)
	}
	if got.Usage.Input != wantInput || got.Usage.Output != wantOutput {
		t.Errorf("usage = input %d, output %d; want %d and %d",
			got.Usage.Input, got.Usage.Output, wantInput, wantOutput)
	}
}

// A gzip body cut at a flush boundary records nothing, and this is the case
// where that guard is the only thing stopping a wrong number.
//
// TestTap_TruncatedGzipRecordsNothing above says the ReadAll guard is
// indistinguishable by outcome, and for the truncation it takes that is true:
// cut inside a compressed block, the decompressed prefix drags in whatever
// padding followed the document, and ParseResponse rejects it.
//
// It is not true in general, and the measurement that produced it never tried
// the case that matters. gzip.Writer.Flush ends a block: every byte written
// before it decompresses on its own, and nothing after it is in the prefix.
// A provider that flushes per chunk — which is what streaming compression does
// — and then loses the connection leaves exactly this on the wire: a stream
// that opens, decompresses to a WHOLE document, and then ends early.
//
// Without the guard that document is parsed and its usage entered in the
// ledger as measured, from a body the tap KNOWS is incomplete. The count is
// not even wrong in a visible way: it is the usage of the part that arrived.
func TestTap_AGzipBodyCutAtAFlushBoundaryRecordsNothing(t *testing.T) {
	doc := `{"type":"message","content":[],"usage":{"input_tokens":4242}}`
	var full bytes.Buffer
	zw := gzip.NewWriter(&full)
	if _, err := zw.Write([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	// The flush is the whole fixture: it closes the block holding doc, so the
	// bytes up to here decompress to doc and to nothing else.
	if err := zw.Flush(); err != nil {
		t.Fatal(err)
	}
	cut := full.Len()
	if _, err := zw.Write([]byte(strings.Repeat("x", 200000))); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	truncated := full.Bytes()[:cut]

	// The premise, checked rather than assumed: this opens, decodes the whole
	// document, and THEN fails. If any of the three stops holding, the test
	// below is measuring something else.
	zr, err := gzip.NewReader(bytes.NewReader(truncated))
	if err != nil {
		t.Fatalf("the fixture fails at NewReader, so it exercises the guard above "+
			"this one, not this one: %v", err)
	}
	prefix, err := io.ReadAll(io.LimitReader(zr, MaxResponseBytes))
	if err == nil {
		t.Fatal("the fixture decompressed cleanly, so ReadAll never errors and the " +
			"guard is not reached")
	}
	if string(prefix) != doc {
		t.Fatalf("the prefix is %q, not the whole document. Unless a COMPLETE "+
			"document survives the cut, ParseResponse rejects it and the guard "+
			"changes nothing", prefix)
	}
	if ledger.ParseResponse(prefix).Usage == nil {
		t.Fatal("the prefix does not parse, so deleting the guard would report " +
			"nothing anyway and this test proves nothing")
	}

	rec := httptest.NewRecorder()
	tap := &responseTap{ResponseWriter: rec}
	tap.Header().Set("Content-Type", "application/json")
	tap.Header().Set("Content-Encoding", "gzip")
	tap.WriteHeader(http.StatusOK)
	if _, err := tap.Write(truncated); err != nil {
		t.Fatal(err)
	}

	if got := tap.result(); got.Usage != nil {
		t.Fatalf("a gzip body that ended early reported usage %+v. The bytes that "+
			"arrived do parse — that is the trap — but the tap knows the body is "+
			"incomplete, and a figure from a truncated response enters the ledger "+
			"indistinguishable from a measured one", got.Usage)
	}
}

// A non-streaming OpenAI response is read by the OpenAI parser.
//
// The streamed shape has had a proxy-level test since #234 — TestOAE1 and
// TestOAE2 in openaistream_e2e_test.go — and the gzip-streamed shape has
// TestOAE3. The plain JSON answer had none, so the last conditional in
// result() chose ParseOpenAIResponse for every test run and no test cared.
//
// It is the same defect those three were written for, on the shape a client
// that did not ask to stream gets. The two families report usage under
// different names: OpenAI sends prompt_tokens and completion_tokens in a body
// whose "object" is chat.completion, and ParseResponse requires "type":
// "message" and returns an empty Response for anything else. Read this body
// with the Anthropic parser and the turn records no tokens at all — not a
// smaller number, nothing — and `replay cost` prices it as free.
//
// Driven through the server rather than by constructing a tap, because the
// value the guard reads is set by the route. A tap built by hand with
// openai: true would pass with the routing broken.
func TestTap_ANonStreamingOpenAIResponseIsReadByTheOpenAIParser(t *testing.T) {
	const (
		wantPrompt     = 4242
		wantCached     = 4000
		wantCompletion = 77
	)
	up := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, `{"id":"c1","object":"chat.completion","model":"gpt-4o",`+
			`"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},`+
			`"finish_reason":"stop"}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,`+
			`"total_tokens":%d,"prompt_tokens_details":{"cached_tokens":%d}}}`,
			wantPrompt, wantCompletion, wantPrompt+wantCompletion, wantCached); err != nil {
			t.Error(err)
		}
	})
	base, dir, _ := startProxy(t, up, "")

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`
	resp, err := http.Post(base+chatCompletionsPath, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close() //nolint:errcheck // test read
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatal(err)
	}

	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 {
		t.Fatal("the proxy forwarded a chat completion and wrote no ledger record")
	}
	got := recs[0].Response.Usage
	if got == nil {
		t.Fatal("a completed chat completion carrying usage recorded none. Its body " +
			"is not an Anthropic message, so the Anthropic parser returns an empty " +
			"response and the turn prices as free")
	}
	if got.Output != wantCompletion {
		t.Errorf("completion tokens = %d, want %d", got.Output, wantCompletion)
	}
	if got.CacheRead != wantCached {
		t.Errorf("cached tokens = %d, want %d: the cached share arrives nested in "+
			"prompt_tokens_details, which only the OpenAI parser reads",
			got.CacheRead, wantCached)
	}
}
