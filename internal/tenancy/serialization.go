package tenancy

import (
	"encoding/json"
	"fmt"
)

// UnmarshalJSON refuses a document whose owner cannot be trusted, rather
// than accepting whatever survives decoding. A tenant_id that is missing,
// empty, the unresolved sentinel, or fails the same character and length
// rule fresh input is held to is corrupted ownership, not an ordinary zero
// value: a document that lost its owner must say so by failing, not by
// quietly becoming nobody's.
//
// This is deliberately looser than ValidateTenantID on one point: the
// reserved word LocalTenant ("LOCAL") IS accepted here, because a Tenant
// legitimately carrying the local/free default is a real, already-produced
// value (ResolveTenant("") returns exactly this), and it must be able to
// round-trip. ValidateTenantID's reservation exists to stop a caller from
// forging that default through fresh, unresolved input; it is not a ban on
// the value ever being carried once it legitimately exists. TenantUnknown
// is never legitimate to carry, in either direction, and remains refused
// here.
func (t *Tenant) UnmarshalJSON(b []byte) error {
	var wire struct {
		ID        TenantID  `json:"tenant_id"`
		AccountID AccountID `json:"account_id,omitempty"`
	}
	if err := json.Unmarshal(b, &wire); err != nil {
		return err
	}
	if wire.ID == "" || wire.ID == TenantUnknown || !idPattern.MatchString(string(wire.ID)) {
		return fmt.Errorf("tenant_id %q is not a carryable identity", wire.ID)
	}
	if wire.AccountID != "" {
		if wire.AccountID == AccountUnknown || !idPattern.MatchString(string(wire.AccountID)) {
			return fmt.Errorf("account_id %q is not a carryable identity", wire.AccountID)
		}
	}
	t.ID = wire.ID
	t.AccountID = wire.AccountID
	return nil
}
