package ledger

import (
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A provider that answered and declined is not a record the reader lost.
//
// 874f026 split local refusals out of Skipped and stopped there: the branch it
// added keys on Record.Refusal, which only recordRefusal sets. A provider 401,
// or a 429 that was never retried into success, is forwarded, answered, and
// written to the ledger complete with its status, its retry count and its
// rate-limit headers. It carries no usage, so it fell to the same Skipped
// fallback and the report called it a line the parser could not interpret.
//
// Read, refused, unreadable, and now a fourth: the provider was reached and
// did not return a usable response. That is an observation, not a reading
// failure and not a decision Replay made.
//
// The ordering in Add is load-bearing rather than incidental. Every local
// refusal carries an HTTP status of its own — 400 for the spend cap, the loop
// guard, the error budget and the pre-flight ceiling, 503 for the circuit
// breaker — so a classification keyed on status alone would swallow all of
// them. Refusal is checked first, and RF3 below is what notices if it stops
// being.

// pfRecord builds a ledger record with a given status and no usable response.
func pfRecord(session string, status int) Record {
	return Record{SessionID: session, Status: status}
}

// pfSession writes records and reads the session back.
func pfSession(t *testing.T, recs ...Record) *transcript.Session {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if err := s.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	sess, err := ReadFile(filepath.Join(dir, sessionFileName("s1")+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

// PF1: every status at or above 400 is a provider failure, and the observed
// status is kept.
//
// 401 and 429 share the category deliberately. Splitting them would mean the
// Session layer asserting that one is an authentication problem and the other
// a quota problem, and a status code on its own establishes neither.
func TestPF1_StatusesAtOrAbove400AreProviderFailures(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 429, 500, 502, 503} {
		sess := pfSession(t, pfRecord("s1", status))
		pf := sess.ProviderFailures
		if pf.Total() != 1 {
			t.Errorf("status %d: provider failures = %d, want 1", status, pf.Total())
		}
		if pf.ByStatus[status] != 1 {
			t.Errorf("status %d: not recorded under its own status: %v", status, pf.ByStatus)
		}
		if sess.Skipped != 0 {
			t.Errorf("status %d: also counted as Skipped = %d; a record the provider "+
				"answered is not a line the parser could not read", status, sess.Skipped)
		}
		if sess.Refusals != 0 {
			t.Errorf("status %d: counted as a Refusal = %d; Replay did not answer this "+
				"one locally", status, sess.Refusals)
		}
	}
}

// PF2: an upstream failure that never produced a status is a provider failure
// with no status.
//
// It is kept apart from the status counts rather than filed under 0, because 0
// is not an HTTP status and printing it as one would be inventing a response
// the provider never sent.
func TestPF2_ATransportFailureIsCountedWithoutAStatus(t *testing.T) {
	// A zero status does not identify itself: an empty record has one too. The
	// prompt is what separates them, because handle summarizes before it
	// forwards, so a request that reached the provider carries its messages
	// whatever came back. PF8 is the other side of that line.
	failed := pfRecord("s1", 0)
	failed.Prompt = Prompt{Messages: []Message{{Role: "user"}}}
	sess := pfSession(t, failed)
	pf := sess.ProviderFailures
	if pf.NoStatus != 1 {
		t.Errorf("NoStatus = %d, want 1", pf.NoStatus)
	}
	if len(pf.ByStatus) != 0 {
		t.Errorf("a status-less failure was filed under a status: %v", pf.ByStatus)
	}
	if sess.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", sess.Skipped)
	}
}

// PF3: a local refusal stays a refusal, whatever status it carries.
//
// This is the regression that matters most. refusalSpendCap, refusalLoop,
// refusalErrorBudget and refusalPreFlight all write status 400, and
// refusalCircuitOpen writes 503. Checking status before Refusal would reclassify
// every one of them and undo 874f026 entirely.
func TestPF3_ALocalRefusalIsNotAProviderFailure(t *testing.T) {
	for _, status := range []int{400, 503} {
		r := pfRecord("s1", status)
		r.Refusal = "spend_cap"
		r.RefusalReason = "daily cap reached"
		sess := pfSession(t, r)
		if sess.Refusals != 1 {
			t.Errorf("status %d: Refusals = %d, want 1", status, sess.Refusals)
		}
		if sess.ProviderFailures.Total() != 0 {
			t.Errorf("status %d: a guard Replay fired itself was counted as a provider "+
				"failure; 874f026 exists to keep these apart", status)
		}
	}
}

// PF4: a 3xx is outside the category.
//
// The boundary is at or above 400, matching the bucketing the proxy's own
// stats already use. A redirect is not a failure and is not claimed as one.
func TestPF4_A3xxIsNotAProviderFailure(t *testing.T) {
	for _, status := range []int{301, 302, 304, 399} {
		sess := pfSession(t, pfRecord("s1", status))
		if sess.ProviderFailures.Total() != 0 {
			t.Errorf("status %d was counted as a provider failure; the boundary is 400", status)
		}
		if sess.Skipped != 1 {
			t.Errorf("status %d: Skipped = %d, want 1 — it is outside the new category "+
				"and its existing treatment is unchanged", status, sess.Skipped)
		}
	}
}

// PF5: a successful request is untouched, and contributes its usage as before.
func TestPF5_ASuccessfulRequestIsNotAProviderFailure(t *testing.T) {
	ok := Record{SessionID: "s1", Status: 200}
	ok.Prompt = Prompt{Messages: []Message{{Role: "user"}}}
	ok.Response = Response{Usage: &transcript.Usage{Input: 10, Output: 2}}
	sess := pfSession(t, ok)
	if sess.ProviderFailures.Total() != 0 {
		t.Errorf("a 200 with usage was counted as a provider failure")
	}
	if len(sess.Lanes) == 0 {
		t.Fatal("the successful request did not become a request")
	}
}

// PF6: a provider failure contributes no tokens.
//
// Absence is not zero. The record carries no usage, and nothing may supply one
// on its behalf.
func TestPF6_AProviderFailureContributesNoUsage(t *testing.T) {
	ok := Record{SessionID: "s1", Status: 200}
	ok.Prompt = Prompt{Messages: []Message{{Role: "user"}}}
	ok.Response = Response{Usage: &transcript.Usage{Input: 10, Output: 2}}

	alone := pfSession(t, ok)
	// Both failures carry their prompt, as a forwarded request does: handle
	// summarizes before it forwards, so the messages are on the record whatever
	// came back. Without them the record would be diverted by the empty-record
	// half of the condition and this test would not be exercising usage at all.
	rateLimited := pfRecord("s1", 429)
	rateLimited.Prompt = Prompt{Messages: []Message{{Role: "user"}}}
	transport := pfRecord("s1", 0)
	transport.Prompt = Prompt{Messages: []Message{{Role: "user"}}}
	withFailure := pfSession(t, ok, rateLimited, transport)

	count := func(s *transcript.Session) (reqs, input, output int) {
		for _, l := range s.Lanes {
			for _, r := range l.Requests {
				reqs++
				input += r.Usage.Input
				output += r.Usage.Output
			}
		}
		return
	}
	r1, i1, o1 := count(alone)
	r2, i2, o2 := count(withFailure)
	if r1 != r2 || i1 != i2 || o1 != o2 {
		t.Errorf("provider failures changed the totals: requests %d→%d, input %d→%d, output %d→%d",
			r1, r2, i1, i2, o1, o2)
	}
	if withFailure.ProviderFailures.Total() != 2 {
		t.Errorf("provider failures = %d, want 2", withFailure.ProviderFailures.Total())
	}
}

// PF7: a failure retried into success leaves only the successful record.
//
// The retry layer drains and discards each failed attempt, so the only
// surviving evidence is the final response with its retry count. The category
// counts records, and there is no failure record here to count.
func TestPF7_AFailureRetriedIntoSuccessIsNotCounted(t *testing.T) {
	ok := Record{SessionID: "s1", Status: 200, Retries: 2}
	ok.Prompt = Prompt{Messages: []Message{{Role: "user"}}}
	ok.Response = Response{Usage: &transcript.Usage{Input: 10, Output: 2}}
	sess := pfSession(t, ok)
	if sess.ProviderFailures.Total() != 0 {
		t.Errorf("a request that retried into success was counted as a failure; only " +
			"records that survive are counted, and the failed attempts were discarded")
	}
}

// PF8: a record that explains nothing is still Skipped.
//
// The residual cases are deliberately untouched. This one has no usage, no
// messages, no refusal and no failure status, so nothing accounts for it.
func TestPF8_ARecordThatExplainsNothingIsStillSkipped(t *testing.T) {
	sess := pfSession(t, Record{SessionID: "s1"})
	if sess.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", sess.Skipped)
	}
	if sess.ProviderFailures.Total() != 0 {
		t.Errorf("an unexplained record was counted as a provider failure")
	}
}

// PF9: a failure that nevertheless reported usage is not a ProviderFailure.
//
// The OpenAI-compatible reader has no type gate: ParseOpenAIResponse
// (openai.go:118-133) unmarshals any JSON body and sets Usage from any non-zero
// token counts, whatever the status was. So a recorded request can carry a
// status at or above 400 and real provider-reported usage at the same time.
//
// That usage was observed. Classifying the record as a ProviderFailure would
// take it out of the session totals and out of the priced figures, which is
// deleting a measurement because the HTTP request failed. The category is an
// observational classification, not an accounting mechanism, so the record
// stays usage-bearing and stays outside the category.
func TestPF9_AFailureCarryingUsageIsNotAProviderFailure(t *testing.T) {
	// With a prompt, it is an ordinary request and never reaches the branch.
	full := pfRecord("s1", 400)
	full.Prompt = Prompt{Messages: []Message{{Role: "user"}}}
	full.Response = Response{Usage: &transcript.Usage{Input: 7, Output: 3}}

	sess := pfSession(t, full)
	if sess.ProviderFailures.Total() != 0 {
		t.Errorf("a 400 carrying usage was counted as a provider failure; its measured " +
			"tokens would leave the totals")
	}
	if len(sess.Lanes) == 0 || len(sess.Lanes[0].Requests) != 1 {
		t.Fatalf("the record stopped being a request: %d lanes", len(sess.Lanes))
	}
	if got := sess.Lanes[0].Requests[0].Usage; got.Input != 7 || got.Output != 3 {
		t.Errorf("observed usage changed: input %d, output %d, want 7 and 3", got.Input, got.Output)
	}

	// Without a prompt it cannot become a request, so it falls to the existing
	// Skipped case. It is still not a ProviderFailure: the usage is the reason,
	// not the missing prompt.
	noPrompt := pfRecord("s1", 429)
	noPrompt.Response = Response{Usage: &transcript.Usage{Input: 5}}
	sess2 := pfSession(t, noPrompt)
	if sess2.ProviderFailures.Total() != 0 {
		t.Errorf("a 429 carrying usage was counted as a provider failure")
	}
	if sess2.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1: its existing treatment is unchanged", sess2.Skipped)
	}
}

// PF10: the observed statuses stay apart.
//
// A single failure count would answer nobody: 401 and 500 are different
// observations, and the status is the only evidence the record carries about
// what came back.
func TestPF10_ExactStatusesAreNotCollapsed(t *testing.T) {
	sess := pfSession(t,
		pfRecord("s1", 401), pfRecord("s1", 429), pfRecord("s1", 429), pfRecord("s1", 500))
	pf := sess.ProviderFailures
	for status, want := range map[int]int{401: 1, 429: 2, 500: 1} {
		if pf.ByStatus[status] != want {
			t.Errorf("ByStatus[%d] = %d, want %d: %v", status, pf.ByStatus[status], want, pf.ByStatus)
		}
	}
	if len(pf.ByStatus) != 3 {
		t.Errorf("statuses were collapsed into %d buckets: %v", len(pf.ByStatus), pf.ByStatus)
	}
}

// PF11: 300 exactly is outside the category, as the boundary says.
func TestPF11_Status300IsNotAProviderFailure(t *testing.T) {
	sess := pfSession(t, pfRecord("s1", 300))
	if sess.ProviderFailures.Total() != 0 {
		t.Errorf("300 was counted as a provider failure; the boundary is 400")
	}
}
