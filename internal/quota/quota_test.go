package quota

import (
	"strconv"
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

// QT-7: a handful of counter steps on BOTH arms is still a refusal.
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
	if c.ReadRate <= 0 {
		t.Fatalf("reads must move the counter, or Compare refuses on the divide-by-zero " +
			"branch and the minimum-evidence guard is still untested")
	}
	if c.Reportable {
		t.Errorf("reported a ratio from %d read and %d write samples, below the step floor: %+v",
			c.ReadN, c.WriteN, c)
	}
}

// QT-8: the subscription surface reports utilization, not remaining, and it
// RISES.
//
// Measured on live traffic 2026-09-06. A Claude Code subscription session
// returns none of the documented API rate-limit headers. It returns instead:
//
//	anthropic-ratelimit-unified-5h-utilization      0.2
//	anthropic-ratelimit-unified-7d-utilization      0.27
//	anthropic-ratelimit-unified-status              allowed
//	anthropic-ratelimit-unified-representative-claim five_hour
//
// So the flat seat - the whole population this package was written for -
// reports a fraction of a window consumed, on two windows at once, and the
// number goes UP. Reading it with the falling-counter logic finds nothing at
// all and reports "no evidence", which is the worst possible failure here
// because it is indistinguishable from a provider that sends no quota data.
//
// PASS: a rise of 0.01 is a sample, and a fall is treated as a window reset.
// FAIL: silence on the surface that matters most.
func TestQT8_UtilizationRisesAndIsStillASample(t *testing.T) {
	mk := func(at time.Time, util string, wrote bool) ledger.Record {
		r := rec(at, 0, wrote, "a")
		r.Quota = map[string]string{"anthropic-ratelimit-unified-5h-utilization": util}
		return r
	}
	got := Samples([]ledger.Record{
		mk(t0, "0.20", true),
		mk(t0.Add(time.Second), "0.21", true),
	})
	if len(got) != 1 {
		t.Fatalf("utilization produced %d samples, want 1: %+v", len(got), got)
	}
	if got[0].Spent <= 0 {
		t.Errorf("a rise in utilization must be positive consumption: %+v", got[0])
	}
}

// QT-9: utilization falling is a window reset, not negative consumption.
func TestQT9_FallingUtilizationIsAReset(t *testing.T) {
	mk := func(at time.Time, util string) ledger.Record {
		r := rec(at, 0, false, "a")
		r.Quota = map[string]string{"anthropic-ratelimit-unified-5h-utilization": util}
		return r
	}
	got := Samples([]ledger.Record{
		mk(t0, "0.98"),
		mk(t0.Add(time.Second), "0.01"), // the 5h window rolled
		mk(t0.Add(2*time.Second), "0.02"),
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 sample after the reset, got %d: %+v", len(got), got)
	}
}

// QT-10: a request that moved the counter by nothing still counts as observed.
//
// This test asserted the opposite until QT-11 caught it. The reasoning was
// that a zero delta is not evidence a request was free - true - and the
// conclusion drawn was to discard the pair, which was wrong and fatal.
//
// Utilization carries two decimals, so one step is 1% of a window and was
// measured at roughly 475,000 write-equivalent tokens. Almost every individual
// request therefore moves the counter by nothing. The entire signal is the
// FREQUENCY of movement, so the requests that did not move it are the
// denominator. Dropping them leaves each arm at one step per average moving
// request and the ratio is 1.000 in every possible world.
//
// PASS: the pair is observed, carrying zero consumption.
// FAIL: dropped, which restores the degenerate estimator.
func TestQT10_ZeroDeltasAreObservedWithZeroConsumption(t *testing.T) {
	mk := func(at time.Time, util string) ledger.Record {
		r := rec(at, 0, false, "a")
		r.Quota = map[string]string{"anthropic-ratelimit-unified-5h-utilization": util}
		return r
	}
	got := Samples([]ledger.Record{
		mk(t0, "0.20"),
		mk(t0.Add(time.Second), "0.20"),
		mk(t0.Add(2*time.Second), "0.20"),
	})
	if len(got) != 2 {
		t.Fatalf("expected both pairs observed, got %d: %+v", len(got), got)
	}
	for _, s := range got {
		if s.Spent != 0 {
			t.Errorf("unchanged utilization must record zero consumption, not %d: %+v", s.Spent, s)
		}
		if s.Tokens <= 0 {
			t.Errorf("a zero-delta sample still has to carry its tokens, or it is not a "+
				"denominator: %+v", s)
		}
	}
}

// QT-11: the estimator must recover a known ratio.
//
// This is the test the package should have opened with. Simulation of the
// measured instrument - 1% resolution, one step per ~475,000 write-equivalent
// tokens - showed the original estimator, median of non-zero deltas, returning
// exactly 1.00 whether the true ratio was 12.5 or 1.0. Not noisy: DEGENERATE.
// Observed deltas are integers and almost always exactly 1, so the median of
// each arm is 1 and the quotient is 1 by construction, for every possible
// world. A measurement that cannot come out differently is not a measurement,
// and it would have reported "no, subscriptions do not charge like the bill"
// with total confidence and no evidence.
//
// The replacement is a rate: steps per token, per arm, aggregated. It discards
// nothing and quantization averages out. In simulation it recovers 12.51 and
// 0.98 against truths of 12.5 and 1.0.
//
// PASS: both worlds recovered within tolerance.
// FAIL: the arms collapse to the same number again.
func TestQT11_TheEstimatorRecoversAKnownRatio(t *testing.T) {
	// One step of the counter, in write-equivalent tokens, from live data.
	const step = 475_000
	const prefix = 157_000

	build := func(readWeight float64) []ledger.Record {
		var recs []ledger.Record
		at := t0
		acc, last := 0.0, 0
		for i := 0; i < 4000; i++ {
			wrote := i%10 == 0
			cost := float64(prefix)
			if !wrote {
				cost = float64(prefix) * readWeight
			}
			acc += cost
			last = int(acc / step)
			// Utilization rises one hundredth per step, wrapping is not
			// exercised here.
			r := rec(at, 0, wrote, "a")
			r.Quota = map[string]string{
				"anthropic-ratelimit-unified-5h-utilization": ftoa(float64(last) / 100),
			}
			if wrote {
				r.Response.Usage.CacheCreation = prefix
				r.Response.Usage.CacheRead = 0
			} else {
				r.Response.Usage.CacheCreation = 0
				r.Response.Usage.CacheRead = prefix
			}
			recs = append(recs, r)
			at = at.Add(time.Second)
		}
		return recs
	}

	for _, c := range []struct {
		name       string
		readWeight float64
		want       float64
	}{
		{"quota mirrors the bill", 0.1 / 1.25, 12.5},
		{"quota counts raw tokens", 1.0, 1.0},
	} {
		smp := Samples(build(c.readWeight))
		got := Compare(smp)
		t.Logf("%s: samples=%d readN=%d writeN=%d readRate=%.3e writeRate=%.3e ratio=%.3f why=%q",
			c.name, len(smp), got.ReadN, got.WriteN, got.ReadRate, got.WriteRate, got.Ratio, got.Why)
		if !got.Reportable {
			t.Errorf("%s: refused on a full simulated run: %s", c.name, got.Why)
			continue
		}
		lo, hi := c.want*0.75, c.want*1.25
		if got.Ratio < lo || got.Ratio > hi {
			t.Errorf("%s: ratio = %.2f, want near %.2f. If both cases come out at 1.00 the "+
				"estimator is degenerate again.", c.name, got.Ratio, c.want)
		}
	}
}

// ftoa renders a two-decimal counter reading. The wire carries two decimals;
// the simulation runs past 1.0 because it models more than one window's worth
// of traffic, and the estimator is a ratio of rates so the absolute level
// cancels.
func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}
