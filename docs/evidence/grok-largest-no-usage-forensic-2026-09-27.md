# Why the largest session reports no usage

```text
RESULT: PARTIALLY_EXPLAINED
```

`grok usage` reports nothing for session `01a01136` because the session has no
`usage.json`, and that file is the only thing the command reads. The refusal is
fully explained, and the explanation is shallow.

What it does not settle is whether **581,144,282** is this session's usage. The
artifact that would confirm it was never written. Measuring the one session
where both surfaces exist shows they disagree by 13%, so the figure is
**NOT_MEASURED** rather than confirmed or refuted.

## 1. Raw artifact inventory

`~/.grok/sessions/%2FUsers%2Fdaniel%2FDevelopment%2FDoorKik/01a01136-57f8-7672-a9c6-b24e02d1e4c9`

| file | bytes |
|---|---:|
| `updates.jsonl` | 126,582,285 |
| `events.jsonl` | 22,178,957 |
| `chat_history.jsonl` | 5,116,387 |
| `rewind_points.jsonl` | 98,048 |
| `resources_state.json` | 7,382 |
| `signals.json` | 1,879 |
| `summary.json` | 1,105 |
| `background_tasks_manifest.json` | 543 |
| `terminal/` | 15 call logs, 51 bytes or empty |
| **`usage.json`** | **absent** |
| **`subagents/`** | **absent** |

Every file is dated **2026-08-31 00:41**.

## 2. Exact CLI behaviour

```text
$ grok usage 01a01136-57f8-7672-a9c6-b24e02d1e4c9
Error: No usage recorded for session '01a01136-…'.
```

Measured over all 48 sessions on this machine: `grok usage` **succeeds on 35 of
35 sessions that have a `usage.json` and fails on all 13 that do not.** Where it
succeeds its output equals the file. Presence of that one file predicts the
outcome perfectly.

## 3. Token-field analysis

653 of 15,657 lines carry usage, at path `params.update.usage`, all on method
`_x.ai/session/update` (653 of 895 such records). The other 14,762 lines are
`session/update` and carry none.

The usage object has **12** fields, not the 11 previously recorded here:
`apiDurationMs`, `cacheCreationTokens`, `cachedReadTokens`, `costUsdTicks`,
`inputTokens`, `modelCalls`, `modelUsage`, `numTurns`, `outputTokens`,
`reasoningTokens`, `totalTokens`, and **`usageIsIncomplete`**.

The schema is **identical** to that of a session that has a `usage.json`, which
rules out malformed or partial metadata as the cause.

| quantity | value |
|---|---:|
| sum of `inputTokens` over 653 records | **581,144,282** |
| max | 3,931,891 |
| last | 754,685 |
| records flagged `usageIsIncomplete` | 58 (64,583,632 tokens) |

## 4. Monotonicity

245 decreases across 652 transitions. **The records are not a running total.**

`numTurns` is not a turn index and its semantics are `NOT_MEASURED`: in the
comparison session it takes 19 distinct values (max 61) across 131 records,
while `turnCount` is 131. Grouping by it is meaningless, and a within-turn
cumulative model built on it reconstructs nothing.

## 5. Parent/child topology

This session has no `subagents/` and no child sessions, so duplicate
hierarchical accounting is not available as an explanation.

The comparison session has 33. **None of their 33 `child_session_id`s exist as
session directories on disk**, and `meta.json` carries `tool_calls`, `turns` and
`duration_ms` but **no token fields at all**. Subagent token usage has no
independent record anywhere in the corpus.

## 6. Competing explanations

| explanation | verdict | evidence |
|---|---|---|
| CLI source contract | **supported** | 35/35 succeed with the file, 0/13 without |
| artifact lifecycle | **supported** | §7 |
| serialization/materialization | **supported** | 653 usage records exist, unmaterialised |
| genuinely missing usage | **refuted** | the records exist with a full schema |
| malformed metadata | **refuted** | schema identical to sessions that work |
| cumulative vs delta | **refuted** | 245 decreases; the within-turn model fails |
| parent/child | **excluded here** | no subagents in this session |
| high model-call turns diverge | **refuted** | §8 |

## 7. Cross-artifact information gain

| | sessions | date range |
|---|---:|---|
| **with** `usage.json` | 35 | 2026-09-11 12:59 → 2026-09-27 15:14 |
| **without** | 13 | 2026-08-31 00:41 → 2026-09-06 14:18 |

**The split is perfect, with no overlap.** `usage.json` began being written
between 2026-09-06 and 2026-09-11. Sessions older than that boundary hold usage
in `updates.jsonl` that was never materialised into the file the CLI reads.

This is visible only by relating sessions to each other. No single artifact
shows it.

## 8. Strongest falsifier, tested

> "`grok usage` only reads `usage.json`, and this session has no `usage.json`."

It **completely explains the refusal** and no deeper primitive is needed for it.

It does not explain the number. In the only session where both surfaces exist,
131 usage records stand against 131 `usage.json` turns, and the turns sum
exactly to the recorded session total of 222,856,819 (the ledger is internally
consistent). The records sum to 193,337,607.

> **CORRECTED 2026-09-27, after the events.jsonl experiment.** The table and
> the "one-turn lag" originally printed here were produced by comparing the two
> files *positionally*. `usage.json` skips `turnNumber` 122 and 123, so every
> position after the gap was compared against the wrong turn. Under a timestamp
> join there is no lag, and the agreement count is **120 of 131**, not 113. The
> corrected decomposition is in
> `grok-events-jsonl-gap-experiment-2026-09-27.md`; the summary stands below.

Joining turns to records on timestamp rather than position, 120 of 131 agree and
10 differ. The 29,519,212 gap decomposes exactly:

| component | tokens | status |
|---|---:|---|
| band, `turnNumber` 103-110 | 29,290,876 | **unexplained** |
| `turnNumber` 124, a turn merging two records | +965,575 | explained |
| `turnNumber` 128, flagged `usageIsIncomplete` | +228,336 | explained |
| the orphan record belonging to turn 124 | -965,575 | explained |

Removing the explained components leaves the residual **exactly equal to the
8-turn band**. Two candidate causes for that band were tested and **both fail**:
model-call volume as a threshold (one *matching* turn has 85 calls) and subagent
presence (subagents also start on turns that agree exactly).

One co-occurrence is solid, tested as a 2x2 across all 131 turns: the stream
under-reports tokens only when it also under-reports `modelCalls`, with **zero
off-diagonal** (10 / 120 / 0 / 0). Session-wide the stream is short by 333 model
calls. That localises the defect to omitted calls rather than token arithmetic,
and is a co-occurrence, not a demonstrated mechanism.

The direction is consistent where it can be checked: in the 8-turn band the
ledger always exceeds the stream. That makes a bare `updates.jsonl` sum a
**plausible lower bound** rather than an overcount, which inverts the natural
worry about a large number. It does not make 581,144,282 a measurement.

## 9. Relation to the accounting-topology hypothesis

**Not supported.** The earlier framing expected a reconstructable topology. What
the artifacts show is a materialisation boundary in the vendor's file layout,
plus two unexplained stream defects. Source contract and version boundary, not
hidden structure.

## 10. NOT_MEASURED

The true usage of this session. The cause of the 8-turn hole, now the sole residual. Why 333 model calls are absent from the stream. The semantics of
`numTurns` and of `usageIsIncomplete`. Where subagent tokens are accounted.
Whether the 09-06/09-11 boundary matches a specific Grok release; the installed
CLI is 1.0.41 and no changelog was consulted. Whether `events.jsonl` carries an
independent account; its 22MB were not parsed.

## 11. One next experiment

**Run, and answered: UNEXPLAINED.** `events.jsonl` contains no token-bearing
field at any depth, so it cannot account for the band or any part of it. The
full result, including the corrections above, is in
`grok-events-jsonl-gap-experiment-2026-09-27.md`.

This closes the `events.jsonl` reconstruction path permanently. 581,144,282
stays NOT_MEASURED and the source contract is the whole story for the refusal.
