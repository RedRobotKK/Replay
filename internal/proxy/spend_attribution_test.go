package proxy

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// SP-7: a refusal names the spender.
//
// The day cap is shared. Today it is shared by one person's agent lanes; behind
// a team proxy it would be shared by an organisation. Either way a refusal that
// says only "daily spend cap reached" hands the reader no action: the developer
// whose next request is refused is very often not the one who spent the budget,
// and there is nothing in the message to tell them which lane did.
//
// Attribution has to be earned rather than asserted. Two things make it easy to
// get wrong, and both are tested here: a session's running total spans its whole
// life while the day total resets at UTC midnight, so the two are different
// windows; and the session table evicts, so the true leader may already be gone
// and naming the largest survivor would be a confident guess. Where the
// accounting cannot support the claim the message says so instead.

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

// SP7-1: the largest day spender is named, not the caller and not the most
// recent.
//
// PASS: the refusal handed to an innocent session names the one that spent it.
// FAIL: naming the caller, which is the intuitive implementation and is exactly
// backwards - the caller is the victim of the cap, not its cause.
func TestSP7_DayCapNamesTheLargestSpender(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 100})
	g.Record("lane-heavy", 70, 0)
	g.Record("lane-light", 25, 0)
	g.Record("lane-tiny", 5, 0) // most recent, and the smallest

	reason := g.Check("lane-innocent")
	if reason == "" {
		t.Fatal("the day cap must refuse, or this test asserts nothing")
	}
	if !strings.Contains(reason, "lane-heavy") {
		t.Errorf("the refusal does not name the session that spent the budget: %q", reason)
	}
	for _, wrong := range []string{"lane-innocent", "lane-tiny", "lane-light"} {
		if strings.Contains(reason, wrong) {
			t.Errorf("the refusal names %q, which did not lead the spend: %q", wrong, reason)
		}
	}
}

// SP7-2: attribution uses today's spend, never a session's lifetime.
//
// A session's counter accumulates for its whole life; the day counter resets at
// UTC midnight. A long-running lane from yesterday therefore has a large
// lifetime total and may have spent nothing today. Attributing on the lifetime
// figure would blame it for a budget it did not touch.
//
// PASS: the session that actually spent today is named.
// FAIL: yesterday's heavy lane named for today's cap.
func TestSP7_AttributionIsScopedToTheDayNotTheSessionLifetime(t *testing.T) {
	day1 := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	g := NewSpendGuard(SpendLimits{DayTokens: 100})
	g.now = fixedClock(day1)
	g.Record("lane-yesterday", 99, 0)

	g.now = fixedClock(day1.Add(24 * time.Hour))
	g.Record("lane-today", 100, 0)

	reason := g.Check("lane-innocent")
	if reason == "" {
		t.Fatal("the day cap must refuse on the new day")
	}
	if strings.Contains(reason, "lane-yesterday") {
		t.Errorf("a session that spent nothing today was blamed for today's cap, which means "+
			"attribution is reading the session lifetime rather than the day: %q", reason)
	}
	if !strings.Contains(reason, "lane-today") {
		t.Errorf("the refusal does not name today's spender: %q", reason)
	}
}

// SP7-3: when the accounting cannot support a name, the message says so.
//
// The session table evicts least-recently-seen past maxSpendSessions and
// discards that session's spend with it (SP-8). The day total is unaffected, so
// after enough churn the surviving sessions no longer add up to it, and the real
// leader may be one of the evicted. Naming the largest survivor would be a
// guess wearing the costume of a measurement.
//
// PASS: the refusal discloses that attribution is partial.
// FAIL: a confident name over incomplete accounting - the failure mode this
// whole project exists to refuse.
func TestSP7_IncompleteAccountingIsDisclosedNotGuessed(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 1_000_000})

	// One heavy spender, then enough churn to evict it.
	g.Record("lane-the-real-cause", 500_000, 0)
	for i := 0; i < maxSpendSessions+16; i++ {
		g.Record(fmt.Sprintf("lane-%d", i), 500, 0)
	}
	if _, ok := g.session["lane-the-real-cause"]; ok {
		t.Fatal("the heavy session was not evicted, so this test does not exercise the gap")
	}

	reason := g.Check("lane-innocent")
	if reason == "" {
		t.Fatal("the day cap must refuse")
	}
	if !strings.Contains(reason, "partly") && !strings.Contains(reason, "accounted") {
		t.Errorf("attribution is incomplete and the refusal does not disclose it. Naming the "+
			"largest survivor here would blame a lane that spent 500 tokens for a "+
			"500,000-token overrun: %q", reason)
	}
}

// SP7-4: a dollar cap attributes in dollars.
//
// PASS: the leader by spend, in the unit that tripped.
// FAIL: attributing a dollar cap by token count, which picks the wrong lane
// whenever a cheap model is chatty and an expensive one is terse.
func TestSP7_DollarCapAttributesByDollars(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayUSD: 10})
	g.Record("lane-chatty-cheap", 900_000, 1.00)
	g.Record("lane-terse-costly", 4_000, 9.00)

	reason := g.Check("lane-innocent")
	if reason == "" {
		t.Fatal("the dollar cap must refuse")
	}
	if !strings.Contains(reason, "lane-terse-costly") {
		t.Errorf("a dollar cap was attributed to the lane with the most tokens rather than the "+
			"most spend: %q", reason)
	}
}

// SP7-5: the session cap still refuses, and does not blame a third party.
//
// The session cap's subject is the caller, so there is nobody else to name.
// This exists because the obvious implementation of SP7-1 - compute a leader and
// append it to every reason - would attach an unrelated lane's name to a
// refusal about the caller's own budget.
//
// PASS: the session cap reason names no other lane.
// FAIL: leader text leaking onto a refusal it does not describe.
func TestSP7_SessionCapDoesNotNameAnotherLane(t *testing.T) {
	g := NewSpendGuard(SpendLimits{SessionTokens: 50, DayTokens: 10_000})
	g.Record("lane-other", 900, 0)
	g.Record("lane-mine", 50, 0)

	reason := g.Check("lane-mine")
	if reason == "" {
		t.Fatal("the session cap must refuse")
	}
	if !strings.Contains(reason, "session spend cap") {
		t.Fatalf("expected the session cap to trip first: %q", reason)
	}
	if strings.Contains(reason, "lane-other") {
		t.Errorf("a session-cap refusal names an unrelated lane: %q", reason)
	}
}

// SP7-6: the refusal carries the session id it was given and nothing else.
//
// Refusal reasons reach the ledger and the client. The ledger already asserts
// that a refusal record leaks no key, no bearer token and no home path
// (refusal_test.go); attribution must not become the hole in that.
//
// PASS: a hostile session id is reproduced verbatim and nothing else appears.
// FAIL: any interpolation of an environment value, a path, or a credential.
func TestSP7_AttributionLeaksNothingBeyondTheSessionID(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	g.Record("sess-abc123", 10, 0)

	reason := g.Check("other")
	for _, forbidden := range []string{"/Users/", "/home/", "sk-ant", "Bearer ", "x-api-key"} {
		if strings.Contains(reason, forbidden) {
			t.Errorf("the attributed refusal leaked %q: %q", forbidden, reason)
		}
	}
	if !strings.Contains(reason, "sess-abc123") {
		t.Errorf("the refusal should name the spender it was given: %q", reason)
	}
}
