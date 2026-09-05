package proxy

import (
	"bytes"
	"log"
	"net/http/httptest"
	"strings"
	"testing"
)

// A request on a path this build cannot parse is forwarded unchanged. That is
// the right behaviour and it is also the dangerous one, because everything
// Replay does hangs off parsing: no ledger record, no spend cap, no loop
// detector, and no secret masking. The user configured those and believes they
// are on.
//
// This is the same failure the day cap had before it was persisted, and the
// same one CapNotEnforced exists to shout about: protection that quietly is not
// there is worse than protection nobody claimed.
func TestUnparsedTrafficIsCountedAndAnnouncedOnce(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0), Spend: NewSpendGuard(SpendLimits{SessionTokens: 1000})}, stats: newStats()}

	for i := 0; i < 3; i++ {
		s.noteUnparsed("/v1/chat/completions")
	}

	if got := s.stats.unparsedTotal(); got != 3 {
		t.Fatalf("every unparsed request must be counted: %d", got)
	}
	// Once, not per request: a warning printed on every call is noise the
	// operator learns to scroll past.
	if n := strings.Count(buf.String(), "NOT PARSED"); n != 1 {
		t.Fatalf("the warning must fire once, fired %d times:\n%s", n, buf.String())
	}
	line := buf.String()
	for _, want := range []string{"/v1/chat/completions", "spend cap", "mask"} {
		if !strings.Contains(strings.ToLower(line), strings.ToLower(want)) {
			t.Fatalf("the warning must name the path and what is inert: missing %q in\n%s", want, line)
		}
	}
}

// A second unfamiliar path is its own warning: the operator has changed
// something and needs to know this one is unprotected too.
func TestASecondUnparsedPathWarnsAgain(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	s.noteUnparsed("/v1/chat/completions")
	s.noteUnparsed("/v1/responses")
	if n := strings.Count(buf.String(), "NOT PARSED"); n != 2 {
		t.Fatalf("each new path warns once, got %d:\n%s", n, buf.String())
	}
}

// The count reaches the metrics endpoint, because a number a monitor can watch
// outlives a log line nobody re-reads.
func TestUnparsedCountReachesMetrics(t *testing.T) {
	s := &Server{cfg: Config{}, stats: newStats()}
	s.noteUnparsed("/v1/chat/completions")
	m := s.stats.metrics()
	if !strings.Contains(m, "replay_unparsed_requests_total 1") {
		t.Fatalf("unparsed traffic is invisible on /replay/metrics:\n%s", m)
	}
}

// Parsed traffic must not trip it, or the warning means nothing.
func TestParsedTrafficIsNotCountedAsUnparsed(t *testing.T) {
	s := &Server{cfg: Config{}, stats: newStats()}
	_ = httptest.NewRequest("POST", "/v1/messages", nil)
	if got := s.stats.unparsedTotal(); got != 0 {
		t.Fatalf("nothing unparsed happened: %d", got)
	}
}
