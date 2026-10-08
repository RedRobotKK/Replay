package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/tenancy"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// SP-10 (docs/requirements.md): a persisted policy pin is owned by
// (tenant, sessionID), never by sessionID alone. This file is the
// ledger-level half of the acceptance tests - internal/proxy's
// sp10_tenancy_test.go is the HTTP-boundary half, crossing the real
// applyPolicy -> decidePolicy -> Store.Pin/SetPin path.

// TestSP10_LedgerPinsDoNotCollideAcrossTenants is Phase 1's required
// ledger-level test: tenantA+sessionX and tenantB+sessionX cannot collide
// inside the persisted store, in either direction.
//
// PASS: each tenant's SetPin only ever affects its own entry; Pin reads
// back exactly what that tenant itself wrote, never the other's.
// FAIL (206aee2): one shared map[string]Pin keyed by SessionID alone means
// the second SetPin call for "shared" overwrites the first tenant's entry
// outright.
func TestSP10_LedgerPinsDoNotCollideAcrossTenants(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	const shared = "shared-session"
	at := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)

	if err := store.SetPin(tenantA, Pin{SessionID: shared, Policy: "context-edit", Trigger: 1, Keep: 2, Decision: "applied", At: at}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPin(tenantB, Pin{SessionID: shared, Decision: "control", Trial: TrialControl, At: at}); err != nil {
		t.Fatal(err)
	}

	pa, ok := store.Pin(tenantA, shared)
	if !ok || pa.Decision != "applied" || pa.Trigger != 1 || pa.Keep != 2 {
		t.Fatalf("tenant A's pin was disturbed by tenant B's SetPin: %+v %v", pa, ok)
	}
	pb, ok := store.Pin(tenantB, shared)
	if !ok || pb.Decision != "control" || pb.Trial != TrialControl || pb.Policy != "" {
		t.Fatalf("tenant B's pin is wrong: %+v %v", pb, ok)
	}

	// The reverse order too: a tenant with no pin of its own must never
	// read any other tenant's.
	if _, ok := store.Pin(tenancy.TenantID("nobody"), shared); ok {
		t.Fatal("a tenant that never called SetPin for this session must have no pin")
	}

	// A different session id under tenant A must not pick up tenant B's
	// pin either - distinct (tenant, session) pairs are fully independent.
	if _, ok := store.Pin(tenantA, "a-different-session"); ok {
		t.Fatal("an unrelated session id must have no pin")
	}
}

// TestSP10_MarkControlDoesNotCrossTenants: the trial arm MarkControl
// restores onto a session must come from that session's own tenant's pin,
// never another tenant's.
func TestSP10_MarkControlDoesNotCrossTenants(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	const tenantA, tenantB tenancy.TenantID = "acme-co", "widget-co"
	at := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	// Tenant A's session was a control; tenant B's identically-named
	// session was never pinned at all.
	if err := store.SetPin(tenantA, Pin{SessionID: "s", Decision: "control", Trial: TrialControl, At: at}); err != nil {
		t.Fatal(err)
	}

	sessA := &transcript.Session{ID: "s"}
	store.MarkControl(tenantA, sessA)
	if sessA.Trial != TrialControl {
		t.Fatalf("tenant A's own control pin must mark its session: got %q", sessA.Trial)
	}

	sessB := &transcript.Session{ID: "s"}
	store.MarkControl(tenantB, sessB)
	if sessB.Trial != "" {
		t.Fatalf("CROSS-TENANT LEAK: tenant B's session was marked %q from tenant A's pin", sessB.Trial)
	}
}

// --- Phase 4: backward compatibility with a real pre-SP-10 .pins file. ---

// TestSP10_LegacyPinsFileLoadsAsLocalTenant exercises the REAL load path:
// a genuine pre-tenant-aware .pins file (no tenant_id field at all, the
// only shape any released version through 206aee2 ever wrote) sits on
// disk, and a real Store is opened over that directory.
//
// PASS: every legacy line becomes a LocalTenant entry; LocalTenant's
// session behaves exactly as it always did; no other tenant can read a
// legacy entry; a new tenant-scoped pin written afterwards for the same
// session id coexists without colliding with the legacy LocalTenant entry.
func TestSP10_LegacyPinsFileLoadsAsLocalTenant(t *testing.T) {
	dir := t.TempDir()
	// A real file, hand-written in exactly the shape every pre-SP-10
	// release produced: no tenant_id field.
	legacy := `{"session_id":"s1","policy":"context-edit","trigger":200000,"keep":6,"decision":"applied","at":"2026-01-01T00:00:00Z"}` + "\n" +
		`{"session_id":"s2","decision":"no policy configured","at":"2026-01-01T00:00:00Z"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, pinsFile), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	p1, ok := store.Pin(tenancy.LocalTenant, "s1")
	if !ok || p1.Decision != "applied" || p1.Trigger != 200000 || p1.Keep != 6 {
		t.Fatalf("legacy entry s1 must load as LocalTenant: %+v %v", p1, ok)
	}
	p2, ok := store.Pin(tenancy.LocalTenant, "s2")
	if !ok || p2.Policy != "" {
		t.Fatalf("legacy entry s2 (no policy) must also load as LocalTenant: %+v %v", p2, ok)
	}

	// No other tenant may read a legacy entry: a legacy session id must
	// not accidentally become another tenant's state.
	if _, ok := store.Pin(tenancy.TenantID("acme-co"), "s1"); ok {
		t.Fatal("a legacy pin must not be readable under any tenant other than LocalTenant")
	}

	// A brand-new tenant-scoped pin for the SAME session id must coexist
	// with the legacy LocalTenant entry without disturbing it.
	if err := store.SetPin(tenancy.TenantID("acme-co"), Pin{SessionID: "s1", Decision: "control", Trial: TrialControl}); err != nil {
		t.Fatal(err)
	}
	p1again, ok := store.Pin(tenancy.LocalTenant, "s1")
	if !ok || p1again.Decision != "applied" {
		t.Fatalf("the legacy LocalTenant entry must survive a new tenant's SetPin for the same session id: %+v %v", p1again, ok)
	}
	pAcme, ok := store.Pin(tenancy.TenantID("acme-co"), "s1")
	if !ok || pAcme.Decision != "control" {
		t.Fatalf("the new tenant-scoped pin: %+v %v", pAcme, ok)
	}

	// Reopen (a real restart) and confirm all of the above still holds
	// from a fresh load, not merely from the in-memory map already built
	// above.
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := reopened.Pin(tenancy.LocalTenant, "s1"); !ok || p.Decision != "applied" {
		t.Fatalf("legacy entry must survive a reload: %+v %v", p, ok)
	}
	if p, ok := reopened.Pin(tenancy.TenantID("acme-co"), "s1"); !ok || p.Decision != "control" {
		t.Fatalf("new tenant entry must survive a reload: %+v %v", p, ok)
	}
}

// TestSP10_MalformedTenantIDInPinsFileIsSkippedNotFatal: a tenant_id field
// that is present but fails tenancy's own validation (a reserved
// sentinel, or a string outside the legal identity grammar) must never be
// silently reinterpreted as LocalTenant or as any other tenant - the line
// is skipped on its own, the same "discard what cannot be trusted" rule
// loadPins already applies to a line that fails to parse at all.
func TestSP10_MalformedTenantIDInPinsFileIsSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	body := `{"session_id":"good","tenant_id":"acme-co","decision":"applied","at":"2026-01-01T00:00:00Z"}` + "\n" +
		// Reserved sentinel masquerading as a tenant id.
		`{"session_id":"bad1","tenant_id":"TENANT_UNKNOWN","decision":"applied","at":"2026-01-01T00:00:00Z"}` + "\n" +
		// Outside the legal identity grammar (space is not a legal character).
		`{"session_id":"bad2","tenant_id":"has space","decision":"applied","at":"2026-01-01T00:00:00Z"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, pinsFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := store.Pin(tenancy.TenantID("acme-co"), "good"); !ok || p.Decision != "applied" {
		t.Fatalf("the well-formed sibling entry must survive: %+v %v", p, ok)
	}
	// A malformed tenant_id must not land on LocalTenant...
	if _, ok := store.Pin(tenancy.LocalTenant, "bad1"); ok {
		t.Fatal("a corrupted tenant_id must not be reinterpreted as LocalTenant")
	}
	if _, ok := store.Pin(tenancy.LocalTenant, "bad2"); ok {
		t.Fatal("a corrupted tenant_id must not be reinterpreted as LocalTenant")
	}
	// ...nor anywhere else it could be queried from: not under the
	// reserved sentinel or the malformed string itself, and - the case
	// that actually distinguishes "skipped" from "fell through unskipped"
	// - not under the empty-string tenant key either. A guard that forced
	// persistedPinTenantKey's failure to continue past the `continue`
	// would store this pin with tenant="" (the zero value), which reads
	// as a LEGITIMATE bucket to Pin and SetPin (both are ordinary map
	// operations on an empty string key) unless something checks it.
	if _, ok := store.Pin(tenancy.TenantUnknown, "bad1"); ok {
		t.Fatal("a reserved sentinel must never become a live tenant bucket")
	}
	if _, ok := store.Pin(tenancy.TenantID("has space"), "bad2"); ok {
		t.Fatal("a malformed string must never become a live tenant bucket under its own spelling")
	}
	if _, ok := store.Pin(tenancy.TenantID(""), "bad1"); ok {
		t.Fatal("a malformed tenant_id must be skipped outright, not fall through and land under the empty-string tenant")
	}
	if _, ok := store.Pin(tenancy.TenantID(""), "bad2"); ok {
		t.Fatal("a malformed tenant_id must be skipped outright, not fall through and land under the empty-string tenant")
	}
}

// --- Phase 5: concurrency under -race. ---

// TestSP10_ConcurrentSetPinAcrossTenantsIsRaceFreeAndIsolated: concurrent
// SetPin calls for the same tenant/same session, different tenants/same
// session, and different tenants/different sessions must not race
// (go test -race) and must never let one tenant's write land under
// another tenant's key.
func TestSP10_ConcurrentSetPinAcrossTenantsIsRaceFreeAndIsolated(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	const n = 50
	var wg sync.WaitGroup

	// Same tenant, same session: concurrent writers, no lost-update
	// requirement beyond "the store does not panic or corrupt", since
	// production never calls SetPin twice for one (tenant,session) - the
	// first-pin-wins rule lives in decidePolicy, above this layer.
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			_ = store.SetPin(tenancy.TenantID("acme-co"), Pin{SessionID: "contended", Decision: "applied", Trigger: i})
		}(i)
	}

	// Different tenants, same session id: must never cross.
	tenants := []tenancy.TenantID{"acme-co", "widget-co", "umbrella-corp"}
	wg.Add(len(tenants))
	for _, ten := range tenants {
		go func(ten tenancy.TenantID) {
			defer wg.Done()
			_ = store.SetPin(ten, Pin{SessionID: "shared", Decision: string(ten)})
		}(ten)
	}

	// Different tenants, different sessions: each (tenant, session) pair
	// is unique, so a leak here would mean one goroutine's write landed
	// under a key it was never given.
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			ten := tenants[i%len(tenants)]
			_ = store.SetPin(ten, Pin{SessionID: fmt.Sprintf("distinct-%d", i), Decision: string(ten), Trigger: i})
		}(i)
	}
	wg.Wait()

	for _, ten := range tenants {
		p, ok := store.Pin(ten, "shared")
		if !ok {
			t.Fatalf("tenant %s lost its pin under concurrent writers", ten)
		}
		if p.Decision != string(ten) {
			t.Fatalf("CROSS-TENANT LEAK under concurrency: tenant %s's pin for session "+
				"\"shared\" reads decision=%q, which can only be another tenant's write", ten, p.Decision)
		}
	}
	if p, ok := store.Pin(tenancy.TenantID("acme-co"), "contended"); !ok || p.Decision != "applied" {
		t.Fatalf("the contended same-tenant pin must still be readable, uncorrupted: %+v %v", p, ok)
	}
	for i := 0; i < n; i++ {
		ten := tenants[i%len(tenants)]
		p, ok := store.Pin(ten, fmt.Sprintf("distinct-%d", i))
		if !ok || p.Decision != string(ten) || p.Trigger != i {
			t.Fatalf("distinct (tenant,session) pair %d corrupted or cross-wired: %+v %v", i, p, ok)
		}
	}
}

// TestSP10_ConcurrentSetPinSurvivesReload: the same concurrent writers as
// above, but read back from a FRESH Store over the same directory
// afterward (a restart), proving the append-only .pins file itself - not
// just the in-memory map - stayed isolated per tenant under concurrent
// writers.
func TestSP10_ConcurrentSetPinSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	tenants := []tenancy.TenantID{"acme-co", "widget-co", "umbrella-corp"}
	var wg sync.WaitGroup
	wg.Add(len(tenants))
	for _, ten := range tenants {
		go func(ten tenancy.TenantID) {
			defer wg.Done()
			_ = store.SetPin(ten, Pin{SessionID: "shared", Decision: string(ten)})
		}(ten)
	}
	wg.Wait()

	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ten := range tenants {
		p, ok := reopened.Pin(ten, "shared")
		if !ok || p.Decision != string(ten) {
			t.Fatalf("tenant %s's pin did not survive a reload intact: %+v %v", ten, p, ok)
		}
	}
}
