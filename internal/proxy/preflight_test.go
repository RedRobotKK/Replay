package proxy

import (
	"bytes"
	"log"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
)

// preFlightFixture builds a server whose session has already established a
// prefix, which is the only state the guard reads.
func preFlightFixture(t *testing.T, p analysis.PolicyState, priorHash string) (*Server, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0), PreFlight: p}, stats: newStats()}
	st := s.stats.session("sess-1")
	if st == nil {
		t.Fatal("session state was not created; the fixture asserts nothing")
	}
	st.prefixByLane = map[string]string{"": priorHash}
	return s, &buf
}

// rec builds a request record with a prefix of a given size.
func preFlightRec(hash string, systemBytes, toolBytes int) *ledger.Record {
	r := &ledger.Record{SessionID: "sess-1"}
	r.Model = "claude-opus-5"
	r.PrefixHash = hash
	r.Prompt.SystemBytes = systemBytes
	r.Prompt.ToolBytes = toolBytes
	return r
}

// A changed prefix over the ceiling is refused, and the refusal says why.
//
// This is the guard doing its job. 800,000 prefix bytes price at 200,000
// tokens, well clear of a 50,000 ceiling and clear of the error band, so the
// decision is not a straddle.
//
// PASS: the request is answered locally, in the provider's error shape, naming
// the cause and the numbers.
// FAIL: it was forwarded, or refused without saying what happened.
func TestPreFlight_RefusesAChangedPrefixOverTheCeiling(t *testing.T) {
	s, logs := preFlightFixture(t, analysis.PolicyState{CeilingTokens: 50_000, OptInActive: true}, "hash-A")
	w := httptest.NewRecorder()

	if s.preFlight(w, preFlightRec("hash-B", 400_000, 400_000), "") {
		t.Fatal("a 200,000-token prefix re-lay passed a 50,000-token ceiling")
	}
	if w.Code != refusalPreFlight.status {
		t.Errorf("status = %d, want %d", w.Code, refusalPreFlight.status)
	}
	body := w.Body.String()
	for _, want := range []string{refusalPreFlight.errType, "200000", "50000", "tool definitions changed"} {
		if !strings.Contains(body, want) {
			t.Errorf("the refusal body does not contain %q. A guard that refuses without naming "+
				"the cause and the numbers is indistinguishable from a provider outage.\n%s", want, body)
		}
	}
	if !strings.Contains(logs.String(), "REFUSED preflight_deficit") {
		t.Errorf("the refusal was not logged with its counter:\n%s", logs.String())
	}
}

// An unchanged prefix is never refused, however large it is.
//
// A matching prefix is read at the cache rate. Refusing it would cost the user
// a turn to save nothing, which is the defect that killed the wire-level
// blocking design.
//
// PASS: forwarded.
// FAIL: size alone triggered a refusal.
func TestPreFlight_AMatchingPrefixIsNeverRefused(t *testing.T) {
	s, _ := preFlightFixture(t, analysis.PolicyState{CeilingTokens: 1, OptInActive: true}, "hash-A")
	w := httptest.NewRecorder()

	if !s.preFlight(w, preFlightRec("hash-A", 400_000, 400_000), "") {
		t.Error("a request whose prefix matches the session's was refused. It reads from cache " +
			"at any size, so refusing it spends a turn and saves nothing")
	}
	if w.Header().Get(HeaderWarning) != "" {
		t.Errorf("a matching prefix produced a warning: %q", w.Header().Get(HeaderWarning))
	}
}

// A session's first request establishes the prefix and cannot have diverged.
//
// Without this the guard refuses the opening request of every session, because
// an empty prior hash differs from any real one.
//
// PASS: forwarded.
// FAIL: the empty prior hash was treated as a divergence.
func TestPreFlight_TheFirstRequestOfASessionPasses(t *testing.T) {
	s, _ := preFlightFixture(t, analysis.PolicyState{CeilingTokens: 1, OptInActive: true}, "")
	w := httptest.NewRecorder()

	if !s.preFlight(w, preFlightRec("hash-A", 400_000, 400_000), "") {
		t.Error("the first request of a session was refused; it establishes the prefix and has " +
			"nothing to have diverged from")
	}
}

// With no opt-in, nothing is ever refused.
//
// This is the shipped default. If it ever fails, every user of a default build
// starts getting local refusals for traffic the provider would have served.
//
// PASS: forwarded, and silent.
// FAIL: the default became a policy.
func TestPreFlight_TheDefaultNeitherRefusesNorWarns(t *testing.T) {
	s, logs := preFlightFixture(t, analysis.PolicyState{}, "hash-A")
	w := httptest.NewRecorder()

	if !s.preFlight(w, preFlightRec("hash-B", 400_000, 400_000), "") {
		t.Fatal("the zero PreFlight policy refused a request. A ceiling nobody set must not " +
			"refuse anybody's request")
	}
	if w.Header().Get(HeaderWarning) != "" {
		t.Errorf("the default warned: %q", w.Header().Get(HeaderWarning))
	}
	if logs.Len() != 0 {
		t.Errorf("the default logged:\n%s", logs.String())
	}
}

// A ceiling inside the estimate's error band warns and forwards.
//
// The pre-flight token figure is bytes through a fitted ratio, so a ceiling
// within +/-15% of it cannot be resolved. Refusing there reports precision the
// tool does not have; the honest answer is to say so and let it through.
//
// PASS: forwarded, with the warning header naming the band.
// FAIL: refused on noise, or forwarded silently.
func TestPreFlight_AStraddledCeilingWarnsAndForwards(t *testing.T) {
	// 400,000 bytes -> 100,000 tokens, band 85,000 to 115,000.
	s, logs := preFlightFixture(t, analysis.PolicyState{CeilingTokens: 90_000, OptInActive: true}, "hash-A")
	w := httptest.NewRecorder()

	rec := preFlightRec("hash-B", 200_000, 200_000)
	d := analysis.NewPreFlightDeficit(100_000, true, false)
	if !d.WouldRefuse(s.cfg.PreFlight) || !d.Straddles(s.cfg.PreFlight) {
		t.Fatal("the fixture no longer produces a straddling refusal, so this test cannot fail")
	}

	if !s.preFlight(w, rec, "") {
		t.Fatal("a refusal decided inside the estimate's own error band was enforced")
	}
	warn := w.Header().Get(HeaderWarning)
	if !strings.Contains(warn, "preflight:") || !strings.Contains(warn, "not refused") {
		t.Errorf("the straddle warning does not say it passed through: %q", warn)
	}
	if !strings.Contains(logs.String(), "preflight straddle") {
		t.Errorf("the straddle was not logged:\n%s", logs.String())
	}
}

// The override header proceeds once, as it does for every other guard.
//
// PASS: forwarded, and the override reason is logged.
// FAIL: the guard ignored an override every other guard honours.
func TestPreFlight_OverrideProceedsOnceAndIsLogged(t *testing.T) {
	s, logs := preFlightFixture(t, analysis.PolicyState{CeilingTokens: 50_000, OptInActive: true}, "hash-A")
	w := httptest.NewRecorder()

	if !s.preFlight(w, preFlightRec("hash-B", 400_000, 400_000), "reindexing the tool set") {
		t.Fatal("an overridden pre-flight ceiling still refused")
	}
	if !strings.Contains(logs.String(), "preflight ceiling overridden") ||
		!strings.Contains(logs.String(), "reindexing the tool set") {
		t.Errorf("the override was not logged with its reason:\n%s", logs.String())
	}
}

// A pre-flight refusal must be judged against the lane's own prefix.
//
// The guard first shipped reading a session-wide hash, so in the fan-out
// workload it was built for, one lane's request would refuse another lane that
// had changed nothing. Caught by the composition analysis of the 2026-09-06
// trial, not by review.
//
// PASS: a lane whose own prefix is unchanged is forwarded, whatever the other
// lanes are carrying.
// FAIL: a sibling lane's prefix decided this lane's fate.
func TestPreFlight_ASiblingLaneDoesNotTriggerARefusal(t *testing.T) {
	s, _ := preFlightFixture(t, analysis.PolicyState{CeilingTokens: 1, OptInActive: true}, "hash-main")
	st := s.stats.session("sess-1")
	st.prefixByLane["lane-a"] = "hash-a"
	st.prefixByLane["lane-b"] = "hash-b"

	rec := preFlightRec("hash-a", 400_000, 400_000)
	rec.AgentID = "lane-a"

	w := httptest.NewRecorder()
	if !s.preFlight(w, rec, "") {
		t.Error("lane-a sent the same prefix it sent last time and was refused. Its siblings " +
			"carry different tool sets, which is normal in a fan-out session and is not a " +
			"divergence in this lane")
	}
	if w.Header().Get(HeaderWarning) != "" {
		t.Errorf("a stable lane was warned: %q", w.Header().Get(HeaderWarning))
	}
}
