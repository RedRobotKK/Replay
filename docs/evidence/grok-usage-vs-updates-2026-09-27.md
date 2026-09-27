# What `usage.json` and `updates.jsonl` are, 2026-09-27

Independent read of this machine's `~/.grok/sessions` and of Grok 1.0.41's
user guide. No Replay reader was added or changed.

The question is which file an external tool may treat as Grok's own reported
usage. `grok usage <session-id>` is that report. Its stdout is the shape of
`usage.json` (`sessionId`, `updatedAt`, `session`, `turns`). The three files
in [Official grok usage, 2026-09-27](grok-official-usage-2026-09-27.md) are
that stdout.

## Census

48 directories contain `updates.jsonl`. 35 also contain `usage.json`.
13 do not. Every directory was classified by `usage.json` session
`inputTokens` against the sum of `inputTokens` on `turn_completed` events in
that directory's `updates.jsonl`.

| Relationship | Directories |
| --- | ---: |
| Equal | 34 |
| `usage.json` greater | 1 |
| `updates.jsonl` greater | 0 |
| No `usage.json` | 13 |

Usage objects appear on `turn_completed` and on no other `sessionUpdate`.
This pass found 840 such objects. Two further `turn_completed` events in the
discrepancy session have `usage: null` (`stop_reason` `cancelled` and
`error`) and contribute no tokens.

`inputTokens` on those events is not a running total. Consecutive values fall
as well as rise, and on the 34 equal directories the sum of the events is the
`usage.json` session total. The last event is not the total either: the
published 131-turn session ends at 0. The max event is not the total either:
that session's max is 21,452,883 against a session object of 222,856,819.

## What each file is

`updates.jsonl` is the append-only conversation log. The guide calls it the
authoritative log for resume and restore. A `turn_completed` line carries one
turn's usage object. Summing those objects reproduces `usage.json` on 34 of
35 directories. It does not reproduce it on the 35th.

`usage.json` is the session ledger `grok usage` prints. On all 35 files,
`session.inputTokens` equals the sum of `turns[].inputTokens`. The guide says
those session totals cover the whole conversation, including history inherited
by resume or fork, and that per-turn totals are read through `grok usage`.
`/usage` in the TUI is documented as the account allowance plus that session's
totals. The allowance was not opened in this review. The session half of that
screen is the same ledger as `grok usage`, which matches `usage.json`. It was
not observed in the TUI.

`usage.json` is not a second copy of the log. It cannot be rebuilt from the
parent `updates.jsonl` alone. The log cannot be rebuilt from `usage.json`:
eleven lines in the discrepancy session have smaller `inputTokens` than the
`usage.json` turns at the same second, and those smaller numbers are not in
`usage.json`.

## The one directory where they differ

Session `01a09207-8168-7802-b465-0bedbf66f83a`.

| Source | inputTokens |
| --- | ---: |
| `grok usage` / `usage.json` session | 222,856,819 |
| Sum of 131 `turn_completed` records | 193,337,607 |
| Difference | 29,519,212 |

Both sides have 131 records. They are not the same multiset: 11 values are
only in `usage.json` (sum 35,633,999) and 11 values are only in the log (sum
6,114,787). The difference of those sums is 29,519,212.

The session has 33 subagent directories. None contains `updates.jsonl`. Each
contains `meta.json` and `output.json`. Every `meta.json` points at a
`child_session_id` that has its own `usage.json`. All 33 say
`effective_context_source` `new`. The parent's `summary.json` has no
`parent_session_id`. This is not a fork of another session.

Nine `usage.json` turns are larger than the `turn_completed` line at the same
second by exactly the sum of the child-session `inputTokens` that completed
in that window:

| Turn ended (UTC) | Log `inputTokens` | Child sessions completed in the window | `usage.json` input |
| --- | ---: | ---: | ---: |
| 2026-09-12T05:41:50 | 462,700 | 344,971 + 305,221 | 1,112,892 |
| 2026-09-12T05:42:13 | 469,527 | 288,832 + 476,175 | 1,234,534 |
| 2026-09-12T05:43:30 | 478,135 | 673,076 + 1,395,085 | 2,546,296 |
| 2026-09-12T05:46:22 | 484,050 | 5,314,013 | 5,798,063 |
| 2026-09-12T05:47:34 | 990,407 | 2,008,514 + 1,547,843 + 1,652,233 + 909,139 | 7,108,136 |
| 2026-09-12T05:49:12 | 330,080 | 1,764,978 | 2,095,058 |
| 2026-09-12T05:51:49 | 330,050 | 7,085,694 | 7,415,744 |
| 2026-09-12T05:54:52 | 837,407 | 5,525,102 | 6,362,509 |
| 2026-09-14T00:35:21 | 268,260 | 228,336 | 496,596 |

Each `usage.json` figure equals the log figure plus those children. The nine
child sums add to 29,519,212. No child session's full `inputTokens` equals
any parent `turn_completed` `inputTokens`, so the parent log does not already
hold those totals as its own lines.

One further `usage.json` turn, at 2026-09-13T23:37:59, is 1,464,171, which is
the sum of two log lines (965,575 at 23:36:37 and 498,596 at 23:37:59) and
not a child total. One `usage.json` turn records 0 input at
2026-09-14T01:01:47. No `turn_completed` line has `inputTokens` 0. That turn
does not move the sum.

The other 24 children, completed on 2026-09-11, sit in windows where
`usage.json` already equals the log. They are not part of the 29,519,212.
Their own session files still exist. Whether those parent turns also contain
the same tokens as the child files is **UNRESOLVED**. The exact additive
identity is only the 29,519,212.

## What does not explain it

Compaction does not. This session auto-compacted once, after 61
`turn_completed` events, at prompt index 62. Context went from 401,088 tokens
to 8,080. The log was not rewritten: the `turn_completed` lines are still
there. The turns that differ are numbered 103 and later.

Resume and fork do not, for this session. There is no `parent_session_id`.
Every subagent meta says `effective_context_source` `new`.

A monotonic running total does not. The log decreases. The published
multi-turn files decrease. Summing the log did not multiply the 34 equal
sessions.

`subagents/**/updates.jsonl` does not exist. The child totals live in the
child session directories, which are ordinary sessions with their own
`usage.json` and, for the published one-turn child `01a09421`, an
`updates.jsonl` whose sum equals that `usage.json`.

## Double-counting

The 29,519,212 is in the parent's `usage.json` and in the child sessions'
own `usage.json`. Summing every `usage.json` counts those tokens twice.

The parent's `turn_completed` lines do not contain that 29,519,212. The child
directories do. Summing every `turn_completed` `inputTokens` counts those
tokens once.

That is why an older reader refused to add a `usage.json` that was larger
than the log. The refusal matches this session. It is the wrong rule for the
other 34, where the files are equal, and it is the wrong number for `grok
usage` of the parent: the CLI reports 222,856,819, not 193,337,607.

## Sessions with no `usage.json`

Thirteen directories. Eleven hold one small `turn_completed` (about 17k to
582k input). Two hold none. One holds 653 `turn_completed` records summing to
581,144,282 and no `usage.json` at all. A tool that reads only `usage.json`
drops that session. A tool that reads only the log can sum it, and that sum
has not been checked against `grok usage`, because `grok usage` has no file
there. That total is log-only, not Grok's reported usage.

## Cache and cost

Unchanged, and not re-interpreted. On the live `turn_completed` records,
`inputTokens + outputTokens == totalTokens`. `cachedReadTokens` sits inside
input. Adding it again double-counts. `cacheCreationTokens` is zero on every
such record in this pass. That is an observed zero, not a proof that writes
were free. `costUsdTicks / 1e10` is the guide's conversion. It was not
compared with `/usage` or an invoice. No dollar figure follows from this
file.

## Production recommendation

**AUTHORITATIVE SOURCE: usage.json**, for reproducing `grok usage` of a
session that has the file.

That is the file the CLI prints. It matches the CLI on the three published
sessions. It matches the sum of its own turns on all 35 files. The log matches
it on 34 and misses 29,519,212 on the 35th, and that miss is child-session
usage the parent ledger includes and the parent log does not.

A machine-wide sum of `usage.json` is not that claim. It counts the
29,519,212 twice. A machine-wide sum of `turn_completed` records does not
count that 29,519,212 twice, and it is the only record for the 581,144,282
session that has no `usage.json`. Neither sum is "what Grok reported" for
every session on the machine. A release that prints one of them under that
name is wrong.

## Tests a reader has to pass

Real files, not a synthetic running total. The published JSON is the CLI
stdout. The discrepancy numbers above are the real session, reduced to
counts. The raw `updates.jsonl` is a conversation log and is not copied into
the tree.

| Case | Fixture | Pass | Fail |
| --- | --- | --- | --- |
| One turn | Published `session-one-turn.json` | Total 1,764,978 | Anything else |
| Several turns, max and last disagree | Published `session-01a0e417.json` | Total 7,142,396, the sum of the six turns | 3,716,413 (the max) or 2,363,388 (the last) |
| Official output | Each published file | Reader total equals `session.inputTokens` | A different aggregation |
| Parent versus log | `01a09207` counts in this file | Reported `grok usage` is 222,856,819. Log sum 193,337,607 is shown as the log, not as `grok usage` | Printing 193,337,607 as what Grok reported |
| Do not sum parent and children | The 29,519,212 child totals, which also have their own `usage.json` | A machine total does not add those tokens twice | Parent `usage.json` plus the child `usage.json` files presented as one bill |
| Cache | Any published turn | `input + output` is the total. Cache is a share of input | Cache added on top |
| Zero `cacheCreationTokens` | The published files, all zero | Reported as observed zero | Reported as free writes or as a saving |
| No `usage.json` | The 653-record session, log sum 581,144,282 | Labeled log-only, not dropped and not called `grok usage` | Omitted, or presented as the CLI's report |
| No log | Not observed here | Absent, not zero | A fabricated total |
| Cancelled turn | Two `turn_completed` lines in `01a09207` with `usage` null | Not added | Added as zero |
| Compaction | The one auto-compact in `01a09207` | Not treated as deleting the earlier `turn_completed` lines | A total that starts over at the compact |
| Resume | Not observed on this parent (`parent_session_id` absent) | No inherited-history claim | A test that invents a parent id |

## Correction, later the same day

The child-session addition above is the net of the 29,519,212. It is not the
whole positional disagreement. Eighteen of 131 positions differ. Seven in the
tail are a one-step lag: the log's value is the previous `usage.json` turn.
That lag does not account for the sum. The sum is the eight early positions
plus one later position of 228,336, which are the child totals.
[The methodology audit](grok-claude-methodology-audit-2026-09-27.md) separates
those two facts. This page had folded them together.

## Correction to the release review

[Grok release review, 2026-09-27](grok-release-review-2026-09-27.md) left the
29,519,212 unexplained and said a second on-disk copy had not been found.
The copy is not another `updates.jsonl`. It is the child sessions'
`usage.json`, and the parent's `usage.json` turns are larger than the
parent's log by exactly those child totals. The release rule stands: do not
print one unlabeled number when the log and `usage.json` disagree. The source
of the disagreement is no longer unresolved.
