//go:build mutation

package blackbox

import (
	"go/token"
	"testing"
)

// The black-box gate holds each surface to the frozen mutants its E2E test
// kills, found by that test's name. The name must be the one the wiring gate
// requires and one Go can declare, or a hyphenated surface is held to nothing.
func TestBB_NamesAHyphenatedSurfacesE2E(t *testing.T) {
	cases := map[string]string{
		"outcome":        "TestE2E_Outcome",
		"outcome-record": "TestE2E_OutcomeRecord",
		"a-b-c":          "TestE2E_ABC",
		"--version":      "TestE2E_Version",
	}
	for name, want := range cases {
		got := surface{Names: []string{name}}.E2E()
		if got != want {
			t.Errorf("%q: E2E %q, want %q", name, got, want)
		}
		if !token.IsIdentifier(got) {
			t.Errorf("%q: %q is not a Go identifier", name, got)
		}
	}
}
