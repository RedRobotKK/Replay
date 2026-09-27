# The session `grok usage` will not report, 2026-09-27

```text
RESULT:
EXPLAINED
```

The command reports no usage because this session has no `usage.json`.
The token counts are per-turn fields on `turn_completed` events. The
command does not add them. That is the whole of the anomaly. It does not
require parent/child reconciliation.

No reader was written. The corpus was not modified.

## 1. Session

| | |
| --- | --- |
| ID | `01a01136-57f8-7672-a9c6-b24e02d1e4c9` |
| Path | `~/.grok/sessions/…/DoorKik/01a01136-57f8-7672-a9c6-b24e02d1e4c9` |
| Created | 2026-08-17T19:32:49Z |
| Last active | 2026-08-31T07:41:10Z |
| Model in `summary.json` | `grok-4.6` |
| `usage.json` | absent |
| `updates.jsonl` | present, 15,657 lines, 0 parse failures |
| `subagents/` | absent |
| `meta.json` | absent |

Other files present, none of them containing `inputTokens`: `events.jsonl`,
`chat_history.jsonl`, `rewind_points.jsonl`, `signals.json`,
`resources_state.json`, `background_tasks_manifest.json`, `summary.json`.
`summary.json` has no `parent_session_id`.

## 2. CLI

```text
command: grok usage 01a01136-57f8-7672-a9c6-b24e02d1e4c9
stdout:  empty
stderr:  Error: No usage recorded for session '01a01136-57f8-7672-a9c6-b24e02d1e4c9'.
exit:    1
```

`DIRECTLY_OBSERVED` this pass. The installed binary is `grok 1.0.41`.
The readable source of that command is not on disk. The binary contains the
error string next to `Failed to read usage for session`, and it contains the
filename `usage.json` in the same region as `summary.json` and `signals.json`.
The July 5 binary still in `~/.grok/downloads` contains none of those three
strings. `grok-1.0.25`, dated 2026-09-11, contains all three. The first
`usage.json` on this machine is a session created 2026-09-11T19:52:40Z.
Every session created on or before 2026-09-06 has no `usage.json`. Every
session created on or after 2026-09-11 has one.

`INFERRED`: this August session was written by a client that did not emit
`usage.json`, and the current command looks for that file. Which binary was
actually running on August 17 is `NOT_MEASURED` if some other build was
deleted.

## 3. What the log contains

`turn_completed` lines: 660. Of those, 653 have a usage object and 7 have
`usage: null`. No other event type carries `inputTokens`.

Fields on all 653 objects: `inputTokens`, `outputTokens`, `totalTokens`,
`cachedReadTokens`, `cacheCreationTokens`, `reasoningTokens`, `modelCalls`,
`apiDurationMs`, `modelUsage`, `numTurns`. `costUsdTicks` is on 595.
`usageIsIncomplete` is on 58, and it is true on all 58. Those 58 have no
`costUsdTicks`.

| Quantity | Value |
| --- | ---: |
| Sum of `inputTokens` | 581,144,282 |
| Sum of `outputTokens` | 978,298 |
| Sum of `totalTokens` | 582,122,580 |
| Sum of `cachedReadTokens` | 548,144,640 |
| `cacheCreationTokens` non-zero | 0 of 653 |
| Sum of `modelCalls` | 2,265 |
| Minimum input | 29,380 |
| Maximum input | 3,931,891 |

`input + output = total` on 653/653. `cachedReadTokens <= inputTokens` on
653/653. Adding the cache on top of input plus output matches 0/653.

The figure 581,882,542 is not this session. It is the sum of input on every
directory that has events and no `usage.json`: this session's 581,144,282
plus ten one-turn directories (17,370 to 581,528 each). Eleven directories,
not one.

## 4. Token semantics

Not cumulative. Across the 652 steps from one usage object to the next,
input falls 245 times, rises 407 times, and is equal 0 times. Every input
value is unique. Every `prompt_id` on these 653 objects is unique. A
difference between successive records is not a turn total. The record itself
is the turn total: one `turn_completed`, one usage object, one prompt id.

`usageIsIncomplete: true` on 58 objects. 53 of those have a `prompt_id`
beginning `subagent`. Five do not. Five `subagent` prompt ids are not marked
incomplete. The flag and the prompt prefix are related and not the same set.

## 5. Children

`subagent_spawned` and `subagent_finished`: 58 each. `effective_context_source`
is `new` on 7 and `resumed` on 51. All 58 `child_session_id` values are
absent from `~/.grok/sessions`. There is no child `usage.json` and no child
`updates.jsonl` to add.

The 58 turns whose prompt id begins `subagent` sum to 62,012,873 input
tokens. That quantity is already inside the 581,144,282. It is not a second
copy stored elsewhere, because the child directories are gone.

## 6. Authority

| Artifact | What it is here | Class |
| --- | --- | --- |
| `usage.json` | Not present | CLI_REPORTED is the error, not a number |
| `updates.jsonl` usage objects | One per completed turn that carried usage | EVENT_OBSERVED |
| `meta.json` / child ledgers | Not present | UNKNOWN as amounts |
| `grok usage` | Refuses the session | CLI_REPORTED |

The 581,144,282 is the sum of event fields. It is not a number the CLI
reported. It is not an invoice.

## 7. Information gain

`usage.json` alone: there is no file. That is sufficient for the error.
`updates.jsonl` alone: 653 per-turn usage objects, sum 581,144,282, 245
decreases, 58 marked incomplete. `grok usage` does not state that sum.
Relating the two adds only the negative fact that the command's refusal
corresponds to the missing file, which the command already says. Relating
the log to the session tree adds that the 58 named child sessions are not
on disk. That does not produce a reconstructed ledger. There is no equation
here that no single artifact states.

## 8. Explanations

| Explanation | Support | Against | Status |
| --- | --- | --- | --- |
| The CLI reports `usage.json`, and this session has none | Error reproduced. File absent. Binary strings. Date split around 2026-09-11 | The function was not decompiled | EXPLAINED for the message |
| The 581,144,282 is a cumulative total | A large number | 245 decreases, unique prompt ids, input+output=total on every object | Refuted |
| Several records are one turn | | 653 objects, 653 distinct prompt ids | Refuted |
| Duplicate events | | No repeated input value, no repeated prompt id | Refuted |
| Child ledgers make up the sum | 58 spawn records and 62,012,873 on subagent prompt ids, already inside the sum | All 58 child ids are absent | NOT_MEASURED as a reconstruction |
| The earlier parent/child identity | That identity needs a parent `usage.json` and child ledgers | This session has neither | Not the same mechanism |

## 9. Prior topology work

Unrelated to the nine-turn identity on `01a09207`. That case had a parent
ledger, child ledgers, and an equation between them. This case has events
and a command that refuses them. Forcing them into one primitive would hide
that the command's behavior is the missing file.

## 10. Claim boundary

`grok usage` does not report this session. `DIRECTLY_OBSERVED`.

The log's per-turn input sums to 581,144,282. `DIRECTLY_OBSERVED`.

That sum is not what the runtime reported, and it is not a bill.
`DIRECTLY_OBSERVED` for the first, `NOT_MEASURED` for the second.

Why the August client did not write `usage.json` beyond "the binaries still
on disk gained that filename on 2026-09-11" is `INFERRED` from dates and
strings, not from a writer log.

## 11. NOT_MEASURED

Which binary wrote the August session. What the 58 missing child sessions
contained. Whether `usageIsIncomplete` would have changed a ledger that was
never written. Any dollar amount.

## 12. Next experiment

One only. On a session created after 2026-09-11, delete nothing, and compare
`grok usage` with the presence of `usage.json` after a turn whose prompt id
begins `subagent` and whose object has `usageIsIncomplete: true`. The
question is whether that flag suppresses the ledger or only the cost field.
This August session cannot answer it, because the ledger file does not exist.
