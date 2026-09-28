# Experiment status

**Updated** 2026-09-28 · **Branch** `feat/perf-contract` · **Phase** DeepSeek measurement campaign

## Current state

| | |
|---|---|
| active experiment | none running |
| last completed | **F4 billing reconciliation: RESOLVED (A)** |
| live calls to date | ~1,074 |
| observed spend | **$0.74** (balance $46.89 -> $46.15) |
| billing reconciliation | **1.93% at n=200, within the 3% declared before the run** |

## Current hypothesis

Replay can identify provider-specific interventions that reduce cost with
outcome preserved, but a recommendation is only useful with its applicability
condition attached. Prefix ordering is the worked example: 90.4% cheaper when
the prefix recurs, exactly no effect when it does not.

## Blockers

1. ~~Derived cost over-states observed spend ~3x.~~ **CLEARED.** The gap was an
   analysis failure: a balance read before billing settled, compared against a
   derived figure covering only 440 of ~574 calls. See
   `deepseek/f4-reconciliation-2026-09-28.md`.
2. TTFT is unmeasurable without a streaming client, so no latency result can be
   attributed to prefill.
3. **Cache-hit pricing is unvalidated.** Both reconciliation batches ran with
   `cache_read` of exactly 0, so the cheapest rate in the table is the one
   entirely untested. Any saving claimed from caching rests on an unverified rate.

## Next actions, in order

**Awaiting explicit instruction before starting D1.**

1. D1 cache mechanics: minimum cacheable prefix, TTL, what invalidates.
2. Validate cache-hit pricing, which F4 could not: a reconciliation batch with a
   large shared prefix so `cache_read` dominates.
3. D7 model transfer: does `thinking:{type:disabled}` help on `deepseek-v4-pro`.

## Agents

None active. No sub-agents were launched this phase; all work ran in the parent
session, so there are no child runs to account for.

## Latest commits

`fbff0e1` F4 reconciliation · `4909c53` findings · `ae09b9a` campaign prereg · `dc64275` gap analysis ·
`d67d16b` surface map · `efc6bc2` cold/warm baseline
