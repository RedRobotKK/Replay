package proxy

import (
	"bytes"
	"log"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A guard firing is the product's strongest claim and today it leaves almost no
// trace. The counter reaches /replay/metrics and nothing else: no log line, no
// ledger record. The observable behaviour of a guard saving somebody money
// overnight is that the log simply stops and the agent shows a provider-shaped
// error. This pins the fix.

func TestRefusalIsLoggedWithAttribution(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	w := httptest.NewRecorder()

	s.refuseSession(w, "session-abcdef123456789", refusalLoop,
		"the same Bash call was just made 12 times in a row", 0)

	got := buf.String()
	for _, want := range []string{"REFUSED", "loop", "session-abcd", "12 times"} {
		if !strings.Contains(got, want) {
			t.Fatalf("log line %q is missing %q", got, want)
		}
	}
	// The session id is shortened the way every other log line shortens it.
	if strings.Contains(got, "session-abcdef123456789") {
		t.Fatalf("the full session id should not be logged verbatim: %q", got)
	}
}

func TestRefusalStillAnswersTheClient(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	w := httptest.NewRecorder()

	s.refuseSession(w, "s1", refusalCircuitOpen, "the provider has been failing", 30*time.Second)

	if w.Code != refusalCircuitOpen.status {
		t.Fatalf("status %d, want %d", w.Code, refusalCircuitOpen.status)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("a circuit-open refusal must still carry Retry-After")
	}
	if !strings.Contains(w.Body.String(), refusalCircuitOpen.errType) {
		t.Fatalf("the body must keep the provider error shape: %s", w.Body.String())
	}
}

// A refusal must never be counted as a provider failure. refusalCircuitOpen is
// 503 and IsRetryableStatus covers 500-599, so anything that routes a refusal
// into Breaker.Observe re-arms the cooldown on every refused request and the
// circuit never closes.
func TestRefusalIsNeverObservedAsAProviderFailure(t *testing.T) {
	br := NewBreaker(BreakerSettings{Failures: 2, Cooldown: time.Minute})
	br.Observe(true)
	br.Observe(true)
	if ok, _, _ := br.Allow(); ok {
		t.Fatal("fixture: the circuit should be open here")
	}

	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0), Breaker: br}, stats: newStats()}
	for i := 0; i < 20; i++ {
		s.refuseSession(httptest.NewRecorder(), "s1", refusalCircuitOpen, "holding", time.Second)
	}
	// Twenty refusals must not have touched the breaker's failure count.
	// Still open, and still refusing: nothing reset or re-armed it.
	if ok, _, _ := br.Allow(); ok {
		t.Fatal("twenty refusals changed the breaker's state")
	}
}

// Nil-safety: most test configs have no logger and no guards.
func TestRefusalSurvivesAMinimalConfig(t *testing.T) {
	s := &Server{cfg: Config{}, stats: newStats()}
	w := httptest.NewRecorder()
	s.refuseSession(w, "", refusalSpendCap, "cap reached", 0)
	if w.Code != refusalSpendCap.status {
		t.Fatalf("status %d", w.Code)
	}
}
