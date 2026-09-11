package main

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
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

// FetchedAtInEffect must distinguish a loaded document from the compiled table.
//
// The branch returning "" for the compiled case was unobserved: every test
// reached rulesNotice directly, and none went through the accessor that decides
// which of the two states doctor is describing.
func TestFetchedAtInEffectSeparatesLoadedFromCompiled(t *testing.T) {
	if got := cachemodel.FetchedAtInEffect(); got != "" {
		t.Errorf("with no document loaded, FetchedAtInEffect() = %q, want \"\": the "+
			"compiled table is a floor and has no fetch date", got)
	}
	restore := cachemodel.Override(&cachemodel.Rules{
		Version: "test-2026-09-10", FetchedAt: "2026-09-01T00:00:00Z",
	})
	defer restore()
	if got := cachemodel.FetchedAtInEffect(); got != "2026-09-01T00:00:00Z" {
		t.Errorf("with a document loaded, FetchedAtInEffect() = %q, want its fetchedAt", got)
	}
}

// An empty fetch date is not a date, and must not parse as the zero instant.
//
// A rules document may carry no fetch date at all — a hand-edited file with
// the field blank, or a fetcher that wrote the key and not the value. The
// value time.Parse hands back alongside its error is time.Time{}, so a caller
// that read the time before checking the bool would be told the table was
// fetched at the zero instant: an unreadable document rendered as a dated one,
// aged by two thousand years, and reported with a confidence nothing earned.
// The pair must be (zero, false), and false is the half that carries it.
//
// Nothing observed this before because both callers trim and test for "" on
// their own account before they ever reach here, so no test on either surface
// arrived with an empty string in hand. The condition is only reachable by
// calling parseFetchedAt directly, which is what this does.
//
// Note what is deliberately not asserted: that an early `if s == ""` performs
// the rejection. It did, and it was removed, because neither layout parses ""
// and the branch returned exactly what the code beneath it already returns —
// unobservable by construction, and guard-reachability reported it INERT. The
// contract below survives that edit and any other spelling of the function.
func TestParseFetchedAtRefusesAnEmptyDate(t *testing.T) {
	for _, s := range []string{"", "   ", "\t\n "} {
		at, ok := parseFetchedAt(s)
		if ok {
			t.Errorf("parseFetchedAt(%q) = %v, true: a blank fetch date is the absence\n"+
				"of a date, not a date this build can read", s, at)
		}
		if !at.IsZero() {
			t.Errorf("parseFetchedAt(%q) returned %v with ok=false; the time a rejected\n"+
				"parse hands back must be the zero value, not a partial one", s, at)
		}
	}
}
