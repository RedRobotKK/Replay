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
consistent). The records sum to 193,337,607. Comparing them position by
position:

| turns | relation | n |
|---|---|---:|
| 0–101 | record equals the turn exactly | 102 |
| **102–109** | **record far below the turn** | **8** |
| 110–120 | record equals the turn exactly | 11 |
| 121–122, 126 | neither | 3 |
| 123–125, 127–130 | **record equals the *previous* turn** | 7 |

Two distinct defects, not one drift:

- **A tail lag.** From turn 123 the stream trails the ledger by exactly one
  turn. The last record carries turn 129's value and the ledger's final turn is
  0, so the closing turn's usage never reaches the stream at all.
- **An 8-turn hole.** Turns 102–109 account for **29,290,876 of the 29,519,212
  total gap: 99.2%**. It is a localized contiguous anomaly, not a systematic
  per-turn accounting difference.

Two candidate causes were tested and **both fail**:

- *Model-call volume.* Mismatching turns have median 9 `modelCalls` against 4
  for matching ones, but only 34% of turns with 17 or more calls mismatch, and
  one matching turn has 85. Not a discriminator.
- *Subagents.* 14 subagents start at turns 99–100, immediately before the band,
  which is suggestive. But subagents also start at turns 1, 19, 29, 30, 51 and
  124, and every one of those turns matches exactly. Not a discriminator.

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

The true usage of this session. The cause of the 8-turn hole. The semantics of
`numTurns` and of `usageIsIncomplete`. Where subagent tokens are accounted.
Whether the 09-06/09-11 boundary matches a specific Grok release; the installed
CLI is 1.0.41 and no changelog was consulted. Whether `events.jsonl` carries an
independent account; its 22MB were not parsed.

## 11. One next experiment

Parse `events.jsonl` for turns 102–109 of the comparison session and test
whether it accounts for the missing 29,290,876. It is the only unexamined
artifact large enough to hold a second account, and the band gives it a sharp,
falsifiable target rather than a whole-session comparison.

If it closes that band, `events.jsonl` becomes the reconstruction path and this
session's figure becomes estimable with a stated error. If it carries no usage,
581,144,282 stays NOT_MEASURED permanently and the source contract is the whole
story.
