package analysis

import (
	"fmt"
	"sort"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// Prefix-safe ordering: the order to do a set of planned actions in, so the
// cached prefix survives as much of it as possible.
//
// The mechanism is narrow and worth stating plainly, because it is not obvious
// and the saving is free. A prompt cache matches on the longest common prefix.
// Most agent work only appends — a tool result, a file read, another turn —
// and appending costs nothing. A few things rewrite the front of the prompt:
// editing CLAUDE.md or a memory file, adding or renaming an MCP server, whose
// tool definitions live in the system prompt. Each of those forces a full
// re-prefill of everything cached.
//
// So the order matters for exactly one reason: N such actions interleaved with
// appends cost N re-prefills, while the same N done together cost one.
//
// What this deliberately does NOT do is decide whether an action is worth
// taking. An earlier shape of this idea answered `would_break(action)` with a
// boolean, and a review argued convincingly for cutting it: a human reading a
// report applies judgement, but an agent calling a boolean mid-flow treats it
// as a gate, and gates earn trust monotonically. The error modes are
// asymmetric in the direction Replay cannot see — a wrong "safe" costs one
// re-prefill, bounded; a wrong "breaks" makes the agent skip a legitimate read,
// which costs a worse engineering outcome that is unbounded, silent, and
// invisible to a tool that measures spend rather than success. The bill would
// fall while the work got worse, on instrumentation incapable of noticing.
//
// This therefore returns arithmetic and a truth tier, never a verdict.

// Scope is what an action touches, which is the only thing that decides
// whether it invalidates the cached prefix.
type Scope int

const (
	// ScopeAppend adds to the end of the conversation. Free.
	ScopeAppend Scope = iota
	// ScopeSystem rewrites the system prompt, including CLAUDE.md.
	ScopeSystem
	// ScopeTools changes tool definitions, which live in the system prompt —
	// so adding, removing or renaming an MCP server lands here. This is the
	// one people do not expect, and it is why a tool that reports on prompt
	// caching has to be careful about its own tool list.
	ScopeTools
	// ScopeMemory rewrites a memory file loaded into every session.
	ScopeMemory
)

// invalidates reports whether an action rewrites cached content.
func (s Scope) invalidates() bool { return s != ScopeAppend }

// Action is one planned step. Tokens and Scope are supplied by the caller:
// Replay does not forecast how large a tool result will be, and does not guess
// what an action touches.
type Action struct {
	Name   string
	Scope  Scope
	Tokens int
}

// PlanOrder is the result: a recommended order and what it saves.
type PlanOrder struct {
	Order                []Action
	Invalidations        int
	InvalidationsAsGiven int
	SavedTokens          int
	SavedUSD             float64
	Priced               bool
	AlreadyOptimal       bool
	// Tier is always "structural". This is the arithmetic consequence of a
	// stated model applied to numbers the caller provided; nothing here was
	// measured off a wire or fitted to observations.
	Tier   string
	Basis  string
	Advice string
}

// OrderPlan recommends an order for a plan.
//
// prefixTokens is how much cached prefix is at risk, and the caller supplies
// it. Replay will not infer it: the byte-to-token fit carries error bars up to
// ±159% across sessions, and a figure derived from that has no business
// driving a recommendation.
func OrderPlan(plan []Action, prefixTokens int, model string) PlanOrder {
	out := PlanOrder{
		Tier: "structural",
		Basis: "structural: counts and scopes you supplied, times one re-prefill each. " +
			"Nothing here was measured off the wire or fitted to observations.",
	}
	if len(plan) == 0 {
		out.AlreadyOptimal = true
		out.Advice = "No actions to order."
		return out
	}

	out.InvalidationsAsGiven = countInvalidations(plan)

	// Move every invalidating action to the front, keeping appends in the
	// order the caller wrote them. Reordering an invalidation is safe: it
	// rewrites the prefix wherever it sits. Reordering the work is not — a
	// person who reads a file before running tests meant that, and a cache
	// optimiser has no standing to overrule it.
	ordered := make([]Action, 0, len(plan))
	idx := make([]int, len(plan))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ia, ib := plan[idx[a]], plan[idx[b]]
		return ia.Scope.invalidates() && !ib.Scope.invalidates()
	})
	for _, i := range idx {
		ordered = append(ordered, plan[i])
	}
	out.Order = ordered
	out.Invalidations = countInvalidations(ordered)

	saved := out.InvalidationsAsGiven - out.Invalidations
	if saved > 0 && prefixTokens > 0 {
		out.SavedTokens = saved * prefixTokens
	}
	out.AlreadyOptimal = out.InvalidationsAsGiven == out.Invalidations

	if price, ok := cachemodel.PriceFor(model); ok && out.SavedTokens > 0 {
		out.Priced = true
		out.SavedUSD = float64(out.SavedTokens) / 1e6 * price.InputPerMTok
	}

	switch {
	case prefixTokens <= 0:
		out.Advice = "No cached prefix to protect, so there is nothing to save by reordering."
	case out.AlreadyOptimal:
		out.Advice = fmt.Sprintf("Already grouped: %d re-prefill(s) either way.", out.Invalidations)
	default:
		out.Advice = fmt.Sprintf(
			"Doing the %d prefix-rewriting action(s) together costs %d re-prefill(s) instead of %d, "+
				"leaving %d tokens cached that would otherwise be re-billed.",
			countInvalidatingActions(plan), out.Invalidations, out.InvalidationsAsGiven, out.SavedTokens)
	}
	return out
}

// countInvalidations counts re-prefills: one per run of invalidating actions.
// Two rewrites in a row cost one; the same two split by an append cost two.
func countInvalidations(plan []Action) int {
	n, inRun := 0, false
	for _, a := range plan {
		if a.Scope.invalidates() {
			if !inRun {
				n++
				inRun = true
			}
			continue
		}
		inRun = false
	}
	return n
}

func countInvalidatingActions(plan []Action) int {
	n := 0
	for _, a := range plan {
		if a.Scope.invalidates() {
			n++
		}
	}
	return n
}
