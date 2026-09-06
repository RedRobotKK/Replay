package analysis

import (
	"math"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// A warm prefix has a deadline, and nothing tells the user it is coming.
//
// Measured across 1,506 transcripts: TTL expiry is 39 breaks and 10,586,000
// re-billed tokens, 33.9% of all waste, at a mean of 271,436 tokens per event -
// ten times the next-largest cause. It is also the only large cause where the
// expensive thing has NOT HAPPENED YET. A break from a changed prefix is over
// by the time it is visible; an idle prefix is a deadline that can still be
// beaten, by sending anything at all.
//
// This measures; it does not act. Nothing here rewrites a request, so ADR-0001
// stands untouched, and the decision stays with the person whose money it is.
const (
	// warnWithin is how close to the deadline the warning fires.
	//
	// A warning that fires from the first turn is a banner, and a banner is
	// muted inside a week. This one is quiet until the deadline is close
	// enough that acting on it is a single keystroke.
	warnWithin = 60 * time.Second

	// warnFloorTokens is the smallest loss worth interrupting anyone for.
	//
	// 56% of sessions in the measured corpus have no avoidable spend at all. A
	// tool that speaks on every session is right rarely and ignored quickly,
	// so the floor is set where the loss is worth a sentence: roughly a
	// tenth of the mean TTL break.
	warnFloorTokens = 25_000
)

// IdleRisk is what a session stands to lose if its cache expires.
type IdleRisk struct {
	Model        string
	PrefixTokens int
	Idle         time.Duration
	TTL          time.Duration
	// Remaining is time left before the cache expires, zero once it has.
	Remaining time.Duration
	// ExcessTokens is the effective-token cost of losing it: the difference
	// between rewriting the prefix and reading it.
	ExcessTokens int
	ExcessUSD    float64
	Warn         bool
}

// MeasureIdleRisk reports what an idle session is about to lose.
//
// ttl zero means the provider default. Passing it explicitly matters because a
// session that bought the one-hour TTL has a different deadline, and applying
// the five-minute one to it would warn hourly about a cache that is fine.
func MeasureIdleRisk(model string, prefixTokens int, idle, ttl time.Duration) IdleRisk {
	if ttl <= 0 {
		ttl = cachemodel.TTLShort
	}
	r := IdleRisk{Model: model, PrefixTokens: prefixTokens, Idle: idle, TTL: ttl}
	if rem := ttl - idle; rem > 0 {
		r.Remaining = rem
	}

	// Below the model's floor nothing was cached, so nothing can expire.
	// Warning here would name a loss that cannot occur, which teaches the
	// reader to ignore the next one.
	if prefixTokens < cachemodel.MinCacheablePrefix(model) {
		return r
	}

	// The loss is the difference between rewriting and reading, in the model's
	// own multiples. The newest models read at 0.025 rather than 0.10, so the
	// same prefix is MORE expensive to lose there, and a hardcoded 1.15 would
	// understate it by a fifth.
	write := cachemodel.WriteMultiplier(ttl)
	read := cachemodel.ReadMultiplierFor(model)
	// Rounded, not truncated: 1.15 x 700,000 is 805,000, and float arithmetic
	// makes it 804999.9999999999. Truncating would print a figure one token
	// short of the obvious answer, which reads as a bug in a report whose
	// whole claim is arithmetic care.
	r.ExcessTokens = int(math.Round(float64(prefixTokens) * (write - read)))
	if p, ok := cachemodel.PriceFor(model); ok {
		r.ExcessUSD = float64(r.ExcessTokens) / 1_000_000 * p.InputPerMTok
	}

	r.Warn = r.Remaining > 0 && r.Remaining <= warnWithin && r.ExcessTokens >= warnFloorTokens
	return r
}
