package proxy

import "testing"

// The error budget divides error-content prompt tokens by the session's prompt
// tokens. The denominator, st.tally, accumulates every request of the session
// regardless of which agent lane sent it. The numerator was written from
// AnalyzeLane, which sees one lane, into a single session-wide field, so with
// a sub-agent running the two sides of the ratio measured different things and
// whichever lane rescored last decided the numerator.
//
// This is a live guard: the ratio reaches ErrorBudget.Check and refuses real
// requests. Both sides have to be scoped the same way.
func TestErrorBudgetCountsEveryLaneOfTheSession(t *testing.T) {
	s := newStats()
	st := s.session("sess-1")
	st.tally.PromptTokens = 1_000_000

	s.setLaneErrors("sess-1", "", 40_000)
	s.setLaneErrors("sess-1", "agent-1", 25_000)

	errs, prompt := s.errorTokens("sess-1")
	if errs != 65_000 {
		t.Fatalf("the numerator must cover every lane the denominator covers: got %d, want 65000", errs)
	}
	if prompt != 1_000_000 {
		t.Fatalf("denominator changed: %d", prompt)
	}
}

// The sharp edge of the old shape: a quiet lane rescoring after a busy one
// replaced the busy lane's contribution, so the budget silently stopped
// seeing the errors it exists to catch.
func TestAQuietLaneDoesNotEraseABusyOne(t *testing.T) {
	s := newStats()
	st := s.session("sess-1")
	st.tally.PromptTokens = 500_000

	s.setLaneErrors("sess-1", "agent-busy", 90_000)
	s.setLaneErrors("sess-1", "agent-quiet", 0)

	if errs, _ := s.errorTokens("sess-1"); errs != 90_000 {
		t.Fatalf("a later quiet lane erased an earlier busy one: got %d, want 90000", errs)
	}

	// And a lane rescoring again replaces only its own figure, because each
	// rescore recomputes that lane's total rather than adding a delta.
	s.setLaneErrors("sess-1", "agent-busy", 12_000)
	if errs, _ := s.errorTokens("sess-1"); errs != 12_000 {
		t.Fatalf("a lane's rescore must replace its own contribution only: got %d, want 12000", errs)
	}
}

// Lanes of one session must not become separate sessions. ADR-0003 pins the
// policy at a session's first request for the life of that session, so keying
// the session map by agent would give every sub-agent its own pin, and would
// split the spend cap per agent as well. The fix belongs in the numerator.
func TestLanesShareOneSessionStateAndOnePolicyPin(t *testing.T) {
	s := newStats()
	main := s.session("sess-1")
	s.setLaneErrors("sess-1", "agent-1", 1)
	if got := len(s.sessions); got != 1 {
		t.Fatalf("lanes must not create sessions: %d entries", got)
	}
	if s.session("sess-1") != main {
		t.Fatal("a lane rescore replaced the session state")
	}
}

func TestErrorTokensOnAnUnknownSessionIsZero(t *testing.T) {
	s := newStats()
	if errs, prompt := s.errorTokens("nope"); errs != 0 || prompt != 0 {
		t.Fatalf("got %d/%d, want 0/0", errs, prompt)
	}
}
