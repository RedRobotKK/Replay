package proxy

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The join under test.
//
// A cache break's cause is decided by comparing one response's usage against
// the usage of the request BEFORE it in the same lane. "Before" was decided by
// the order responses finished arriving, and nothing on the wire says that the
// request which finished most recently is the one whose cache entry this
// response read. When two requests of a lane are in flight together — which is
// what a coding agent does by construction, fanning out sub-agents and issuing
// parallel tool calls — the two usage records can be transposed, and the
// classification follows the transposition.
//
// arXiv:2608.07899 measured this shape on OpenTelemetry-style agent telemetry:
// 99.5-100% F1 at detecting that something went wrong, and origin-step accuracy
// capped at 0.5%. Detection survives without a decision-to-provenance link;
// attribution does not.

// The three usages below are chosen so that each of the overlapping pair reads
// less than the other one wrote. That is what makes them transposable: neither
// order is arithmetically privileged, so whichever arrives second is the one
// declared broken, and the cause follows the arrival.
var (
	seedUsage = transcript.Usage{Input: 10, CacheCreation: 20_000}
	usageA    = transcript.Usage{Input: 500, CacheRead: 800, CacheCreation: 1_000}
	usageB    = transcript.Usage{Input: 700, CacheRead: 700, CacheCreation: 1_200}
)

func corrRecord(id, session, model string, at time.Time, correlation string, u transcript.Usage) *ledger.Record {
	usage := u
	r := &ledger.Record{
		Timestamp:   at,
		SessionID:   session,
		RequestID:   id,
		Status:      200,
		LatencyMS:   200,
		Correlation: correlation,
	}
	r.Model = model
	// One prefix for the whole lane: the point of the test is the classifier
	// reaching the usage-and-timing causes, not the prefix short-circuit.
	r.PrefixHash = "p0"
	r.Response.Usage = &usage
	return r
}

// RC1 is the case this change exists for.
//
// Two requests were in flight in the same lane at the same time. Their usage
// records can reach the classifier in either order, because the order is the
// order two responses happened to finish. A per-event cause that changes when
// they are transposed is not a measurement of anything; it is a reading of
// which response was quicker.
//
// PASS: both orders report the same cause for both requests, and that cause is
// NOT MEASURED.
// FAIL: a confident cause, or two different answers for the same pair.
func TestRC1_TransposedConcurrentRecordsGetTheSameAnswer(t *testing.T) {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	// The lane's opening request, alone on the wire: it is what both of the
	// overlapping requests below could legitimately have read.
	seed := func() *ledger.Record {
		return corrRecord("req_seed", "s1", "claude-opus-5", at, ledger.CorrelationLaneSerial, seedUsage)
	}
	// A and B overlapped each other. Both carry lane-overlap because overlap
	// is a property of the pair, not of whichever one was sent second. They
	// ran different models, which is a cause the classifier will name with
	// certainty against whichever request it believes came before.
	reqA := func() *ledger.Record {
		return corrRecord("req_a", "s1", "claude-opus-5", at.Add(time.Second), ledger.CorrelationLaneOverlap, usageA)
	}
	reqB := func() *ledger.Record {
		return corrRecord("req_b", "s1", "claude-sonnet-4-5", at.Add(1100*time.Millisecond), ledger.CorrelationLaneOverlap, usageB)
	}

	causes := func(order ...*ledger.Record) map[string]cachemodel.BreakCause {
		s := newStats()
		out := map[string]cachemodel.BreakCause{}
		for _, rec := range order {
			// The lane's opening request has nothing before it, so it has no
			// outcome; only the two that do are compared.
			if oc := s.observe(rec); oc != nil {
				out[rec.RequestID] = oc.Cause
			}
		}
		return out
	}

	forward := causes(seed(), reqA(), reqB())
	transposed := causes(seed(), reqB(), reqA())

	for _, id := range []string{"req_a", "req_b"} {
		if forward[id] != transposed[id] {
			t.Errorf("%s was classified %q when it finished first and %q when it finished second; "+
				"the answer is a reading of which response was quicker, not of what happened",
				id, forward[id], transposed[id])
		}
		if forward[id] != cachemodel.CauseNotMeasured {
			t.Errorf("%s reported cause %q while another request of its lane was in flight; "+
				"the predecessor is not determined, so there is nothing to be confident about",
				id, forward[id])
		}
	}
}

// RC2 is the other half: the degrade must not be blanket.
//
// A lane that ran one request at a time has an unambiguous predecessor, and
// refusing to name a cause there would be the instrument declining to read
// something it can read.
//
// PASS: a serial lane still names the cause it can prove.
// FAIL: NOT MEASURED everywhere, which is a check that cannot fail.
func TestRC2_SerialLaneStillNamesItsCause(t *testing.T) {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	s := newStats()
	s.observe(corrRecord("req_seed", "s2", "claude-opus-5", at, ledger.CorrelationLaneSerial, seedUsage))
	// An hour later, well past the cache TTL: a cause that does not depend on
	// which response arrived first.
	broke := corrRecord("req_next", "s2", "claude-opus-5", at.Add(time.Hour), ledger.CorrelationLaneSerial, usageB)
	oc := s.observe(broke)
	if oc == nil || oc.Cause == "" {
		t.Fatalf("serial lane produced no cause: %+v", oc)
	}
	if oc.Cause == cachemodel.CauseNotMeasured {
		t.Fatalf("serial lane reported NOT MEASURED; nothing overlapped, so the predecessor is the one that completed")
	}
}

// RC3 crosses the join.
//
// RC1 and RC2 hand the classifier a record with the field already set, which
// is exactly the shape ADR-0018 warns about: a field nothing writes passes
// every test that writes it by hand. This one runs two real requests through
// the real proxy against an upstream that holds both open, and reads the
// field back off the ledger.
//
// PASS: both records say their lane overlapped.
// FAIL: the proxy never measured it, and the classifier's input is a constant.
func TestRC3_ProxyRecordsThatTwoRequestsOverlapped(t *testing.T) {
	up := &holdingUpstream{hold: make(chan struct{}), arrived: make(chan string, 4)}
	base, dir, _ := startProxy(t, up, "")

	wg := postConcurrently(t, base, "session-fanout", 2)
	// Both are inside the upstream handler, so both are genuinely in flight.
	up.expectArrival(t, "session-fanout")
	up.expectArrival(t, "session-fanout")
	close(up.hold)
	wg.Wait()

	recs := waitLedger(t, dir, 2)
	if len(recs) != 2 {
		t.Fatalf("ledger records = %d, want 2", len(recs))
	}
	for i, rec := range recs {
		if rec.Correlation != ledger.CorrelationLaneOverlap {
			t.Errorf("record %d correlation = %q, want %q: two requests were open at once and the ledger does not say so",
				i, rec.Correlation, ledger.CorrelationLaneOverlap)
		}
	}
}

// postConcurrently sends n requests that the upstream will hold open together.
func postConcurrently(t *testing.T, base, session string, n int) *sync.WaitGroup {
	t.Helper()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(requestBody))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(HeaderSessionID, session)
			req.Header.Set("X-Test-Hold", "1")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}()
	}
	return &wg
}

// RC4: a lane that ran its requests one after another is recorded as serial,
// so RC3's assertion is not true of every record the proxy writes.
func TestRC4_ProxyRecordsASerialLaneAsSerial(t *testing.T) {
	up := &upstream{t: t}
	base, dir, _ := startProxy(t, up, "")
	for i := 0; i < 2; i++ {
		resp := post(t, base, "/v1/messages", nil)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	recs := waitLedger(t, dir, 2)
	if len(recs) != 2 {
		t.Fatalf("ledger records = %d, want 2", len(recs))
	}
	for i, rec := range recs {
		if rec.Correlation != ledger.CorrelationLaneSerial {
			t.Errorf("record %d correlation = %q, want %q: nothing overlapped",
				i, rec.Correlation, ledger.CorrelationLaneSerial)
		}
	}
}

// RID1: the provider's own request id is captured wherever a provider sends
// one.
//
// Anthropic sends `request-id`; the OpenAI-compatible family sends
// `x-request-id`, and this build read only the first. Every record written for
// a Cursor, DeepSeek or OpenAI session therefore carried no provider id at all
// and fell back to a locally synthesised one, which is the arrival-order join
// this change is about.
//
// PASS: the id lands on the record.
// FAIL: it is dropped and the record is unjoinable to the provider's own trace.
func TestRID1_XRequestIDIsCaptured(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-request-id", "req_openai_1")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, messageResponse)
	})
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 {
		t.Fatalf("ledger records = %d, want 1", len(recs))
	}
	if recs[0].RequestID != "req_openai_1" {
		t.Fatalf("RequestID = %q, want %q: the provider sent an id and it was dropped",
			recs[0].RequestID, "req_openai_1")
	}
}

// RID2: `request-id` still wins when both are present, so adding the fallback
// did not move the Anthropic surface off the header it has always used.
func TestRID2_RequestIDWinsOverXRequestID(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("request-id", "req_anthropic")
		w.Header().Set("x-request-id", "req_other")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, messageResponse)
	})
	base, dir, _ := startProxy(t, up, "")
	resp := post(t, base, "/v1/messages", nil)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	recs := waitLedger(t, dir, 1)
	if len(recs) != 1 {
		t.Fatalf("ledger records = %d, want 1", len(recs))
	}
	if recs[0].RequestID != "req_anthropic" {
		t.Fatalf("RequestID = %q, want the provider's own request-id", recs[0].RequestID)
	}
}

// RC5: the claim carries the label.
//
// ADR-0018 rule 4: saying "not measured" has to be cheap on every surface. The
// break line is where the proxy makes its per-event claim, and a reader who
// never opens the ledger has only that line. Documenting the limit elsewhere
// is the silence this repository keeps finding.
//
// PASS: the line says the attribution was not measured and why.
// FAIL: the line names a cause with the same confidence as a measured one.
func TestRC5_TheBreakLineSaysWhenAttributionWasNotMeasured(t *testing.T) {
	up := &holdingUpstream{hold: make(chan struct{}), arrived: make(chan string, 4)}
	base, _, logs := startProxy(t, up, "")

	wg := postConcurrently(t, base, "session-fanout-log", 2)
	up.expectArrival(t, "session-fanout-log")
	up.expectArrival(t, "session-fanout-log")
	close(up.hold)
	wg.Wait()

	waitFor(t, "the second request's break line", func() bool {
		return strings.Contains(logs.String(), "cache break")
	})
	out := logs.String()
	if !strings.Contains(out, string(cachemodel.CauseNotMeasured)) {
		t.Fatalf("break line does not say the attribution was not measured:\n%s", out)
	}
	if !strings.Contains(out, "correlated by "+ledger.CorrelationLaneOverlap) {
		t.Fatalf("break line does not name how the two requests were correlated:\n%s", out)
	}
}
