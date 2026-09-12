package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The response tap: every byte reaches the client first, and a parser is fed
// on the side.
//
// The invariant is that the tap must never affect delivery, and it holds in
// three separate places. Write forwards before it parses, so a parser that is
// slow or wrong cannot delay a stream. Buffering stops at MaxResponseBytes
// and sets dropped rather than growing without bound, so a large response
// cannot exhaust memory and a dropped body is reported as no observation
// rather than a partial one — absence and zero are different values.
// Flush is forwarded so that streamed events are not held behind buffering.
//
// It is separated from the request path because the two have opposite
// obligations. The request path may transform bytes, under ADR-0003 and
// under masking, and is allowed to be slow. This one may do neither: it
// observes a response and never rewrites it. The one rewrite that does
// happen on the way out, rehydration, deliberately does not live here — it
// hangs off ModifyResponse in masking.go, so that "the tap does not change
// bytes" stays true of this file with no exceptions to remember.
//
// Splitting it does not add a test for the gzip or over-limit paths.

// tapKey carries the response tap to the reverse proxy's response hook.
type tapKey struct{}

// responseTap passes every byte to the client immediately and keeps a
// parser fed on the side. For event streams the parser consumes lines as
// they pass; for JSON responses the body is buffered up to the cap.
type responseTap struct {
	ostream *ledger.OpenAIStreamParser
	// openai selects the OpenAI-compatible response parser. The two shapes
	// report usage differently enough that guessing from the body would be a
	// heuristic where the request path already knows the answer.
	openai bool
	http.ResponseWriter
	// upstreamFailed is set by the error handler when no response arrived.
	upstreamFailed bool
	// rehydrate, when set, rewrites the response body on its way through.
	rehydrate *rehydration
	// onHeaders, when set, is called once with the status as the response
	// begins.
	onHeaders func(status int)
	status    int
	stream    *ledger.StreamParser
	buffer    bytes.Buffer
	gz        bool
	dropped   bool
}

func (t *responseTap) WriteHeader(code int) {
	t.status = code
	if t.onHeaders != nil {
		t.onHeaders(code)
		t.onHeaders = nil
	}
	ct := t.Header().Get("Content-Type")
	t.gz = strings.EqualFold(t.Header().Get("Content-Encoding"), "gzip")
	// Not redundant with result()'s own event-stream branch, which reparses a
	// buffered body to the same answer. Buffering stops at MaxResponseBytes,
	// so without the incremental parser a stream longer than the cap is
	// dropped and the turn is ledgered with no usage at all —
	// TestTap_AnEventStreamPastTheBufferCapIsStillMeasured is the boundary
	// where the two paths stop agreeing.
	if ledger.IsEventStream(ct) && !t.gz {
		if t.openai {
			t.ostream = &ledger.OpenAIStreamParser{}
		} else {
			t.stream = &ledger.StreamParser{}
		}
	}
	t.ResponseWriter.WriteHeader(code)
}

func (t *responseTap) Write(p []byte) (int, error) {
	if t.status == 0 {
		t.WriteHeader(http.StatusOK)
	}
	n, err := t.ResponseWriter.Write(p)
	// The tap must never affect delivery: parse after forwarding, and stop
	// buffering rather than grow without bound.
	switch {
	case t.ostream != nil:
		_, _ = t.ostream.Write(p[:n]) // never fails
	case t.stream != nil:
		_, _ = t.stream.Write(p[:n]) // StreamParser.Write never fails
	case t.buffer.Len()+n <= MaxResponseBytes:
		t.buffer.Write(p[:n])
	default:
		t.dropped = true
	}
	return n, err
}

// Flush forwards flushes so streamed events are not held by buffering.
func (t *responseTap) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (t *responseTap) result() ledger.Response {
	if t.ostream != nil {
		return t.ostream.Result()
	}
	if t.stream != nil {
		return t.stream.Result()
	}
	if t.dropped {
		return ledger.Response{}
	}
	body := t.buffer.Bytes()
	if t.gz {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return ledger.Response{}
		}
		decoded, err := io.ReadAll(io.LimitReader(zr, MaxResponseBytes))
		if err != nil {
			// A body that ended early is not one we may report a figure from.
			//
			// This was documented as indistinguishable by outcome and kept on
			// intent alone. A truncated gzip stream decompresses its prefix,
			// and cut INSIDE a compressed block that prefix always carries
			// whatever padding followed the document, so ParseResponse below
			// rejects it and returns the same empty Response this line does.
			// Measured across six truncation points at two padding sizes:
			// every one read partially, none parsed.
			//
			// That measurement never tried the cut that matters, and the
			// conclusion drawn from it was wrong. gzip.Writer.Flush ends a
			// block: everything written before it decompresses on its own, and
			// nothing after it reaches the prefix. A provider that flushes per
			// chunk — which is what streaming compression does — and then
			// loses the connection leaves a stream that opens, decompresses to
			// a WHOLE document, and ends early. Delete this line and that
			// document is parsed and its usage entered as measured, from a
			// response the tap knows is incomplete: not visibly wrong, just
			// the usage of the part that arrived.
			// TestTap_AGzipBodyCutAtAFlushBoundaryRecordsNothing builds that
			// stream, and fails without this line.
			//
			// The intent it was kept on is now also the outcome. A read error
			// means WE KNOW the body is incomplete, which the parser cannot
			// know — it only sees bytes that do not parse.
			return ledger.Response{}
		}
		body = decoded
	}
	if ledger.IsEventStream(t.Header().Get("Content-Type")) {
		// Gzip skipped the incremental parsers in WriteHeader. The route
		// still knows which family this is; guessing from the body would
		// put an OpenAI usage frame through the Anthropic parser and
		// record the turn as free.
		if t.openai {
			sp := &ledger.OpenAIStreamParser{}
			_, _ = sp.Write(body)
			return sp.Result()
		}
		sp := &ledger.StreamParser{}
		_, _ = sp.Write(body) // StreamParser.Write never fails
		return sp.Result()
	}
	if t.openai {
		return ledger.ParseOpenAIResponse(body)
	}
	return ledger.ParseResponse(body)
}
