package quota

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// Does a cache break burn a flat seat's quota the way it burns a bill?
//
// Nobody knows. The provider bills a cache write at 1.25x and a read at 0.1x,
// so a break costs a metered user 12.5x a read. Whether the SUBSCRIPTION limit
// counts the same way is undocumented, and it is the question that decides
// whether Replay has anything to say to the 80% of users on a flat seat.
//
// This package answers it by subtraction rather than by assumption: the drop in
// the provider's own "remaining" between two consecutive requests is what that
// request actually spent. No thresholds, no risk levels, no forecast - those
// would all need a number this is trying to produce.

func rec(at time.Time, remaining int64, wrote bool, session string) ledger.Record {
	u := &ledger.Usage{Input: 10, Output: 5}
	if wrote {
		u.CacheCreation = 1000
	} else {
		u.CacheRead = 1000
	}
	return ledger.Record{
		Timestamp: at,
		SessionID: session,
		Response:  ledger.Response{Usage: u},
		Quota:     map[string]string{"anthropic-ratelimit-tokens-remaining": itoa(remaining)},
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var b []byte
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

var t0 = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

// QT-1: consumption is the drop in the provider's own counter.
//
// PASS: two requests 500 apart yield a 500-token sample.
// FAIL: anything derived from the request's own token counts, which is what the
// bill already says and is exactly the assumption under test.
func TestQT1_ConsumptionIsTheDropInRemaining(t *testing.T) {
	got := Samples([]ledger.Record{
		rec(t0, 400_000, false, "a"),
		rec(t0.Add(time.Second), 399_500, false, "a"),
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 sample from 2 records, got %d", len(got))
	}
	if got[0].Spent != 500 {
		t.Errorf("spent = %d, want 500", got[0].Spent)
	}
}

// QT-2: the counter is account-wide, so pairing must be global, not per session.
//
// A rate limit is charged against the account, not the conversation. With
// parallel lanes, another session's request lands between two of mine and
// consumes quota I would otherwise attribute to myself. Pairing within a
// session silently inflates every sample by whatever the other lanes spent -
// and fan-out is exactly the workload this product is about.
//
// PASS: records are ordered by time across sessions and paired as they occur.
// FAIL: per-session pairing, which reports session "a" spending 3,000 when it
// spent 1,000 and session "b" spent 2,000.
func TestQT2_PairingIsGlobalNotPerSession(t *testing.T) {
	got := Samples([]ledger.Record{
		rec(t0, 100_000, false, "a"),
		rec(t0.Add(1*time.Second), 99_000, false, "b"), // b spent 1,000
		rec(t0.Add(2*time.Second), 97_000, false, "a"), // a spent 2,000
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 samples, got %d: %+v", len(got), got)
	}
	for _, s := range got {
		if s.Spent == 3_000 {
			t.Fatalf("a sample of 3,000 means pairing ran within a session and absorbed "+
				"another lane's spend: %+v", got)
		}
	}
}

// QT-3: a window reset ends the chain rather than producing a negative.
//
// PASS: the rise is skipped and no sample crosses it.
// FAIL: a negative sample, or worse a large positive one from pairing across
// the reset, which would land in the middle of any median.
func TestQT3_AResetBreaksTheChain(t *testing.T) {
	got := Samples([]ledger.Record{
		rec(t0, 10_000, false, "a"),
		rec(t0.Add(time.Second), 9_000, false, "a"),
		rec(t0.Add(2*time.Second), 400_000, false, "a"), // window reset
		rec(t0.Add(3*time.Second), 399_000, false, "a"),
	})
	for _, s := range got {
		if s.Spent <= 0 {
			t.Errorf("a non-positive sample survived a reset: %+v", got)
		}
	}
	if len(got) != 2 {
		t.Errorf("expected 2 samples either side of the reset, got %d: %+v", len(got), got)
	}
}

// QT-4: a record with no quota header breaks the chain.
//
// Without it the next delta spans two requests and is silently doubled.
//
// PASS: no sample bridges the gap.
// FAIL: one 900-token sample where two requests were made.
func TestQT4_AMissingReadingBreaksTheChain(t *testing.T) {
	blank := rec(t0.Add(time.Second), 0, false, "a")
	blank.Quota = nil
	got := Samples([]ledger.Record{
		rec(t0, 10_000, false, "a"),
		blank,
		rec(t0.Add(2*time.Second), 9_100, false, "a"),
	})
	if len(got) != 0 {
		t.Errorf("a sample bridged a request with no reading: %+v", got)
	}
}

// QT-5: the comparison refuses to report without evidence on both arms.
//
// This is the guard that matters most. The ledger today holds 15 records and
// zero cache-read requests, so the honest output is a refusal. A ratio computed
// from two samples would be quoted as "12.5x confirmed" the moment it appeared
// on a screen.
//
// PASS: not reportable, and it says which arm is short.
// FAIL: a number.
func TestQT5_ComparisonRefusesWithoutBothArms(t *testing.T) {
	var recs []ledger.Record
	rem := int64(1_000_000)
	for i := 0; i < MinPerArm+5; i++ { // writes only
		recs = append(recs, rec(t0.Add(time.Duration(i)*time.Second), rem, true, "a"))
		rem -= 1000
	}
	c := Compare(Samples(recs))
	if c.Reportable {
		t.Errorf("reported a ratio with no cache-read samples at all: %+v", c)
	}
	if c.Why == "" {
		t.Error("a refusal must say what is missing")
	}
	if c.Ratio != 0 {
		t.Errorf("an unreportable comparison must carry no ratio, got %v", c.Ratio)
	}
}

// QT-6: with both arms populated it reports the ratio it measured.
//
// Constructed so writes cost 10x reads. The point is that the number comes from
// the data: if the provider does not apply cache multipliers to the limit, this
// comes back near 1.0 and the whole premise is refuted, which is what makes it
// worth running.
//
// PASS: ratio near 10.
// FAIL: a constant, or the billing multiplier echoed back.
func TestQT6_ReportsTheMeasuredRatio(t *testing.T) {
	var recs []ledger.Record
	rem := int64(10_000_000)
	at := t0
	for i := 0; i < MinPerArm+2; i++ {
		recs = append(recs, rec(at, rem, false, "a"))
		rem -= 100
		at = at.Add(time.Second)
		recs = append(recs, rec(at, rem, false, "a"))
		rem -= 1000
		at = at.Add(time.Second)
		recs = append(recs, rec(at, rem, true, "a"))
		rem -= 100
		at = at.Add(time.Second)
	}
	c := Compare(Samples(recs))
	if !c.Reportable {
		t.Fatalf("both arms are populated and it refused: %+v", c)
	}
	if c.Ratio < 8 || c.Ratio > 12 {
		t.Errorf("ratio = %.2f, want near 10 (the ratio built into the fixture): %+v", c.Ratio, c)
	}
}

// QT-7: a handful of samples on BOTH arms is still a refusal.
//
// Written because the mutation run caught QT-5 not testing what it claimed.
// QT-5 supplies writes only, so ReadN is zero and Compare refuses on the
// divide-by-zero branch instead. Setting MinPerArm to 0 left the whole suite
// green: the minimum-evidence guard - the one thing standing between a
// three-sample fluke and "12.5x confirmed" being repeated as settled - was
// never exercised.
//
// PASS: both arms populated but thin, and it still refuses.
// FAIL: a ratio from a handful of points.
func TestQT7_ThinEvidenceOnBothArmsStillRefuses(t *testing.T) {
	var recs []ledger.Record
	rem := int64(1_000_000)
	at := t0
	for i := 0; i < 3; i++ {
		recs = append(recs, rec(at, rem, false, "a"))
		rem -= 100
		at = at.Add(time.Second)
		recs = append(recs, rec(at, rem, true, "a"))
		rem -= 1000
		at = at.Add(time.Second)
	}
	s := Samples(recs)
	c := Compare(s)
	// The fixture has to have both arms, or this repeats QT-5's mistake.
	if c.ReadN == 0 || c.WriteN == 0 {
		t.Fatalf("fixture does not populate both arms (read=%d write=%d), so this "+
			"test cannot exercise the minimum-evidence guard", c.ReadN, c.WriteN)
	}
	if c.ReadMedian <= 0 {
		t.Fatalf("reads must spend something, or Compare refuses on the divide-by-zero " +
			"branch and the minimum is still untested")
	}
	if c.Reportable {
		t.Errorf("reported a ratio from %d read and %d write samples: %+v", c.ReadN, c.WriteN, c)
	}
}
