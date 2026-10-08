package proxy

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// SP-8 (docs/requirements.md): eviction may not widen a cap.
//
// The guard's per-session table is bounded at maxSpendSessions and evicted
// least-recently-seen entries by deleting them outright. Deleting discarded
// that session's accumulated spend along with the map entry, so a session
// that went quiet long enough to churn out of the table came back to a
// fresh session budget - the cap's whole point, undone by the guard's own
// bookkeeping. For one developer the day cap backstops this. For many
// tenants with parallel sub-agent lanes the table churns continuously and
// the session cap becomes advisory.
//
// This file's design choice: durable accounting, not bare fail-closed
// admission. A session evicted from the live table is demoted into a
// second, separately-bounded table (g.retired, maxRetiredSpendSessions per
// tenant) rather than discarded, so returning to it restores its own past
// spend. Fail-closed is the backstop for the one case durability itself
// cannot cover: a tenant whose live AND retired tables are both already at
// capacity cannot admit a session it has never seen without discarding
// someone's accounting, so Check refuses that admission before the request
// ever reaches the provider, and Record never discards an existing entry to
// force one through regardless (TestSP8_RecordNeverDiscardsAnExistingSessionWhenBothTablesAreFull).
//
// Why this preserves the existing eviction-mechanics tests
// (TestSpendGuardEvictsLeastRecentlyUsedUnderAFrozenClock,
// TestSpendGuard_EvictsLeastRecentlyUsedWhenTheClockCannotSeparateRecords,
// TestSpendGuard_AStillActiveHeavySessionOutlivesIdleOnes) and SP-7's
// attribution tests (spend_attribution_test.go) unmodified: those tests
// assert which session is evicted FROM THE LIVE TABLE, never that the
// evicted session's accounting is gone afterward, and SP-7's attribution
// (dayLeader) deliberately still ranges only g.session[tenant] - the live
// table - so a session sitting in g.retired still does not count toward
// "the survivors add up to the day total", and TestSP7_IncompleteAccountingIsDisclosedNotGuessed's
// partial-attribution assertion is exercised exactly as before. The mechanics
// are extended, not replaced, and no SP-5/SP-6/SP-7 test in this package
// needed a single line changed.

// TestSP8_EvictedSessionIsNotGrantedAFreshSessionBudgetOnReturn is SP-8's own
// acceptance criterion verbatim: "A session evicted and then seen again is
// not granted a fresh session budget."
//
// This is the RED test. Against the guard before SP-8, "victim" is deleted
// outright on eviction, so when it returns Check finds no record of it at
// all, treats it as a brand-new session with zero spend, and allows it
// straight through a cap it had already exhausted.
//
// PASS: victim, having already exhausted its session cap and then been
// churned out of the live table, is still refused when it returns.
// FAIL (today's behaviour): the returning victim is allowed, because
// eviction discarded its accounting.
func TestSP8_EvictedSessionIsNotGrantedAFreshSessionBudgetOnReturn(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 100})
	frozen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return frozen }

	g.Record(tenancy.LocalTenant, "victim", 100, 0, false)
	if reason := g.Check(tenancy.LocalTenant, "victim"); reason == "" {
		t.Fatal("fixture: victim should already be at its own session cap")
	}

	// Enough churn from other sessions to evict victim from the live table.
	for i := 0; i < maxSpendSessions; i++ {
		g.Record(tenancy.LocalTenant, fmt.Sprintf("filler-%d", i), 1, 0, false)
	}
	g.mu.Lock()
	_, stillLive := g.session[tenancy.LocalTenant]["victim"]
	g.mu.Unlock()
	if stillLive {
		t.Fatal("fixture: victim must have been evicted from the live table, or this test exercises nothing")
	}

	if reason := g.Check(tenancy.LocalTenant, "victim"); reason == "" {
		t.Fatal("SP-8: an evicted session returned and was granted a fresh session budget")
	}
}

// TestSP8_EvictedSessionsPartialSpendCarriesForward is the same property at
// a value below the cap, so it cannot be satisfied by accident (e.g. a bug
// that just always refuses a returning session regardless of its actual
// balance).
//
// PASS: a session evicted with 60 of its 100-token cap spent is allowed one
// more 30-token request (now at 90) and refused the request after that
// (would reach 130).
// FAIL: the returning session's balance reads as zero (fresh) or as
// anything other than the 60 it actually spent.
func TestSP8_EvictedSessionsPartialSpendCarriesForward(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 100})
	frozen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return frozen }

	g.Record(tenancy.LocalTenant, "quiet", 60, 0, false)
	for i := 0; i < maxSpendSessions; i++ {
		g.Record(tenancy.LocalTenant, fmt.Sprintf("filler-%d", i), 1, 0, false)
	}
	g.mu.Lock()
	_, stillLive := g.session[tenancy.LocalTenant]["quiet"]
	_, inRetired := g.retired[tenancy.LocalTenant]["quiet"]
	g.mu.Unlock()
	if stillLive || !inRetired {
		t.Fatal("fixture: quiet must have been evicted into the retired table, or this test exercises nothing")
	}

	if reason := g.Check(tenancy.LocalTenant, "quiet"); reason != "" {
		t.Fatalf("quiet returned at 60 of 100 and must not be refused yet: %q", reason)
	}
	g.Record(tenancy.LocalTenant, "quiet", 30, 0, false)
	if reason := g.Check(tenancy.LocalTenant, "quiet"); reason != "" {
		t.Fatalf("quiet at 90 of 100 must not be refused yet: %q", reason)
	}
	g.Record(tenancy.LocalTenant, "quiet", 30, 0, false)
	if reason := g.Check(tenancy.LocalTenant, "quiet"); reason == "" {
		t.Fatal("quiet is now at 120 of a 100-token cap and must be refused; " +
			"its past spend did not carry forward correctly")
	}
}

// TestSP8_WhenRetiredIsAlsoFullANeverSeenSessionFailsClosed is the second
// half of the design: durable accounting needs its own bound, and this is
// what happens when BOTH the live table and its retired remembrance are
// full for a tenant. There is nowhere to put a session this guard has never
// seen without discarding somebody's accounting, so admission itself is
// refused - the fail-closed half of "durable for the life of the cap
// window, or eviction must fail closed."
//
// PASS: once live and retired are both at capacity, a session id that has
// never been seen is refused by Check before it would ever reach the
// provider.
// FAIL: the stranger is admitted, which is only possible by silently
// discarding an existing session's durable accounting - the exact defect
// SP-8 names, one level removed.
func TestSP8_WhenRetiredIsAlsoFullANeverSeenSessionFailsClosed(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 1 << 30})
	frozen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return frozen }

	total := maxSpendSessions + maxRetiredSpendSessions
	for i := 0; i < total; i++ {
		g.Record(tenancy.LocalTenant, fmt.Sprintf("s-%d", i), 1, 0, false)
	}
	g.mu.Lock()
	liveLen := len(g.session[tenancy.LocalTenant])
	retiredLen := len(g.retired[tenancy.LocalTenant])
	g.mu.Unlock()
	if liveLen != maxSpendSessions || retiredLen != maxRetiredSpendSessions {
		t.Fatalf("fixture: want live=%d retired=%d, got live=%d retired=%d",
			maxSpendSessions, maxRetiredSpendSessions, liveLen, retiredLen)
	}

	if reason := g.Check(tenancy.LocalTenant, "stranger"); reason == "" {
		t.Fatal("SP-8: both tables are full and a never-seen session was admitted anyway, " +
			"which can only happen by silently discarding another session's accounting")
	}
}

// TestSP8_RecordNeverDiscardsAnExistingSessionWhenBothTablesAreFull is
// Record's own defensive depth for the same extreme case Check already
// refuses pre-flight: if Record is ever reached for a never-seen session
// while both tables are full (the narrow race between a Check that saw
// room and enough concurrent new sessions landing first), it must not
// discard any existing tracked session to force the newcomer in. The
// newcomer gets no session-level slot; its tokens still land on the
// tenant's day total, which remains the backstop.
//
// PASS: every session tracked (live or retired) before the call is still
// tracked, identically, after it; the stranger gets no slot in either
// table; the stranger's tokens are still visible in the tenant's day total.
// FAIL: any previously-tracked session's entry changed or disappeared, or
// the stranger's tokens vanished entirely (the day cap backstop silently
// stopped working too).
func TestSP8_RecordNeverDiscardsAnExistingSessionWhenBothTablesAreFull(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 1 << 30})
	frozen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return frozen }

	total := maxSpendSessions + maxRetiredSpendSessions
	for i := 0; i < total; i++ {
		g.Record(tenancy.LocalTenant, fmt.Sprintf("s-%d", i), 1, 0, false)
	}

	g.mu.Lock()
	beforeLive := snapshot(g.session[tenancy.LocalTenant])
	beforeRetired := snapshot(g.retired[tenancy.LocalTenant])
	beforeDay := g.dayUsed[tenancy.LocalTenant].tokens
	g.mu.Unlock()

	g.Record(tenancy.LocalTenant, "stranger", 7, 0, false)

	g.mu.Lock()
	afterLive := snapshot(g.session[tenancy.LocalTenant])
	afterRetired := snapshot(g.retired[tenancy.LocalTenant])
	afterDay := g.dayUsed[tenancy.LocalTenant].tokens
	_, strangerLive := g.session[tenancy.LocalTenant]["stranger"]
	_, strangerRetired := g.retired[tenancy.LocalTenant]["stranger"]
	g.mu.Unlock()

	for id, tok := range beforeLive {
		if afterLive[id] != tok {
			t.Errorf("live session %q changed from %d to %d tokens when admitting a stranger with no room", id, tok, afterLive[id])
		}
	}
	for id, tok := range beforeRetired {
		if afterRetired[id] != tok {
			t.Errorf("retired session %q changed from %d to %d tokens when admitting a stranger with no room", id, tok, afterRetired[id])
		}
	}
	if strangerLive || strangerRetired {
		t.Error("the stranger was given a tracked slot despite both tables being full")
	}
	if afterDay != beforeDay+7 {
		t.Errorf("tenant day total: got %d, want %d; the day-cap backstop must still see untracked spend", afterDay, beforeDay+7)
	}
}

// snapshot copies a session map's token totals for before/after comparison.
func snapshot(m map[string]*spend) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v.tokens
	}
	return out
}

// TestSP8_FullTableForOneTenantDoesNotCrossToAnother: tenant isolation on
// the capacity dimension SP-8 adds. A tenant whose table is full and
// churning must never make another tenant's own, empty table look full.
//
// PASS: tenant B is admitted and tracked normally while tenant A's table is
// full and actively retiring sessions.
// FAIL: tenant B is refused by tableFull, or tenant B's session is found
// under a session id tenant A happens to have just retired.
func TestSP8_FullTableForOneTenantDoesNotCrossToAnother(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 1 << 30})
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	frozen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return frozen }

	for i := 0; i < maxSpendSessions+1; i++ {
		g.Record(tenantA, fmt.Sprintf("a-%d", i), 1, 0, false)
	}
	g.mu.Lock()
	_, aRetiredExists := g.retired[tenantA]["a-0"]
	bLive := len(g.session[tenantB])
	bRetired := len(g.retired[tenantB])
	g.mu.Unlock()
	if !aRetiredExists || bLive != 0 || bRetired != 0 {
		t.Fatal("fixture: tenant A should have retired a-0 and tenant B should have no table yet")
	}

	if reason := g.Check(tenantB, "b-session"); reason != "" {
		t.Fatalf("tenant B was refused admission by tenant A's full, churning table: %q", reason)
	}
	g.Record(tenantB, "b-session", 5, 0, false)
	if _, ok := g.session[tenantB]["b-session"]; !ok {
		t.Fatal("tenant B's session was not admitted into its own live table")
	}

	// A session id tenant A happens to have retired must not make tenant B
	// look like it has a record of that same id.
	if reason := g.Check(tenantB, "a-0"); reason != "" {
		t.Fatalf("tenant B was refused for a session id only tenant A has ever seen: %q", reason)
	}
}

// TestSP8_RetiredAccountingDoesNotCrossTenants is the data-isolation sibling
// of the capacity-isolation test above, and the one a collapsed (non
// tenant-nested) retired map would actually fail: tenant A's session
// reaches its own session cap, is evicted into retired, and tenant B's
// byte-identical session id must start under its own fresh cap rather than
// inherit tenant A's exhausted one.
//
// PASS: tenant B's session of the identical name is unrefused at 5 of its
// own 10-token cap, immediately after tenant A's same-named session was
// retired at 10 of 10.
// FAIL: tenant B is refused, which can only happen if the retired table is
// shared across tenants rather than nested per tenant like the live one.
func TestSP8_RetiredAccountingDoesNotCrossTenants(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 10})
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	const sameName = "shared-looking-session"
	frozen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return frozen }

	g.Record(tenantA, sameName, 10, 0, false)
	if reason := g.Check(tenantA, sameName); reason == "" {
		t.Fatal("fixture: tenant A's session cap should already be tripped")
	}
	for i := 0; i < maxSpendSessions; i++ {
		g.Record(tenantA, fmt.Sprintf("filler-%d", i), 1, 0, false)
	}
	g.mu.Lock()
	_, stillLive := g.session[tenantA][sameName]
	_, inRetired := g.retired[tenantA][sameName]
	g.mu.Unlock()
	if stillLive || !inRetired {
		t.Fatal("fixture: tenant A's session must have been evicted into retired, or this test exercises nothing")
	}

	if reason := g.Check(tenantB, sameName); reason != "" {
		t.Fatalf("tenant B inherited tenant A's retired accounting for the identical session id: %q", reason)
	}
	g.Record(tenantB, sameName, 5, 0, false)
	if reason := g.Check(tenantB, sameName); reason != "" {
		t.Fatalf("tenant B's session should be under its own fresh 10-token cap at 5: %q", reason)
	}
}

// TestSP8_ConcurrentDistinctSessionsNearCapacityDoNotRaceOrLoseUpdates:
// many goroutines, many distinct never-before-seen session ids, run close
// to the combined live+retired capacity under `go test -race`. Catches a
// lost update under contention and a panic on the eviction/retirement
// boundary condition, neither of which a serial test can see.
func TestSP8_ConcurrentDistinctSessionsNearCapacityDoNotRaceOrLoseUpdates(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 1 << 30})
	const n = maxSpendSessions + maxRetiredSpendSessions - 10
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g.Record(tenancy.LocalTenant, fmt.Sprintf("race-%d", i), 1, 0, false)
		}(i)
	}
	wg.Wait()

	g.mu.Lock()
	tokens := 0
	if du, ok := g.dayUsed[tenancy.LocalTenant]; ok {
		tokens = du.tokens
	}
	liveLen := len(g.session[tenancy.LocalTenant])
	retiredLen := len(g.retired[tenancy.LocalTenant])
	g.mu.Unlock()

	if tokens != n {
		t.Fatalf("day total: got %d, want %d (a lost update under concurrency)", tokens, n)
	}
	if liveLen > maxSpendSessions {
		t.Errorf("live table exceeded its bound: %d > %d", liveLen, maxSpendSessions)
	}
	if retiredLen > maxRetiredSpendSessions {
		t.Errorf("retired table exceeded its bound: %d > %d", retiredLen, maxRetiredSpendSessions)
	}
}

// TestSP8_DayCapAttributionIsUnaffectedByRetirement pins SP-7's attribution
// rule explicitly against SP-8's change, beyond relying on
// spend_attribution_test.go staying green unmodified: a session's move from
// live to retired must not change dayLeader's own accounting, which
// deliberately still ranges only the live table.
func TestSP8_DayCapAttributionIsUnaffectedByRetirement(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 1_000_000})
	frozen := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return frozen }

	g.Record(tenancy.LocalTenant, "lane-the-real-cause", 500_000, 0, false)
	for i := 0; i < maxSpendSessions+16; i++ {
		g.Record(tenancy.LocalTenant, fmt.Sprintf("lane-%d", i), 500, 0, false)
	}
	g.mu.Lock()
	_, stillLive := g.session[tenancy.LocalTenant]["lane-the-real-cause"]
	_, inRetired := g.retired[tenancy.LocalTenant]["lane-the-real-cause"]
	g.mu.Unlock()
	if stillLive || !inRetired {
		t.Fatal("fixture: the heavy session must be retired, not live, or this does not exercise the gap")
	}

	reason := g.Check(tenancy.LocalTenant, "lane-innocent")
	if reason == "" {
		t.Fatal("the day cap must refuse")
	}
	if !strings.Contains(reason, "partly") && !strings.Contains(reason, "accounted") {
		t.Errorf("attribution is incomplete and the refusal does not disclose it: %q", reason)
	}
}
