package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Replay re-derives everything it already knew, on every run.
//
// Measured: `replay cost` over 1,483 transcripts takes 6.3s wall and 19.6s of
// CPU. Almost none of that work is new. Transcripts are append-only files that
// mostly never change again, and a corpus grows by a handful of sessions a day.
// A run that parses 1,483 files to learn about 3 is a full scan of a table that
// wanted an index.
//
// This is also the original thesis pointed at the tool itself: do not re-read
// what you already understood.
//
// The hard part is not caching, it is INVALIDATION. A cached figure derived by
// code that has since changed, or priced from a table that has since moved, is
// worse than no cache: it is a wrong number that arrives fast and looks
// identical to a right one.

func touch(t *testing.T, path, body string, mod time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

// CC-1: an unchanged file is served from the index.
func TestCC1_UnchangedFileIsReused(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.jsonl")
	mod := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	touch(t, f, "{}\n", mod)

	c := newCostCache(filepath.Join(dir, "idx.json"), "v1")
	c.put(f, costUnit{ID: "a", CostUSD: 1.5}, []string{"req_1"})
	if u, _, ok := c.get(f); !ok || u.CostUSD != 1.5 {
		t.Fatalf("an unchanged file was not served from the index: %+v ok=%v", u, ok)
	}
}

// CC-2: a changed file is not.
//
// PASS: a miss after the content and mtime move.
// FAIL: a stale figure, which is the whole risk of caching a derived number.
func TestCC2_ChangedFileIsRederived(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.jsonl")
	touch(t, f, "{}\n", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	c := newCostCache(filepath.Join(dir, "idx.json"), "v1")
	c.put(f, costUnit{ID: "a", CostUSD: 1.5}, []string{"req_1"})

	touch(t, f, "{}\n{}\n", time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC))
	if _, _, ok := c.get(f); ok {
		t.Error("a file that grew was still served from the index")
	}
}

// CC-3: a file that changes without its mtime moving is still caught.
//
// Restores from backup, checkouts and clock skew all produce this. Size is the
// cheap second signal; without it the index trusts a timestamp an attacker or
// an rsync can set.
func TestCC3_SizeChangeAloneInvalidates(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.jsonl")
	mod := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	touch(t, f, "{}\n", mod)
	c := newCostCache(filepath.Join(dir, "idx.json"), "v1")
	c.put(f, costUnit{ID: "a", CostUSD: 1.5}, []string{"req_1"})

	touch(t, f, "{}\n{}\n{}\n", mod) // same mtime, different size
	if _, _, ok := c.get(f); ok {
		t.Error("a file whose size changed under an unchanged mtime was reused")
	}
}

// CC-4: a schema change invalidates everything.
//
// The index stores figures DERIVED by this binary. When the derivation changes
// - a fixed attribution bug, a new price table - every stored figure is a
// result from code that no longer exists. This is the invalidation that
// actually matters, and the one a cache keyed only on file identity gets wrong.
//
// PASS: a different schema key sees nothing.
// FAIL: yesterday's algorithm answering today, fast and wrong.
func TestCC4_SchemaChangeInvalidatesEverything(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.jsonl")
	touch(t, f, "{}\n", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	path := filepath.Join(dir, "idx.json")

	old := newCostCache(path, "rules-2026-09-01")
	old.put(f, costUnit{ID: "a", CostUSD: 1.5}, []string{"req_1"})
	if err := old.save(); err != nil {
		t.Fatal(err)
	}

	fresh := newCostCache(path, "rules-2026-10-01")
	if err := fresh.load(); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := fresh.get(f); ok {
		t.Error("a figure derived under an older schema was served to a newer binary")
	}
}

// CC-5: the index survives a round trip, or it saves nothing at all.
func TestCC5_IndexRoundTrips(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.jsonl")
	touch(t, f, "{}\n", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	path := filepath.Join(dir, "idx.json")

	a := newCostCache(path, "v1")
	a.put(f, costUnit{ID: "a", CostUSD: 2.25}, []string{"req_1"})
	if err := a.save(); err != nil {
		t.Fatal(err)
	}
	b := newCostCache(path, "v1")
	if err := b.load(); err != nil {
		t.Fatal(err)
	}
	u, _, ok := b.get(f)
	if !ok || u.CostUSD != 2.25 {
		t.Errorf("the index did not survive a round trip: %+v ok=%v", u, ok)
	}
}

// CC-6: a corrupt index is a miss, not a crash.
func TestCC6_CorruptIndexIsAMiss(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "idx.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newCostCache(path, "v1")
	if err := c.load(); err != nil {
		t.Errorf("a corrupt index must degrade to a cold run, not an error: %v", err)
	}
}

// CC-7: the index key changes when the cached STRUCT changes.
//
// This is the defect the red team found, and it shipped for forty minutes.
// AvoidableTokens was added to costUnit and the schema literal stayed
// "replay.cost.v1", so every entry already on disk deserialized with the new
// field absent. The same binary on the same machine printed 763k tokens warm
// and 31.4M cold - and the dollar column, which was already cached, agreed in
// both. Two adjacent lines implied $197 per million tokens, which is no
// model's price.
//
// costcache.go promised exactly this and did not deliver it: "keyed on the
// file's identity AND on a schema string the caller derives from EVERYTHING
// the figure depends on". The code version is part of everything.
//
// A constant someone must remember to bump is not a fix - it is the same
// defect with a comment. The key now derives from the struct's own field
// names, so adding, removing or renaming a field invalidates the index whether
// or not anyone remembered.
//
// PASS: two different shapes produce two different keys.
// FAIL: a key that ignores the shape, which serves yesterday's fields to
// today's renderer.
func TestCC7_SchemaKeyTracksTheStructShape(t *testing.T) {
	got := unitSchema()
	if got == "" {
		t.Fatal("the schema key is empty, so it distinguishes nothing")
	}
	// Every field name must be represented: a key over a subset would miss
	// exactly the field that was just added.
	for _, f := range []string{"avoidableTokens", "costUsd", "requests", "breaks", "model"} {
		if !contains(got, f) {
			t.Errorf("the schema key omits %q, so adding it would not invalidate the index: %q",
				f, got)
		}
	}
}

// CC-8: an index written before a field existed is not served.
//
// PASS: the older file is discarded whole.
// FAIL: a hit, which is the bug: the dollar figure is right and the token
// figure silently zero.
func TestCC8_AnIndexFromAnOlderShapeIsDiscarded(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.jsonl")
	touch(t, f, "{}\n", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	path := filepath.Join(dir, "idx.json")

	// Written by a binary whose costUnit had a different shape.
	old := newCostCache(path, "replay.cost.v1/prices/rules/OLDSHAPE")
	old.put(f, costUnit{ID: "a", CostUSD: 1.5}, []string{"req_1"})
	if err := old.save(); err != nil {
		t.Fatal(err)
	}

	fresh := newCostCache(path, "replay.cost.v1/prices/rules/"+unitSchema())
	if err := fresh.load(); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := fresh.get(f); ok {
		t.Error("served an entry written under a different struct shape")
	}
}
