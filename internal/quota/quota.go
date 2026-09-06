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

// MinPerArm is how many samples each arm needs before a ratio is reported.
//
// A declared threshold, not a measured one, and it is deliberately blunt: a
// median over fewer than twenty points moves with a single outlier, and the
// figure this produces would be quoted as settled the moment it reached a
// screen. It bounds embarrassment, not error.
const MinPerArm = 20

// Sample is one request's measured consumption.
type Sample struct {
	Limit string
	Spent int64
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
			case d > 0:
				u := r.Response.Usage
				out = append(out, Sample{Limit: limit, Spent: d, Wrote: u != nil && u.CacheCreation > 0})
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
	ReadN, WriteN           int
	ReadMedian, WriteMedian float64
	// Ratio is write median over read median, and is zero unless Reportable.
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

// Compare reports the ratio between what a cache write and a cache read spend
// against the limit, or refuses and says which arm is short.
//
// It refuses rather than reporting a wide interval because the output is a
// single memorable number that will be repeated without its caveat.
func Compare(s []Sample) Comparison {
	var reads, writes []int64
	for _, x := range s {
		if x.Wrote {
			writes = append(writes, x.Spent)
			continue
		}
		reads = append(reads, x.Spent)
	}
	c := Comparison{
		ReadN: len(reads), WriteN: len(writes),
		ReadMedian: median(reads), WriteMedian: median(writes),
	}
	switch {
	case c.ReadN < MinPerArm && c.WriteN < MinPerArm:
		c.Why = fmt.Sprintf("not enough evidence on either arm: %d cache-read and %d cache-write "+
			"samples, %d needed on each", c.ReadN, c.WriteN, MinPerArm)
	case c.ReadN < MinPerArm:
		c.Why = fmt.Sprintf("only %d cache-read samples, %d needed; run the proxy over sessions "+
			"that reuse a warm prefix", c.ReadN, MinPerArm)
	case c.WriteN < MinPerArm:
		c.Why = fmt.Sprintf("only %d cache-write samples, %d needed", c.WriteN, MinPerArm)
	case c.ReadMedian <= 0:
		c.Why = "cache reads spent nothing measurable against the limit, so a ratio would divide by zero"
	default:
		c.Reportable = true
		c.Ratio = c.WriteMedian / c.ReadMedian
	}
	return c
}
