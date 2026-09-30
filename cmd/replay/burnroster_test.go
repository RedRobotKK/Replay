package main

import (
	"sort"
	"strings"
	"testing"
)

// The roster of surfaces reaching the cross-surface report is pinned.
//
// docs/PRODUCTION-WIRING.md states which vendor surfaces reach production
// reporting, and a document cannot check itself. This test is the other half:
// add a surface to `replay burn` without updating the matrix, or quietly drop
// one, and it fails here naming what changed.
//
// It pins names only. What each surface reports is the business of its own
// tests, and duplicating those assertions here would make this a second place
// to update rather than a guard.
func TestPW1_TheBurnSurfaceRosterMatchesTheProductionMatrix(t *testing.T) {
	// The matrix rows marked PRODUCTION-WIRED against the local-store
	// reporting path, in docs/PRODUCTION-WIRING.md.
	want := []string{"claude-code", "codex", "grok", "ollama"}

	got := []string{
		burnCodex("", t.TempDir()).name,
		burnOllama("", t.TempDir()).name,
		burnClaudeCode("", t.TempDir()).name,
		burnGrok("", t.TempDir()).name,
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("burn reports surfaces %v, the matrix declares %v.\n"+
			"Update docs/PRODUCTION-WIRING.md and this list together, or the "+
			"document is claiming support the binary does not provide.",
			got, want)
	}
}

// Every surface in the report says what its tokens mean and what it can say
// about quota.
//
// A blank unit makes two unaddable columns look addable, which is the defect
// the report's own header text exists to prevent. A blank quota reads as
// "nothing used" rather than "cannot answer".
func TestPW2_EverySurfaceDeclaresItsUnitAndItsQuotaStanding(t *testing.T) {
	for _, s := range []surfaceBurn{
		burnCodex("", t.TempDir()),
		burnOllama("", t.TempDir()),
		burnClaudeCode("", t.TempDir()),
		burnGrok("", t.TempDir()),
	} {
		if strings.TrimSpace(s.unit) == "" {
			t.Errorf("%s declares no unit; its token column would read as addable with the others", s.name)
		}
		if strings.TrimSpace(s.quota) == "" {
			t.Errorf("%s declares no quota standing; a blank cell reads as nothing used rather than cannot answer", s.name)
		}
	}
}
