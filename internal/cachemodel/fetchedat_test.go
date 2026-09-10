package cachemodel

import "testing"

// FetchedAtInEffect must separate a loaded document from the compiled table.
//
// The branch returning "" for the compiled case was unobserved by this
// package's own tests. A cross-package test does not close it:
// guard-reachability neutralises a guard and runs the tests of the package the
// guard lives in, which is the right scope — a guard nothing in its own package
// exercises is one that moved without its tests noticing.
func TestFetchedAtInEffect(t *testing.T) {
	if got := FetchedAtInEffect(); got != "" {
		t.Errorf("with no document loaded, FetchedAtInEffect() = %q, want empty: the "+
			"compiled table is a floor and has no fetch date to age", got)
	}
	restore := Override(&Rules{Version: "test-2026-09-10", FetchedAt: "2026-09-01T00:00:00Z"})
	if got := FetchedAtInEffect(); got != "2026-09-01T00:00:00Z" {
		t.Errorf("with a document loaded, FetchedAtInEffect() = %q, want its fetchedAt", got)
	}
	restore()
	if got := FetchedAtInEffect(); got != "" {
		t.Errorf("after the override was removed, FetchedAtInEffect() = %q, want empty", got)
	}
}
