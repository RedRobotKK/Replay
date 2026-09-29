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

## Harness hardening, 2026-09-28 (zero cost, no API calls)

| item | status |
|---|---|
| **F2** | **RECLASSIFIED NOT_MEASURED.** Tested parameter shape was not the documented control |
| **O1** truncation guard | **IMPLEMENTED.** `finish_reason` in `length`/`max_tokens` raises `Truncated` instead of returning a deliverable. No automatic retry; recovery needs `allow_truncated=True` |
| **O2** identity telemetry | **IMPLEMENTED.** `requested_model`, `model_returned`, `system_fingerprint`, `finish_reason` recorded per call. A fingerprint identifies a serving configuration, **not** model weights |
| **O3** explore/synthesize split | **NOT IMPLEMENTED — NEEDS DESIGN.** See below |
| cache | **untouched**, by decision. Already 93.4% / 77.3% |

**Why O3 was not implemented.** The harness has no agent loop to split.
`runner.py` exposes `run_arm`, which is single-shot: zero `tool_calls` handling
and zero round control. The loop that ran WP-01 and WP-02 lived in untracked
ad-hoc scripts. Implementing O3 means adding an agent-loop module to the harness,
which is an architectural change, so it was left for a design pass rather than
improvised.

Tests: `python3 experiment/harness/test_adapter.py` — 6 pass. Mutation-checked
twice: deleting the guard fails 2 tests; narrowing it to chat's `length`
vocabulary alone fails the Anthropic `max_tokens` case. Go suite unaffected at
30 ok / 0 FAIL, `go vet` and `gofmt` clean.

## Fan-out infrastructure, 2026-09-28 (zero cost, no API calls)

`experiment/harness/fanout.py`. Built because it is the shape the measurements
reward and the one we had never used: DeepSeek sustained 64 concurrent requests
with no throttling and median latency that FELL from 662 ms to 575 ms, and a
cached read costs 50x less than a miss on flash. Every DeepSeek call so far has
been sequential, which wasted that.

It also sidesteps both measured failure modes rather than mitigating them. Each
question is a single call with its own checker, so there is no loop to run away
in (WP-01 burned 28 rounds) and no synthesis turn for reasoning to starve
(three occurrences).

Controls, each mutation-checked:

- **Budget gates dispatch, not reporting.** Spend commits when a request is
  sent, so a ceiling checked afterwards is a report. Workers reserve before
  sending; refused questions are recorded, never dropped, because a silently
  shortened fan-out looks identical to one nobody answered.
- **Outcomes stay separate.** `ANSWERED`, `TRUNCATED`, `REFUSED_BUDGET`, and an
  undecided checker are four different facts. The pass-rate denominator is
  decided answers only: a call that ran out of room did not answer wrongly, it
  did not answer.
- **A checker that raises decides nothing.** It is recorded undecided rather
  than scored as the model being wrong.
- **Identity is carried.** `model_returned` and `system_fingerprint` are
  collected per run so a routing question is answerable from the record.

Tests: `python3 experiment/harness/test_fanout.py`, 6 pass, no credential
needed. Mutations killed: removing the dispatch gate fails 2, folding truncated
and refused calls into the pass rate fails 3.

**Not yet run against the live API.** Requires spend authorisation.

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
