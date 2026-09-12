package proxy

import (
	"bytes"
	"compress/gzip"
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
