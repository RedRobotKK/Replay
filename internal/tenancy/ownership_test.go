package tenancy

import "testing"

// Ownership and isolation: the mutation-critical branches named in the
// build plan (tenant swap, missing filter, ownership bypass, missing
// identity reaching protected state, cross-tenant fallback, disabled
// check) are each pinned by one of the cases below.

// Baseline: a tenant can read back what it stored.
func TestTenancy_RegistryRoundTrip(t *testing.T) {
	r := NewRegistry()
	if err := r.Put(LocalTenant, "setting", "value-A"); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	got, ok, err := r.Get(LocalTenant, "setting")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok {
		t.Fatal("stored value was not found under the same tenant")
	}
	if got != "value-A" {
		t.Fatalf("got %v, want value-A", got)
	}
}

// Tenant swap / cross-tenant fallback: two tenants, the same key. Neither
// may read the other's value, and a tenant storing nothing under a key must
// not see the other's value either (no fallback to "the only entry with
// this key").
//
// This is the test a "tenant swap" or "missing filter" mutant (dropping the
// tenant from the lookup key, or ignoring it) must fail.
func TestTenancy_RegistryNeverCrossesTenants(t *testing.T) {
	r := NewRegistry()
	const tenantA, tenantB TenantID = "acme-co", "widget-co"

	if err := r.Put(tenantA, "budget", 100); err != nil {
		t.Fatalf("Put(A) failed: %v", err)
	}

	if _, ok, err := r.Get(tenantB, "budget"); err != nil {
		t.Fatalf("Get(B) returned an error rather than a clean miss: %v", err)
	} else if ok {
		t.Fatal("tenant B read tenant A's value under the same key: isolation failed")
	}

	if err := r.Put(tenantB, "budget", 200); err != nil {
		t.Fatalf("Put(B) failed: %v", err)
	}
	gotA, okA, err := r.Get(tenantA, "budget")
	if err != nil || !okA || gotA != 100 {
		t.Fatalf("tenant A's own value changed after tenant B wrote the same key: got=%v ok=%v err=%v", gotA, okA, err)
	}
	gotB, okB, err := r.Get(tenantB, "budget")
	if err != nil || !okB || gotB != 200 {
		t.Fatalf("tenant B could not read back its own value: got=%v ok=%v err=%v", gotB, okB, err)
	}
}

// Account-as-tenant confusion, structurally: an AccountID is never accepted
// where a TenantID is required. This cannot even be expressed, let alone
// compiled, which is a stronger guarantee than a runtime check. Documented
// here so the property has a named test rather than existing only as an
// absence of a compile error nobody wrote down.
func TestTenancy_RegistryRequiresATenantIDNotAnAccountID(t *testing.T) {
	r := NewRegistry()
	var acct AccountID = "acme-co-billing"
	// r.Put(acct, "x", 1) would not compile: Put takes a TenantID. Converting
	// explicitly is exactly the forgery this type separation exists to
	// require someone to write out loud before it could ever do harm.
	if err := r.Put(TenantID(acct), "x", 1); err != nil {
		t.Fatalf("an explicit conversion is legal Go and must still behave like any other tenant id: %v", err)
	}
}

// Missing identity reaching protected state: TenantUnknown, and the empty
// TenantID, must never be accepted by Put or Get. This is the "disabled
// check" mutant's target: removing this guard would let an unresolved
// request silently read or write a shared bucket, which is the exact
// failure ADR-0015 names.
func TestTenancy_RegistryRefusesUnknownTenant(t *testing.T) {
	r := NewRegistry()
	if err := r.Put(LocalTenant, "k", "v"); err != nil {
		t.Fatalf("setup Put failed: %v", err)
	}

	if err := r.Put(TenantUnknown, "k", "forged"); err == nil {
		t.Fatal("Put accepted TenantUnknown; an unresolved identity must never reach stored state")
	}
	if err := r.Put("", "k", "forged"); err == nil {
		t.Fatal("Put accepted an empty TenantID; an unresolved identity must never reach stored state")
	}
	if _, ok, err := r.Get(TenantUnknown, "k"); err == nil || ok {
		t.Fatalf("Get served a value for TenantUnknown: ok=%v err=%v", ok, err)
	}
	if _, ok, err := r.Get("", "k"); err == nil || ok {
		t.Fatalf("Get served a value for the empty tenant: ok=%v err=%v", ok, err)
	}

	// And it must not have corrupted or merged into LocalTenant's own entry.
	got, ok, err := r.Get(LocalTenant, "k")
	if err != nil || !ok || got != "v" {
		t.Fatalf("LocalTenant's legitimate entry was disturbed: got=%v ok=%v err=%v", got, ok, err)
	}
}

// Cross-tenant fallback to the local default specifically: a tenant that
// never stored a key must not read LocalTenant's value for that same key,
// even though LocalTenant is itself a perfectly valid, frequently-used
// tenant. "Fall back to the default bucket on a miss" is a plausible
// shortcut to write by accident, and it is exactly ADR-0015's failure mode
// if it ever reached a live, shared surface: a tenant with no data of its
// own would transparently see someone else's.
func TestTenancy_RegistryNeverFallsBackToLocalTenant(t *testing.T) {
	r := NewRegistry()
	const other TenantID = "widget-co"

	if err := r.Put(LocalTenant, "shared-looking-key", "local's value"); err != nil {
		t.Fatalf("Put(LocalTenant) failed: %v", err)
	}

	if _, ok, err := r.Get(other, "shared-looking-key"); err != nil {
		t.Fatalf("Get(other) returned an error rather than a clean miss: %v", err)
	} else if ok {
		t.Fatal("a tenant with no entry of its own read LocalTenant's value for the same key")
	}
}

// A miss (key never stored) under a valid tenant must be a clean "not
// found", not an error. Needed so TestTenancy_RegistryRefusesUnknownTenant's
// "err == nil || ok" checks above are actually discriminating a refusal from
// an ordinary miss, rather than both looking the same.
func TestTenancy_RegistryOrdinaryMissIsNotAnError(t *testing.T) {
	r := NewRegistry()
	_, ok, err := r.Get(LocalTenant, "never-stored")
	if err != nil {
		t.Fatalf("an ordinary miss under a valid tenant must not error: %v", err)
	}
	if ok {
		t.Fatal("a key that was never stored must not be found")
	}
}
