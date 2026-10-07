package proxy

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// SP-5/SP-6 (docs/requirements.md): tenant identity resolved before any cap
// is consulted, and the spend guard's own accumulators scoped by it.
//
// This file used to hold one characterization test,
// TestSpendGuard_DayCapIsProcessWideAndAttributed, pinning the gap on
// purpose: "PASS today: one session exhausts the day cap, an unrelated
// session is refused... FAIL on the first assertion: SP-6 landed. Replace
// this with SP-6's own acceptance criterion - tenant A exhausting its cap
// refuses A and not B." TestSP6_DayCapDoesNotCrossTenants below is exactly
// that replacement: it asserts the opposite of what the characterization
// test pinned, and it is RED against the pre-SP-6 guard (Check/Record took
// no tenant argument at all, so this file did not compile against it) and
// GREEN against the tenant-scoped one.

// TestSP6_DayCapDoesNotCrossTenants is SP-6's own acceptance test, cited by
// its row in docs/requirements.md verbatim: "Two tenants, one cap value:
// tenant A exhausting its day cap refuses A and not B."
//
// PASS: tenant A's day cap trips and refuses A's next request; tenant B,
// who has spent nothing, is unrefused under the identical cap value.
// FAIL: B is refused (the day cap pooled both tenants into one bucket,
// which is the exact pre-SP-6 behaviour TestSpendGuard_DayCapIsProcessWideAndAttributed
// used to pin), or A is not refused (the cap stopped working at all).
func TestSP6_DayCapDoesNotCrossTenants(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 100})
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	const sessA, sessB = "session-of-developer-a", "session-of-developer-b"

	if g.Check(tenantB, sessB) != "" {
		t.Fatal("tenant B must start unrefused, or this test proves nothing")
	}

	// Tenant A burns its own whole day budget.
	g.Record(tenantA, sessA, 100, 0, false)

	if reason := g.Check(tenantA, sessA); reason == "" {
		t.Fatal("tenant A exhausted its own day cap and must be refused")
	}
	if reason := g.Check(tenantB, sessB); reason != "" {
		t.Fatalf("tenant B spent nothing and must not be refused by tenant A's day cap: %q", reason)
	}
}

// TestSP6_SessionCapDoesNotCrossTenants is SP-6's second half: "Session caps
// key on {tenant, session}". The same literal session id under two
// different tenants must not share a session budget - the composed-string-
// key failure mode this scoping has to avoid even when a session id is
// reused (a client-chosen value, not something the proxy controls).
//
// PASS: tenant A's session cap trips on "shared-looking-session"; tenant
// B's own session of the identical name starts fresh.
// FAIL: tenant B inherits tenant A's session spend because the two keys
// collided.
func TestSP6_SessionCapDoesNotCrossTenants(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 50})
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	const sameName = "shared-looking-session"

	g.Record(tenantA, sameName, 50, 0, false)
	if reason := g.Check(tenantA, sameName); reason == "" {
		t.Fatal("tenant A's session cap must have tripped")
	}
	if reason := g.Check(tenantB, sameName); reason != "" {
		t.Fatalf("tenant B's identically-named session must start under its own fresh budget: %q", reason)
	}
}

// TestSP6_DollarCapDoesNotCrossTenants is the dollar-denominated sibling of
// TestSP6_DayCapDoesNotCrossTenants: SP-6 scopes both dimensions
// independently, and a dollar cap is the one most likely to still read a
// shared accumulator if only the token path were converted.
func TestSP6_DollarCapDoesNotCrossTenants(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayUSD: 10})
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"

	g.Record(tenantA, "sess-a", 1, 10, false)
	if reason := g.Check(tenantA, "sess-a"); reason == "" {
		t.Fatal("tenant A's dollar cap must have tripped")
	}
	if reason := g.Check(tenantB, "sess-b"); reason != "" {
		t.Fatalf("tenant B must not be refused by tenant A's dollar spend: %q", reason)
	}
}

// TestSP5_CheckFailsClosedOnAnUnresolvedTenant: SP-5 requires an unresolved
// tenant to be refused, never pooled. Check deciding "no identity, so
// nothing to check, so allow" would let through exactly the request SP-5
// names.
//
// PASS: both TenantUnknown and the empty TenantID refuse, even with every
// cap otherwise satisfied.
// FAIL: an unresolved tenant is let through on an enabled guard.
func TestSP5_CheckFailsClosedOnAnUnresolvedTenant(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 1_000_000})
	if reason := g.Check(tenancy.TenantUnknown, "sess"); reason == "" {
		t.Fatal("Check must refuse TenantUnknown rather than allow it through")
	}
	if reason := g.Check("", "sess"); reason == "" {
		t.Fatal("Check must refuse the empty TenantID rather than allow it through")
	}
}

// TestSP5_RecordRefusesToAccountForAnUnresolvedTenant: the write-side
// counterpart. An unresolved tenant's Record must not land in any
// accumulator at all - not LocalTenant's, not a new "unknown" bucket two
// different unresolved callers would come to share.
//
// PASS: after Record(TenantUnknown, ...) and Record("", ...), neither
// TenantUnknown nor the empty string has a day total, and LocalTenant's own
// day total is unaffected.
// FAIL: either call created an accumulator, under any key.
func TestSP5_RecordRefusesToAccountForAnUnresolvedTenant(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	g.Record(tenancy.LocalTenant, "sess-local", 1, 0, false)

	g.Record(tenancy.TenantUnknown, "sess", 100, 0, false)
	g.Record("", "sess", 100, 0, false)

	g.mu.Lock()
	_, unknownExists := g.dayUsed[tenancy.TenantUnknown]
	_, emptyExists := g.dayUsed[""]
	localTokens := 0
	if du, ok := g.dayUsed[tenancy.LocalTenant]; ok {
		localTokens = du.tokens
	}
	g.mu.Unlock()

	if unknownExists {
		t.Error("Record(TenantUnknown, ...) created an accumulator under TenantUnknown")
	}
	if emptyExists {
		t.Error(`Record("", ...) created an accumulator under the empty tenant`)
	}
	if localTokens != 1 {
		t.Fatalf("LocalTenant's own day total must be unaffected by the refused calls: got %d, want 1", localTokens)
	}
}

// TestSP6_PersistedStateKeepsTenantAttributionAcrossARestart: SP-6 names
// spendState as the second accumulator, beside dayUsed, that gains a tenant
// dimension. This is the restart test: two tenants spend, the process
// "dies" (a fresh SpendGuard), LoadState restores both, and neither
// tenant's restored figure is attributed to the other.
//
// PASS: after the restart, tenant A's restored day total refuses A at the
// same cap, tenant B's restored total refuses B independently, and
// swapping either figure onto the other tenant would change the outcome
// (checked directly against the two totals captured mid-test).
// FAIL: either tenant's restored spend is missing, zero, or has leaked into
// the other tenant's bucket.
func TestSP6_PersistedStateKeepsTenantAttributionAcrossARestart(t *testing.T) {
	dir := t.TempDir()
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"

	before := NewSpendGuard(SpendLimits{DayUSD: 10})
	before.LoadState(dir) // nothing on disk yet; must not panic
	before.Record(tenantA, "sess-a", 1, 7.00, false)
	before.Record(tenantB, "sess-b", 1, 2.00, false)
	before.SaveState(dir)

	// The process dies and a fresh guard restores from the same directory.
	after := NewSpendGuard(SpendLimits{DayUSD: 10})
	after.LoadState(dir)

	if msg := after.Check(tenantA, "sess-a-new"); msg != "" {
		t.Fatalf("tenant A restored at $7.00 of $10 must not refuse yet: %q", msg)
	}
	if msg := after.Check(tenantB, "sess-b-new"); msg != "" {
		t.Fatalf("tenant B restored at $2.00 of $10 must not refuse yet: %q", msg)
	}

	// Tenant A spends $3.50 more: $10.50 of $10 must refuse A only.
	after.Record(tenantA, "sess-a-new", 1, 3.50, false)
	if msg := after.Check(tenantA, "sess-a-new"); msg == "" {
		t.Fatal("tenant A's restored spend did not carry forward: $10.50 of $10 was allowed")
	}
	if msg := after.Check(tenantB, "sess-b-new"); msg != "" {
		t.Fatalf("tenant A's restart-restored overrun must not refuse tenant B: %q", msg)
	}
}

// TestSP6_MalformedPersistedTenantKeyIsSkippedNotFatal: one corrupted tenant
// entry in the state file must cost only its own bucket, not every other
// tenant's legitimate one in the same file - and must not be coerced into
// LocalTenant or any other real tenant, which would be a cross-tenant
// attribution bug hiding inside error recovery.
//
// PASS: the well-formed tenant's restored cap still fires; the corrupted
// key contributes no accumulator under any tenant, including LocalTenant.
// FAIL: the corrupted entry is silently merged into LocalTenant (or any
// other key), or the well-formed tenant's entry is lost because one sibling
// in the same file could not be trusted.
func TestSP6_MalformedPersistedTenantKeyIsSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().UTC().Format("2006-01-02")
	// Hand-crafted: one legitimate tenant, one reserved sentinel masquerading
	// as a tenant key, one empty key. json.Marshal would never produce the
	// latter two from this package's own TenantID values (Record refuses
	// them), so this is deliberately a file no version of this code wrote.
	body := `{"day":"` + today + `","tenants":{"acme-co":{"tokens":5,"usd":0},"TENANT_UNKNOWN":{"tokens":999,"usd":0},"":{"tokens":999,"usd":0}}}`
	writeSpendStateFile(t, dir, body)

	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	g.LoadState(dir) // must not panic, must not error out

	if msg := g.Check(tenancy.LocalTenant, "sess"); msg != "" {
		t.Fatalf("a corrupted sibling entry must not land on LocalTenant: %q", msg)
	}
	g.mu.Lock()
	acmeTokens := 0
	if du, ok := g.dayUsed["acme-co"]; ok {
		acmeTokens = du.tokens
	}
	_, unknownExists := g.dayUsed[tenancy.TenantUnknown]
	_, emptyExists := g.dayUsed[""]
	g.mu.Unlock()

	if acmeTokens != 5 {
		t.Fatalf("the well-formed sibling tenant's entry must survive: got %d tokens, want 5", acmeTokens)
	}
	if unknownExists || emptyExists {
		t.Fatal("a corrupted tenant key must not become a live accumulator under any key")
	}
}

// TestSP6_LegacyPreTenancyStateFileRestoresAsLocalTenant: an existing
// solo-developer install's spend-day.json, written by every version of this
// code before SP-6, has no tenant key at all - a flat {day,tokens,usd}.
// "Local/default behaviour stays exactly as compatible as before" (this
// unit's own build note) has to cover restart persistence too, or every
// solo developer who upgrades loses their day cap's memory on the first
// restart after upgrading, which is the exact failure spendStateFile's own
// doc comment calls worse than no cap at all.
//
// PASS: a legacy file's figure restores under tenancy.LocalTenant and
// refuses LocalTenant's own next request appropriately.
// FAIL: the legacy figure is lost (day cap silently resets on upgrade), or
// lands under some tenant other than LocalTenant.
func TestSP6_LegacyPreTenancyStateFileRestoresAsLocalTenant(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().UTC().Format("2006-01-02")
	body := `{"day":"` + today + `","tokens":9,"usd":0}`
	writeSpendStateFile(t, dir, body)

	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	g.LoadState(dir)

	if msg := g.Check(tenancy.LocalTenant, "sess"); msg != "" {
		t.Fatalf("9 of 10 must not refuse yet: %q", msg)
	}
	g.Record(tenancy.LocalTenant, "sess", 1, 0, false)
	if msg := g.Check(tenancy.LocalTenant, "sess"); msg == "" {
		t.Fatal("the legacy file's spend did not restore under LocalTenant: 10 of 10 was allowed")
	}
}

// TestSP6_AmbiguousPersistedStateIgnoresLegacyFieldsWhenTenantsPresent: a
// file carrying BOTH the new tenants map and the legacy flat fields is not
// a shape any version of this code writes (SaveState elides the legacy
// fields whenever it writes at all). Guessing how to merge the two readings
// risks folding a stray legacy figure into whatever tenant key happens to
// collide with it. The documented rule is that the tenant-scoped shape wins
// outright and the legacy fields are ignored.
//
// PASS: with both present, only the tenants map's own figure is live; the
// legacy figure does not additionally land on LocalTenant or on the
// colliding key.
// FAIL: the legacy figure is added on top of (or merged into) the tenant
// map's own entry.
func TestSP6_AmbiguousPersistedStateIgnoresLegacyFieldsWhenTenantsPresent(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().UTC().Format("2006-01-02")
	// LocalTenant appears both as a key in the tenants map (5 tokens) and in
	// the legacy top-level fields (999 tokens). Only the tenants map's 5 may
	// be live.
	body := `{"day":"` + today + `","tokens":999,"usd":0,"tenants":{"LOCAL":{"tokens":5,"usd":0}}}`
	writeSpendStateFile(t, dir, body)

	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	g.LoadState(dir)

	g.mu.Lock()
	tokens := 0
	if du, ok := g.dayUsed[tenancy.LocalTenant]; ok {
		tokens = du.tokens
	}
	g.mu.Unlock()
	if tokens != 5 {
		t.Fatalf("the legacy top-level fields must be ignored once a tenants map is present: got %d, want 5", tokens)
	}
}

// TestSP6_ConcurrentSameTenantRecordingIsNotLost: concurrent requests for
// the same tenant (and the same session) must not lose an update to a race.
// The guard's single mutex already serialized every access before SP-6;
// this pins that the deeper, tenant-scoped map changed nothing about that.
//
// PASS: 200 goroutines each recording 1 token for the same tenant and
// session leave exactly 200 tokens accounted.
// FAIL: fewer than 200 (a lost update) under `go test -race`, or a race
// report.
func TestSP6_ConcurrentSameTenantRecordingIsNotLost(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 1 << 30})
	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.Record(tenancy.LocalTenant, "sess", 1, 0, false)
		}()
	}
	wg.Wait()

	g.mu.Lock()
	tokens := 0
	if du, ok := g.dayUsed[tenancy.LocalTenant]; ok {
		tokens = du.tokens
	}
	g.mu.Unlock()
	if tokens != n {
		t.Fatalf("got %d tokens recorded across %d concurrent calls, want %d (a lost update)", tokens, n, n)
	}
}

// TestSP6_ConcurrentCrossTenantRecordingIsIsolated: concurrent requests for
// two DIFFERENT tenants must never cross-attribute under race, not only
// under a serial call sequence. A bug that drops the tenant from a lookup
// under contention (as opposed to always) would pass every serial test
// above and still corrupt a live multi-tenant proxy.
//
// PASS: after 200 interleaved goroutines split evenly between two tenants,
// each tenant's own day total is exactly 100, under `go test -race`.
// FAIL: either tenant's total is wrong, or the race detector reports a
// conflict.
func TestSP6_ConcurrentCrossTenantRecordingIsIsolated(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 1 << 30})
	const perTenant = 100
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	var wg sync.WaitGroup
	for i := 0; i < perTenant; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			g.Record(tenantA, "sess-a", 1, 0, false)
		}()
		go func() {
			defer wg.Done()
			g.Record(tenantB, "sess-b", 1, 0, false)
		}()
	}
	wg.Wait()

	g.mu.Lock()
	var tokensA, tokensB int
	if du, ok := g.dayUsed[tenantA]; ok {
		tokensA = du.tokens
	}
	if du, ok := g.dayUsed[tenantB]; ok {
		tokensB = du.tokens
	}
	g.mu.Unlock()
	if tokensA != perTenant {
		t.Errorf("tenant A: got %d tokens, want %d", tokensA, perTenant)
	}
	if tokensB != perTenant {
		t.Errorf("tenant B: got %d tokens, want %d", tokensB, perTenant)
	}
}

// TestSP6_DayRolloverResetsEveryTenantAtMidnightUTC: g.day is one shared
// fact (midnight is the same instant for everyone); only the per-tenant
// totals reset with it. Two tenants with live spend, the clock crosses UTC
// midnight, and both must reset together - not one tenant rolling over
// while the other's stale total survives into the new day.
//
// PASS: both tenants are unrefused after the roll.
// FAIL: either tenant's pre-midnight spend survives into the new day.
func TestSP6_DayRolloverResetsEveryTenantAtMidnightUTC(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	day := time.Date(2026, 9, 2, 23, 59, 0, 0, time.UTC)
	g.now = func() time.Time { return day }

	g.Record(tenantA, "sess-a", 10, 0, false)
	g.Record(tenantB, "sess-b", 10, 0, false)
	if g.Check(tenantA, "sess-a") == "" {
		t.Fatal("fixture: tenant A should be capped before the roll")
	}
	if g.Check(tenantB, "sess-b") == "" {
		t.Fatal("fixture: tenant B should be capped before the roll")
	}

	g.now = func() time.Time { return day.Add(2 * time.Minute) }
	if reason := g.Check(tenantA, "sess-a"); reason != "" {
		t.Fatalf("tenant A's day counter must reset after midnight: %q", reason)
	}
	if reason := g.Check(tenantB, "sess-b"); reason != "" {
		t.Fatalf("tenant B's day counter must reset after midnight: %q", reason)
	}
}

// TestSP6_LocalDefaultBehaviourIsUnchanged: the zero-configuration path -
// no tenant header, ResolveTenant("") - must behave exactly as the guard
// did before SP-6 for a solo workflow with exactly one tenant. This is
// effectively TestSpendGuardCapsSessionAndDay (guards_test.go) restated
// explicitly against tenancy.LocalTenant, so a regression here is caught by
// name as a tenancy compatibility break rather than only as a generic spend
// guard failure.
func TestSP6_LocalDefaultBehaviourIsUnchanged(t *testing.T) {
	resolved, err := tenancy.ResolveTenant("")
	if err != nil || resolved != tenancy.LocalTenant {
		t.Fatalf("ResolveTenant(\"\") = %q, %v; want LocalTenant, nil", resolved, err)
	}

	g := NewSpendGuard(SpendLimits{SessionTokens: 100, DayTokens: 150})
	if g.Check(resolved, "a") != "" {
		t.Fatal("fresh session must be allowed")
	}
	g.Record(resolved, "a", 60, 0, false)
	if g.Check(resolved, "a") != "" {
		t.Fatal("under the cap must be allowed")
	}
	g.Record(resolved, "a", 40, 0, false)
	if reason := g.Check(resolved, "a"); reason == "" {
		t.Fatal("session cap must refuse the next request")
	}
}

// --- HTTP-boundary integration: SP-5's own half, "resolved at the proxy
// boundary", proven end to end rather than only at the SpendGuard unit
// level above. ---

// TestSP5_HTTPRequestWithNoTenantHeaderRunsAsLocalDefault: a client that
// never sends HeaderTenantID - every existing local workflow - must see
// identical behaviour to before this change: forwarded, 200, no refusal.
func TestSP5_HTTPRequestWithNoTenantHeaderRunsAsLocalDefault(t *testing.T) {
	up := &upstream{t: t}
	base, dir, _ := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 1_000_000})})
	resp := post(t, base, "/v1/messages", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("a request with no tenant header must be forwarded as before: status %d", resp.StatusCode)
	}
	waitLedger(t, dir, 1)
}

// TestSP5_HTTPRequestWithAnUnresolvableTenantIsRefused: SP-5's acceptance
// criterion verbatim, at the boundary - "A request with no resolvable
// tenant is refused with a distinct reason, and contributes to no
// accumulator."
//
// PASS: a reserved sentinel sent as the tenant header is refused with
// replay_tenant_unresolved before the request reaches the upstream, and the
// status endpoint's running cost is untouched by it.
// FAIL: the request is forwarded, or counted as cost.
func TestSP5_HTTPRequestWithAnUnresolvableTenantIsRefused(t *testing.T) {
	up := &upstream{t: t}
	base, dir, _ := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 1_000_000})})

	resp := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: "TENANT_UNKNOWN"})
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("an unresolvable tenant must be refused with 400, got %d", resp.StatusCode)
	}
	body := readAll(t, resp)
	if !strings.Contains(body, "replay_tenant_unresolved") {
		t.Fatalf("refusal body does not name the tenant guard: %s", body)
	}

	recs := waitLedger(t, dir, 1)
	if recs[0].Refusal != "tenant_unresolved" {
		t.Fatalf("ledger record does not attribute the refusal to the tenant guard: %+v", recs[0])
	}

	if up.seen().requests != 0 {
		t.Fatal("a request whose tenant could not be resolved must never reach the upstream")
	}
	if st := getStatus(t, base); st.CostUSD != 0 {
		t.Fatalf("an unresolved-tenant refusal must contribute no cost: $%.6f", st.CostUSD)
	}
}

// TestSP5_HTTPTwoTenantsDoNotShareADayCapAcrossRealRequests: the full
// integration of SP-5's resolution and SP-6's scoped guard together, not
// each in isolation - proof that the header is actually read at the
// boundary and actually reaches the same guard instance's per-tenant table.
//
// PASS: tenant A's requests exhaust a tiny day-token cap and A's next
// request is refused; tenant B, sending the identical cap value and never
// having sent a byte, is still forwarded.
// FAIL: tenant B is refused (the boundary resolved a header but the guard
// still pooled both under one bucket), or neither tenant is ever refused
// (resolution is not reaching Check/Record at all).
func TestSP5_HTTPTwoTenantsDoNotShareADayCapAcrossRealRequests(t *testing.T) {
	up := &upstream{t: t}
	// messageResponse's usage totals 4+30+300+2 = 336 tokens; one request
	// exactly exhausts this cap.
	base, dir, _ := startProxyWith(t, up, Config{Spend: NewSpendGuard(SpendLimits{DayTokens: 336})})

	respA1 := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: "acme-co"})
	if respA1.StatusCode != 200 {
		t.Fatalf("tenant A's first request should be under its own fresh cap: status %d", respA1.StatusCode)
	}
	respA1.Body.Close()
	waitLedger(t, dir, 1)

	respA2 := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: "acme-co", HeaderSessionID: "session-a-2"})
	defer respA2.Body.Close()
	if respA2.StatusCode == 200 {
		t.Fatal("tenant A exhausted its own day cap and the next request must be refused")
	}

	respB := post(t, base, "/v1/messages", map[string]string{HeaderTenantID: "widget-co", HeaderSessionID: "session-b-1"})
	defer respB.Body.Close()
	if respB.StatusCode != 200 {
		body := readAll(t, respB)
		t.Fatalf("tenant B spent nothing under its own identical day cap and must be forwarded: status %d, body %s", respB.StatusCode, body)
	}
}

// writeSpendStateFile is a test helper that plants a hand-crafted
// spend-day.json, for fixtures no version of SaveState would itself
// produce (a reserved or empty tenant key, both legacy and tenant-scoped
// fields at once).
func writeSpendStateFile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, spendStateFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readAll drains and returns a response body as a string, for assertions
// against the refusal message. t.Helper so a failure in the read itself
// reports at the caller.
func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
