# The Grok source contract: `usage.json`, and why `updates.jsonl` is not the ledger

**Measured 2026-09-27 on 48 local sessions. Read-only audit. No reader was
written, ported or shipped for this page.**

Companion to [Do not ship the max Grok
reader](grok-reader-do-not-ship-2026-09-27.md), which established that the
aggregation is the sum of turns. This page answers the question that comes
next and was still open: **which local artifact is the sum of turns taken
from.**

## The answer, and the observation that settles it

`grok usage <session>` **succeeds on 35 of 35 sessions that have a
`usage.json`, and fails on every session that does not**, with:

```text
Error: No usage recorded for session '01a01136-…'.
```

That session is the largest on this machine. Its `updates.jsonl` holds 653
records summing to **581,144,282** input tokens. **Grok does not count any of
it.** Two further sessions with `updates.jsonl` records and no `usage.json`
behave the same way.

So the 13 sessions without `usage.json` are not sessions whose usage is
recorded elsewhere. They are sessions Grok reports as having no usage. A reader
that sums their `updates.jsonl` invents **581,882,542** input tokens that the
tool itself does not report.

**`grok usage` reads `usage.json`. `usage.json` is the ledger.**

## Corpus, exact counts

| | sessions |
|---|---:|
| discovered | **48** |
| `usage.json` + `updates.jsonl` | 35 |
| `updates.jsonl` only | **13** |
| `usage.json` only | 0 |
| neither | 0 |
| `usage.json` parse failures | 0 |

| | input tokens |
|---|---:|
| in sessions with `usage.json` | 288,053,908 |
| in sessions without | 581,882,542 |
| share of `updates.jsonl` tokens Grok does not report | **66.9%** |

**Coverage by session favours `usage.json`; coverage by token appears to favour
`updates.jsonl`. That second reading is the trap.** The tokens it would add are
the ones Grok declines to report.

## Invariants measured

| # | invariant | result |
|---|---|---|
| I3 | `sum(turns) == session.inputTokens` in `usage.json` | **35/35** |
| I4 | `usage.json == sum(updates.jsonl)` | **34/35 equal, 1 higher, 0 lower** |
| I6 | `turnCount == updates record count` | **35/35 equal** |
| I5 | subagents explain the one gap | **no**: that session has no `subagents/**/updates.jsonl` |
| I8 | `cacheCreationTokens` | **OBSERVED ZERO** corpus-wide. Not evidence writes are free |
| I10 | records are a running total | **falsified**: 310 decreases across 3 sessions |

## The single divergence is per-record drift, not missing records

Session `01a09207`: both surfaces carry **131** records, and they differ by
**29,519,212**. Record counts match, so nothing is missing. **18 of 131 turns
disagree**, and the disagreement has structure:

| turn | `usage.json` | `updates.jsonl` |
|---:|---:|---:|
| 123 | 1,731,445 | 498,596 |
| 124 | 1,018,814 | **1,731,445** |
| 125 | 517,246 | **1,018,814** |

From turn 124 the `updates.jsonl` value is the **previous** turn's
`usage.json` value. The records lag by one. In an earlier block, turns 103-110,
`updates.jsonl` carries values several times smaller instead.

**What causes the lag and the small block is `NOT_MEASURED`.** The local files
do not record why. Compaction, a partial write, and a different quantity being
written are all consistent with what is on disk, and nothing here separates
them.

## `usage.json` is live, so a golden test must use a published snapshot

Session `01a0e417` is 7,142,396 input tokens over 6 turns in the fixture
published on this branch, and **16,622,681 over 12 turns** when read today. The
file grows with the session. A test that reads the live corpus is not a test.
The committed fixtures are the pin.

## Production contract

| question | evidence | result |
|---|---|---|
| authoritative token source | `grok usage` succeeds 35/35 with `usage.json`, 0/13 without | **PASS: `usage.json`** |
| multi-turn aggregation | `sum(turns) == session` 35/35 | **PASS: sum** |
| max/last rejection | 01a0e417 sum 7,142,396 vs max 3,716,413 vs last 2,363,388; 01a09207 sum 222,856,819 vs max 21,452,883 vs last **0** | **PASS** |
| cache accounting | `input + output == totalTokens` and `cachedRead <= input` on every published session | **PASS: cache is inside input** |
| subagent accounting | no subagent `updates.jsonl` in the divergent session | **PASS: not the explanation** |
| missing-file behaviour | 13 sessions have no `usage.json`; Grok reports no usage for them | **PASS: report no usage, do not sum updates** |
| cost semantics | guide states 10^10 ticks/USD; no invoice reconciliation performed | **NOT_MEASURED** |
| corpus coverage | 48 sessions, 0 parse failures, 0 usage-only, 0 neither | **PASS** |

## SHIP SOURCE CONTRACT: `usage.json`

`updates.jsonl` is rejected as the authoritative source on two independent
grounds, either of which is sufficient: it drifts per record without changing
record count, undetectably where there is no `usage.json` to check it against;
and on 13 sessions it carries records for usage Grok reports as not existing.

`updates.jsonl` retains one legitimate use: as a **cross-check** against
`usage.json` where both exist, which is how the drift above was found.

## What this does not settle

Whether the drift is compaction, partial writes or another quantity.
Whether `usage.json` can itself truncate or reset; **not observed, not proven
absent**. Whether ticks reconcile to an invoice. Whether any of this holds on a
second machine, which is the largest open question and is `NOT_MEASURED`.
