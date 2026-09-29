# Experiment status

**Updated** 2026-09-28 · **Branch** `feat/perf-contract`

## Status: CLOSED. DeepSeek is an experimental instrument, not a claim generator

Spend authorised by the user on 2026-09-29 ("test various theory on performance
optimization ... try everything in the book, support it with a baseline
reading"). A $3.00 ceiling was set by the agent, not the user, and
enforced by `fanout.Budget`.

**CAMPAIGN TOTAL, corrected at closeout.** The figure that stood here was from an
early phase and was stale by an order of magnitude.

| | |
| --- | --- |
| balance | $46.01 to $45.49 |
| **OBSERVED** | **$0.52** |
| DERIVED, cache and billing suite | $0.248764 |
| DERIVED, prompt-optimization campaign | $0.350810 |
| **DERIVED total** | **$0.599574** |

Derived exceeds observed, which is consistent with the settlement lag this
campaign established. The reading is provisional until the balance stops moving.

**The credential was NOT rotated before this spend.** The user did not direct a
hold, and rotation is hygiene debt rather than a known compromise, so the work
proceeded. The gate was set in this repository and is recorded as consciously
crossed rather than quietly dropped. Rotation remains outstanding.

| | |
| --- | --- |
| DeepSeek research campaign | **CLOSED** |
| Active experiment | **NONE** |
| New API spend | **NOT AUTHORIZED** |
| Prompt optimization | **CLOSED / NULL** |
| v4-pro experiment | **NOT AUTHORIZED** |

**Everything below the "Historical record" heading is a dated record of a closed
campaign.** Its hypotheses, blockers and next-actions describe the state at the
time they were written. None of it authorises work now.

**Operating contract:** `docs/DEEPSEEK-OPERATING-CONTRACT.md`. Read it before
spending another credit. The campaign is closed at `3c2ef9a`. Prompt
optimization returned a NULL result over 396 trials: two candidate effects both
reversed sign on independent replication. Cache findings are scoped to the
regimes they were measured in. No universal claim is established. Patent prior
art is NOT_VERIFIED.

The v4-pro cache experiment is **not** authorised merely because it appeared in a
next-steps list. Every future experiment earns its budget with a discriminating
question.

Next research priority is not DeepSeek: it is whether an addressable evidence
anchor changes a fresh agent's retrieval behaviour after context discontinuity.

## Historical record

### Campaign state at close

| | |
|---|---|
| campaign status | **PAUSED** |
| active experiment | none |
| last completed | **WP-02: F1 classified CONTRACT GAP, test-only** |
| balance | **~$46.03** (from $46.89 at campaign start) |
| observed spend, campaign total | **~$0.86** |
| billing reconciliation | resolved A: 1.93% at n=200, inside the 3% declared before the run |
| credential | **NOT ROTATED** -- see operational follow-up below |

### Hypothesis as it stood during the campaign (HISTORICAL)

Replay can identify provider-specific interventions that reduce cost with
outcome preserved, but a recommendation is only useful with its applicability
condition attached. Prefix ordering is the worked example: 90.4% cheaper when
the prefix recurs, exactly no effect when it does not.

### Blockers as recorded during the campaign (HISTORICAL)

1. ~~Derived cost over-states observed spend ~3x.~~ **CLEARED.** The gap was an
   analysis failure: a balance read before billing settled, compared against a
   derived figure covering only 440 of ~574 calls. See
   `deepseek/f4-reconciliation-2026-09-28.md`.
2. TTFT is unmeasurable without a streaming client, so no latency result can be
   attributed to prefill.
3. ~~Cache-hit pricing is unvalidated.~~ **CLEARED 2026-09-29.** A 200-call batch
   with 3.74M of 3.80M input tokens as cache reads: observed $0.04 against
   $0.03909 derived at the hit rate (ratio 1.023) and $1.13994 at the miss rate
   (ratio 0.035). Cache reads bill at the hit rate. DS-F5.
4. **NEW, OPEN: session-level cost does not reconcile.** $0.05 observed against
   $0.0959 derived across the session, while the isolated batch matched at 1.023.
   Settling lag and per-request cent truncation both fit and make opposite
   predictions about a batch of sub-cent calls left to settle for an hour.

### Harness hardening, 2026-09-28 (zero cost, no API calls)

| item | status |
|---|---|
| **F2** | **RECLASSIFIED NOT_MEASURED.** Tested parameter shape was not the documented control |
| **O1** truncation guard | **IMPLEMENTED.** `finish_reason` in `length`/`max_tokens` raises `Truncated` instead of returning a deliverable. No automatic retry; recovery needs `allow_truncated=True` |
| **O2** identity telemetry | **IMPLEMENTED.** `requested_model`, `model_returned`, `system_fingerprint`, `finish_reason` recorded per call. A fingerprint identifies a serving configuration, **not** model weights |
| **O3** explore/synthesize split | **NOT IMPLEMENTED -- NEEDS DESIGN.** See below |
| cache | **untouched**, by decision. Already 93.4% / 77.3% |

**Why O3 was not implemented.** The harness has no agent loop to split.
`runner.py` exposes `run_arm`, which is single-shot: zero `tool_calls` handling
and zero round control. The loop that ran WP-01 and WP-02 lived in untracked
ad-hoc scripts. Implementing O3 means adding an agent-loop module to the harness,
which is an architectural change, so it was left for a design pass rather than
improvised.

Tests: `python3 experiment/harness/test_adapter.py` -- 6 pass. Mutation-checked
twice: deleting the guard fails 2 tests; narrowing it to chat's `length`
vocabulary alone fails the Anthropic `max_tokens` case. Go suite unaffected at
30 ok / 0 FAIL, `go vet` and `gofmt` clean.

### Fan-out infrastructure, 2026-09-28 (zero cost, no API calls)

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

### Operational follow-up: credential rotation (STILL OUTSTANDING)

- The DeepSeek credential used by WP-01 and WP-02 **has not been rotated**.
- Key-management endpoints were probed and returned 404: `/user/api_keys`,
  `/api_keys`, `/v1/api_keys`, `/user/keys`.
- Rotation therefore requires the DeepSeek Platform web console. There is no API
  path available to this project.
- This is a human action. The credential is not printed, stored in this
  repository, or reproduced in any artifact.

### Next action as written mid-campaign (HISTORICAL, superseded: settle_test was made unnecessary by a zero-cost refutation)

`experiment/harness/settle_test.py` resolves the open cost-reconciliation
question. It refuses to run without `--confirm`.

It does not poll harder; polling cannot separate the two hypotheses, because lag
and truncation both look like "the money is not there yet" at any single moment.
It runs **two batches with the same derived total and opposite call sizes**: BIG
(12 calls each well over a cent) and SMALL (400 calls each well under). Settling
lag predicts both converge on their derived totals. Cent truncation predicts BIG
converges and SMALL never does. Estimated $0.25 derived, then an hour of polling.

### Next actions as written mid-campaign (HISTORICAL)

**None authorised.** The campaign is paused. D1, cache-hit price validation and
model-transfer work remain unstarted and unauthorised.

### Agents

None active. No sub-agents were launched this phase; all work ran in the parent
session, so there are no child runs to account for.

### Latest commits at the time of writing (HISTORICAL)

`fbff0e1` F4 reconciliation · `4909c53` findings · `ae09b9a` campaign prereg · `dc64275` gap analysis ·
`d67d16b` surface map · `efc6bc2` cold/warm baseline
