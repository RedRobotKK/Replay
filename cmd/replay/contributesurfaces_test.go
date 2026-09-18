package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// A contribution says which surface it came from and which models ran on it.
//
// `observation.Corpus` gained `surfaces` and `models` on 2026-09-17 so a pooled
// figure could be split by vendor. The fields existed and nothing filled them,
// which is the worst of both states: the receiver accepts a field every
// submission omits, so the pool looks like it can answer "what does Astra cost
// against what Fable costs" and answers it with nothing.
//
// Two properties:
//
//  1. A contribution from a Claude Code corpus names that surface and the
//     models it actually saw, so the pool can group by vendor without guessing
//     from `rulesVersion`, which names the price document rather than the run.
//  2. Absent stays absent. A build that measured no models emits no `models`
//     key at all rather than an empty object, because Digested marshals the
//     struct and an empty container present in the bytes would change the
//     digest of every submission written before today.
func TestContributionNamesItsSurfaceAndModels(t *testing.T) {
	// Contributing is fail-closed behind an opt-in file, which is the right
	// default and has to be satisfied here rather than worked around.
	home := withHome(t)
	writeConsent(t, home, "corpus_opt_in = true\n")
	dir := t.TempDir()
	breaks, repeated := 3, 4
	share := 0.05
	f := corpusFigures{
		Tasks: 9, TotalUSD: 12.5, RebilledUSD: 1.25, RebilledShare: 0.1,
		MedianTaskUSD: 0.8, CacheBreaks: &breaks, ReReads: &repeated, ErrorShare: &share,
		Surfaces: []string{"claude-code"},
		Models:   map[string]int{"claude-opus-5": 120, "claude-fable-5-1": 7},
	}
	path, _, err := contributeCorpus("launch-2026-09", dir, f, time.Now())
	if err != nil {
		t.Fatalf("contributeCorpus: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)

	// Parsed rather than string-matched: the file on disk is indented, while
	// the digest is taken over the compact form, so a substring assertion here
	// would be testing the formatter.
	var doc struct {
		Surfaces []string       `json:"surfaces"`
		Models   map[string]int `json:"models"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("the submission is not JSON: %v\n%s", err, got)
	}
	if len(doc.Surfaces) != 1 || doc.Surfaces[0] != "claude-code" {
		t.Errorf("surfaces = %v, want [claude-code]:\n%s", doc.Surfaces, got)
	}
	for model, want := range map[string]int{"claude-opus-5": 120, "claude-fable-5-1": 7} {
		if doc.Models[model] != want {
			t.Errorf("models[%s] = %d, want %d:\n%s", model, doc.Models[model], want, got)
		}
	}
	// Sorted on the wire, because encoding/json sorts map keys and the receiver
	// rebuilds the bytes on that assumption. fable sorts before opus.
	if i, j := strings.Index(got, "fable"), strings.Index(got, "opus"); i > j {
		t.Errorf("the model histogram is not in sorted order, so the receiver cannot reproduce the bytes:\n%s", got)
	}
}

func TestContributionOmitsSurfacesAndModelsWhenNoneWereMeasured(t *testing.T) {
	home := withHome(t)
	writeConsent(t, home, "corpus_opt_in = true\n")
	dir := t.TempDir()
	f := corpusFigures{Tasks: 9, TotalUSD: 12.5, RebilledUSD: 1.25, RebilledShare: 0.1, MedianTaskUSD: 0.8}
	path, _, err := contributeCorpus("launch-2026-09", dir, f, time.Now())
	if err != nil {
		t.Fatalf("contributeCorpus: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, absent := range []string{"surfaces", "models"} {
		if strings.Contains(got, absent) {
			t.Errorf("%q serialised when nothing was measured, which changes the digest of every older submission:\n%s", absent, got)
		}
	}
}
