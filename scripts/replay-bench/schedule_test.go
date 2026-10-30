package main

import (
	"strings"
	"testing"
)

// The 2026-09-03 measurement ran 300 requests direct and then 300 proxied.
// Anything that drifts over the run — thermal state, another process, the GC
// heap growing — lands entirely on one arm. Interleaving puts it on both.

func TestOrderAlternatesWhichArmGoesFirst(t *testing.T) {
	// ABBA counterbalancing: even slots run A then B, odd slots B then A,
	// so neither arm is systematically first.
	got := Order(4, []string{"direct", "proxied"})
	want := "direct,proxied|proxied,direct|direct,proxied|proxied,direct"
	var rows []string
	for _, slot := range got {
		rows = append(rows, strings.Join(slot, ","))
	}
	if strings.Join(rows, "|") != want {
		t.Errorf("Order = %v, want %s", rows, want)
	}
}

func TestOrderIsNeverAllOfOneArmThenAllOfTheOther(t *testing.T) {
	flat := []string{}
	for _, slot := range Order(10, []string{"A", "B"}) {
		flat = append(flat, slot...)
	}
	if len(flat) != 20 {
		t.Fatalf("got %d entries, want 20", len(flat))
	}
	// The blocked design this replaces: AAAAAAAAAABBBBBBBBBB. In an
	// interleaved order the first half must contain both arms.
	seenB := false
	for _, a := range flat[:10] {
		if a == "B" {
			seenB = true
		}
	}
	if !seenB {
		t.Errorf("first half is all A: %v", flat)
	}
	count := map[string]int{}
	for _, a := range flat {
		count[a]++
	}
	if count["A"] != 10 || count["B"] != 10 {
		t.Errorf("arms unbalanced: %v", count)
	}
}

func TestOrderWithThreeArmsRotatesEveryArmThroughFirst(t *testing.T) {
	first := map[string]int{}
	for _, slot := range Order(6, []string{"a", "b", "c"}) {
		if len(slot) != 3 {
			t.Fatalf("slot %v lost an arm", slot)
		}
		first[slot[0]]++
	}
	for _, a := range []string{"a", "b", "c"} {
		if first[a] == 0 {
			t.Errorf("arm %q never ran first: %v", a, first)
		}
	}
}
