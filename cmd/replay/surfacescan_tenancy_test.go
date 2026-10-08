package main

import (
	"regexp"
	"testing"
)

// RPL-C019 / SP-10 adjudication (architecture/governance panel,
// 2026-10-07): RPL-C019 stands and SP-10 does not violate it; the
// TestXW6 detector itself was overbroad and needed to be narrowed
// semantically, not the claim weakened or TestXW6 deleted or neutered.
//
// RPL-C019 guards against a provider-account/billing identity being
// used as a correlation handle in the ledger/transcript/cost-report
// path. internal/tenancy.TenantID is not such an identity: commit
// 646736d ("tenancy: the identity primitive ADR-0028 names before the
// proxy that needs it") established TenantID and AccountID as
// deliberately distinct Go types, predating SP-10 and not written to
// rationalize it. TenantID is Replay's internal ownership/namespace
// partition for hosted multi-tenancy; SP-10 uses it in
// internal/ledger/store.go only to keep two Replay tenants' persisted
// policy pins from colliding when their client-chosen session ids
// happen to match - an ownership partition, not a provider-account
// correlation handle. AccountID remains unused in this path as a real
// provider/billing identity.
//
// referencesRegisteredTenancyPrimitive is accountShapedIdentityHits'
// (surfacescan_test.go) one semantic exception, for TenantID only: see
// its own doc comment below, after Phase 3's narrowing, for the actual
// rule. This file carries the regression tests that pin that rule down.

// TestXW6_InternalTenancyPrimitiveTenantIDIsNotFlagged is the RED/GREEN
// regression for the narrowing itself: a synthetic fragment using
// ADR-0028's registered internal/tenancy.TenantID primitive - the same
// shape SP-10 actually put into internal/ledger/store.go (import
// internal/tenancy, a map keyed by tenancy.TenantID, a TenantID field
// documented as the pin's owner) - must not be flagged as a
// provider-account identity merely because it is spelled "TenantID".
//
// RED (run against 206aee2's unmodified, purely lexical detector, before
// any semantic narrowing existed): FAILS. The lexical detector matches
// the bare word "TenantID" unconditionally, with no regard for whether
// the file imports internal/tenancy or redeclares the identifier itself.
// Captured RED evidence (go test -run TestXW6_InternalTenancyPrimitiveTenantIDIsNotFlagged -v):
//
//	--- FAIL: TestXW6_InternalTenancyPrimitiveTenantIDIsNotFlagged (0.00s)
//	    surfacescan_tenancy_test.go:NN: internal/tenancy's own registered TenantID primitive
//	    must not be flagged as a provider-account identity; got [TenantID TenantID]
func TestXW6_InternalTenancyPrimitiveTenantIDIsNotFlagged(t *testing.T) {
	src := `package ledger

import (
	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// Pin is owned by (tenant, SessionID): a persisted policy pin belongs
// to (tenant, SessionID), never to SessionID alone (SP-10).
type Pin struct {
	SessionID string ` + "`json:\"session_id\"`" + `
	TenantID  string ` + "`json:\"tenant_id,omitempty\"`" + `
}

// pins is nested by tenant, exactly like internal/proxy's stats.sessions
// and SpendGuard.dayUsed (SP-6, SP-9).
type Store struct {
	pins map[tenancy.TenantID]map[string]Pin
}
`
	if hits := accountShapedIdentityHits(src); len(hits) != 0 {
		t.Fatalf("internal/tenancy's own registered TenantID primitive must not be "+
			"flagged as a provider-account identity; got %v", hits)
	}
}

// TestXW6_GenuineProviderAccountIdentityIsStillFlagged is RPL-C019's
// positive control, restated here against accountShapedIdentityHits
// directly: a synthetic correlation-path fragment carrying a genuine
// provider-account/billing identity - AccountID, documented exactly as
// internal/tenancy.Account's own doc comment describes it, "the hosted
// billing and signup relationship itself: who can be charged, who signed
// up" - must still be caught. This must pass both before Phase 3's
// narrowing (the unmodified lexical detector already catches it) and
// after (the narrowing touches TenantID only, never AccountID): the
// detector is being made more precise, not weaker.
func TestXW6_GenuineProviderAccountIdentityIsStillFlagged(t *testing.T) {
	src := `package ledger

// AccountID is the hosted billing and signup relationship itself: who
// can be charged, who signed up (ADR-0028's internal/tenancy.Account).
// A Record carrying it would let two records from different provider
// accounts be correlated by this field - exactly what RPL-C019 forbids.
type Record struct {
	AccountID string
}
`
	if hits := accountShapedIdentityHits(src); len(hits) == 0 {
		t.Fatal("a genuine provider-account identity (AccountID) must still be flagged; " +
			"RPL-C019 exists to catch exactly this")
	}
}

// TestXW6_LocallyRedeclaredTenantIDIsStillFlagged: a file that never
// imports the registered internal/tenancy primitive at all, but declares
// its own local "TenantID" identifier, must still be flagged. The
// narrowing is not a license to ignore every field spelled "TenantID" -
// only the registered primitive is exempt, and a disguised
// provider-account identity wearing that name, with no connection to
// ADR-0028's actual package, is exactly the case this pins down.
func TestXW6_LocallyRedeclaredTenantIDIsStillFlagged(t *testing.T) {
	src := `package evilcorrelator

// A locally redeclared TenantID masquerading as Replay's internal
// primitive, but never importing internal/tenancy at all.
type TenantID string

type Record struct {
	TenantID TenantID
}
`
	if hits := accountShapedIdentityHits(src); len(hits) == 0 {
		t.Fatal("a locally redeclared TenantID that does not import the registered " +
			"internal/tenancy primitive must still be flagged")
	}
}

// TestXW6_RedeclaredTenantIDWithTenancyImportStillFlagged: a file that
// DOES import the registered internal/tenancy package but ALSO declares
// its own, different "TenantID" type must still have that locally
// redeclared identifier flagged. Importing the real primitive package
// does not retroactively launder an unrelated, locally redeclared
// TenantID in the same file - the two must not be conflated, or a
// disguised provider-account identity could hide behind an incidental
// import of the genuine package.
func TestXW6_RedeclaredTenantIDWithTenancyImportStillFlagged(t *testing.T) {
	src := `package shady

import (
	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// This file imports the registered primitive package, but ALSO
// redeclares its own, different TenantID type.
type TenantID string

type Record struct {
	Real   tenancy.TenantID
	Shadow TenantID
}
`
	if hits := accountShapedIdentityHits(src); len(hits) == 0 {
		t.Fatal("a file that redeclares its own TenantID type, even while importing " +
			"internal/tenancy, must still be flagged: the import alone does not prove " +
			"every TenantID in the file is the registered primitive")
	}
}

// TestXW6_GenuineAccountIDInTenancyImportingFileIsStillFlagged: a file
// that legitimately imports the registered internal/tenancy package for
// its TenantID usage (exactly SP-10's own shape in
// internal/ledger/store.go) but ALSO carries a genuine AccountID field
// must still have that AccountID flagged. The TenantID exception is
// narrow by construction - it names "TenantID" specifically, nothing
// else - but this pins the boundary down directly: legitimately
// importing internal/tenancy must never become a blanket exemption for
// every account-shaped identifier in the same file, or a real
// provider-account identity could be smuggled in behind it.
func TestXW6_GenuineAccountIDInTenancyImportingFileIsStillFlagged(t *testing.T) {
	src := `package ledger

import (
	"github.com/RedRobotKK/Replay/internal/tenancy"
)

type Pin struct {
	SessionID string
	TenantID  string
	// AccountID does not belong here; it is planted to prove the
	// exception stays scoped to TenantID even in a file that
	// legitimately uses the registered primitive.
	AccountID string
}

type Store struct {
	pins map[tenancy.TenantID]map[string]Pin
}
`
	hits := accountShapedIdentityHits(src)
	found := false
	for _, h := range hits {
		if h == "AccountID" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a genuine AccountID in a file that legitimately imports internal/tenancy "+
			"for TenantID must still be flagged; got hits=%v", hits)
	}
}

// tenancyPackageImport and localTenantIDRedeclaration are the two facts
// referencesRegisteredTenancyPrimitive needs. tenancyPackageImport is
// whether the file pulls in ADR-0028's registered tenancy primitive
// (internal/tenancy, commit 646736d - "tenancy: the identity primitive
// ADR-0028 names before the proxy that needs it", which predates SP-10
// and established TenantID and AccountID as deliberately distinct Go
// types) at all. localTenantIDRedeclaration is whether the file ALSO
// declares its own, different "TenantID" identifier - which would mean
// the import's presence proves nothing about any particular TenantID
// occurrence in that file, since a disguised provider-account identity
// could otherwise be laundered behind an unrelated import of the real
// primitive (TestXW6_RedeclaredTenantIDWithTenancyImportStillFlagged
// pins this down).
var (
	tenancyPackageImport       = regexp.MustCompile(`"github\.com/RedRobotKK/Replay/internal/tenancy"`)
	localTenantIDRedeclaration = regexp.MustCompile(`\btype\s+TenantID\b`)
)

// referencesRegisteredTenancyPrimitive reports whether src's own
// TenantID occurrences are attributable to ADR-0028's registered
// internal/tenancy.TenantID rather than to a provider-account identity
// merely spelled "TenantID".
//
// RPL-C019 guards against a provider-account/billing identity being used
// as a correlation handle; TenantID is not such an identity (the
// architecture/governance panel's 2026-10-07 adjudication on SP-10: RPL-
// C019 stands, SP-10 does not violate it, and this detector - not the
// claim - was overbroad). internal/tenancy.TenantID and
// internal/tenancy.AccountID are deliberately distinct Go types,
// established by commit 646736d, which predates SP-10 and was not
// written to rationalize it. TenantID is Replay's internal
// ownership/namespace partition for hosted multi-tenancy; in
// internal/ledger/store.go it exists only to keep two Replay tenants'
// persisted policy pins from colliding when their client-chosen session
// ids happen to match (SP-10) - an ownership partition, not a
// provider-account correlation handle. AccountID remains unused in this
// path as a real provider/billing identity, and this exception never
// touches AccountID, OrgID, OrganizationID, OrganisationID, ProjectID or
// WorkspaceID: accountShapedIdentityHits still reports every one of
// those unconditionally.
//
// A file qualifies for the exception only when BOTH hold: it imports the
// registered internal/tenancy package, AND it does not also declare its
// own, different TenantID type. The second condition keeps the exception
// semantic rather than cosmetic - an incidental or decoy import of the
// real package must not retroactively launder an unrelated, locally
// redeclared TenantID identifier in the same file
// (TestXW6_RedeclaredTenantIDWithTenancyImportStillFlagged). A file that
// never imports the package at all gets no exception regardless of what
// it names its own fields (TestXW6_LocallyRedeclaredTenantIDIsStillFlagged).
func referencesRegisteredTenancyPrimitive(src string) bool {
	return tenancyPackageImport.MatchString(src) && !localTenantIDRedeclaration.MatchString(src)
}
