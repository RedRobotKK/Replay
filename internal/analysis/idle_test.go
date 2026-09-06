package analysis

import (
	"testing"
	"time"
)

// A warm prefix has a deadline, and nothing tells you it is coming.
//
// Measured across 1,506 transcripts: TTL expiry is 39 breaks and 10,586,000
// re-billed tokens - 33.9% of all waste, at a mean of 271,436 tokens per
// event, ten times the size of the next-largest cause. One session shows
// "turn 666 (+1h7m18s): read 0 of 702k expected, 702k re-billed". The
// developer went to lunch seven minutes past a one-hour TTL, three times.
//
// This is the one large cause that a warning can address, because it is the
// only one where the expensive thing has not happened yet. A cache break from
// a changed prefix is already over by the time it is visible; an idle prefix
// is a deadline you can still beat.
//
// It is a measurement, not a policy: nothing here rewrites a request, so
// ADR-0001's byte transparency is untouched.

// IW-1: the loss is priced with the model's OWN multipliers.
//
// Losing a warm prefix costs the difference between rewriting it and reading
// it. On opus-5 that is 1.25 - 0.10 = 1.15x. On the newest models the read
// multiple is 0.025, not 0.10, so the same prefix loses 1.225x - and a warning
// that hardcodes one of them is wrong by a fifth on the other.
//
// PASS: both models priced from their own tables.
// FAIL: a single hardcoded multiplier, which is the obvious implementation.
func TestIW1_LossUsesTheModelsOwnMultipliers(t *testing.T) {
	const prefix = 700_000
	opus := MeasureIdleRisk("claude-opus-5", prefix, 4*time.Minute, time.Duration(0))
	newest := MeasureIdleRisk("claude-fable-5-1", prefix, 4*time.Minute, time.Duration(0))

	if got, want := opus.ExcessTokens, 805_000; got != want {
		t.Errorf("opus-5 excess = %d, want %d", got, want)
	}
	if got, want := newest.ExcessTokens, 857_500; got != want {
		t.Errorf("fable-5-1 excess = %d, want %d", got, want)
	}
	if newest.ExcessTokens <= opus.ExcessTokens {
		t.Error("a lower read multiple makes losing the cache MORE expensive, not less")
	}
}

// IW-2: a prefix too small to have cached cannot be lost.
//
// The minimum cacheable prefix varies by model - 512, 1024, 2048, 4096. Below
// it nothing was written, so nothing expires, and a warning would name a loss
// that cannot occur.
//
// This asserts on ExcessTokens rather than Warn, and the difference is the
// whole test. Mutation showed that removing the floor check left the suite
// green: any prefix below the largest floor (4,096) has a notional excess
// under 4,710, which is beneath warnFloorTokens anyway, so Warn was false for
// a reason that had nothing to do with the guard. A test that refuses for the
// wrong reason exercises nothing - the same shape this project has now caught
// three times.
//
// PASS: no loss is computed at all, because none can occur.
// FAIL: a figure for tokens that were never cached.
func TestIW2_BelowTheFloorThereIsNothingToLose(t *testing.T) {
	// haiku-4-5's floor is 4096.
	r := MeasureIdleRisk("claude-haiku-4-5-20251001", 3_000, 4*time.Minute, 0)
	if r.ExcessTokens != 0 {
		t.Errorf("computed a %d token loss on a prefix below the caching floor, where "+
			"nothing was ever written: %+v", r.ExcessTokens, r)
	}
	if r.Warn {
		t.Errorf("warned about a cache that never existed: %+v", r)
	}
}

// IW-3: after expiry there is nothing to warn about.
//
// The point of the warning is that the loss has not happened yet. Once the
// gap exceeds the TTL the tokens are already re-billed, and saying so is a
// report, not a warning.
//
// PASS: silent past the deadline.
// FAIL: a warning that arrives after the money is gone, which is the failure
// mode of every alert that fires on a completed event.
func TestIW3_PastTheDeadlineItIsTooLate(t *testing.T) {
	r := MeasureIdleRisk("claude-opus-5", 700_000, 6*time.Minute, 0)
	if r.Warn {
		t.Errorf("warned after the 5-minute TTL had already passed: %+v", r)
	}
	if r.Remaining > 0 {
		t.Errorf("Remaining = %v, want zero or negative past the deadline", r.Remaining)
	}
}

// IW-4: it warns inside the window and stays quiet outside it.
//
// PASS: quiet at one minute idle, loud at four.
// FAIL: warning from the first turn, which is a permanent banner rather than
// a deadline - and a permanent banner is muted within a week.
func TestIW4_OnlyNearTheDeadline(t *testing.T) {
	early := MeasureIdleRisk("claude-opus-5", 700_000, 1*time.Minute, 0)
	if early.Warn {
		t.Errorf("warned four minutes early: %+v", early)
	}
	late := MeasureIdleRisk("claude-opus-5", 700_000, 4*time.Minute+30*time.Second, 0)
	if !late.Warn {
		t.Errorf("did not warn with 30s left on a 700k prefix: %+v", late)
	}
}

// IW-5: a session that bought the one-hour TTL gets the one-hour deadline.
//
// PASS: quiet at six minutes when the TTL is an hour.
// FAIL: the short deadline applied to a long-TTL session, which would warn
// hourly about a cache that is fine.
func TestIW5_TheLongTTLMovesTheDeadline(t *testing.T) {
	r := MeasureIdleRisk("claude-opus-5", 700_000, 6*time.Minute, time.Hour)
	if r.Warn {
		t.Errorf("warned at 6 minutes on an hour-long TTL: %+v", r)
	}
	if r.TTL != time.Hour {
		t.Errorf("TTL = %v, want 1h", r.TTL)
	}
	near := MeasureIdleRisk("claude-opus-5", 700_000, 59*time.Minute, time.Hour)
	if !near.Warn {
		t.Errorf("did not warn one minute before an hour-long TTL expired: %+v", near)
	}
}

// IW-6: a trivial prefix is not worth interrupting anyone for.
//
// PASS: silent on a prefix whose loss is negligible.
// FAIL: a warning on every short session, which is how a signal becomes
// furniture. 56% of sessions in the measured corpus have no avoidable spend at
// all; a tool that speaks on all of them is right rarely and ignored quickly.
func TestIW6_TrivialLossIsSilent(t *testing.T) {
	r := MeasureIdleRisk("claude-opus-5", 6_000, 4*time.Minute, 0)
	if r.Warn {
		t.Errorf("warned about losing a 6k prefix: %+v", r)
	}
}
