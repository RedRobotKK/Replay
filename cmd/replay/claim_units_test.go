package main

import (
	"reflect"
	"strings"
	"testing"
)

// RPL-C021. Replay offers no cross-surface grand total.
//
// The reason is a unit error, not a feature gap: Ollama's "total" excludes a
// prefix already resident in the KV cache, Codex's write counter is zero on
// every record for client-side reasons, and Grok's USD tick scale is
// undocumented. Adding them produces a number with no unit.
//
// This is enforced by the shape of the type rather than by a convention, so
// the test is structural: no exported path may offer the addition.
func TestC021_NoCrossSurfaceTotalIsReachable(t *testing.T) {
	typ := reflect.TypeOf(surfaceBurn{})

	// POSITIVE CONTROL: the type must actually have been found, or the
	// absence check below is vacuous.
	if typ.NumField() == 0 {
		t.Fatal("surfaceBurn has no fields; reflection found the wrong type and the " +
			"assertions below would pass against anything")
	}

	banned := []string{"grandtotal", "alltotal", "combined", "sumall", "overall"}
	for i := 0; i < typ.NumField(); i++ {
		name := strings.ToLower(typ.Field(i).Name)
		for _, b := range banned {
			if strings.Contains(name, b) {
				t.Errorf("surfaceBurn.%s looks like a cross-surface total. Figures from "+
					"these surfaces are not commensurable: Ollama omits the resident "+
					"prefix from its total, Codex's write counter is a client-side zero, "+
					"and Grok's USD tick scale is undocumented.", typ.Field(i).Name)
			}
		}
	}
	for i := 0; i < reflect.PointerTo(typ).NumMethod(); i++ {
		name := strings.ToLower(reflect.PointerTo(typ).Method(i).Name)
		for _, b := range banned {
			if strings.Contains(name, b) {
				t.Errorf("surfaceBurn has method %s, which offers a cross-surface total",
					reflect.PointerTo(typ).Method(i).Name)
			}
		}
	}

	// The per-surface applicability flags must exist, because they are what
	// keeps "not applicable to this surface" distinguishable from "zero".
	var foundFlag bool
	for i := 0; i < typ.NumField(); i++ {
		if strings.HasPrefix(typ.Field(i).Name, "has") {
			foundFlag = true
		}
	}
	if !foundFlag {
		t.Error("surfaceBurn carries no has* applicability flag. Without one, a " +
			"surface with no cache concept reports 0% cached, which is a false zero.")
	}
}
