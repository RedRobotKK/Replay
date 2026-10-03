//go:build mutation

package blackbox

import (
	"sort"
	"testing"
)

// TestBB_EverySurfaceHasABlackBoxSpec is the completeness guard: the inventory
// comes from dispatch(), the specs are hand-written, and the two must agree in
// both directions. A new subcommand with no black-box spec fails here, and so
// does a spec for a subcommand that no longer exists.
func TestBB_EverySurfaceHasABlackBoxSpec(t *testing.T) {
	var missing, stale []string
	seen := map[string]bool{}
	for _, s := range discoverSurfaces(t) {
		seen[s.Canon()] = true
		if _, ok := specs[s.Canon()]; !ok {
			missing = append(missing, s.Canon())
		}
	}
	for name := range specs {
		if !seen[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	for _, m := range missing {
		t.Errorf("surface %q is dispatched by the binary and has no black-box spec", m)
	}
	for _, s := range stale {
		t.Errorf("spec %q names a surface the binary no longer dispatches", s)
	}
}
