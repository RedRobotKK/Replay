// Package tenancy defines the identity and ownership primitives ADR-0028's
// hosted service names and ADR-0015 (as amended by ADR-0028) requires to
// exist before any commercial layer: a tenant dimension in the spend guard,
// the session table, the metrics surface and the credential path.
//
// This package started as that dimension's shape, not its wiring, and
// docs/design/UNWIRED-LOG.md's entry 18 records both states: added
// 2026-10-07 reachable from nothing, then wired the same day. It is now
// imported by cmd/replay (learn.go, simulate.go), internal/proxy
// (passthrough.go resolves x-replay-tenant-id at the circuit breaker before
// anything is counted; guards.go's SpendGuard nests its session and day
// accumulators by tenant) and internal/ledger (store.go's Pin and policy
// pins are keyed by tenant and session together). internal/transcript does
// not import it and has no tenant dimension; RepositoryID there remains a
// local filesystem grouping, not a tenant key. The metrics listener
// (internal/proxy/metrics_listener.go) still refuses any non-loopback bind
// outright, so ADR-0015's "authenticated metrics" entry condition has
// nothing networked to authenticate yet. What exists here is the
// primitive the wired units consume: a validated, typed identity, a
// resolution rule that preserves today's zero-configuration local default,
// and an ownership store whose isolation is proven by mutation rather than
// assumed.
//
// Four identities this package is careful not to collapse into each other,
// because ADR-0028 names them as different things without defining them
// precisely, which is the gap this package closes:
//
//   - Tenant: the unit a budget, a setting or a piece of hosted state
//     belongs to (ADR-0015's phrase). The ownership boundary itself.
//   - Account: the hosted billing and signup relationship ADR-0028 permits
//     for the hosted surface. May hold many tenants later (one company,
//     many repositories); never the reverse.
//   - Session and Repository already exist elsewhere in this tree
//     (internal/transcript.RepositoryID is a local filesystem grouping)
//     and are not redefined or reused as a tenant key here: two different
//     tenants could have a repository directory of the same name.
//   - User, entitlement (ADR-0023's signed document) and billing customer
//     are named by ADR-0028's vocabulary but have no code here. They are
//     out of scope for this unit and are not stood up speculatively.
package tenancy

import (
	"errors"
	"fmt"
	"regexp"
)

// TenantID names the unit a budget, a setting or a piece of hosted state
// belongs to. It is not a session (ephemeral, process-local, already
// modelled elsewhere), not a repository
// (internal/transcript.RepositoryID, a local filesystem grouping two
// different tenants could equally produce), not a user (nobody is named
// here), and not an account (see AccountID): a tenant is the ownership
// boundary, independent of who pays for it or who is signed in as it.
type TenantID string

// AccountID names the hosted-service billing and signup relationship
// ADR-0028 permits for the hosted surface only. Deliberately a distinct Go
// type from TenantID, not a type alias of the same underlying string:
// ADR-0028's enterprise direction is one account holding many tenants,
// never the reverse, and collapsing the two now would need a breaking
// migration the day that happens. AccountID carries no balance, no plan and
// no payment method (TestTenancy_NoBillingShapedField enforces this on
// Account below); the unit of sale is explicitly undecided
// (ADR-0028, consequence 6), and this type does not pre-empt that decision.
type AccountID string

const (
	// TenantUnknown is returned when a tenant identity could not be
	// established: an explicit input failed validation or named a reserved
	// word. It is never a valid tenant, and Registry (ownership.go) refuses
	// it outright rather than treat it as any kind of shared bucket.
	// Spelled distinctly from transcript.RepositoryUnknown so the two
	// absences, measured by different packages for different reasons, are
	// never compared or printed as though they were the same sentinel.
	TenantUnknown TenantID = "TENANT_UNKNOWN"

	// LocalTenant is the fixed tenant every existing local, zero-identity
	// workflow already runs as. ResolveTenant("") returns this, not
	// TenantUnknown: a solo developer running `replay` today has exactly
	// one identity and has never had to say so, and that must not change
	// for this unit to be additive rather than a regression.
	LocalTenant TenantID = "LOCAL"

	// AccountUnknown is AccountID's equivalent sentinel. There is no
	// "local account" counterpart to LocalTenant: local/free has no
	// account at all, which is ADR-0023's promise as ADR-0028 keeps it for
	// the binary. AccountUnknown exists so a field that carries no account
	// can say so explicitly rather than reading as a zero value by
	// accident.
	AccountUnknown AccountID = "ACCOUNT_UNKNOWN"
)

// reservedTenantIDs are strings a caller-supplied tenant must never be
// allowed to equal. Without this, a request that happens to spell its
// tenant "LOCAL" or "TENANT_UNKNOWN" could silently alias the local default
// or the unresolved sentinel: exactly the identity confusion a multi-tenant
// surface cannot afford, and precisely the kind of thing an adversarial
// input would try first.
var reservedTenantIDs = map[TenantID]bool{
	TenantUnknown: true,
	LocalTenant:   true,
}

var reservedAccountIDs = map[AccountID]bool{
	AccountUnknown: true,
}

// The three ways an identity fails validation, as sentinels every branch of
// ValidateTenantID and ValidateAccountID wraps with %w. The error text still
// quotes the offending input, because a person debugging a flag or a config
// file needs to see it; the sentinels exist so that a caller which must NOT
// repeat that text (the proxy boundary, whose refusal reaches the ledger and
// the ledger's contract is counts and thresholds, never content) can classify
// the failure with errors.Is and never read the message. Classifying by
// substring would be the same leak with an extra step.
var (
	// ErrIdentityEmpty: the input was the empty string. ResolveTenant never
	// returns it (empty resolves to LocalTenant); the validators do.
	ErrIdentityEmpty = errors.New("is empty")
	// ErrIdentityReserved: the input spelled a sentinel (TenantUnknown,
	// LocalTenant, AccountUnknown) a caller may not claim as an identity.
	ErrIdentityReserved = errors.New("is reserved")
	// ErrIdentityIllegal: the input failed idPattern, whether by character
	// set, by length or by its first character.
	ErrIdentityIllegal = errors.New("is not a legal identity " +
		"(ASCII letters, digits, -._: only, 1-128 chars, first character alphanumeric)")
)

// idPattern bounds what an identity may contain: ASCII letters, digits, and
// a small separator set (- _ . :) wide enough for a future namespaced key
// ("org:repo") without this package changing shape. No case folding and no
// normalisation: matching transcript.repositoryIDFromPath's own stated
// reasoning, under-merging two differently-spelled identities as distinct
// is the safer error than silently merging them.
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// ValidateTenantID reports whether raw is a legal, non-reserved tenant
// identity supplied as fresh input. It does not resolve an empty string to
// LocalTenant; ResolveTenant does that. Use this when validating an
// explicit value that must not be defaulted.
func ValidateTenantID(raw string) error {
	if raw == "" {
		return fmt.Errorf("tenant id %w", ErrIdentityEmpty)
	}
	if reservedTenantIDs[TenantID(raw)] {
		return fmt.Errorf("tenant id %q %w", raw, ErrIdentityReserved)
	}
	if !idPattern.MatchString(raw) {
		return fmt.Errorf("tenant id %q %w", raw, ErrIdentityIllegal)
	}
	return nil
}

// ValidateAccountID is ValidateTenantID's counterpart for AccountID. Kept
// as its own function rather than a shared helper parameterised on the
// type, so the two validations can diverge the day ADR-0028's unit of sale
// gives AccountID a shape TenantID does not share.
func ValidateAccountID(raw string) error {
	if raw == "" {
		return fmt.Errorf("account id %w", ErrIdentityEmpty)
	}
	if reservedAccountIDs[AccountID(raw)] {
		return fmt.Errorf("account id %q %w", raw, ErrIdentityReserved)
	}
	if !idPattern.MatchString(raw) {
		return fmt.Errorf("account id %q %w", raw, ErrIdentityIllegal)
	}
	return nil
}

// ResolveTenant is the primitive SP-5 (docs/requirements.md) names: "tenant
// identity is resolved before any cap is consulted ... A request whose
// tenant cannot be resolved is refused, never pooled into a shared bucket."
// This unit does not wire a live request path to it; that is SP-5's
// remaining, separately gated half (resolution "at the proxy boundary").
// This establishes the resolution rule itself, so that wiring has one
// correct place to call rather than inventing its own.
//
// raw == "" resolves to LocalTenant, not an error: every local workflow
// today supplies no tenant identity, and that must keep working with zero
// configuration. Any other validation failure (a reserved word, an illegal
// character, an empty-after-whitespace string, an overlong string) resolves
// to TenantUnknown with a non-nil error, and the caller must treat that as
// a refusal, never substitute LocalTenant or any other tenant: silently
// substituting a default for an unresolved identity is the "pooled into a
// shared bucket" failure ADR-0015 exists to name.
func ResolveTenant(raw string) (TenantID, error) {
	if raw == "" {
		return LocalTenant, nil
	}
	if err := ValidateTenantID(raw); err != nil {
		return TenantUnknown, err
	}
	return TenantID(raw), nil
}
