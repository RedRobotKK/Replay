package main

import (
	"strings"
	"testing"
	"time"
)

// Doctor says nothing about the rules document, and the rules document is what
// every dollar figure is computed against.
//
// `replay upgrade` has StaleNotice, which is arithmetic on the compiled-in
// build date and reaches nothing. There is no equivalent for the table, so an
// operator running a document from six weeks ago is told their binary is
// current and never told their prices are not.
//
// The cost of that is not abstract and it is not the price column. It is the
// cache-read multiple, which enters EffectiveTokens and decides which policy
// the comparison reports as cheaper. internal/cachemodel/anthropic.go says it
// outright: falling back to the older 0.10 tier while current models read at
// 0.025 "overstated the cost of every cached token fourfold, and the bias has a
// direction: it inflates the apparent value of cache-preserving policies". A
// tool that recommends keeping a cache, using a table that overstates what
// reading one costs, is making a claim on its own behalf.
func TestRulesNoticeSaysWhatStalenessCosts(t *testing.T) {
	now := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

	// A fresh document: named, and no warning.
	got := rulesNotice("anthropic-2026-09-01", "2026-09-07", now)
	if !strings.Contains(got, "anthropic-2026-09-01") {
		t.Errorf("a fresh document must still be named, so a reader knows which\n"+
			"table produced their figures. got:\n%s", got)
	}
	if strings.Contains(strings.ToLower(got), "stale") {
		t.Errorf("a three-day-old document is not stale:\n%s", got)
	}

	// An old one: warned, with the consequence named and the command to fix it.
	old := rulesNotice("anthropic-2026-07-01", "2026-07-01", now)
	for _, want := range []string{"71 days", "cache-read", "replay rules --check-prices"} {
		if !strings.Contains(old, want) {
			t.Errorf("a 71-day-old document must name %q, or the notice is a date\n"+
				"with no consequence and no next step. got:\n%s", want, old)
		}
	}

	// The compiled-in table is not stale, it is the floor. Saying "0 days old"
	// about a constant would be a number with no meaning behind it.
	compiled := rulesNotice("anthropic-2026-09-01", "", now)
	if strings.Contains(strings.ToLower(compiled), "stale") || strings.Contains(compiled, "days") {
		t.Errorf("a compiled-in table has no fetch date and must not be aged:\n%s", compiled)
	}
	if !strings.Contains(compiled, "compiled in") {
		t.Errorf("the compiled-in case must say so, because absence of a date and a\n"+
			"fresh date are different states. got:\n%s", compiled)
	}
}

// An unparseable date is unknown, not fresh.
//
// ADR-0018: absence, zero and unknown are three values. A fetchedAt this build
// cannot read must not silently become "current", which is the reading that
// suppresses the warning.
func TestRulesNoticeRefusesToTreatAnUnreadableDateAsFresh(t *testing.T) {
	now := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	got := rulesNotice("anthropic-2026-09-01", "not-a-date", now)
	if strings.Contains(strings.ToLower(got), "stale") {
		t.Errorf("an unreadable date must not be reported as stale either:\n%s", got)
	}
	if !strings.Contains(got, "unreadable") {
		t.Errorf("an unreadable fetch date must say so rather than be dropped, or the\n"+
			"absence of a warning means two different things. got:\n%s", got)
	}
}
