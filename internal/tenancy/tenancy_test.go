package tenancy

import (
	"strings"
	"testing"
)

// Local/free compatibility (README's "no account, no telemetry" promise;
// ADR-0023's local binary posture, kept for the binary by ADR-0028
// amendment 2). Every local workflow today supplies no tenant identity at
// all. ResolveTenant must keep that working with zero configuration.
//
// PASS: no input resolves to LocalTenant with no error.
// FAIL: an error, or any tenant other than the fixed local default.
func TestTenancy_ResolveEmptyIsLocalTenant(t *testing.T) {
	got, err := ResolveTenant("")
	if err != nil {
		t.Fatalf("empty input must resolve without error, got %v", err)
	}
	if got != LocalTenant {
		t.Fatalf("empty input resolved to %q, want the fixed local tenant %q", got, LocalTenant)
	}
}

// Invalid identity handling. An explicit but malformed tenant string must be
// refused, not silently repaired or substituted.
//
// PASS: each case returns TenantUnknown and a non-nil error.
// FAIL: any case is accepted, or resolves to a tenant other than
// TenantUnknown.
func TestTenancy_ResolveInvalidIsRefused(t *testing.T) {
	cases := []string{
		" ",                 // whitespace only
		"has space",         // disallowed character
		"has\tcontrol\x00",  // control / NUL byte
		"-leading-dash",     // must start alphanumeric
		stringOfLength(200), // over the length bound
	}
	for _, raw := range cases {
		got, err := ResolveTenant(raw)
		if err == nil {
			t.Errorf("ResolveTenant(%q) was accepted; it should be refused", raw)
		}
		if got != TenantUnknown {
			t.Errorf("ResolveTenant(%q) = %q on failure, want TenantUnknown", raw, got)
		}
	}
}

// Missing identity: distinguished from invalid identity above. Absence of
// input is the one case that must NOT be refused (it is the local/free
// default); everything else that fails validation must be.
func TestTenancy_ResolveValidIsAccepted(t *testing.T) {
	got, err := ResolveTenant("acme-co")
	if err != nil {
		t.Fatalf("a well-formed tenant id must be accepted, got %v", err)
	}
	if got != TenantID("acme-co") {
		t.Fatalf("got %q, want %q", got, "acme-co")
	}
}

// Reserved sentinels cannot be forged. A caller who happens to spell their
// tenant "LOCAL" or "TENANT_UNKNOWN" must be refused outright rather than
// silently aliasing the real local default or the real unresolved sentinel.
// Without this, a malicious or merely coincidental input could make its
// records indistinguishable from the zero-configuration default's, or from
// an unresolved request's.
func TestTenancy_ReservedSentinelsCannotBeForged(t *testing.T) {
	for _, raw := range []string{string(LocalTenant), string(TenantUnknown)} {
		got, err := ResolveTenant(raw)
		if err == nil {
			t.Errorf("ResolveTenant(%q) must be refused as a reserved word", raw)
		}
		if got != TenantUnknown {
			t.Errorf("ResolveTenant(%q) = %q, want TenantUnknown on refusal", raw, got)
		}
	}
}

// AccountID has no zero-configuration default: local/free has no account at
// all, which is the point. Unlike ResolveTenant, there is no "resolve empty
// to a default" path for accounts.
func TestTenancy_AccountHasNoLocalDefault(t *testing.T) {
	if err := ValidateAccountID(""); err == nil {
		t.Fatal("an empty account id must be refused; there is no default account")
	}
	if err := ValidateAccountID(string(AccountUnknown)); err == nil {
		t.Fatal("the account-unknown sentinel must be refused as a reserved word")
	}
	if err := ValidateAccountID("acme-co-billing"); err != nil {
		t.Fatalf("a well-formed account id must validate, got %v", err)
	}
}

// Account/tenant distinction is a type distinction, not just a naming
// convention: these are different Go types, so the compiler refuses to
// confuse them. This test exists to document and pin that property; if
// TenantID and AccountID were ever collapsed into the same underlying type
// without a wrapper, this file would need to change, which is itself the
// signal.
func TestTenancy_AccountAndTenantAreDistinctTypes(t *testing.T) {
	var tid TenantID = "acme-co"
	var aid AccountID = "acme-co"
	// Both hold the same text but are not comparable across types; the line
	// below would not compile if uncommented, which is the point:
	//   if tid == aid { }
	if string(tid) != string(aid) {
		t.Fatal("sanity check failed: both were constructed from the same literal")
	}
}

func stringOfLength(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// ValidateTenantID's empty-string check is its own guard, distinct from the
// pattern check below it: idPattern also refuses "" (it requires at least
// one character), so a test that only checked err != nil could not tell the
// two apart. This asserts the specific message the empty-string branch
// produces, which the pattern branch does not.
//
// PASS: an empty raw value is refused with "tenant id is empty", not the
// pattern message.
// FAIL: the empty-string branch is removed and the pattern branch's message
// ("not a legal identity") is returned instead, undetected.
func TestTenancy_ValidateTenantIDRefusesEmptyByName(t *testing.T) {
	err := ValidateTenantID("")
	if err == nil {
		t.Fatal("empty tenant id must be refused")
	}
	if err.Error() != "tenant id is empty" {
		t.Fatalf("want the empty-string branch's own message %q, got %q", "tenant id is empty", err.Error())
	}
}

// ValidateAccountID's pattern check refuses an account id that is non-empty,
// not reserved, and still illegal (contains a character idPattern excludes).
// Only this branch, not the empty-string or reserved checks above it, can
// produce this refusal.
//
// PASS: "has space" is refused with the pattern message naming it illegal.
// FAIL: the pattern branch is removed and the value is silently accepted.
func TestTenancy_ValidateAccountIDRefusesIllegalCharacters(t *testing.T) {
	err := ValidateAccountID("has space")
	if err == nil {
		t.Fatal("an account id containing a space must be refused")
	}
	if !strings.Contains(err.Error(), "is not a legal identity") {
		t.Fatalf("want the pattern branch's message naming it illegal, got %q", err.Error())
	}
}

// ValidateAccountID's empty-string check is its own guard, the same way
// ValidateTenantID's is (TestTenancy_ValidateTenantIDRefusesEmptyByName,
// above): idPattern also refuses "" on its own (the pattern requires at
// least one character), so TestTenancy_AccountHasNoLocalDefault's plain
// err == nil check cannot tell the empty-string branch apart from the
// pattern branch firing on the same input. Guard reachability flagged
// exactly this (tenancy.go:140) as run but not depended on by any test.
// This asserts the specific message only the empty-string branch produces.
//
// PASS: an empty raw value is refused with "account id is empty", not the
// pattern message.
// FAIL: the empty-string branch is removed and the pattern branch's
// message ("not a legal identity") is returned instead, undetected.
func TestTenancy_ValidateAccountIDRefusesEmptyByName(t *testing.T) {
	err := ValidateAccountID("")
	if err == nil {
		t.Fatal("empty account id must be refused")
	}
	if err.Error() != "account id is empty" {
		t.Fatalf("want the empty-string branch's own message %q, got %q", "account id is empty", err.Error())
	}
}
