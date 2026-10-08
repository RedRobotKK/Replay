package proxy

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// SP-10 (docs/requirements.md): a persisted policy-context-edit pin is
// owned by (tenant, sessionID), never by sessionID alone. Before this unit,
// decidePolicy read and wrote ledger.Store's pin with
// s.cfg.Store.Pin(sessionID) / SetPin(pin) - no tenant argument at all - so
// a tenant-aware in-memory miss (SP-9's s.stats, nested by tenant) fell
// through to the shared persisted .pins store and could retrieve another
// tenant's policy decision whenever two tenants happened to send the same
// client-chosen session id.
//
// TestSP10_RED_CrossTenantSessionIDCollisionLeaksPersistedPolicyPin (this
// file's prior revision, run against 206aee2's actual decidePolicy and
// Store.Pin/SetPin) failed on both assertions below for exactly this
// reason: widget-co's second request came back carrying acme-co's
// context_management edit, and the one persisted pin for the shared
// session id read back as acme-co's "applied" decision. Kept here as the
// acceptance test for the fix, now exercising the tenant-aware signatures.

// TestSP10_CrossTenantSessionIDCollisionDoesNotLeakPersistedPolicyPin is
// SP-10's own acceptance test, crossing the real production path -
// applyPolicy -> decidePolicy -> Store.Pin/SetPin - at the HTTP boundary,
// on a single live proxy process (the real topology: one `replay serve`
// serving many tenants, SP-5's header resolved per request).
//
// Tenant acme-co and tenant widget-co both use the same client-chosen
// session id. acme-co's first request enables the beta header and is
// admissible, so its decision is pinned "applied". Because
// s.stats.sessions is nested by tenant (SP-9), widget-co's own bucket has
// never seen this session id, so its own first request genuinely misses
// the in-memory layer and must fall through to the real persisted store -
// this is not a contrived condition, it is what happens on every first
// request from any tenant.
//
// PASS: widget-co's own first request (no beta header) is judged fresh,
// from its own request, and decided "skipped: ..."; a later widget-co
// request that does send the beta header still carries no edit, because a
// session decides once and is pinned for its life (PX-8). acme-co's own
// pin, read back from a freshly reopened Store (as a restart would see
// it), is still exactly acme-co's.
// FAIL: widget-co's session is pinned "applied" from acme-co's decision -
// visible either as a context_management edit appearing on widget-co's
// wire body, or as the persisted pin read back under tenant widget-co
// showing decision=applied.
func TestSP10_CrossTenantSessionIDCollisionDoesNotLeakPersistedPolicyPin(t *testing.T) {
	up := &bodyEcho{}
	base, dir, _ := startProxyWith(t, up, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}})
	const sharedSession = "shared-session"
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"

	postWith(t, base, requestBody, map[string]string{
		HeaderTenantID: string(tenantA), HeaderSessionID: sharedSession, "anthropic-beta": policy.BetaFeature,
	})
	postWith(t, base, requestBody, map[string]string{
		HeaderTenantID: string(tenantB), HeaderSessionID: sharedSession,
	})
	postWith(t, base, requestBody, map[string]string{
		HeaderTenantID: string(tenantB), HeaderSessionID: sharedSession, "anthropic-beta": policy.BetaFeature,
	})

	bodies := up.seen()
	if len(bodies) != 3 {
		t.Fatalf("upstream saw %d requests, want 3", len(bodies))
	}
	if strings.Contains(string(bodies[1]), "context_management") {
		t.Fatalf("widget-co's own first request (no beta) must never carry the parameter: %s", bodies[1])
	}
	if strings.Contains(string(bodies[2]), "context_management") {
		t.Fatalf("CROSS-TENANT LEAK: widget-co's second request carries a context_management "+
			"edit it was never pinned for by its own decision: %s", bodies[2])
	}

	store2, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	pinA, okA := store2.Pin(tenantA, sharedSession)
	if !okA || pinA.Decision != string(policy.Applied) {
		t.Fatalf("tenant A's own pin must still read applied: %+v %v", pinA, okA)
	}
	pinB, okB := store2.Pin(tenantB, sharedSession)
	if !okB {
		t.Fatal("tenant B must have its own persisted pin, decided from its own first request")
	}
	if pinB.Decision == string(policy.Applied) {
		t.Fatalf("CROSS-TENANT LEAK (persisted store): tenant B's own pin reads "+
			"decision=applied, which only tenant A's own first request (beta enabled) "+
			"could have produced: %+v", pinB)
	}
}

// TestSP10_RestartDoesNotLeakPersistedPolicyPinAcrossTenants is SP-10's
// restart-persistence acceptance test (Phase 5's "Persistence" case):
// write tenant A's pin, create a fresh Store over the same directory
// (simulating a proxy restart), tenant B using the same session id must
// not retrieve A's pin, and tenant A must still retrieve its own.
func TestSP10_RestartDoesNotLeakPersistedPolicyPinAcrossTenants(t *testing.T) {
	dir := t.TempDir()
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	const sharedSession = "restart-shared-session"

	before, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := before.SetPin(tenantA, ledger.Pin{SessionID: sharedSession, Policy: policy.Name, Trigger: 150000, Keep: 6, Decision: string(policy.Applied)}); err != nil {
		t.Fatal(err)
	}

	// The process dies and a fresh Store restores from the same directory.
	after, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := after.Pin(tenantB, sharedSession); ok {
		t.Fatal("tenant B must not retrieve tenant A's restart-restored pin")
	}
	pinA, ok := after.Pin(tenantA, sharedSession)
	if !ok || pinA.Decision != string(policy.Applied) || pinA.Trigger != 150000 {
		t.Fatalf("tenant A's own pin must survive a restart: %+v %v", pinA, ok)
	}

	// Tenant B now establishes its own, different pin; tenant A's must be
	// untouched by it.
	if err := after.SetPin(tenantB, ledger.Pin{SessionID: sharedSession, Decision: "control: held out of the trial", Trial: "control"}); err != nil {
		t.Fatal(err)
	}
	pinA2, ok := after.Pin(tenantA, sharedSession)
	if !ok || pinA2.Decision != string(policy.Applied) {
		t.Fatalf("tenant B's own SetPin must not disturb tenant A's pin: %+v %v", pinA2, ok)
	}
	pinB, ok := after.Pin(tenantB, sharedSession)
	if !ok || pinB.Decision != "control: held out of the trial" {
		t.Fatalf("tenant B's own pin: %+v %v", pinB, ok)
	}
}

// TestSP10_NamedTenantRequestDoesNotInheritLocalTenantsLegacyPin: a
// pin already exists under tenancy.LocalTenant for a session id - exactly
// what a pre-SP-10, zero-identity local install's own persisted decision
// looks like once it is read back as LocalTenant (loadPins' own rule) -
// and a NAMED tenant then sends its own first request reusing that same
// client-chosen session id. decidePolicy's persisted-pin read
// (s.cfg.Store.Pin(tenant, sessionID)) must be keyed by the request's own
// resolved tenant, never by LocalTenant unconditionally: a caller that
// silently read LocalTenant's pin instead of the real tenant's would pass
// every other test in this file (none of them give LocalTenant a pin at
// all) while still handing a named tenant the local default's policy
// decision outright.
//
// PASS: tenant acme-co, having no pin of its own, decides fresh from its
// own request and its own live config; LocalTenant's legacy pin is read
// back unchanged.
// FAIL: tenant acme-co's request carries LocalTenant's persisted
// trigger/keep parameters instead of its own freshly-decided ones.
func TestSP10_NamedTenantRequestDoesNotInheritLocalTenantsLegacyPin(t *testing.T) {
	dir := t.TempDir()
	seed, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	const sharedSession = "shared-session"
	// LocalTenant's own, earlier-persisted decision: a distinctive
	// trigger/keep that a fresh decision under the live config below
	// would never produce.
	if err := seed.SetPin(tenancy.LocalTenant, ledger.Pin{
		SessionID: sharedSession, Policy: policy.Name, Trigger: 999999, Keep: 9, Decision: string(policy.Applied),
	}); err != nil {
		t.Fatal(err)
	}

	up := &bodyEcho{}
	base, _, _ := startProxyIn(t, up, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 1, KeepLast: 1}}, dir)

	const tenantA tenancy.TenantID = "acme-co"
	postWith(t, base, requestBody, map[string]string{
		HeaderTenantID: string(tenantA), HeaderSessionID: sharedSession, "anthropic-beta": policy.BetaFeature,
	})

	bodies := up.seen()
	if len(bodies) != 1 {
		t.Fatalf("upstream saw %d requests, want 1", len(bodies))
	}
	if triggerSeen(bodies[0]) != "1" {
		t.Fatalf("CROSS-TENANT LEAK: tenant acme-co's request carried trigger=%s, "+
			"which must be its own live config's value (1), not LocalTenant's "+
			"persisted legacy pin (999999): %s", triggerSeen(bodies[0]), bodies[0])
	}

	store2, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	pinA, ok := store2.Pin(tenantA, sharedSession)
	if !ok || pinA.Trigger != 1 || pinA.Keep != 1 {
		t.Fatalf("tenant acme-co must have its own freshly-decided pin: %+v %v", pinA, ok)
	}
	localPin, ok := store2.Pin(tenancy.LocalTenant, sharedSession)
	if !ok || localPin.Trigger != 999999 || localPin.Keep != 9 {
		t.Fatalf("LocalTenant's own pin must be untouched by tenant acme-co's request: %+v %v", localPin, ok)
	}
}
