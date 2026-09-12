package proxy

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
)

// The response half of masking: what happens when rehydration cannot run.
//
// Four conditionals here were reported UNREACHED by guard-reachability (#237),
// all four pre-dating the server.go split. They share one property and it is
// the one worth pinning: A REHYDRATION FAILURE MUST NOT EAT THE RESPONSE. The
// client asked the provider a question; whatever goes wrong on the way back,
// they get the answer, and the operator is told what was not done to it.
//
// That is the same rule masking already keeps on the request side, where a
// vault failure blind-scrubs rather than forwarding a secret. Outbound, the
// failure direction is the other way round: forward the bytes, and say the
// placeholders are still in them.

// erroringReader fails partway, which is what a provider connection dropping
// mid-body looks like from here.
type erroringReader struct {
	data []byte
	n    int
}

func (e *erroringReader) Read(p []byte) (int, error) {
	if e.n >= len(e.data) {
		return 0, errors.New("connection reset mid-body")
	}
	n := copy(p, e.data[e.n:e.n+1])
	e.n += n
	return n, nil
}
func (e *erroringReader) Close() error { return nil }

func jsonResponse(body io.ReadCloser) *http.Response {
	return &http.Response{
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   body,
	}
}

// A body that cannot be read is forwarded as far as it got, and says why.
func TestRehydration_AnUnreadableBodyIsStillForwarded(t *testing.T) {
	h := &rehydration{rh: masking.NewRehydrator(nil, masking.Scopes{})}
	resp := jsonResponse(&erroringReader{data: []byte(`{"type":"message"`)})
	h.modify(resp)

	if h.skipped == "" {
		t.Fatal("a body that could not be read was not recorded as skipped; the operator is " +
			"told nothing about a response rehydration never inspected")
	}
	if !strings.Contains(h.skipped, "body not read") {
		t.Errorf("the reason does not name what failed: %q", h.skipped)
	}
	// The bytes that were read must still reach the client. Dropping them
	// would turn a read error into a truncated answer the client cannot tell
	// from a short reply.
	got, _ := io.ReadAll(resp.Body)
	if len(got) == 0 {
		t.Fatal("the partial body was dropped; a failure to rehydrate must not consume the response")
	}
}

// A body over the cap is forwarded whole and not inspected.
func TestRehydration_AnOversizeBodyIsForwardedUninspected(t *testing.T) {
	big := append([]byte(`{"type":"message","pad":"`), bytes.Repeat([]byte("x"), MaxResponseBytes+64)...)
	big = append(big, []byte(`"}`)...)

	h := &rehydration{rh: masking.NewRehydrator(nil, masking.Scopes{})}
	resp := jsonResponse(io.NopCloser(bytes.NewReader(big)))
	h.modify(resp)

	if h.skipped != "response over the size limit" {
		t.Fatalf("an oversize body was not recorded as skipped: %q", h.skipped)
	}
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(big) {
		t.Fatalf("the oversize body was truncated on the way out: %d bytes in, %d out. "+
			"Refusing to inspect a response is not a reason to shorten it", len(big), len(got))
	}
}

// A rehydration error is reported to the operator as placeholders forwarded.
//
// The condition that sets h.err — `if err != nil` after rh.Body — is
// UNREACHABLE through the current implementation, and that is worth recording
// rather than working around. Body returns early with a nil error whenever
// json.Unmarshal rejects the document, and the only errors literals() can
// return are for a truncated or malformed document, which Unmarshal has
// already rejected. So no input reaches it today.
//
// It is kept, not deleted: it guards another package's error return, the
// signature promises one, and a change in masking could start producing it.
// What can be tested is the behaviour it exists to produce, which is this —
// the operator learns the response went out with placeholders still in it.
func TestRehydration_AnErrorIsReportedAsPlaceholdersForwarded(t *testing.T) {
	var logs bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&logs, "", 0)}}
	// Longer than short()'s 12-character cut, or the assertion below would
	// pass on an id that was never truncated. My first fixture was 8
	// characters, short() returned it whole — correctly — and the test failed
	// against working code.
	rec := &ledger.Record{SessionID: "abcd1234-ffff-0000-tail"}

	s.noteRehydration(rec, &rehydration{
		rh:  masking.NewRehydrator(nil, masking.Scopes{}),
		err: errors.New("vault unavailable"),
	})

	out := logs.String()
	if !strings.Contains(out, "forwarded with placeholders") {
		t.Fatalf("a rehydration error was not reported as placeholders forwarded:\n%s", out)
	}
	if !strings.Contains(out, "vault unavailable") {
		t.Errorf("the cause is not named:\n%s", out)
	}
	if strings.Contains(out, "abcd1234-ffff-0000-tail") {
		t.Errorf("the full session id reached the log; short() exists so it does not:\n%s", out)
	}
	if !strings.Contains(out, "abcd1234-fff") {
		t.Errorf("the truncated id is not there either, so the line cannot be traced:\n%s", out)
	}
}

// The error arm wins over the skipped arm, because an error is the stronger
// fact: a skipped body was never inspected, an errored one was and failed.
func TestRehydration_AnErrorIsReportedRatherThanASkip(t *testing.T) {
	var logs bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&logs, "", 0)}}
	s.noteRehydration(&ledger.Record{SessionID: "abcd1234-ffff-0000-tail"}, &rehydration{
		rh:      masking.NewRehydrator(nil, masking.Scopes{}),
		err:     errors.New("vault unavailable"),
		skipped: "compressed response",
	})
	out := logs.String()
	if !strings.Contains(out, "forwarded with placeholders") {
		t.Fatalf("the error arm did not win:\n%s", out)
	}
	if strings.Contains(out, "compressed response") {
		t.Errorf("both arms reported; the switch is meant to pick one:\n%s", out)
	}
}
