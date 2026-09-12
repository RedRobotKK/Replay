package cachemodel

import (
	"testing"
	"time"
)

// The price table's check window, as a countdown.
//
// PriceTableAgeNoteAt already reports the table's age, which is a number that
// grows. A reader who is being asked to fund the re-verification needs the
// other direction: how long what they were just shown stays good for. Same two
// dates, subtracted the other way round.
//
// The window is anchored to the CHECK, not to the table's own date. An old
// table verified last week is current; a new table nobody has looked at since
// is the one running out.

func day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

// CW-1: the day of the check, the whole window is left.
func TestCW1_FreshCheckHasTheFullWindow(t *testing.T) {
	got, ok := RemainingCheckDays(day("2026-09-07"), "2026-09-07")
	if !ok || got != PriceTableStaleDays {
		t.Errorf("RemainingCheckDays on the check date = %d, %v; want %d, true",
			got, ok, PriceTableStaleDays)
	}
}

// CW-2: it counts DOWN. This is the whole point, so it is asserted as a
// direction across several days rather than as one number: a function that
// returned the age would satisfy a single-value check on the wrong day.
func TestCW2_ItCountsDown(t *testing.T) {
	prev := PriceTableStaleDays + 1
	for d := 0; d <= PriceTableStaleDays; d++ {
		got, ok := RemainingCheckDays(day("2026-09-07").AddDate(0, 0, d), "2026-09-07")
		if !ok {
			t.Fatalf("day %d: inside the window and not ok", d)
		}
		if got >= prev {
			t.Fatalf("day %d: remaining %d did not fall below %d; this is not a countdown",
				d, got, prev)
		}
		prev = got
	}
	if prev != 0 {
		t.Errorf("the window ended at %d days remaining, want 0", prev)
	}
}

// CW-3: past the window there is nothing left to count, and ok is false.
// Returning a negative number would let a caller print "-3 days left", which is
// a countdown that ran past its own end.
func TestCW3_PastTheWindowThereIsNoCountdown(t *testing.T) {
	if got, ok := RemainingCheckDays(day("2026-11-20"), "2026-09-07"); ok {
		t.Errorf("RemainingCheckDays past the window = %d, true; want _, false", got)
	}
}

// CW-4: a date that cannot be read is not a countdown of zero. Absence and zero
// are different values (ADR-0018), and the caller has to be able to tell them
// apart or an unparseable constant silently becomes maximum urgency.
func TestCW4_AnUnreadableDateIsNotZero(t *testing.T) {
	if got, ok := RemainingCheckDays(day("2026-09-12"), "not a date"); ok {
		t.Errorf("an unparseable check date returned %d, true; want _, false", got)
	}
}

// CW-5: a check date in the future is not a longer window. A clock skewed
// backwards, or a constant typo'd forward, must not buy the table extra days.
func TestCW5_AFutureCheckDoesNotExtendTheWindow(t *testing.T) {
	got, ok := RemainingCheckDays(day("2026-09-07"), "2026-12-01")
	if !ok || got != PriceTableStaleDays {
		t.Errorf("a future check date = %d, %v; want it clamped to %d, true",
			got, ok, PriceTableStaleDays)
	}
}
