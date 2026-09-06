package proxy

import (
	"fmt"
	"testing"
	"time"
)

// Eviction must stay least-recently-used when the clock cannot separate the
// records.
//
// The table evicts by scanning for the smallest seen timestamp. That is only
// least-recently-used if the clock can tell two records apart. Windows'
// time.Now has coarse resolution, so a burst of records inside one tick all
// carry the same instant; v.seen.Before(oldestSeen) is then never true and the
// victim is whichever key Go's randomised map iteration happened to yield
// first.
//
// So under a burst the guard silently stops being an LRU and becomes "evict
// something", and the something can be the heavy, still-active session whose
// spend is the whole reason attribution exists. It showed up as a red Windows
// job saying "the heavy session was not evicted", which reads like a test
// problem and is not one.
//
// A frozen clock reproduces it on any platform: every record shares one
// instant, which is exactly the Windows case with the flakiness removed.
func TestSpendGuard_EvictsLeastRecentlyUsedWhenTheClockCannotSeparateRecords(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 1_000_000})
	frozen := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return frozen }

	// Fill the table exactly. "first" is the least recently used from here on,
	// because nothing touches it again.
	g.Record("first", 1, 0)
	for i := 0; i < maxSpendSessions-1; i++ {
		g.Record(fmt.Sprintf("filler-%d", i), 1, 0)
	}
	if len(g.session) != maxSpendSessions {
		t.Fatalf("table should be full: %d", len(g.session))
	}

	// One more session forces exactly one eviction. The victim must be "first".
	g.Record("newcomer", 1, 0)

	if _, ok := g.session["first"]; ok {
		t.Error("the least recently used session survived; something else was evicted " +
			"instead, so the table is not an LRU when the clock is coarse")
	}
	if _, ok := g.session["newcomer"]; !ok {
		t.Error("the newcomer was evicted immediately")
	}
}

// The same property, stated as the thing a user would notice: a heavy spender
// that is still being recorded must not be thrown away in favour of idle
// sessions, or the guard blames a lane that spent almost nothing.
func TestSpendGuard_AStillActiveHeavySessionOutlivesIdleOnes(t *testing.T) {
	g := NewSpendGuard(SpendLimits{DayTokens: 100_000_000})
	tick := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	g.now = func() time.Time { return tick }

	g.Record("heavy", 500_000, 0)
	for i := 0; i < maxSpendSessions-1; i++ {
		g.Record(fmt.Sprintf("idle-%d", i), 1, 0)
	}
	// Touch the heavy lane again: it is now the most recently used entry that
	// the coarse clock still stamps identically to every other.
	g.Record("heavy", 500_000, 0)

	for i := 0; i < 32; i++ {
		g.Record(fmt.Sprintf("late-%d", i), 1, 0)
	}
	if _, ok := g.session["heavy"]; !ok {
		t.Error("the heavy, recently-touched session was evicted while idle sessions " +
			"survived. Attribution would then name an idle lane for its spend")
	}
}
