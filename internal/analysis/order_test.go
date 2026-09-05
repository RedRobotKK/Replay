package analysis

import (
	"strings"
	"testing"
)

// Prefix-safe ordering: not "that broke your cache" but "do these in this
// order and it survives".
//
// The mechanism this rests on: a prompt cache matches on the longest common
// prefix, so anything that changes early content invalidates everything after
// it. In an agent session most work only appends — a tool result, a file read,
// another turn — and appending is free. A few things rewrite the front of the
// prompt: editing CLAUDE.md or a memory file, adding or renaming an MCP server
// (its tool definitions live in the system prompt), changing the system prompt
// itself. Each of those forces a full re-prefill.
//
// The ordering therefore matters for one reason only: N invalidating actions
// interleaved with appends cost N re-prefills, and the same N done together
// cost one. That is the whole optimisation, and it is worth stating plainly
// because it is not obvious and it is free.
//
//	O1  an ordering with no invalidating actions costs nothing
//	O2  interleaving is counted honestly, and batching beats it
//	O3  the recommended order preserves every action, exactly once
//	O4  the saving is priced from the same table every other figure uses
//	O5  it refuses to advise when it has no basis to
//	O6  appends may be reordered freely, but only among themselves

func TestO1_AppendOnlyPlanCostsNothing(t *testing.T) {
	// PASS: a plan that only appends reports zero invalidations and no saving.
	// FAIL: inventing a saving where there is nothing to save, which would
	// make every plan look improvable and the advice worthless.
	plan := []Action{
		{Name: "read main.go", Scope: ScopeAppend, Tokens: 4000},
		{Name: "run tests", Scope: ScopeAppend, Tokens: 1200},
		{Name: "read README", Scope: ScopeAppend, Tokens: 900},
	}
	got := OrderPlan(plan, 50_000, "claude-opus-5")
	if got.Invalidations != 0 {
		t.Errorf("invalidations = %d, want 0: appending never breaks a prefix", got.Invalidations)
	}
	if got.SavedTokens != 0 {
		t.Errorf("savedTokens = %d, want 0", got.SavedTokens)
	}
	if !got.AlreadyOptimal {
		t.Error("an append-only plan is already optimal and must say so")
	}
}

func TestO2_BatchingBeatsInterleaving(t *testing.T) {
	// PASS: the plan as given costs three re-prefills, the recommended order
	// costs one, and the saving is two prefixes' worth of input tokens.
	// FAIL: any count that does not match the mechanism — this is the number
	// the whole feature rests on.
	prefix := 60_000
	// Two of the rewrites are already adjacent. That matters: with every
	// rewrite separated, "one re-prefill per run" and "one per rewrite" give
	// the same answer, and a fixture that cannot tell them apart would let a
	// per-rewrite implementation pass.
	plan := []Action{
		{Name: "edit CLAUDE.md", Scope: ScopeSystem, Tokens: 300},
		{Name: "write memory note", Scope: ScopeMemory, Tokens: 200},
		{Name: "read main.go", Scope: ScopeAppend, Tokens: 4000},
		{Name: "add mcp server", Scope: ScopeTools, Tokens: 800},
		{Name: "run tests", Scope: ScopeAppend, Tokens: 1200},
	}
	got := OrderPlan(plan, prefix, "claude-opus-5")

	if got.InvalidationsAsGiven != 2 {
		t.Errorf("as given = %d re-prefills, want 2: the first two rewrites are adjacent and cost one between them, the third costs another", got.InvalidationsAsGiven)
	}
	if got.Invalidations != 1 {
		t.Errorf("recommended = %d re-prefills, want 1 (the three are contiguous)", got.Invalidations)
	}
	if want := 1 * prefix; got.SavedTokens != want {
		t.Errorf("savedTokens = %d, want %d (one re-prefill of a %d-token prefix)", got.SavedTokens, want, prefix)
	}
	if got.AlreadyOptimal {
		t.Error("a plan that can be improved must not report itself optimal")
	}
}

func TestO3_EveryActionSurvivesExactlyOnce(t *testing.T) {
	// PASS: the recommended order is a permutation of the input.
	// FAIL: a dropped or duplicated action. Advice that silently loses a step
	// is worse than no advice, and this is the cheapest way to be certain.
	plan := []Action{
		{Name: "a", Scope: ScopeAppend, Tokens: 1},
		{Name: "b", Scope: ScopeSystem, Tokens: 2},
		{Name: "c", Scope: ScopeAppend, Tokens: 3},
		{Name: "d", Scope: ScopeTools, Tokens: 4},
		{Name: "e", Scope: ScopeMemory, Tokens: 5},
	}
	got := OrderPlan(plan, 10_000, "claude-opus-5")
	if len(got.Order) != len(plan) {
		t.Fatalf("recommended %d actions for a plan of %d", len(got.Order), len(plan))
	}
	seen := map[string]int{}
	for _, a := range got.Order {
		seen[a.Name]++
	}
	for _, a := range plan {
		if seen[a.Name] != 1 {
			t.Errorf("action %q appears %d times in the recommendation, want exactly 1", a.Name, seen[a.Name])
		}
	}
}

func TestO4_SavingIsPricedFromTheSameTable(t *testing.T) {
	// PASS: the dollar figure equals savedTokens at the model's input price,
	// and an unpriced model reports tokens with no dollars rather than zero.
	// FAIL: a hardcoded rate, or $0.00 presented as a real answer for a model
	// this build cannot price — the difference between "free" and "unknown"
	// is the thing this project exists to keep.
	plan := []Action{
		{Name: "edit CLAUDE.md", Scope: ScopeSystem, Tokens: 100},
		{Name: "read", Scope: ScopeAppend, Tokens: 100},
		{Name: "add tool", Scope: ScopeTools, Tokens: 100},
	}
	got := OrderPlan(plan, 100_000, "claude-opus-5")
	if !got.Priced {
		t.Fatal("claude-opus-5 is in the price table and the saving must be priced")
	}
	if got.SavedUSD <= 0 {
		t.Errorf("savedUSD = %v, want a positive figure for %d saved tokens", got.SavedUSD, got.SavedTokens)
	}

	unknown := OrderPlan(plan, 100_000, "some-model-nobody-has-priced")
	if unknown.Priced {
		t.Error("an unpriced model must not report a dollar figure")
	}
	if unknown.SavedUSD != 0 {
		t.Errorf("savedUSD = %v for an unpriced model; it must be omitted, not zero", unknown.SavedUSD)
	}
	if unknown.SavedTokens != got.SavedTokens {
		t.Error("the token saving does not depend on whether we can price it")
	}
}

func TestO5_RefusesWithoutABasis(t *testing.T) {
	// PASS: an empty plan, or one with no cached prefix to protect, declines
	// to advise and says why.
	// FAIL: confident advice computed from nothing.
	if got := OrderPlan(nil, 50_000, "claude-opus-5"); got.Advice == "" || got.Invalidations != 0 {
		t.Error("an empty plan must decline rather than advise")
	}
	plan := []Action{
		{Name: "edit CLAUDE.md", Scope: ScopeSystem, Tokens: 100},
		{Name: "read", Scope: ScopeAppend, Tokens: 100},
		{Name: "add tool", Scope: ScopeTools, Tokens: 100},
	}
	got := OrderPlan(plan, 0, "claude-opus-5")
	if got.SavedTokens != 0 {
		t.Errorf("savedTokens = %d with no prefix to lose; there is nothing to save", got.SavedTokens)
	}
	if !strings.Contains(strings.ToLower(got.Advice), "no cached prefix") {
		t.Errorf("advice should say why there is nothing to save, got %q", got.Advice)
	}
}

func TestO6_AppendsKeepTheirRelativeOrder(t *testing.T) {
	// PASS: appends come out in the order given.
	// FAIL: reordering work the user sequenced deliberately. The optimisation
	// is allowed to move invalidating actions to the front; it is not allowed
	// to decide that reading a file before running tests was a mistake.
	plan := []Action{
		// Descending sizes on purpose: with equal counts, an implementation
		// that sorted by size would look order-preserving by accident.
		// Sizes deliberately not monotonic. Descending sizes would let an
		// implementation that sorts by size look order-preserving by accident.
		{Name: "step1", Scope: ScopeAppend, Tokens: 300},
		{Name: "invalidate", Scope: ScopeSystem, Tokens: 50},
		{Name: "step2", Scope: ScopeAppend, Tokens: 900},
		{Name: "step3", Scope: ScopeAppend, Tokens: 600},
	}
	got := OrderPlan(plan, 10_000, "claude-opus-5")
	var appends []string
	for _, a := range got.Order {
		if a.Scope == ScopeAppend {
			appends = append(appends, a.Name)
		}
	}
	want := []string{"step1", "step2", "step3"}
	if strings.Join(appends, ",") != strings.Join(want, ",") {
		t.Errorf("appends reordered to %v, want %v", appends, want)
	}
}

// O7: it reports a number and a truth tier, and never a boolean verdict.
//
// A red-team review on 2026-09-05 argued for cutting this idea in its original
// form — `would_break(action)` returning yes or no. The reasoning is worth
// keeping: a human running a report treats output as evidence and applies
// judgement, while an agent calling a boolean mid-flow treats it as a gate,
// and gates earn trust monotonically. Ten correct answers buy the eleventh
// unquestioned.
//
// The error modes are asymmetric in the direction Replay is blind to. A false
// "safe" costs one re-prefill — bounded, and exactly the waste the user was
// already paying. A false "breaks" makes the agent skip a legitimate read,
// which costs a worse engineering outcome: unbounded, silent, and invisible
// here, because Replay measures spend and not success. The metric improves
// while the work degrades, on instrumentation that cannot notice.
//
// So this returns arithmetic, not a verdict. And it labels that arithmetic
// `structural`: it is the consequence of a stated model applied to counts the
// caller supplied. It measures nothing and predicts nothing.
//
// PASS: a truth tier is present and is structural; no field offers a verdict.
// FAIL: a boolean an agent could gate on, or a tier claiming more than the
// method earns.
func TestO7_ReportsATierAndNeverAVerdict(t *testing.T) {
	plan := []Action{
		{Name: "edit CLAUDE.md", Scope: ScopeSystem, Tokens: 100},
		{Name: "read", Scope: ScopeAppend, Tokens: 100},
	}
	got := OrderPlan(plan, 50_000, "claude-opus-5")

	if got.Tier != "structural" {
		t.Errorf("tier = %q, want %q: this is arithmetic over supplied counts, not a measurement or a fit", got.Tier, "structural")
	}
	// The advice must say whose numbers these are. A reader who thinks Replay
	// measured the prefix will trust the figure more than it deserves.
	low := strings.ToLower(got.Advice + got.Basis)
	for _, want := range []string{"you supplied", "structural"} {
		if !strings.Contains(low, want) {
			t.Errorf("the result must say where its inputs came from; %q missing from basis/advice", want)
		}
	}
	if strings.Contains(low, "will break") || strings.Contains(low, "safe to") {
		t.Errorf("this must not phrase itself as a verdict an agent can gate on: %q", got.Advice)
	}
}
