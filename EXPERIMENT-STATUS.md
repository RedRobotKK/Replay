# Experiment status

**Updated** 2026-09-28 · **Branch** `feat/perf-contract`

## Status: PAUSED — awaiting credential rotation and human decision

No further DeepSeek credits are to be spent, no further DeepSeek task run, and no
WP-03 created, until the credential is rotated and a human decision is taken.

## Current state

| | |
|---|---|
| campaign status | **PAUSED** |
| active experiment | none |
| last completed | **WP-02: F1 classified CONTRACT GAP, test-only** |
| balance | **~$46.03** (from $46.89 at campaign start) |
| observed spend, campaign total | **~$0.86** |
| billing reconciliation | resolved A: 1.93% at n=200, inside the 3% declared before the run |
| credential | **NOT ROTATED** — see operational follow-up below |

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

## Operational follow-up: credential rotation

- The DeepSeek credential used by WP-01 and WP-02 **has not been rotated**.
- Key-management endpoints were probed and returned 404: `/user/api_keys`,
  `/api_keys`, `/v1/api_keys`, `/user/keys`.
- Rotation therefore requires the DeepSeek Platform web console. There is no API
  path available to this project.
- This is a human action. The credential is not printed, stored in this
  repository, or reproduced in any artifact.

## Next actions

**None authorised.** The campaign is paused. D1, cache-hit price validation and
model-transfer work remain unstarted and unauthorised.

## Agents

None active. No sub-agents were launched this phase; all work ran in the parent
session, so there are no child runs to account for.

## Latest commits

`fbff0e1` F4 reconciliation · `4909c53` findings · `ae09b9a` campaign prereg · `dc64275` gap analysis ·
`d67d16b` surface map · `efc6bc2` cold/warm baseline
