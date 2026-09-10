package main

import (
	"strings"
	"testing"
)

// The line a first-time reader can act on.
//
// `replay` prints a total, a median and a p90, and none of them tells somebody
// who just installed whether anything is wrong. The comparison that does is the
// reader against themselves — their most expensive session against their own
// median — because it needs no population, no key and no network, and it means
// the same thing on a metered account, a subscription seat and a local model.

func units(costs ...float64) []costUnit {
	out := make([]costUnit, 0, len(costs))
	for i, c := range costs {
		out = append(out, costUnit{ID: string(rune('a'+i)) + "0000000", CostUSD: c})
	}
	return out
}

// ON1: a real outlier is named, with the ratio and the n behind it.
func TestON1_TheOutlierIsNamedWithItsBasis(t *testing.T) {
	u := units(0.50, 0.80, 0.85, 0.90, 3.40)
	note := outlierNote(u, costSummary{TotalUSD: 6.45, Tasks: len(u)})
	if note == "" {
		t.Fatal("a 4x session against a 5-session median produced no note")
	}
	for _, want := range []string{"53%", "6.45", "5 sessions"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note does not carry %q so the reader cannot check it:\n%s", want, note)
		}
	}
}

// ON2: an unremarkable corpus says nothing.
//
// A tool that prints a comparison on every run trains the reader to skip it.
// Below the threshold there is nothing to act on and the right output is
// silence.
func TestON2_AnUnremarkableCorpusIsSilent(t *testing.T) {
	u := units(0.80, 0.85, 0.90, 1.00)
	if note := outlierNote(u, costSummary{TotalUSD: 3.55, Tasks: len(u)}); note != "" {
		t.Errorf("an evenly spread corpus produced a note:\n%s", note)
	}
}

// ON3: too few sessions, or a zero median, says nothing.
//
// The refusals live in analysis.CompareToMedian; this is the check that the
// caller honours them rather than rendering an infinity or a ratio of one.
func TestON3_RefusalsAreHonoured(t *testing.T) {
	if note := outlierNote(units(1.0, 5.0), costSummary{TotalUSD: 6.0, Tasks: 2}); note != "" {
		t.Errorf("two sessions produced a comparison:\n%s", note)
	}
	if note := outlierNote(units(0, 0, 0, 0), costSummary{TotalUSD: 0, Tasks: 4}); note != "" {
		t.Errorf("a zero median produced a comparison:\n%s", note)
	}
	if note := outlierNote(nil, costSummary{TotalUSD: 1, Tasks: 9}); note != "" {
		t.Errorf("no units produced a comparison:\n%s", note)
	}
}

// ON4: the note names the session so the reader can go and look.
//
// A ratio with no way to reach the thing it describes is a fact the reader can
// do nothing with.
func TestON4_TheSessionIsIdentified(t *testing.T) {
	u := units(0.50, 0.85, 0.90, 3.40)
	note := outlierNote(u, costSummary{TotalUSD: 5.65, Tasks: len(u)})
	if !strings.Contains(note, "d0000000") && !strings.Contains(note, "d000000") {
		t.Errorf("the note does not name the session it is about:\n%s", note)
	}
}
