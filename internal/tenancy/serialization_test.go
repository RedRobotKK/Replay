package tenancy

import (
	"encoding/json"
	"strings"
	"testing"
)

// A Tenant round-trips through JSON, carrying its identity as an explicit
// field rather than one reconstructed positionally or inferred from
// context. This is the privacy-seat requirement: a future aggregate record
// cannot have its ownership silently reassigned by editing a document,
// because the owner is read back and validated, not trusted.
func TestTenancy_SerializationRoundTrips(t *testing.T) {
	want := Tenant{ID: "acme-co", AccountID: "acme-co-billing"}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var got Tenant
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if got != want {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, want)
	}
}

// A Tenant with no account bound (the local/free shape: a tenant exists,
// but no hosted account owns it) also round-trips, and the account field
// comes back as the empty AccountID, not AccountUnknown: the two mean
// different things (AccountUnknown is a failed resolution; the empty value
// here means "no account was ever involved").
func TestTenancy_SerializationRoundTripsWithNoAccount(t *testing.T) {
	want := Tenant{ID: LocalTenant}
	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var got Tenant
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if got.ID != LocalTenant {
		t.Fatalf("got ID %q, want %q", got.ID, LocalTenant)
	}
	if got.AccountID != "" {
		t.Fatalf("got AccountID %q, want empty", got.AccountID)
	}
}

// Corrupted ownership: a document whose tenant_id has been edited to the
// unresolved sentinel, blanked, or mangled must be refused on read, not
// silently accepted as though TenantUnknown or "" were an ordinary tenant.
func TestTenancy_SerializationRefusesCorruptedOwner(t *testing.T) {
	cases := []string{
		`{"tenant_id":""}`,
		`{"tenant_id":"TENANT_UNKNOWN"}`,
		`{}`,
		`{"tenant_id":"has space"}`,
	}
	for _, doc := range cases {
		var got Tenant
		if err := json.Unmarshal([]byte(doc), &got); err == nil {
			t.Errorf("Unmarshal(%s) succeeded; a corrupted or missing owner must be refused", doc)
		}
	}
}

// A corrupted account_id (present but invalid) must also be refused, even
// when the tenant_id itself is fine, so a document cannot claim a mangled
// billing relationship by surviving on its tenant half alone.
func TestTenancy_SerializationRefusesCorruptedAccount(t *testing.T) {
	var got Tenant
	doc := `{"tenant_id":"acme-co","account_id":"ACCOUNT_UNKNOWN"}`
	if err := json.Unmarshal([]byte(doc), &got); err == nil {
		t.Errorf("Unmarshal(%s) succeeded; a reserved account sentinel must be refused when carried", doc)
	}
}

// A document that is not well-formed JSON at all (top-level syntax error)
// never reaches UnmarshalJSON: encoding/json validates the whole input
// before dispatching to a custom unmarshaler, so this case is refused by
// the standard library's own scanner, not by this method's wire-decode
// check. The wire-decode check at the top of UnmarshalJSON is reached only
// when the input is syntactically valid JSON but the wrong shape for the
// wire struct, such as a number where tenant_id must be a string.
//
// PASS: a syntactically-invalid document is refused, and a syntactically
// valid document with a type-mismatched tenant_id is refused by the
// wire-decode error specifically.
// FAIL: either case is accepted, or the wire-decode error is swallowed.
func TestTenancy_SerializationRefusesMalformedJSON(t *testing.T) {
	var got Tenant
	if err := json.Unmarshal([]byte(`{"tenant_id":`), &got); err == nil {
		t.Error(`Unmarshal({"tenant_id":) succeeded; a top-level syntax error must be refused`)
	}
	got = Tenant{}
	err := json.Unmarshal([]byte(`{"tenant_id":123}`), &got)
	if err == nil {
		t.Fatal(`Unmarshal({"tenant_id":123}) succeeded; a non-string tenant_id must be refused`)
	}
	if !strings.Contains(err.Error(), "cannot unmarshal number") {
		t.Errorf("want the wire-decode error naming the type mismatch, got %q", err.Error())
	}
}
