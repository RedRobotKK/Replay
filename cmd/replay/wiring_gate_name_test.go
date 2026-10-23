package main

import (
	"go/token"
	"testing"
)

// The E2E test a surface must have is named from the surface, and the name
// has to be one Go can declare: a hyphenated command whose name kept its
// hyphen would require a test nobody can write, so the gate could never pass
// and would never be satisfied by mistake either.
func TestWiringGateNamesAHyphenatedSurfacesTest(t *testing.T) {
	cases := map[string]string{
		"outcome":         "TestE2E_Outcome",
		"cost":            "TestE2E_Cost",
		"outcome-record":  "TestE2E_OutcomeRecord",
		"a-b-c":           "TestE2E_ABC",
		"--version":       "TestE2E_Version",
		"statusline":      "TestE2E_Statusline",
		"outcome-record2": "TestE2E_OutcomeRecord2",
	}
	for name, want := range cases {
		got := wiredSurface{Names: []string{name}}.TestName()
		if got != want {
			t.Errorf("%q: test name %q, want %q", name, got, want)
		}
		if !token.IsIdentifier(got) {
			t.Errorf("%q: %q is not a Go identifier, so no test could satisfy the gate", name, got)
		}
	}
}
