package tenancy

import (
	"errors"
	"strings"
	"testing"
)

// R1 (PR #336 review): the validator's errors carry the raw input, quoted,
// so that a caller debugging a flag or a config file is told exactly what
// was wrong. The proxy boundary must not repeat that text to the ledger,
// and it must not parse the message to find out which branch fired either.
// Each branch therefore wraps one exported sentinel, so a caller can ask
// errors.Is and never look at the string.
//
// PASS: every branch of both validators, and ResolveTenant on top of them,
// is identifiable with errors.Is against exactly one sentinel.
// FAIL: a branch returns a bare fmt.Errorf a caller can only classify by
// substring, which is what the proxy was doing by pasting it whole.
func TestTenancy_ValidationErrorsCarryTheirClass(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"tenant empty", ValidateTenantID(""), ErrIdentityEmpty},
		{"tenant reserved local", ValidateTenantID(string(LocalTenant)), ErrIdentityReserved},
		{"tenant reserved unknown", ValidateTenantID(string(TenantUnknown)), ErrIdentityReserved},
		{"tenant illegal", ValidateTenantID("has space"), ErrIdentityIllegal},
		{"tenant overlong", ValidateTenantID(strings.Repeat("a", 129)), ErrIdentityIllegal},
		{"account empty", ValidateAccountID(""), ErrIdentityEmpty},
		{"account reserved", ValidateAccountID(string(AccountUnknown)), ErrIdentityReserved},
		{"account illegal", ValidateAccountID("has space"), ErrIdentityIllegal},
	}
	sentinels := []error{ErrIdentityEmpty, ErrIdentityReserved, ErrIdentityIllegal}
	for _, tc := range cases {
		if tc.err == nil {
			t.Fatalf("%s: want an error", tc.name)
		}
		for _, s := range sentinels {
			if errors.Is(tc.err, s) != (s == tc.want) {
				t.Errorf("%s: errors.Is(%v, %v) = %v, want %v", tc.name, tc.err, s, errors.Is(tc.err, s), s == tc.want)
			}
		}
	}

	if _, err := ResolveTenant(string(LocalTenant)); !errors.Is(err, ErrIdentityReserved) {
		t.Errorf("ResolveTenant must pass the reserved class through, got %v", err)
	}
	if _, err := ResolveTenant("a/b"); !errors.Is(err, ErrIdentityIllegal) {
		t.Errorf("ResolveTenant must pass the illegal class through, got %v", err)
	}
}

// The sentinels are additive: the messages callers and the existing tests
// already read are byte-for-byte what they were before the classes
// existed, so nothing that prints a validation error changes output.
func TestTenancy_ValidationMessagesAreUnchangedByTheClasses(t *testing.T) {
	cases := map[string]error{
		"tenant id is empty":            ValidateTenantID(""),
		`tenant id "LOCAL" is reserved`: ValidateTenantID("LOCAL"),
		`tenant id "has space" is not a legal identity (ASCII letters, digits, -._: only, 1-128 chars, first character alphanumeric)`: ValidateTenantID("has space"),
		"account id is empty":                      ValidateAccountID(""),
		`account id "ACCOUNT_UNKNOWN" is reserved`: ValidateAccountID("ACCOUNT_UNKNOWN"),
		`account id "has space" is not a legal identity (ASCII letters, digits, -._: only, 1-128 chars, first character alphanumeric)`: ValidateAccountID("has space"),
	}
	for want, err := range cases {
		if err == nil || err.Error() != want {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}
