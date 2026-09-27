# Grok accounting topology, 2026-09-27

No reader was written. Claude's branch was not modified. The question is
whether parent events, child-session ledgers, and the parent `usage.json`
stand in a deterministic relationship, and whether that relationship is
information that no one of those files states.

## 1. Executive finding

On the only parent in this corpus, time-aligned rows obey a checkable
identity. For an ordinary turn, the parent `usage.json` turn equals the
`turn_completed` usage object at the same second. For nine turns, the parent
`usage.json` turn equals that event plus the child-session ledger totals
whose `completed_at` falls in the window since the previous event. Those
nine additions are 29,519,212 input tokens, which is the whole of

```text
222,856,819 - 193,337,607 = 29,519,212
```

A row-index "lag" does not survive a join on timestamp. It is what you see
if you compare row 123 to row 123 after two clocks differ by one second and
one usage turn is the sum of two events.

This identity is demonstrated on one parent. This machine has no second
parent with a `subagents/` directory. It is not a law of Grok. It is a
reconstruction that neither file contains as a sentence.

## 2. Artifact inventory

48 session directories outside `subagents/`.

| Shape | Count |
| --- | ---: |
| `usage.json` and `updates.jsonl` | 35 |
| `updates.jsonl` only | 13 |
| `usage.json` only | 0 |
| Directory with `subagents/` | 1 |

The 35 ledgers are not 35 independent parents. They are:

| Role | Count | Turns |
| --- | ---: | --- |
| Parent `01a09207-8168-7802-b465-0bedbf66f83a` | 1 | 131 usage turns, 131 events with usage, 2 `turn_completed` events with `usage: null` (`cancelled`, `error`) |
| Its children | 33 | 1 turn each. Each child's `usage.json` session totals equal that child's own event sums, on input and on output |
| Other multi-turn ledger, no `subagents/` | 1 | `01a0e417`, the session that wrote these notes |
| CLI report absent | 13 | Includes one directory of 653 events summing to 581,144,282 input tokens. `grok usage` on that id exits 1, `No usage recorded` |

`DIRECTLY_OBSERVED`.

## 3. Parent and child lineage

Every child `meta.json` has `parent_session_id` equal to `01a09207-…`.
Every child `effective_context_source` is `new`. None of the 33
`subagents/<id>/` directories contains `updates.jsonl`. Each contains
`meta.json` and `output.json`. The child's accounting files are the session
directory named by `child_session_id`, not the subdirectory.

The parent's `summary.json` has no `parent_session_id`.

`grok usage` of a session that has `usage.json` reproduces that file. That
was reproduced on the frozen fixtures, including child `01a09421` (1,764,978).
It was not re-run on all 33 children in this pass. `INFERRED` for the rest,
from the file shape, not from 33 command runs.

## 4. Exact arithmetic

Join key: timestamp truncated to a second. 118 of 131 events meet a usage
turn at that second with equal input, output, and total. Two more pairs are
the same numbers one second apart:

| usage `endedAt` | event time | input | output |
| --- | --- | ---: | ---: |
| 05:39:16 | 05:39:17 | 422,383 | 5,321 |
| 05:40:43 | 05:40:44 | 440,613 | 3,196 |

One usage turn is the sum of two events. Event at 23:36:37 is 965,575 input
and 3,516 output, and has no usage row at that second. The usage turn at
23:37:59 is 1,464,171 input and 4,451 output. The event at that second is
498,596 and 935. `965,575 + 498,596 = 1,464,171` and `3,516 + 935 = 4,451`.

One usage turn is 0 input and 0 output at 01:01:47, with no event. The event
at 01:01:46 matches the usage turn at 01:01:46. The two `usage: null` events
are in neither sum.

Nine usage turns are not equal to the event at the same second. Each equals
that event plus the child ledgers completed after the previous event and no
later than this one. Input, and for the eight rows checked field-by-field,
output, total, cached read, and `costUsdTicks`:

| Event time | Event input | Child input | Usage input |
| --- | ---: | ---: | ---: |
| 05:41:50 | 462,700 | 344,971 + 305,221 | 1,112,892 |
| 05:42:13 | 469,527 | 288,832 + 476,175 | 1,234,534 |
| 05:43:30 | 478,135 | 673,076 + 1,395,085 | 2,546,296 |
| 05:46:22 | 484,050 | 5,314,013 | 5,798,063 |
| 05:47:34 | 990,407 | 2,008,514 + 1,547,843 + 1,652,233 + 909,139 | 7,108,136 |
| 05:49:12 | 330,080 | 1,764,978 | 2,095,058 |
| 05:51:49 | 330,050 | 7,085,694 | 7,415,744 |
| 05:54:52 | 837,407 | 5,525,102 | 6,362,509 |
| 00:35:21 | 268,260 | 228,336 | 496,596 |

The nine child sums are 29,519,212. The ninth row's output also adds:
`251 + 5,537 = 5,788`. Its event has no `costUsdTicks`, so cost was not
checked on that row. The eight earlier rows matched cost as well.

Twenty-four other children, completed on 2026-09-11, fall in windows where
the usage turn equals the event. They are not in the 29,519,212. Their
tokens are not added on top of those parent turns.

`DIRECTLY_OBSERVED` for the arithmetic. The word for it is the equation, not
"excess" and not "shortfall."

## 5. Hypothesis tests

| Id | Claim | Result |
| --- | --- | --- |
| H1 | Parent usage turn = parent event + child ledgers, for every child | Refuted. True for nine windows. False for the five 2026-09-11 windows, where children completed and the usage turn still equals the event. |
| H2 | The same quantity stored twice, so parent usage equals the child ledger | Refuted on the nine rows. Parent usage equals event + child, not the child alone. True as a second copy: those child ledgers also exist as their own `usage.json`. Summing every `usage.json` counts the 29,519,212 once in the parent and once in the children. |
| H3 | Events are not 1:1 with usage turns | Supported as a count of exceptions, not as the 29,519,212. Exceptions: two null-usage events, two clocks one second apart, one usage turn equal to two events, one usage turn of zeros. |
| H4 | Compaction or inherited history | Refuted as the source of the 29,519,212. The compact is one event after 61 turns, context 401,088 to 8,080. The nine rows are later. No `parent_session_id`. Every child meta says `effective_context_source` `new`. |
| H5 | A write-order lag explains the sum | Refuted. Joined on time, 120 pairs are the same numbers (118 at the same second, 2 at one second). The index lag is that join done on row number after those insertions. |
| H6 | More than one mechanism | Supported. Ordinary equality, plus nine additive child windows, plus the small set of alignment exceptions in H3. The sum 29,519,212 is only the nine windows. |

## 6. Cross-session replication

The additive identity cannot be repeated on a second parent. There is no
second `subagents/` directory on this machine.

| Control | Result |
| --- | --- |
| One-turn child ledgers | 33/33, usage session totals equal that child's event sums |
| Multi-turn ledger with no children | `01a0e417`. In the equal set: usage sum equals event sum. Not a test of H1 |
| CLI with no `usage.json` | Reproduced on `01a01136` and `01a0788f`. Exit 1, `No usage recorded` |
| Large event directory with no ledger | 653 events, 581,144,282 input tokens, no `usage.json` |

`DIRECTLY_OBSERVED`: the additive identity is a one-parent observation.
`NOT_MEASURED`: whether a second parent would do it.

## 7. Synthetic control

Taken from the row at 05:49:12, not from a preferred story.

```text
parent event input  = 330,080
parent event output = 494
child ledger input  = 1,764,978
child ledger output = 25,194
```

| Hypothesis | Parent usage input would be | This row |
| --- | ---: | --- |
| H1, event plus child | 2,095,058 | 2,095,058 |
| H2, parent usage is the child | 1,764,978 | no |
| H5, parent usage is the previous event | some earlier event | no |
| Index lag | previous usage row | no, once joined on time |

Only H1 hits the row. The same row's output is `494 + 25,194 = 25,688`.

## 8. Information gain

From the parent `usage.json` alone: the turn is 2,095,058. From the parent
event alone: the turn is 330,080. From the child ledger alone: the child is
1,764,978. None of the three states the equation. The equation is knowable
only by relating them, and on these nine rows it is exact, including output
on all nine and cost on the eight whose event has `costUsdTicks`.

Replay can recompute it from the files without asking the CLI. The result
can carry provenance: parent session id, event timestamp, child session ids,
and the three numbers. What it cannot carry is a reason the 2026-09-11
children were left out. That rule is not in the files.

## 9. Candidate primitive

There is a narrow one, and it is not "read `usage.json`."

```text
inputs:  parent turn_completed usage, by timestamp
         parent usage.json turns, by timestamp
         child usage.json totals, by completed_at
check:   usage_turn == event_at_same_second
         or usage_turn == event_at_same_second
                        + sum of child ledgers in the window
output:  which children are inside the reported parent turn,
         and which are not
```

On this parent the check passes for every time-aligned pair: either equal,
or equal to event plus the children in that window. The 2026-09-11 children
are the second kind of answer: they are not inside the parent turn. The
23:37:59 row is a third answer: the usage turn equals two events, and the
check as written above does not include "plus the previous unpaired event."
An implementation that only adds children will fail that one row by 965,575
input and 3,516 output. That failure mode is part of the mechanism, not a
nuisance.

`RECOMMENDATION` that this is the primitive worth a second parent.
`NOT` a recommendation to ship it from one parent.

## 10. Invariants that held here

- `usage.json` `session.inputTokens` equals the sum of its turns, on all 35 files.
- A child session's ledger equals that child's own events, on 33 of 33.
- `input + output = total` on the turns used above.
- Cached read is inside input. It was not added on top in the reconciliation.
- `grok usage` does not report a directory that has no `usage.json`.

## 11. Failure modes

- Joining on row index manufactures a lag. The lag is not in the timestamps.
- Treating every child as inside the parent turn is false for the 24 children on 2026-09-11.
- Summing every `usage.json` counts the nine windows' children twice.
- Summing every event and calling it `grok usage` misses the 29,519,212 on this parent, and also reports 581,144,282 the CLI will not report.
- Ignoring a usage turn that equals two events leaves 965,575 unmatched.
- A one-second clock skew looks like a missing row if the join is exact and the pair is not compared.
- One parent is not a population.

## 12. Claim boundaries

| Sentence | Status |
| --- | --- |
| `grok usage` prints the `usage.json` session object when the file exists | Reproduced on the frozen fixtures and on the two error cases |
| That number is the sum of that file's turns | `DIRECTLY_OBSERVED`, 35/35 |
| That number is every token the model consumed | `NOT_MEASURED` |
| The 29,519,212 is nine child ledgers added into nine parent turns | `DIRECTLY_OBSERVED` as arithmetic |
| Those children are also stored as their own ledgers | `DIRECTLY_OBSERVED` |
| The CLI double-counts them when asked for parent and child separately | `INFERRED` from the two files. Not observed as two CLI bills added by a user |
| Dollars | `NOT_MEASURED` |
| Cache writes are free because `cacheCreationTokens` is 0 | Not a claim. Observed zero only |

## 13. NOT_MEASURED

Why the 2026-09-11 children are left out of the parent turns while the later
nine are added. A second parent. A trace showing the CLI opens `usage.json`.
`costUsdTicks` against `/usage`. Whether the child's input itself contains a
copy of the parent's prompt, which would mean the added 29,519,212 is partly
the same context counted inside the child before it is added to the parent.
That last one is the open question inside the identity, not a reason the
identity failed.

## 14. Next experiment

One more real parent with children, or a refusal to build the check until
that parent exists. The fixture that distinguishes the hypotheses is the
05:49:12 row, not a synthetic running total. The pass condition is
`330,080 + 1,764,978 = 2,095,058` on input and `494 + 25,194 = 25,688` on
output, with the 2026-09-11 children remaining outside the parent turn, and
with the 23:37:59 turn still equal to two events. A test that only adds
children, and a test that only reads `usage.json`, both fail that pair of
rows for different reasons.
