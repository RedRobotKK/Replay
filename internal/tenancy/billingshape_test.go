package tenancy

import (
	"reflect"
	"regexp"
	"testing"
)

// Scope-discipline guard: neither Tenant nor Account may carry a
// billing-shaped field. ADR-0028 consequence 6 leaves the unit of sale
// undecided, and a balance, price, plan or payment-method field here would
// decide it by accident, exactly the premature entitlement/payment coupling
// the billing seat objects to.
//
// Built with a positive control, in this package's own style: a planted
// billing-shaped struct must be caught, or a clean result proves nothing.
func TestTenancy_NoBillingShapedField(t *testing.T) {
	billingShaped := regexp.MustCompile(`(?i)^(balance|price|plan|payment|subscription|card|currency|amount|invoice)`)

	check := func(v any) []string {
		var hits []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			name := rt.Field(i).Name
			if billingShaped.MatchString(name) {
				hits = append(hits, rt.Name()+"."+name)
			}
		}
		return hits
	}

	for _, hit := range check(Tenant{}) {
		t.Errorf("Tenant carries a billing-shaped field: %s", hit)
	}
	for _, hit := range check(Account{}) {
		t.Errorf("Account carries a billing-shaped field: %s", hit)
	}

	// Positive control.
	type planted struct{ PlanTier string }
	if len(check(planted{})) == 0 {
		t.Fatal("the billing-shape detector does not fire on a planted Plan field; a clean result above proves nothing")
	}
}
