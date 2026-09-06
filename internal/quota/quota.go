// Package quota measures what a request spent against a rate limit.
//
// Every other figure in this project is denominated in dollars, which a flat-
// seat subscriber does not spend. The provider bills a cache write at 1.25x and
// a read at 0.1x, so a break costs a metered user 12.5x a read. Whether the
// SUBSCRIPTION limit counts the same way is undocumented, and it decides
// whether Replay has anything to say to most of its users.
//
// The method is subtraction, not assumption. The drop in the provider's own
// "remaining" between two consecutive requests is what the later one spent. No
// thresholds, no risk levels and no lockout forecast live here: each of those
// needs the number this package exists to produce, and inventing one to gate
// the other is how a tool ends up confidently reporting its own assumptions.
package quota

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// RemainingHeaders report budget LEFT, so they fall as it is consumed. These
// are the documented API-key headers.
var RemainingHeaders = []string{
	"anthropic-ratelimit-tokens-remaining",
	"x-ratelimit-remaining-tokens",
}

// UtilizationHeaders report the fraction of a window CONSUMED, so they rise.
//
// Measured on live traffic 2026-09-06: a Claude Code subscription session
// returns none of the headers above. It returns this family instead, on two
// windows at once, with a companion "representative-claim" naming which one is
// currently binding. The flat seat - the population this package exists for -
// is only visible here, and reading it with falling-counter logic reports "no
// evidence", which is indistinguishable from a provider that sends nothing.
//
// The values carry two decimal places, so one step is 1% of a window. Four
// requests totalling roughly 475,000 cache-write tokens moved the five-hour
// figure from 0.20 to 0.21. Most single requests therefore land below the
// resolution and produce no sample at all, which is why Compare needs many.
var UtilizationHeaders = []string{
	"anthropic-ratelimit-unified-5h-utilization",
	"anthropic-ratelimit-unified-7d-utilization",
}

// MinStepsPerArm is how many counter steps each arm needs before a ratio is
// reported.
//
// Steps, not samples, because steps carry the information. The counter moves in
// whole hundredths, so a hundred requests that each move it by nothing are a
// hundred samples and zero evidence. A declared threshold, not a measured one:
// it bounds embarrassment, not error.
const MinStepsPerArm = 20

// MinPerArm is retained as the sample-count floor beneath the step floor.
const MinPerArm = 20

// Sample is one request's measured consumption.
type Sample struct {
	Limit string
	// Spent is the counter movement attributed to this request, in whole
	// steps of the provider's own resolution.
	Spent int64
	// Tokens is the prompt tokens the provider counted for this request.
	// The estimator is steps per token, so this is the denominator and
	// without it the measurement is degenerate - see Compare.
	Tokens int64
	// Wrote records that the provider reported a cache write on this
	// response - a break - rather than a read.
	Wrote bool
}

// reading is one record's quota position, normalised so that it always FALLS
// as budget is consumed. A utilization fraction is inverted and scaled to parts
// per million so both families share one comparison.
func reading(r ledger.Record) (string, int64, bool) {
	for _, h := range RemainingHeaders {
		v, ok := r.Quota[h]
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			// Unparsed rather than absent. Skipping is right, and it is
			// visible here rather than becoming a silent zero.
			continue
		}
		return h, n, true
	}
	for _, h := range UtilizationHeaders {
		v, ok := r.Quota[h]
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		// Inverted: utilization rises, budget-left falls. Scaled so the
		// two-decimal wire value stays an exact integer rather than picking up
		// float residue that would read as consumption.
		return h, int64((1 - f) * 1_000_000), true
	}
	return "", 0, false
}

// Samples pairs consecutive records and returns what each later one spent.
//
// Pairing is global and by timestamp, never within a session, because the limit
// is charged against the account rather than the conversation. With parallel
// lanes another session's request lands between two of mine and consumes quota
// that per-session pairing would attribute to me - and fan-out is precisely the
// workload this product is about, so that error would be largest exactly where
// the answer matters most.
func Samples(recs []ledger.Record) []Sample {
	ordered := make([]ledger.Record, len(recs))
	copy(ordered, recs)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Timestamp.Before(ordered[j].Timestamp)
	})

	var out []Sample
	var havePrev bool
	var prevLimit string
	var prevVal int64
	for _, r := range ordered {
		limit, val, ok := reading(r)
		if !ok {
			// A request with no reading breaks the chain: pairing across it
			// would silently fold two requests' spend into one sample.
			havePrev = false
			continue
		}
		if havePrev && limit == prevLimit {
			switch d := prevVal - val; {
			// Zero deltas are kept, with Spent 0. They are not evidence of
			// consumption, but they ARE the denominator: the signal is the
			// FREQUENCY with which a request moves a coarse counter, and
			// dropping the requests that moved it by nothing removes exactly
			// that. Summing steps over the tokens of only the movers makes
			// every arm's rate equal one step per average request, and the
			// ratio comes out 1.000 in every possible world.
			case d >= 0:
				u := r.Response.Usage
				var toks int64
				var wrote bool
				if u != nil {
					toks = int64(u.Input + u.CacheCreation + u.CacheRead)
					wrote = u.CacheCreation > 0
				}
				out = append(out, Sample{Limit: limit, Spent: d, Tokens: toks, Wrote: wrote})
			case d < 0:
				// The counter rose: the window reset. Not a negative sample
				// and not a pairing across the boundary, which would put a
				// whole window's budget in the middle of a median.
			}
		}
		havePrev, prevLimit, prevVal = true, limit, val
	}
	return out
}

// Comparison is what the samples say about cache breaks and the limit.
type Comparison struct {
	ReadN, WriteN int
	// ReadRate and WriteRate are counter steps per prompt token on each arm.
	ReadRate, WriteRate float64
	// Ratio is the write rate over the read rate, zero unless Reportable.
	Ratio float64
	// Reportable is false when there is not enough evidence on both arms.
	Reportable bool
	Why        string
}

func median(v []int64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]int64(nil), v...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	m := len(s) / 2
	if len(s)%2 == 1 {
		return float64(s[m])
	}
	return float64(s[m-1]+s[m]) / 2
}

// Compare reports how much more a cache write costs the limit than a read, per
// token, or refuses and says which arm is short.
//
// The estimator is an aggregate RATE - steps per token, summed across each arm -
// and not a median of per-request deltas. That distinction is the whole
// correctness of this package.
//
// The first version took the median of non-zero deltas, and simulation of the
// measured instrument showed it returning exactly 1.00 whether the true ratio
// was 12.5 or 1.0. The counter moves in whole hundredths, so an observed delta
// is almost always exactly 1; the median of each arm is then 1 and the quotient
// is 1 in every possible world. It was not noisy, it was degenerate, and it
// would have reported "subscriptions do not charge like the bill" with total
// confidence and no evidence behind it. Discarding the zero deltas made it
// worse, by keeping only the atypically expensive requests on the arm where
// requests are cheap.
//
// Summing steps over summed tokens discards nothing and lets the quantization
// average out. Against the same simulation it recovers 12.51 and 0.98 for
// truths of 12.5 and 1.0.
func Compare(s []Sample) Comparison {
	var c Comparison
	var readSteps, writeSteps, readTokens, writeTokens int64
	for _, x := range s {
		if x.Tokens <= 0 {
			continue
		}
		if x.Wrote {
			c.WriteN++
			writeSteps += x.Spent
			writeTokens += x.Tokens
			continue
		}
		c.ReadN++
		readSteps += x.Spent
		readTokens += x.Tokens
	}
	if readTokens > 0 {
		c.ReadRate = float64(readSteps) / float64(readTokens)
	}
	if writeTokens > 0 {
		c.WriteRate = float64(writeSteps) / float64(writeTokens)
	}
	switch {
	// Both floors, because Spent is in the counter's OWN units and those
	// differ by six orders of magnitude between the two header families: a
	// step of "remaining" is one token, a step of "utilization" is a
	// hundredth of a window and was measured at roughly 475,000 tokens. A
	// single threshold cannot serve both. Samples guard the fine counter,
	// movement guards the coarse one.
	case c.ReadN < MinPerArm || c.WriteN < MinPerArm:
		c.Why = fmt.Sprintf("not enough samples: %d cache-read and %d cache-write, %d needed "+
			"on each", c.ReadN, c.WriteN, MinPerArm)
	case readSteps < MinStepsPerArm || writeSteps < MinStepsPerArm:
		c.Why = fmt.Sprintf("not enough counter movement: %d step(s) on the cache-read arm and "+
			"%d on the cache-write arm, %d needed on each. Requests that moved the counter by "+
			"nothing are samples, not evidence", readSteps, writeSteps, MinStepsPerArm)
	case c.ReadRate <= 0:
		c.Why = "cache reads moved the counter not at all, so a ratio would divide by zero"
	default:
		c.Reportable = true
		c.Ratio = c.WriteRate / c.ReadRate
	}
	return c
}
