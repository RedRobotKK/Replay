# 21. Unknown-model cache-read multiple is two questions

**Status:** Accepted
**Date:** 2026-09-11

## Context

The compiled price table has two cache-read multiples: `ReadMultiplier` = 0.10,
and `readMultiplierNewest` = 0.025 for the Fable/Mythos 5.1 tier.

`unknownModel` is the fallback for an unrecognised model id. It carries
`ReadMult: readMultiplierNewest` (0.025) and `priced: false`.

Two arguments pull that row in opposite directions, and they look like one
question because they share a field.

**For EffectiveTokens and policy comparison (the instrument).** The multiple is
not displayed like the dollar column, which is suppressed when a model is
unpriced. It enters `EffectiveTokens`, which the policy comparison is decided
on and which reports the label measured rather than estimated. Putting 0.10 on
an unknown model while current models read at 0.025 overstates every cached
token fourfold. The bias has a direction: it inflates the apparent value of
cache-preserving policies against cache-clearing ones. That is the tool puffing
itself. 0.025 is conservative **for the instrument**.

**For dollars (the invoice).** An unknown model is usually a new, more expensive
model. A dollar cap that treats it as cheap fails open on the invoice. Fail
conservative **for money** is refuse, or price at the most expensive known row.
Guessing a multiple so a dollar figure can be printed is the failure
[TOKEN-PRICES.md](../TOKEN-PRICES.md) named: the guard fails open, silently,
precisely when it matters most.

They are not the same question. One field was being asked to answer both.

## Decision

Keep the split. Do not collapse it.

**Dollars.** `PriceFor` returns `ok=false` for an unknown model. Do not guess a
list price. Already the behaviour; recorded here so it is not "fixed" by
putting a multiple on the row.

**EffectiveTokens / policy.** `unknownModel.ReadMult` stays
`readMultiplierNewest` (0.025). Do not put `ReadMultiplier` (0.10) on
`unknownModel`. Do not invent a third multiple.

## Consequences

An unknown model contributes cache-read weight at the newest, cheapest tier to
policy comparison, and contributes no dollar figure at all.

The failure direction is named per question. For the instrument, the failure we
accept is understating the benefit of keeping the cache, so the tool cannot
recommend itself. For money, the failure we accept is refusing to print a
dollar figure, so a cap cannot treat an unknown model as free or cheap.

A later reader who notices 0.025 looks too cheap for a new model is looking at
the money question, and the money question is already answered by
`priced: false`.

The pin is `TestUnknownModelReadMultiplePinsTheSplit` in
`internal/cachemodel/anthropic_test.go`: `unknownModel.ReadMult == 0.025` and
`PriceFor(unknown)` is `!ok`. If either side moves, that test fails, and the
record has to be superseded rather than silently rot.

## Alternatives considered

**Put 0.10 on `unknownModel`.** The older table-wide multiple. Overstates cached
tokens 4x in `EffectiveTokens` and inflates cache-preserving policies.
Rejected: that is the instrument puffing itself.

**Put 0.025 on `unknownModel` and also price it.** Makes the dollar column
appear. An unknown model is usually new and dearer; treating it as a
Fable/Mythos-tier cheap read fails open on the invoice. Rejected: money already
refuses.

**Invent a third multiple** (0, 1, or most-expensive-known as a read ratio). 0
makes cache reads free in `EffectiveTokens`, which is the instrument lying the
other way. 1 prices a cache read as an uncached token and would dominate every
policy comparison with a number nobody measured. Most-expensive-known is the
money-side conservative and belongs on the dollar path, which already returns
`ok=false`. Rejected: two questions, two answers, no third constant.

**Price unknown models at the most expensive known row.** Defensible for money
([TOKEN-PRICES.md](../TOKEN-PRICES.md) option 2). Not taken here: `PriceFor`
already returns `ok=false`, and guessing a dollar figure is a different
decision. This record is about the read multiple, not about whether to refuse
or to upper-bound.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
