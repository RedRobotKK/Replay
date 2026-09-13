package proxy

import (
	"fmt"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/policy"
)

// EV1: eviction is decided by age, not by Go's map iteration order.
//
// TestSessionStateIsBounded already asserts the oldest session goes. It could
// not fail on Linux or macOS, because time.Now there advances every nanosecond
// and every session gets a distinct lastSeen. On Windows time.Now advances in
// steps of roughly 15ms, so a loop that admits twenty sessions gives them all
// ONE timestamp, the strict Before comparison never fires, and the session
// evicted is whichever key the randomised map iteration reached first. It
// failed on the Windows runner on 2026-09-13 and had never failed anywhere
// else.
//
// So this test does not wait for a coarse clock, it writes one: every session
// is given an identical lastSeen, which is the state Windows produces on its
// own. Then it evicts repeatedly, because map order is randomised per range
// statement and a single eviction that happens to pick correctly proves
// nothing.
//
// PASS: the first session admitted is always the one dropped.
// FAIL: the proxy discards a live session and keeps a stale one, on whichever
// platform has the coarsest clock.
func TestEV1_EvictionUnderACoarseClockDropsTheOldestNotARandomOne(t *testing.T) {
	// Twenty rounds: a tie broken by map order lands on the right key with
	// probability 1/maxSessions each round, so surviving twenty is not luck.
	for round := 0; round < 20; round++ {
		s := newStats()
		for i := 0; i < maxSessions; i++ {
			s.pin(fmt.Sprintf("s-%d", i), nil, policy.NotConfigured, time.Time{})
		}
		if len(s.sessions) != maxSessions {
			t.Fatalf("round %d: setup holds %d sessions, want %d", round, len(s.sessions), maxSessions)
		}

		// The coarse clock, written directly: one reading for all of them.
		frozen := time.Date(2026, 9, 13, 20, 9, 16, 0, time.UTC)
		for _, st := range s.sessions {
			st.lastSeen = frozen
		}

		// One more session forces exactly one eviction.
		s.pin("newcomer", nil, policy.NotConfigured, time.Time{})

		if _, _, ok := s.pinned("s-0"); ok {
			t.Fatalf("round %d: s-0 was admitted first and survived. Eviction fell "+
				"through to map order, so the proxy drops an arbitrary session "+
				"rather than the least recently used.", round)
		}
		if _, _, ok := s.pinned("newcomer"); !ok {
			t.Fatalf("round %d: the session that caused the eviction was itself evicted", round)
		}
		for i := 1; i < maxSessions; i++ {
			id := fmt.Sprintf("s-%d", i)
			if _, _, ok := s.pinned(id); !ok {
				t.Fatalf("round %d: %s was evicted, but only s-0 should have been", round, id)
			}
		}
	}
}

// EV2: a session with a genuinely older timestamp still goes first.
//
// EV1 pins the tie-break. On its own that would be satisfied by ignoring
// lastSeen entirely and evicting in admission order, which would discard a
// session that is still sending in favour of one that went quiet an hour ago.
// lastSeen is updated on every request, so it is the real recency signal and
// admission order only settles ties.
func TestEV2_AGenuinelyOlderSessionIsEvictedAheadOfAnEarlierAdmittedOne(t *testing.T) {
	s := newStats()
	for i := 0; i < maxSessions; i++ {
		s.pin(fmt.Sprintf("s-%d", i), nil, policy.NotConfigured, time.Time{})
	}
	recent := time.Date(2026, 9, 13, 20, 9, 16, 0, time.UTC)
	for _, st := range s.sessions {
		st.lastSeen = recent
	}
	// s-0 was admitted first, but the LAST one admitted went quiet an hour ago.
	stale := fmt.Sprintf("s-%d", maxSessions-1)
	s.sessions[stale].lastSeen = recent.Add(-time.Hour)

	s.pin("newcomer", nil, policy.NotConfigured, time.Time{})

	if _, _, ok := s.pinned(stale); ok {
		t.Errorf("%s had not been seen for an hour and survived eviction", stale)
	}
	if _, _, ok := s.pinned("s-0"); !ok {
		t.Error("s-0 was evicted on admission order alone, which would discard a " +
			"session that is still sending in favour of one that went quiet")
	}
}
