package proxy

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// SP-9 (docs/requirements.md): the session table gains a tenant dimension.
//
// ADR-0015, amended by ADR-0028, names four surfaces that must gain a tenant
// dimension before any shared deployment is built on top of them: the spend
// guard, the session table, the metrics surface and the credential path.
// SP-5 through SP-8 did the first in full - SpendGuard.session/retired/dayUsed
// are all keyed by tenancy.TenantID. This file is the second: stats.sessions
// (internal/proxy/state.go) is still, after SP-8, a single flat
// map[string]*sessionState with no tenant key at all, even though SP-5 has
// already resolved a tenant for every guarded request since before SP-6
// existed. Two different tenants whose clients happen to send the same
// session id - plausible, since a session id is a client-chosen string
// (HeaderSessionID), never issued or namespaced by this proxy - collide on
// one shared *sessionState, and inherit each other's error-budget
// accounting, policy pin, cache-break classification and pre-flight lane
// history. That is a correctness defect in shipped, reachable code today
// (no "centralised mode" flag gates it off: SP-5's tenant header is read on
// every request), not a hypothetical future requirement.
//
// This file is RED against 76f1e92 the same way spend_tenancy_test.go's own
// header describes SP-6's tests being RED against the pre-SP-6 guard: every
// call below passes a tenancy.TenantID as the session methods' first
// argument, and none of session/setLaneErrors/errorTokens/pinned/pin/
// trialSession/laneSnapshot/observe/rescore/noteBreach took one before this
// change, so this file did not compile against 76f1e92 at all.

// TestSP9_ErrorBudgetAccountingDoesNotCrossTenantsOnASessionIDCollision is
// SP-9's own acceptance criterion for the error budget: two tenants send the
// identical session id; tenant A's lanes carry heavy error content, tenant
// B's carries none. Tenant B's error accounting must read as B's own (zero),
// never A's.
//
// PASS: tenant B's errorTokens is 0 of its own prompt tokens. FAIL (the
// pre-SP-9 shape): B's lookup finds A's *sessionState under the shared key
// and reports A's error tokens as its own, which could trip B's error
// budget refusal for traffic B never sent.
func TestSP9_ErrorBudgetAccountingDoesNotCrossTenantsOnASessionIDCollision(t *testing.T) {
	s := newStats()
	const tenantA, tenantB tenancy.TenantID = "tenant-a", "tenant-b"
	const sharedSessionID = "sess-shared"

	stA := s.session(tenantA, sharedSessionID)
	stA.tally.PromptTokens = 1_000_000
	s.setLaneErrors(tenantA, sharedSessionID, "", 900_000)

	stB := s.session(tenantB, sharedSessionID)
	stB.tally.PromptTokens = 1_000_000

	errsA, promptA := s.errorTokens(tenantA, sharedSessionID)
	if errsA != 900_000 || promptA != 1_000_000 {
		t.Fatalf("tenant A's own accounting changed: got %d/%d, want 900000/1000000", errsA, promptA)
	}
	errsB, promptB := s.errorTokens(tenantB, sharedSessionID)
	if errsB != 0 {
		t.Fatalf("tenant B inherited tenant A's error tokens on a session id collision: got %d, want 0", errsB)
	}
	if promptB != 1_000_000 {
		t.Fatalf("tenant B's own prompt tokens changed: got %d, want 1000000", promptB)
	}
}

// TestSP9_PolicyPinDoesNotCrossTenantsOnASessionIDCollision: ADR-0003 pins a
// session's context-edit decision at its first request, for that session's
// life. Two tenants sharing a session id must each get their own pin, never
// one deciding for the other.
//
// PASS: tenant B, pinned second, keeps its own decision. FAIL: B's pin()
// call is a no-op because pinned() already found A's decision under the
// shared key ("a decision already made is kept").
func TestSP9_PolicyPinDoesNotCrossTenantsOnASessionIDCollision(t *testing.T) {
	s := newStats()
	const tenantA, tenantB tenancy.TenantID = "tenant-a", "tenant-b"
	const sharedSessionID = "sess-shared"

	s.pin(tenantA, sharedSessionID, nil, policy.Control, time.Time{})
	if _, decision, ok := s.pinned(tenantA, sharedSessionID); !ok || decision != policy.Control {
		t.Fatalf("tenant A's own pin is missing: ok=%v decision=%q", ok, decision)
	}

	if _, _, ok := s.pinned(tenantB, sharedSessionID); ok {
		t.Fatalf("tenant B saw a pin before making one, on a session id collision with tenant A")
	}
	s.pin(tenantB, sharedSessionID, nil, policy.Applied, time.Time{})
	if _, decision, ok := s.pinned(tenantB, sharedSessionID); !ok || decision != policy.Applied {
		t.Fatalf("tenant B's pin lost to tenant A's on a session id collision: ok=%v decision=%q", ok, decision)
	}
	// A's own pin must still read back unchanged.
	if _, decision, ok := s.pinned(tenantA, sharedSessionID); !ok || decision != policy.Control {
		t.Fatalf("tenant A's pin changed after tenant B pinned the same session id: ok=%v decision=%q", ok, decision)
	}
}

// TestSP9_SessionTableIsBoundedPerTenantNotGlobally mirrors SpendGuard's own
// maxSpendSessions discipline (guards.go): the bound applies within one
// tenant's own table, so one tenant filling its table to maxSessions cannot
// evict another tenant's sessions, and cannot be evicted by them.
//
// PASS: tenant A's maxSessions sessions all survive tenant B filling its own
// table to the same bound. FAIL: a global bound would have evicted A's
// oldest sessions when B's sessions were admitted.
func TestSP9_SessionTableIsBoundedPerTenantNotGlobally(t *testing.T) {
	s := newStats()
	const tenantA, tenantB tenancy.TenantID = "tenant-a", "tenant-b"

	for i := 0; i < maxSessions; i++ {
		s.session(tenantA, sessIDForTest("a", i))
	}
	for i := 0; i < maxSessions; i++ {
		s.session(tenantB, sessIDForTest("b", i))
	}
	if got := len(s.sessions[tenantA]); got != maxSessions {
		t.Fatalf("tenant A's table was evicted by tenant B filling its own: %d entries, want %d", got, maxSessions)
	}
	if got := len(s.sessions[tenantB]); got != maxSessions {
		t.Fatalf("tenant B's own table did not fill: %d entries, want %d", got, maxSessions)
	}
	// The very first session A admitted must still be live: nothing evicted
	// it, because B's admissions never touch A's table.
	if _, ok := s.sessions[tenantA][sessIDForTest("a", 0)]; !ok {
		t.Fatalf("tenant A's oldest session was evicted by tenant B's unrelated traffic")
	}
}

// TestSP9_GuardDoesNotRefuseOnAnotherTenantsErrorBudget exercises the actual
// production call site (passthrough.go's guard(), not the stats package
// directly): s.cfg.ErrorBudget.Check(s.stats.errorTokens(tenant,
// rec.SessionID)). A unit test against *stats alone cannot prove this wiring;
// it is entirely possible to tenant-scope stats.sessions correctly and still
// pass the wrong tenant - or a hardcoded one - at the one place that reads
// it for a live refusal decision. That exact mutation (hardcoding
// tenancy.LocalTenant at this call site) builds cleanly and passed every
// other test in this package; only this test kills it.
//
// PASS: tenant A's heavy error-budget breach refuses A's own next request,
// and tenant B - same session id, clean accounting - is unrefused.
// FAIL: B is refused on A's error content, or A is not refused at all (the
// budget stopped working).
func TestSP9_GuardDoesNotRefuseOnAnotherTenantsErrorBudget(t *testing.T) {
	s := &Server{
		cfg: Config{
			ErrorBudget: ErrorBudget{Share: 0.5},
		},
		stats: newStats(),
	}
	const tenantA, tenantB tenancy.TenantID = "tenant-a", "tenant-b"
	const sharedSessionID = "sess-shared"

	stA := s.stats.session(tenantA, sharedSessionID)
	stA.tally.PromptTokens = errorBudgetMinPromptTokens * 2
	s.stats.setLaneErrors(tenantA, sharedSessionID, "", errorBudgetMinPromptTokens*2)

	stB := s.stats.session(tenantB, sharedSessionID)
	stB.tally.PromptTokens = errorBudgetMinPromptTokens * 2

	recA := &ledger.Record{SessionID: sharedSessionID}
	reqA := httptest.NewRequest(http.MethodPost, "http://x/v1/messages", nil)
	if s.guard(httptest.NewRecorder(), reqA, recA, tenantA) {
		t.Fatal("tenant A's own error-heavy session was not refused; the budget stopped working")
	}

	recB := &ledger.Record{SessionID: sharedSessionID}
	reqB := httptest.NewRequest(http.MethodPost, "http://x/v1/messages", nil)
	if !s.guard(httptest.NewRecorder(), reqB, recB, tenantB) {
		t.Fatal("tenant B was refused on tenant A's error budget, on a session id collision")
	}
}

// TestSP9_ApplyPolicyReadsEachTenantsOwnPin is the same wiring proof as
// TestSP9_GuardDoesNotRefuseOnAnotherTenantsErrorBudget, for the other
// production call site threaded through in this change: policyinject.go's
// applyPolicy, which calls s.stats.pinned(tenant, rec.SessionID). ADR-0003
// pins a session's context-edit decision at its first request for that
// session's life; two tenants sharing a session id must each get their own
// pin applied to their own bytes.
//
// Each tenant's pin is seeded directly via s.stats.pin so neither request
// reaches decidePolicy's ledger-persisted-pin path (internal/ledger's Pin
// store is sessionID-keyed with no tenant dimension at all - a separate,
// already-identified gap this unit does not fix; seeding the in-memory pin
// isolates the one wiring path this unit changed).
//
// PASS: tenant A's body is spliced with A's own trigger/keep parameters,
// tenant B's with B's. FAIL: applyPolicy read the wrong tenant's bucket and
// spliced the other tenant's parameters, or applied nothing.
func TestSP9_ApplyPolicyReadsEachTenantsOwnPin(t *testing.T) {
	var logs bytes.Buffer
	s := &Server{cfg: Config{
		Logger:      log.New(&logs, "", 0),
		ContextEdit: &policy.ContextEdit{TriggerTokens: 1, KeepLast: 1}, // policyConfigured() only
	}, stats: newStats()}

	const tenantA, tenantB tenancy.TenantID = "tenant-a", "tenant-b"
	const sharedSessionID = "sess-shared"
	editA := &policy.ContextEdit{TriggerTokens: 111_000, KeepLast: 3}
	editB := &policy.ContextEdit{TriggerTokens: 222_000, KeepLast: 7}
	s.stats.pin(tenantA, sharedSessionID, editA, policy.Applied, time.Now())
	s.stats.pin(tenantB, sharedSessionID, editB, policy.Applied, time.Now())

	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "http://x/v1/messages", nil)
		req.Header.Set("anthropic-beta", policy.BetaFeature)
		return req
	}
	body := []byte(`{}`)

	recA := &ledger.Record{SessionID: sharedSessionID}
	outA := s.applyPolicy(newReq(), recA, body, true, tenantA)
	wantA, _ := editA.Apply(body)
	if !bytes.Equal(outA, wantA) {
		t.Fatalf("tenant A got the wrong splice:\n  got:  %s\n  want: %s", outA, wantA)
	}

	recB := &ledger.Record{SessionID: sharedSessionID}
	outB := s.applyPolicy(newReq(), recB, body, true, tenantB)
	wantB, _ := editB.Apply(body)
	if !bytes.Equal(outB, wantB) {
		t.Fatalf("tenant B got the wrong splice (likely tenant A's pin, on a session id collision):\n  got:  %s\n  want: %s", outB, wantB)
	}
	if bytes.Equal(outA, outB) {
		t.Fatal("both tenants produced the identical spliced body, so this test could not have told them apart")
	}
}

// TestSP9_PreFlightJudgesEachTenantsOwnLaneHistory is the third wiring proof,
// for preflight.go's call to s.stats.laneSnapshot(tenant, rec.SessionID,
// rec.AgentID). A changed prefix is judged against THIS lane's own previous
// request (preFlight's own comment: "never the session's"); it must also
// never be judged against another TENANT's lane that merely shares a session
// id.
//
// Two tenants share a session id and each already established their own
// prior prefix. Tenant B's new request repeats tenant A's prior prefix,
// which is not a divergence for A but is one for B's own history.
//
// PASS: tenant B is refused (its own prefix diverged, over the ceiling).
// FAIL: laneSnapshot read the wrong tenant's bucket, found nothing (or the
// wrong lane), and let the divergence through unrefused.
func TestSP9_PreFlightJudgesEachTenantsOwnLaneHistory(t *testing.T) {
	s := &Server{cfg: Config{
		Logger:    log.New(&bytes.Buffer{}, "", 0),
		PreFlight: analysis.PolicyState{CeilingTokens: 1, OptInActive: true},
	}, stats: newStats()}
	const tenantA, tenantB tenancy.TenantID = "tenant-a", "tenant-b"
	const sharedSessionID = "sess-shared"

	stA := s.stats.session(tenantA, sharedSessionID)
	stA.lane("").prefixHash, stA.lane("").seen = "hash-A", true
	stB := s.stats.session(tenantB, sharedSessionID)
	stB.lane("").prefixHash, stB.lane("").seen = "hash-B", true

	recB := &ledger.Record{SessionID: sharedSessionID}
	recB.Model = "claude-opus-5"
	recB.PrefixHash = "hash-A" // matches A's prior, not B's own ("hash-B")
	recB.Prompt.SystemBytes = 400_000
	recB.Prompt.ToolBytes = 400_000

	if s.preFlight(httptest.NewRecorder(), recB, "", tenantB) {
		t.Fatal("tenant B's prefix change was judged against tenant A's lane history " +
			"and let through unrefused, on a session id collision")
	}
}

// TestSP9_EnterLaneOverlapDoesNotCrossTenantsOnASessionIDCollision covers the
// fourth production call site threaded through in this change
// (passthrough.go's s.stats.enterLane(tenant, rec.SessionID, rec.AgentID)),
// which feeds correlation() and, through it, whether a cache break is
// reported NOT MEASURED ("two or more requests of this lane were in flight
// together") or attributed to a real cause. Two tenants sharing a session id
// must never mark each other's requests as overlapping.
//
// PASS: tenant A's in-flight request does not mark tenant B's own request
// (same session id, different tenant) as overlapping, and vice versa.
// FAIL: the two tenants' flags are the same *bool (or either was flipped by
// the other), which is exactly what a shared, unscoped lane would do.
func TestSP9_EnterLaneOverlapDoesNotCrossTenantsOnASessionIDCollision(t *testing.T) {
	s := newStats()
	const tenantA, tenantB tenancy.TenantID = "tenant-a", "tenant-b"
	const sharedSessionID = "sess-shared"

	flagA, doneA := s.enterLane(tenantA, sharedSessionID, "")
	flagB, doneB := s.enterLane(tenantB, sharedSessionID, "")
	defer doneA()
	defer doneB()

	if flagA == flagB {
		t.Fatal("both tenants got the same overlap flag for the same session id; their lanes are not isolated")
	}
	if *flagA {
		t.Fatal("tenant A's request was marked overlapping by tenant B's unrelated in-flight request")
	}
	if *flagB {
		t.Fatal("tenant B's request was marked overlapping by tenant A's unrelated in-flight request")
	}
}

func sessIDForTest(prefix string, i int) string {
	return fmt.Sprintf("%s-%d", prefix, i)
}
