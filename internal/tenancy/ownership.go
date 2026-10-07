package tenancy

import (
	"fmt"
	"sync"
)

// Tenant is what owns a session, a repository association, a setting, a
// measurement or an entitlement in a hosted deployment, in the sense SP-5
// uses "unit a budget belongs to". Nothing in this tree today points a
// session, a repository, a measurement or an entitlement at a Tenant; this
// struct is the shape a future wiring (SP-5/SP-6 for spend, a hosted
// settings store for the rest) would attach to those records. It is not a
// retrofit of them, and it has no lifecycle of its own beyond resolution:
// a Tenant value springs into existence wherever ResolveTenant or a literal
// is used, is never persisted and never deleted by this package. A future
// hosted store defines creation and deletion; this unit defines what it
// would create and delete.
//
// AccountID is the optional hosted-billing relationship this tenant
// belongs to. Its zero value, the empty string, means "not bound to any
// account" - every tenant in local/free mode, by construction, since
// local/free has no account at all. AccountUnknown is a different value
// from the empty string on purpose: AccountUnknown means a resolution was
// attempted and failed, where the empty string means none was attempted.
type Tenant struct {
	ID        TenantID  `json:"tenant_id"`
	AccountID AccountID `json:"account_id,omitempty"`
}

// Account is the hosted billing and signup relationship itself: who can be
// charged, who signed up, in ADR-0028's vocabulary. Its lifecycle is
// entirely future - nothing in this repository creates, stores or looks one
// up. It is defined now, alongside Tenant, only so the two concepts are
// never represented by a single type standing in for both. It deliberately
// carries no plan, balance, payment method or billing-cycle field:
// ADR-0028 consequence 6 leaves the unit of sale undecided, and giving
// Account a billing shape here would decide it by accident.
// TestTenancy_NoBillingShapedField enforces the absence.
type Account struct {
	ID AccountID `json:"account_id"`
}

// scopedKey composes a tenant and a caller key into one string a tenant can
// never collide out of. Length-prefixing the tenant defeats any separator
// the caller's own key might contain: there is no byte sequence a key can
// hold that makes two different tenants produce the same scoped key,
// because the boundary is fixed by a count read before it, not by a
// character the key could also contain.
func scopedKey(tenant TenantID, key string) string {
	return fmt.Sprintf("%d:%s:%s", len(string(tenant)), tenant, key)
}

// Registry is a tenant-scoped store: the demonstration that ADR-0015's
// isolation rule is enforceable as a data-structure property, not only as a
// request-time check that is easy to skip once and hard to notice skipping.
// SP-6 names the live surfaces this same discipline must eventually reach
// (the spend guard's accumulators, the session table); Registry is not
// those surfaces, it is the contract they will be held to.
type Registry struct {
	mu   sync.Mutex
	data map[string]any
}

// NewRegistry returns an empty tenant-scoped registry.
func NewRegistry() *Registry {
	return &Registry{data: map[string]any{}}
}

// Put stores value under key, scoped to tenant. It refuses TenantUnknown
// and the empty TenantID outright: an unresolved identity must never reach
// stored state, which is the "missing identity reaching protected state"
// failure this method exists to make impossible rather than merely
// unlikely.
func (r *Registry) Put(tenant TenantID, key string, value any) error {
	if tenant == TenantUnknown || tenant == "" {
		return fmt.Errorf("refusing to store a value under an unresolved tenant (key %q)", key)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[scopedKey(tenant, key)] = value
	return nil
}

// Get retrieves the value stored under key for tenant. It never returns a
// value stored under a different tenant, even when that other tenant is
// LocalTenant and even when the two tenants' keys are byte-identical: the
// tenant is part of the lookup itself, not a filter applied after the fact.
// The bool return distinguishes "nothing is stored here" from "something is
// stored, under a different tenant", the same way a missing map key is
// distinguished from a present one elsewhere in this codebase (ADR-0018:
// absence, zero and unknown are three values).
func (r *Registry) Get(tenant TenantID, key string) (any, bool, error) {
	if tenant == TenantUnknown || tenant == "" {
		return nil, false, fmt.Errorf("refusing to read a value for an unresolved tenant (key %q)", key)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.data[scopedKey(tenant, key)]
	return v, ok, nil
}
