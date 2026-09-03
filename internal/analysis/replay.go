package analysis

import (
	"fmt"
	"time"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/transcript"
)

// PolicyResult scores one context layout over a lane.
type PolicyResult struct {
	Name string
	Tally
	// ReachableLive says whether a proxy can apply this policy and how.
	ReachableLive string
	// Guardrail names the risk a live trial must watch.
	Guardrail string
	// Estimated is true when the score depends on the byte-to-token fit.
	Estimated bool
}

// Tally accumulates usage across requests and prices it. The same tally
// serves as-run sessions, simulated policies, and the proxy's per-session
// totals, so one definition of "cost" exists.
type Tally struct {
	Requests int
	// PromptTokens is the sum of prompt sizes across requests.
	PromptTokens int
	Reads        int
	Writes       int
	Output       int
	// EffectiveTokens prices writes and reads with the provider multipliers.
	EffectiveTokens float64
	// CostUSD is the list-price cost when the model is in the price table,
	// and zero otherwise. Priced reports say which.
	CostUSD float64
	// Misses counts requests that read nothing from cache after the first.
	Misses int
}

// Add records one request's usage.
func (t *Tally) Add(u transcript.Usage, model string) {
	if t.Requests > 0 && u.CacheRead == 0 {
		t.Misses++
	}
	t.Requests++
	t.PromptTokens += u.PromptTotal()
	t.Reads += u.CacheRead
	t.Writes += u.CacheCreation
	t.Output += u.Output
	t.EffectiveTokens += cachemodel.EffectiveTokens(u, model)
	if p, ok := cachemodel.PriceFor(model); ok {
		t.CostUSD += cachemodel.CostUSD(u, p)
	}
}

// CachedShare is cache reads divided by prompt tokens.
func (t Tally) CachedShare() float64 {
	return cachemodel.CachedShare(t.Reads, t.PromptTokens)
}

// AssumptionNote is printed with every replay table.
const AssumptionNote = "replayed savings assume the agent would have behaved identically under the alternative layout"

// AsRun scores the lane from reported usage alone. Nothing is estimated.
func AsRun(lane *transcript.Lane) PolicyResult {
	r := PolicyResult{Name: "as-run", ReachableLive: "n/a"}
	for _, req := range lane.Requests {
		r.Add(req.Usage, req.Model)
	}
	return r
}

// cacheState is the simulator's view of what the provider has cached.
type cacheState struct {
	prefix    int
	lastTouch time.Time
	ttl       time.Duration
	minPrefix int
}

// newCacheState seeds the simulator with whatever the first request found
// already cached (the shared system prefix from an earlier session), so a
// policy is never charged for a cold start the real session did not pay.
func newCacheState(lane *transcript.Lane, ttl time.Duration) *cacheState {
	state := &cacheState{ttl: ttl}
	if len(lane.Requests) > 0 {
		first := lane.Requests[0]
		state.prefix = first.Usage.CacheRead
		state.lastTouch = first.Timestamp
		state.minPrefix = cachemodel.MinCacheablePrefix(first.Model)
	}
	return state
}

// serve returns the read and write for a request of the given prompt size
// and uncached tail, then updates the state. Cache reads refresh the TTL.
// A cacheable prefix below the model's minimum is never cached.
//
// observed is what the client's own behavior made readable on this turn,
// or -1 when the invariant held. On turns where the as-run read fell short
// (a client-side break) or exceeded the prediction (a sibling request
// extended the prefix) the simulation follows the observation, so the only
// thing that differs from as-run is the policy under test.
func (c *cacheState) serve(at time.Time, prompt, tail int, observed int) (read, write int) {
	cacheable := prompt - tail
	warm := c.prefix > 0 && at.Sub(c.lastTouch) <= c.ttl && cacheable >= c.minPrefix
	switch {
	case !warm:
		read = 0
	case observed >= 0:
		read = min(observed, cacheable)
	default:
		read = min(c.prefix, cacheable)
	}
	write = cacheable - read
	c.prefix = cacheable
	c.lastTouch = at
	return read, write
}

// observedAvailability returns, per turn, the prefix the client's own
// behavior left readable: -1 when the invariant held (no constraint), the
// actual read otherwise. TTL expiries are not client behavior and are left
// for the simulator to decide.
func observedAvailability(cal *Calibration) []int {
	out := make([]int, len(cal.Turns))
	for i, t := range cal.Turns {
		out[i] = -1
		if t.Outcome == cachemodel.ReadExceeded {
			out[i] = t.Actual
		}
		if t.Outcome == cachemodel.ReadBroken && t.Gap <= cachemodel.TTLOf(t.Previous.Usage) {
			out[i] = t.Actual
		}
	}
	return out
}

// WithTTL replays the lane with a different cache TTL. The prompt sizes are
// the reported ones; only expiry and write multipliers change. It is
// measured, not estimated, because no byte-to-token conversion is involved.
func WithTTL(cal *Calibration, ttl time.Duration) PolicyResult {
	lane := cal.Lane
	r := PolicyResult{Name: fmt.Sprintf("ttl-%s", ttl), ReachableLive: "yes: Claude Code setting promptCacheTtl (5m or 1h); the proxy never changes client markers", Guardrail: "none"}
	state := newCacheState(lane, ttl)
	available := observedAvailability(cal)
	for i, req := range lane.Requests {
		read, write := state.serve(req.Timestamp, req.Usage.PromptTotal(), req.Usage.Input, available[i])
		r.Add(cachemodel.SimulatedUsage(req.Usage.Input, write, read, req.Usage.Output, ttl), req.Model)
	}
	return r
}

// ContextEditPolicy describes the provider's server-side clearing of old
// tool results. Clearing shrinks the prompt but invalidates the cache from
// the earliest cleared block, so it trades a periodic write for smaller
// reads; the simulator accounts for both.
type ContextEditPolicy struct {
	// KeepLast is how many most recent tool results survive a clear.
	KeepLast int
	// TriggerTokens is the prompt size at which a clear happens.
	TriggerTokens int
}

// clearHysteresis is how far below the trigger a clear aims, as a share of
// the trigger. Clearing in bulk is what keeps the cache from being
// invalidated on every turn; the provider exposes the same idea as a
// minimum amount to clear.
const clearHysteresis = 0.25

// WithContextEdit replays the lane under a context-editing policy. The
// result is estimated because block sizes come from the fit.
func WithContextEdit(cal *Calibration, p ContextEditPolicy, fit TokenFit) PolicyResult {
	lane := cal.Lane
	available := observedAvailability(cal)
	r := PolicyResult{
		Name:          fmt.Sprintf("context-edit(keep=%d,trigger=%dk)", p.KeepLast, p.TriggerTokens/1000),
		ReachableLive: "yes: request parameter set by the proxy",
		Guardrail:     "re-read rate and failed edits: unknown until a live trial",
		Estimated:     true,
	}
	state := newCacheState(lane, cachemodel.TTLShort)
	// cleared remembers tool results removed by earlier clears; once
	// cleared they stay cleared.
	cleared := map[blockKey]bool{}

	for i, req := range lane.Requests {
		state.ttl = cachemodel.TTLOf(req.Usage)
		prompt, tail := req.Usage.PromptTotal(), req.Usage.Input

		// Sizes of tool results still present, in context order, with the
		// estimated token offset where each begins.
		type slot struct {
			key    blockKey
			tokens int
			offset int
		}
		var results []slot
		offset := fit.UnseenPrefix.Total()
		removed := 0
		for _, m := range req.Context {
			for bi, b := range m.Blocks {
				size := fit.EstimateTokens(b.Bytes)
				if b.Kind == transcript.KindToolResult {
					key := blockKey{m.UUID, bi}
					if cleared[key] {
						removed += size
						continue
					}
					results = append(results, slot{key: key, tokens: size, offset: offset})
				}
				offset += size
			}
		}
		prompt -= removed

		// Clear when over the trigger, oldest first, until the prompt is
		// comfortably under the trigger or only KeepLast results remain.
		invalidateFrom := -1
		if prompt > p.TriggerTokens && len(results) > p.KeepLast {
			target := int(float64(p.TriggerTokens) * (1 - clearHysteresis))
			for len(results) > p.KeepLast && prompt > target {
				v := results[0]
				results = results[1:]
				cleared[v.key] = true
				prompt -= v.tokens
				if invalidateFrom < 0 {
					invalidateFrom = v.offset
				}
			}
		}

		read, write := state.serve(req.Timestamp, prompt, tail, available[i])
		if invalidateFrom >= 0 && read > invalidateFrom {
			// The cache survives only up to the earliest cleared block.
			write += read - invalidateFrom
			read = invalidateFrom
			state.prefix = prompt - tail
		}
		r.Add(cachemodel.SimulatedUsage(tail, write, read, req.Usage.Output, state.ttl), req.Model)
	}
	return r
}
