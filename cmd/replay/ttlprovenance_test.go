package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TTL provenance disclosure regression. The cache-write dollar figure must
// carry its basis when it rests on the assumed short-TTL default.

func TestTTLBasisHumanDisclosure(t *testing.T) {
	t.Run("assumed_shown", func(t *testing.T) {
		s := costSummary{Tasks: 1, WriteUSD: 12.34, CacheWriteTTLBasis: "assumed_short"}
		out := renderCost(s, 0, 0, &strings.Builder{}, "")
		if !strings.Contains(out, "TTL basis not measured") {
			t.Fatalf("assumed cache-write price missing its disclosure:\n%s", out)
		}
		if !strings.Contains(out, "5-minute rate") {
			t.Fatalf("disclosure does not name the assumed TTL:\n%s", out)
		}
	})
	t.Run("measured_clean", func(t *testing.T) {
		s := costSummary{Tasks: 1, WriteUSD: 12.34, CacheWriteTTLBasis: "measured"}
		out := renderCost(s, 0, 0, &strings.Builder{}, "")
		if strings.Contains(out, "TTL basis not measured") {
			t.Fatalf("measured cache-write price must not show the assumed disclosure:\n%s", out)
		}
	})
	t.Run("no_writes_no_disclosure", func(t *testing.T) {
		s := costSummary{Tasks: 1, WriteUSD: 0}
		out := renderCost(s, 0, 0, &strings.Builder{}, "")
		if strings.Contains(out, "TTL basis not measured") {
			t.Fatalf("no cache writes must not claim an assumption:\n%s", out)
		}
	})
}

func TestTTLBasisJSONDisclosure(t *testing.T) {
	t.Run("assumed_field", func(t *testing.T) {
		s := costSummary{WriteUSD: 12.34, CacheWriteTTLBasis: "assumed_short"}
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"cacheWriteTtlBasis":"assumed_short"`) {
			t.Fatalf("JSON missing assumed basis: %s", b)
		}
	})
	t.Run("measured_field", func(t *testing.T) {
		s := costSummary{WriteUSD: 12.34, CacheWriteTTLBasis: "measured"}
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"cacheWriteTtlBasis":"measured"`) {
			t.Fatalf("JSON missing measured basis: %s", b)
		}
	})
	t.Run("empty_omitted", func(t *testing.T) {
		s := costSummary{WriteUSD: 0}
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "cacheWriteTtlBasis") {
			t.Fatalf("empty basis must be omitted: %s", b)
		}
	})
}

// TestTTLBasisCostUnitJSON pins the per-task/per-lane row surface too: costUnit
// is serialized as the `tasks`/`lanes` arrays, so its basis field must carry
// the same distinction as the summary.
func TestTTLBasisCostUnitJSON(t *testing.T) {
	u := costUnit{ID: "s1", WriteUSD: 5.0, CacheWriteTTLBasis: "assumed_short"}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"cacheWriteTtlBasis":"assumed_short"`) {
		t.Fatalf("costUnit JSON missing assumed basis: %s", b)
	}

	m := costUnit{ID: "s2", WriteUSD: 5.0, CacheWriteTTLBasis: "measured"}
	b, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"cacheWriteTtlBasis":"measured"`) {
		t.Fatalf("costUnit JSON missing measured basis: %s", b)
	}
}

// foldTTLBasis merges per-unit basis strings correctly: assumed dominates.
func TestFoldTTLBasis(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"", "", ""},
		{"measured", "", "measured"},
		{"", "measured", "measured"},
		{"measured", "measured", "measured"},
		{"assumed_short", "measured", "assumed_short"},
		{"measured", "assumed_short", "assumed_short"},
		{"assumed_short", "", "assumed_short"},
	}
	for _, tc := range cases {
		if got := foldTTLBasis(tc.a, tc.b); got != tc.want {
			t.Errorf("foldTTLBasis(%q,%q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}
