# 22. Unknown-model read multiple: the bias runs the other way

**Status:** Accepted (option A, implemented in #271, merged 2026-09-12)
**Date:** 2026-09-12
**Supersedes:** [0021](0021-unknown-model-cache-read-multiple.md), in its reasoning. The split it
draws is kept.

## Why this exists

ADR-0021 records a decision worth keeping and argues for it from two statements
that are wrong. Records are never edited here, so the correction is a new one.

**What 0021 gets right, and this keeps:** the dollars question and the
read-multiple question are separate, they have opposite failure directions, and
naming them separately is the contribution. `PriceFor` returning `!ok` for an
unknown model — refusing to guess a list price rather than inventing one — is
correct and stays.

## Correction 1: the ADR contradicts itself about who reads at 0.025

0021 states the table correctly in its Context:

> `readMultiplierNewest` = 0.025 **for the Fable/Mythos 5.1 tier**

and then argues from the opposite twelve lines later:

> Putting 0.10 on an unknown model **while current models read at 0.025**
> overstates every cached token fourfold.

Counted in `internal/cachemodel/anthropic.go` on 2026-09-12: **19 rows in the
table, 2 at `readMultiplierNewest` (0.025), 15 at `ReadMultiplier` (0.10).** The
two are `fable-5-1` and `mythos-5-1`. `opus-5`, `sonnet-5`, `haiku-4-5` and
every 4.x row read at 0.10.

So "current models read at 0.025" describes two rows out of nineteen, and the
row is assigned to a tier **no Claude model occupies**. `EffectiveTokens`
short-circuits foreign ids to input alone, so `unknownModel.ReadMult` is only
ever reachable for an **Anthropic-family** id the table does not know — a future
`claude-opus-6` — whose every nearest neighbour reads at 0.10.

## Correction 2: the direction of the bias is inverted

This is the load-bearing one, and it is checkable from one line of source:

```go
return float64(u.Input) + writeEquivalent(u) + float64(u.CacheRead)*ReadMultiplierFor(model)
```

A **higher** multiple makes a cached read cost **more** effective tokens. A
policy that preserves the cache is one with many reads, so a higher multiple
makes cache-preserving score **worse** — and cache-preserving layouts are what
`replay` recommends.

0021 says the opposite:

> Putting 0.10 on an unknown model … inflates the apparent value of
> cache-preserving policies against cache-clearing ones. That is the tool
> puffing itself. 0.025 is conservative **for the instrument**.

Worked over ten turns — a base preserving cache at 1k input + 100k read per
turn, a candidate clearing it at 101k input per turn — the cache-clearing
candidate scores **+818%** worse at 0.10 and **+2786%** worse at 0.025. The
lower multiple makes the tool's own advice look three times more valuable.

**Under 0021's own stated principle — the tool must not puff itself — the
conservative instrument choice is the higher multiple, which is the option it
rejects.**

## The decision this leaves open

The facts above do not settle it, and this ADR deliberately does not either.
Both remaining options are defensible and the choice is the owner's:

**A. Move the unknown row to `ReadMultiplier` (0.10).** Matches every neighbour
the row can actually be reached for, and follows 0021's own principle once the
direction is corrected. Costs: a genuinely cheaper future tier would be
overstated until the table learns it.

**B. Keep 0.025 and state the real reason.** There may be one — "the cheapest
number is the one an operator is least surprised by when a figure turns out
wrong" is a reasonable argument. It is simply a *different* argument from the
one 0021 makes, and it should be written as itself rather than as conservatism.

What is not defensible is the current state: the number kept for a reason that
the source refutes.

### Settled: A

The owner took **A**. `readMultiplierUnknown = ReadMultiplier` landed on `main`
in #271 on 2026-09-12, and the symbol now names the rule rather than the
Fable/Mythos tier, which is the `Consequences` requirement below.

The A/B text above is left exactly as written. It records that the choice was
genuinely open at the time, and that is the part a later reader needs; editing
it to read as though A were obvious would be the same defect this ADR exists to
correct.

Option B's cost, now carried: a genuinely cheaper future tier is overstated
until the table learns it. The guard against that is
`TestUnknownModelReadMultipleIsTheDearestInTheTable`, which derives its answer
from the table rather than pinning 0.10, so a new dearest tier moves it and a
failure names the reason.

## Consequences

Whichever is chosen, `readMultiplierNewest` is the wrong symbol at this call
site. It names a *tier* and is being used as a *rule* ("the conservative one"),
so the day a newer tier reads dearer than 0.10, the symbol still says `Newest`,
the record still says `Accepted`, and the argument silently inverts again. The
call site should name the rule it means.

Also unrecorded by 0021 and worth knowing: `activeRow` consults an installed
rules document **before** the compiled table in both `PriceFor` and
`ReadMultiplierFor`, so an operator's feed can make a model this ADR calls
unknown both priced and read at an arbitrary multiple. The decision here governs
the default, not the behaviour.

## How this was found

A twelve-seat review panel read #253 against the tree it describes. Two seats
reached correction 2 independently. Every figure above was re-derived from
source before being written down, because an ADR that can be refuted from the
repository it describes teaches readers to stop trusting the directory — which
is the cost 0021 is already carrying.
