package transcript

import (
	"math"
	"testing"
)

// A zero-token record must not disable break detection for the rest of the
// session.
//
// observeCache divides CacheRead by PromptTotal to get the cached share, and
// `if prompt <= 0 { return }` is what stops that being a division by zero.
// CI's guard-reachability reviewer reported the guard SURVIVED: neutralising it
// left the whole suite green, so nothing observed it.
//
// The consequence is worse than a bad number on one line. Without the guard a
// zero-prompt record makes share NaN, and NaN is then stored in prevShare —
// where every subsequent comparison against it is false, by IEEE rule. The
// warm-to-cold transition can never fire again, so one malformed or empty usage
// record silently switches off cache-break detection for everything after it.
//
// A session that reports no breaks because it had none, and a session that
// reports no breaks because a NaN is sitting in a comparison, look identical.

// CZ1: a zero-prompt record leaves later detection working.
//
// PASS: the warm-to-cold transition after it is still found.
// FAIL: no break, because prevShare holds NaN and every comparison against it
// is false.
func TestCZ1_AZeroPromptRecordDoesNotPoisonDetection(t *testing.T) {
	s := &CodexSession{}

	// Warm: nearly all of the prompt came from cache.
	s.observeCache(Usage{Input: 200, CacheRead: 8000})
	// The record that must not poison anything: nothing in the prompt at all.
	s.observeCache(Usage{Input: 0, CacheRead: 0})
	// Cold: a large prompt with nothing cached. This is the transition.
	s.observeCache(Usage{Input: 9000, CacheRead: 0})

	if len(s.Breaks) == 0 {
		t.Fatal("the warm-to-cold transition was not detected after a zero-prompt record. " +
			"prevShare holds NaN, and every comparison against NaN is false, so no break " +
			"can be found for the rest of the session.")
	}
}

// CZ2: prevShare never becomes NaN.
//
// The mechanism, asserted directly. CZ1 catches the consequence; this catches
// the state that causes it, so a future change that produces NaN by another
// route is caught at the source rather than three turns later.
func TestCZ2_PrevShareIsNeverNaN(t *testing.T) {
	s := &CodexSession{}
	for _, u := range []Usage{
		{Input: 200, CacheRead: 8000},
		{},                             // no tokens at all
		{Input: 0, CacheRead: 0},       // explicit zeroes
		{Input: 9000, CacheRead: 0},    // cold
		{Input: 100, CacheRead: 20000}, // warm again
	} {
		s.observeCache(u)
		if math.IsNaN(s.prevShare) || math.IsInf(s.prevShare, 0) {
			t.Fatalf("prevShare is %v after usage %+v; every later comparison against it "+
				"is false, which disables detection silently", s.prevShare, u)
		}
	}
}

// CZ3: a genuine warm-to-cold transition is still detected without any zero
// record.
//
// The anti-overcorrection guard. A fix that made observeCache return early too
// often would satisfy CZ1 and CZ2 by never detecting anything.
func TestCZ3_TheOrdinaryTransitionStillFires(t *testing.T) {
	s := &CodexSession{}
	s.observeCache(Usage{Input: 200, CacheRead: 8000})
	s.observeCache(Usage{Input: 9000, CacheRead: 0})
	if len(s.Breaks) != 1 {
		t.Fatalf("a plain warm-to-cold transition produced %d breaks, want 1", len(s.Breaks))
	}
}
