# Caching across providers, and what that means for the engine

**2026-09-04.** Written after ADR-0007, which this partly corrects.

## The mistake in the current design

`internal/cachemodel` is one file, `anthropic.go`. There is no provider interface. The package
exports `MinCacheablePrefix(model)`, `ClassifyRead(prev, cur)`, `TTLOf(usage)` and
`WriteMultiplier(ttl)` as free functions, and the ledger's usage shape carries `CacheCreation`,
`CacheRead` and `ContextManagement` because that is what one provider returns.

**Replay has modelled one mechanism and called it prompt caching.** The assumption is baked into
three layers, not one: the model, the ledger schema, and the analysis that reads it.

That is a defensible v0.1 decision and a bad v1 foundation, because the mechanisms are not variations
on a theme. They are three different products with three different failure modes.

## Three families, not one spectrum

| | **Explicit breakpoint** | **Implicit prefix** | **Rented cache** |
|---|---|---|---|
| Who decides what is cached | You do, by marking | Nobody, it just happens | You do, by creating an object |
| What you control | Placement, TTL | Prefix stability, routing hints | TTL, and what goes in |
| How you pay | Premium on the write | Nothing extra | **Rent, per unit time** |
| How you lose money | Writing a cache you never read | You do not, you only fail to save | **Renting a cache you underuse** |
| The optimisation problem | **Placement** | **Hygiene** | **Utilisation** |

**The third column is the one that breaks the current engine's assumptions.** In a rented model
caching is a bet: you pay storage for a window, and if you do not reuse the content enough before it
expires you are worse off than not caching. Replay's advice today assumes more caching is better,
which is true in the first two families and false in the third.

The second column matters differently. With no markers, the only lever is **not perturbing the
prefix**, so the useful output is not "put a breakpoint here" but "this is what changed, and here is
the turn it changed on". Replay already computes that, `replay diff`, and on an implicit provider it
is not a diagnostic, it is **the entire product**.

## The correct decomposition

Three things are currently fused and should not be.

**1. Mechanism strategy.** What can be controlled, what an intervention costs, and what advice is
even meaningful. One implementation per family, not per provider.

**2. Provider semantics as versioned data, not Go constants.** Today `RulesVersion` is a const in a
compiled file, so a provider changing a number needs a release. It should be a dated document the
binary loads and can be corrected without shipping a binary, because **these numbers change faster
than release cycles do.**

**3. A normalised usage record, with the raw payload kept.** Every provider reports cache hits in its
own shape and under its own field names. The engine should read one normalised shape; the ledger
should also keep the provider's own object verbatim, because a field we did not know mattered is
exactly what tomorrow's calibration needs.

```
                    ┌─────────────────────────┐
   provider response│  raw usage, kept as-is  │
                    └────────────┬────────────┘
                                 │ adapter, one per provider
                    ┌────────────▼────────────┐
                    │  normalised usage       │  prompt, cached_read,
                    │                         │  cached_write, rent, model, ts
                    └────────────┬────────────┘
                                 │
              ┌──────────────────▼──────────────────┐
              │ mechanism strategy                  │
              │ explicit | implicit | rented        │
              └──────────────────┬──────────────────┘
                                 │ reads
                    ┌────────────▼────────────┐
                    │ rules document, dated   │  minimum prefix, TTLs,
                    │ + empirical bounds      │  multipliers, granularity
                    └─────────────────────────┘
```

## The part that matters most, and it is not the abstraction

**Every published number in a rules document is a claim, and Replay is the only tool in this space
positioned to check them.**

The 2026-09-03 corpus already did it by accident. The rules said the minimum cacheable prefix was
512 tokens; observation bounded it at **at most 40,563**, with nothing uncached ever seen below.
Either the published figure is wrong, or stale, or right but untestable from that sample. **The tool
found a disagreement between documentation and behaviour on eleven sessions from one laptop.**

So the rules document should carry, per field, both what the provider says and what has been
observed:

```yaml
provider: example
rules_version: example-2026-09-01
mechanism: implicit_prefix
models:
  - id: example-model
    min_cacheable_prefix:
      documented: 1024
      observed: { upper_bound: 40563, lower_bound: null, sessions: 11, machines: 1 }
      status: unverified        # unverified | consistent | contradicted
    cache_granularity:
      documented: 128
      observed: { status: untested }
```

**`status: contradicted` is the most valuable field in this design.** It is the thing no provider
dashboard will ever show you, and it is only reachable by replaying real traffic.

## What this changes about ADR-0007

ADR-0007 assumed one learning problem. There are three, and they need different data.

**Explicit:** learn breakpoint placement. Needs sessions with varied structure. Depth helps.

**Implicit:** learn what perturbs a prefix, and whether routing affinity holds. **This needs breadth
above everything** — the interesting variable is client diversity, because the causes are client
behaviours: a re-rendered history, a reordered tool list, a timestamp in a system prompt. Twenty
machines running the same client teach you almost nothing here. Twenty different clients teach you
the taxonomy.

**Rented:** learn utilisation curves. Needs long sessions and, critically, **the counterfactual**:
what the same work would have cost uncached. That is the only family where Replay would have to model
a decision the user might regret, rather than a saving they missed.

ADR-0007's k-anonymity and robust-statistics rules still hold. Its assumption that one aggregate
improves one model does not.

## Sequencing, and the honest constraint

**Do not build the abstraction first.** A provider interface with one implementation is a guess about
the second one, and this design's whole argument is that documented behaviour and real behaviour
diverge. The order that respects that:

1. **Normalise the usage record now**, and keep the raw payload. This is cheap, it is not a guess, and
   without it a second provider cannot be added at all.
2. **Move the rules out of Go and into a dated document**, with `documented` and `observed` per field.
   This is the change that makes the tool a measuring instrument rather than a calculator.
3. **Add the second provider only when there is real traffic to calibrate against**, and let its
   awkwardness dictate the interface. The first family that is not explicit-breakpoint will break
   assumptions that are currently invisible.
4. **Then** generalise the corpus per mechanism family.

## What is not settled

**Specific published figures for any provider, including the one already implemented, are not settled here.** Minimums,
TTLs, granularities, discounts and eligibility rules change on a cadence faster than this document
will be revised, and several are not documented precisely anywhere.

**That uncertainty is the reason for the design, not an argument against it.** Any architecture that
hardcodes a provider's numbers in compiled constants is wrong on a timescale of months. The one
proposed here treats every number as a dated claim with an observation attached, which is the only
form that survives being out of date: it can be **contradicted by evidence** rather than silently
believed.
