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
	// PromptTokens is the sum of prompt sizes across requests.
	PromptTokens int
	// EffectiveTokens prices writes and reads with the provider multipliers.
	EffectiveTokens float64
	// CachedShare is cache reads divided by prompt tokens.
	CachedShare float64
	// Misses counts requests that read nothing from cache after the first.
	Misses int
	// CostUSD is the list-price cost when the model is in the price table,
	// and zero otherwise. Priced reports say which.
	CostUSD float64
	// ReachableLive says whether a proxy can apply this policy and how.
	ReachableLive string
	// Guardrail names the risk a live trial must watch.
	Guardrail string
	// Estimated is true when the score depends on the byte-to-token fit.
	Estimated bool
}

// AssumptionNote is printed with every replay table.
const AssumptionNote = "replayed savings assume the agent would have behaved identically under the alternative layout"

// AsRun scores the lane from reported usage alone. Nothing is estimated.
func AsRun(lane *transcript.Lane) PolicyResult {
	r := PolicyResult{Name: "as-run", ReachableLive: "n/a"}
	var reads int
	for i, req := range lane.Requests {
		r.PromptTokens += req.Usage.PromptTotal()
		r.EffectiveTokens += cachemodel.EffectiveTokens(req.Usage, req.Model)
		if p, ok := cachemodel.PriceFor(req.Model); ok {
			r.CostUSD += cachemodel.CostUSD(req.Usage, p)
		}
		reads += req.Usage.CacheRead
		if i > 0 && req.Usage.CacheRead == 0 {
			r.Misses++
		}
	}
	if r.PromptTokens > 0 {
		r.CachedShare = float64(reads) / float64(r.PromptTokens)
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
	name := fmt.Sprintf("ttl-%s", ttl)
	r := PolicyResult{Name: name, ReachableLive: "yes: Claude Code setting promptCacheTtl (5m or 1h); the proxy never changes client markers", Guardrail: "none"}
	state := newCacheState(lane, ttl)
	mult := cachemodel.WriteMultiplier(ttl)
	available := observedAvailability(cal)
	var reads int
	for i, req := range lane.Requests {
		prompt, tail := req.Usage.PromptTotal(), req.Usage.Input
		read, write := state.serve(req.Timestamp, prompt, tail, available[i])
		r.PromptTokens += prompt
		r.EffectiveTokens += float64(tail) + float64(write)*mult + float64(read)*cachemodel.ReadMultiplierFor(req.Model)
		if p, ok := cachemodel.PriceFor(req.Model); ok {
			r.CostUSD += cachemodel.PromptCostUSD(tail, write, read, mult, p) + float64(req.Usage.Output)*p.OutputPerMTok/1_000_000
		}
		reads += read
		if i > 0 && read == 0 {
			r.Misses++
		}
	}
	if r.PromptTokens > 0 {
		r.CachedShare = float64(reads) / float64(r.PromptTokens)
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
	var reads int
	// cleared maps a tool result's uuid+block index to the request index
	// where it was cleared; once cleared it stays cleared.
	cleared := map[string]bool{}

	for i, req := range lane.Requests {
		state.ttl = cachemodel.TTLOf(req.Usage)
		prompt, tail := req.Usage.PromptTotal(), req.Usage.Input
		mult := cachemodel.WriteMultiplier(state.ttl)

		// Sizes of tool results still present, in context order, with the
		// estimated token offset where each begins.
		type slot struct {
			key    string
			tokens int
			offset int
		}
		var results []slot
		offset := fit.UnseenPrefixTokens
		removed := 0
		for _, m := range req.Context {
			for bi, b := range m.Blocks {
				size := fit.EstimateTokens(b.Bytes)
				if b.Kind == transcript.KindToolResult {
					key := fmt.Sprintf("%s/%d", m.UUID, bi)
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
		r.PromptTokens += prompt
		r.EffectiveTokens += float64(tail) + float64(write)*mult + float64(read)*cachemodel.ReadMultiplierFor(req.Model)
		if p, ok := cachemodel.PriceFor(req.Model); ok {
			r.CostUSD += cachemodel.PromptCostUSD(tail, write, read, mult, p) + float64(req.Usage.Output)*p.OutputPerMTok/1_000_000
		}
		reads += read
		if i > 0 && read == 0 {
			r.Misses++
		}
	}
	if r.PromptTokens > 0 {
		r.CachedShare = float64(reads) / float64(r.PromptTokens)
	}
	return r
}
