# Experiment status

**Updated** 2026-09-28 · **Branch** `feat/perf-contract` · **Phase** DeepSeek measurement campaign

## Current state

| | |
|---|---|
| active experiment | none running |
| last completed | D6 interventions: reasoning control, prefix ordering x recurrence |
| live calls to date | 440 |
| observed spend | $0.05 (balance $46.89 -> $46.84) |
| derived spend | $0.1458 — **2.9x the observed figure, unresolved** |

## Current hypothesis

Replay can identify provider-specific interventions that reduce cost with
outcome preserved, but a recommendation is only useful with its applicability
condition attached. Prefix ordering is the worked example: 90.4% cheaper when
the prefix recurs, exactly no effect when it does not.

## Blockers

1. **Derived cost over-states observed spend ~3x.** No DeepSeek dollar figure
   should ship until this is explained. Highest priority.
2. TTFT is unmeasurable without a streaming client, so no latency result can be
   attributed to prefill.

## Next actions, in order

1. Resolve F4. Controlled test: record balance, spend a known amount well above
   the cent floor, wait, re-read. Distinguishes billing lag from a wrong rate.
2. D1 cache mechanics: minimum cacheable prefix, TTL, what invalidates.
3. D7 model transfer: does `thinking:{type:disabled}` help on `deepseek-v4-pro`.

## Agents

None active. No sub-agents were launched this phase; all work ran in the parent
session, so there are no child runs to account for.

## Latest commits

`4909c53` findings · `ae09b9a` campaign prereg · `dc64275` gap analysis ·
`d67d16b` surface map · `efc6bc2` cold/warm baseline
