package proxy

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
)

// Secret masking, both directions: placeholders out, values back in.
//
// This is the one place in the program that does not fail open. Every other
// feature degrades by doing less; a redaction that degrades by doing less
// puts a credential on the wire, and the difference is that the secret
// leaves the machine. masking.Mask already implements that — when the vault
// cannot store a mapping it blind-scrubs the region and returns the SAFE
// body alongside the error — and the caller's job is to keep the scrubbed
// body rather than fall back to the original. A caller that saw a non-nil
// error and returned the original once shipped, and
// internal/masking/failclosed_test.go could not see it, because that test
// drives Masker.Mask, which was already correct. The guard now lives beside
// the caller it guards, and this file is that neighbourhood.
//
// The response half is here for the same reason: a placeholder only exists
// because the request half made one, and the two are one round trip. It
// carries its own rule — a compressed body or one past the size limit is
// forwarded exactly as it arrived and the skip is reported, never rewritten
// blind — which is why a rehydrated response is requested uncompressed.
//
// Nothing here reads or writes a thinking block, a signature, or a
// cache_control marker; the placeholders are the only bytes it touches.
// Separating the file does not enforce that, it only makes a patch that
// breaks it land somewhere a reviewer is looking for it.

// mask replaces secrets in the body with placeholders and records what it did.
//
// Masking fails SECURE, which is the one place in this program that does not
// fail open. Every other feature degrades by doing less; a redaction that
// degrades by doing less puts a credential on the wire, and the difference is
// that the secret leaves the machine.
//
// masking.Mask already implements that: when the vault cannot store a mapping
// it blind-scrubs the region and returns the SAFE body alongside the error, so
// the stream survives and the credential does not. This function used to see a
// non-nil error and return the ORIGINAL body, discarding the scrubbed one, so
// the fix never reached anything that ships. internal/masking/failclosed_test.go
// could not see it, because it drives Masker.Mask, which was already correct.
// The guard for this now lives beside the caller it guards.
func (s *Server) mask(rec *ledger.Record, body []byte) []byte {
	out, report, err := s.cfg.Masker.Mask(body)
	if err != nil {
		s.cfg.Logger.Printf("MASKING DEGRADED session=%s: a secret was blind-scrubbed and cannot be rehydrated, because the vault could not store it: %v", short(rec.SessionID), err)
		// Recorded, not only logged. A degraded request that leaves a single
		// stderr line is indistinguishable afterwards from one that masked
		// cleanly, and the ledger is where anything about this request is
		// looked up later.
		rec.MaskDegraded = true
	}
	if report.Total() > 0 {
		rec.Masked = report
		s.cfg.Logger.Printf("masked %d secret(s) session=%s: %s", report.Total(), short(rec.SessionID), report)
	}
	return out
}

// headerAcceptEncoding is dropped from requests whose response will be
// rehydrated.
const headerAcceptEncoding = "Accept-Encoding"

// rehydration is one response's rehydration: set up on the response
// hook, read by the handler's bookkeeping once the body has passed.
type rehydration struct {
	rh     *masking.Rehydrator
	stream *masking.StreamRehydrator
	report masking.RehydrationReport
	// skipped says why the body was not inspected, when it was not.
	skipped string
	err     error
}

// modify installs the rehydrating body. A compressed response, or one
// past the size limit, is forwarded as it is and the skip is reported.
func (h *rehydration) modify(resp *http.Response) {
	if resp.Header.Get("Content-Encoding") != "" {
		h.skipped = "compressed response"
		return
	}
	ct := resp.Header.Get("Content-Type")
	switch {
	case ledger.IsEventStream(ct):
		h.stream = h.rh.NewStream()
		resp.Body = masking.NewTransformReader(resp.Body, h.stream)
		// The rewritten stream's length is unknown; a declared one
		// would cut the client off.
		resp.Header.Del("Content-Length")
		resp.ContentLength = -1
	case strings.HasPrefix(strings.ToLower(ct), "application/json"):
		body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
		if err != nil {
			h.skipped = "body not read: " + err.Error()
			resp.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(body), resp.Body), Closer: resp.Body}
			return
		}
		if len(body) > MaxResponseBytes {
			h.skipped = "response over the size limit"
			resp.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(body), resp.Body), Closer: resp.Body}
			return
		}
		out, report, err := h.rh.Body(body)
		if err != nil {
			h.err = err
			out = body
		}
		h.report = report
		// The original body is fully read; its close only releases the
		// connection, which the replacement's close does in turn.
		resp.Body = readCloser{Reader: bytes.NewReader(out), Closer: resp.Body}
		resp.ContentLength = int64(len(out))
		resp.Header.Set("Content-Length", strconv.Itoa(len(out)))
	}
}

// result is the report once the body has passed.
func (h *rehydration) result() masking.RehydrationReport {
	if h.stream != nil {
		return h.stream.Report()
	}
	return h.report
}

type readCloser struct {
	io.Reader
	io.Closer
}

// noteRehydration records and logs what a response's rehydration did
// (MK-6): counts by destination, never a value or a path.
func (s *Server) noteRehydration(rec *ledger.Record, h *rehydration) {
	rep := h.result()
	rec.Rehydrated, rec.RehydrationDenied = rep.Restored, rep.Denied
	switch {
	case h.err != nil:
		s.cfg.Logger.Printf("rehydration session=%s: response forwarded with placeholders: %v", short(rec.SessionID), h.err)
	case h.skipped != "":
		s.cfg.Logger.Printf("rehydration skipped session=%s: %s", short(rec.SessionID), h.skipped)
	}
	if len(rep.Restored) > 0 {
		s.cfg.Logger.Printf("rehydrated %d placeholder(s) session=%s: %s", rep.Total(), short(rec.SessionID), rep.RestoredSummary())
	}
	if len(rep.Denied) > 0 {
		s.cfg.Logger.Printf("rehydration denied session=%s: %s", short(rec.SessionID), rep.DeniedSummary())
	}
}

// noteUnmasked warns that secret masking does not cover a path, once per path.
//
// This is the narrower sibling of noteUnparsed and exists for the same reason.
// Once /v1/chat/completions became readable it stopped being announced as
// unparsed, and masking still does not understand its body shape. An operator
// who ran with --mask would have had every reason to think secrets were being
// redacted on that traffic. A gap that used to be announced and quietly stops
// being announced is worse than one that never was.
func (s *Server) noteUnmasked(path string) {
	if !s.stats.noteUnmasked(path) || s.cfg.Logger == nil {
		return
	}
	s.cfg.Logger.Printf("NOT MASKED %s: this path is read, guarded and ledgered, but --mask "+
		"understands only %s, so secrets in this traffic are forwarded in clear.", path, messagesPath)
}
