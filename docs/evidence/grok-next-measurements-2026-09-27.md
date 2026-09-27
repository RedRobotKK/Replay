# What is left to measure on Grok, 2026-09-27

A protocol, not a result. The numbers already in hand are in
[Official grok usage, 2026-09-27](grok-official-usage-2026-09-27.md).
Nothing below has been run except where it cites that file or a command
already executed. A proposed arm is not a finding.

## Already measured, and not to be rerun for a tighter score

On the published `grok usage` JSON, `session.inputTokens` equals the sum of
`turns[].inputTokens`. The turns are not monotonic. This session's first turn
is 3,716,413 input tokens and its second is 348,136. The 131-turn session
falls on 60 of 130 steps, peaks at 21,452,883, and ends at 0.

On 832 `modelUsage` records read from this machine's `updates.jsonl` the same
day: `inputTokens + outputTokens == totalTokens` on 832/832, and
`inputTokens + outputTokens + cachedReadTokens == totalTokens` on 1/832.
`cachedReadTokens` sits inside input. Adding it again double-counts.
`cacheCreationTokens` was 0 on every turn of the three published sessions.

The working-tree `replay grok` keeps the record with the largest
`inputTokens` per `updates.jsonl` and prints that as a cumulative total.
On this machine that sum of maxima was 78,413,855. The sum of the same
records was 828,573,565. The sum of 35 `usage.json` session inputs was
276,210,235. Those three numbers are not the same quantity.

Do not score another trial with a `searched` command-shape proxy, and do not
treat a branch name in `git branch` as retrieval of the file on that branch.
Those two proxies are how an earlier reading reported a miss as a failure to
notice evidence that had never entered context.

## Tests that can still come out false

Headless form, from this client's own guide: `grok -p`, `--output-format json`,
a fixed `--tools` list, then `grok usage <session-id>`. The official ledger is
the outcome. The old scorer is not.

**1. Reader arithmetic on the frozen logs.** No model call. For every session,
four numbers from the same files: `grok usage` session input, the sum of its
turns, the max turn, and the last turn. Then the sum-of-turns rule in
`6074075:cmd/replay/grok.go`, which was read and not executed. If max and sum
agree on a multi-turn session, the max reader survives for that file. The
three published sessions already disagree.

**2. Planted second reader, on Grok, with the answer not in the prompt.**
The earlier arms were `claude-sonnet-5`. One of those no-cursor agents found
commit `6074075` anyway, by `git log --all`, so a floor is not guaranteed.
Four arms, same prompt, no commit id and no branch name supplied:

| Arm | Fresh session receives |
| --- | --- |
| A | No cursor |
| B | Three-row claims table, no anchor |
| C | Table plus `git branch -a`, then look |
| D | Table plus one command that can show `grok.go` on another branch |

Score whether that file's contents entered a tool result, and the call index
when they did. Stop if A and B match. That falsifies "the table changes
search" for this model. Arm D is the cell the earlier anchor experiment never
ran: a check sufficient to settle the claim, rather than a check the agent
can finish early. n=10, interleaved.

**3. Compaction is the reboot this client actually performs.** Auto-compact
fires at 85% of the window (`[session] auto_compact_threshold_percent`).
`/compact` takes a keep-note. `PreCompact` and `PostCompact` exist, matched
on `manual` or `auto`. After compaction, hand back only the original open
question. Compare no note, a keep-note that names the file to open, and a
keep-note that states the conclusion. The third is leakage. The outcome is
whether the other `grok.go` is retrieved again.

**4. Memory against the same cursor.** Memory is off unless `GROK_MEMORY=1`
or `[memory] enabled = true`. The guide says an index is injected once, and
the model still has to open a topic. One topic names the sufficient command.
One topic states the claim and names no command. Fresh session, no resumed
history. If only the command-bearing topic changes the first search, memory
is acting as a cursor. If neither does, the index is not sufficient.

**5. Whether ticks move the allowance.** `grok usage` is the session ledger.
`/usage` is the account allowance. Read the allowance before and after one
session and compare the move to `costUsdTicks / 1e10`. The guide states that
scale. No bill was opened, so the scale is not yet a measurement. Until the
allowance moves by that amount, quota is not managed off the tick field.

**6. Prefix stability on `grok-4.7-build`.** This session is the baseline:
turns 2 through 5 were 175,281 to 358,340 input tokens after a first turn of
3,716,413, and the long tool-using turn was 2,363,388. Two headless runs of
one task, one allowed to read files whole, one restricted to search. Compare
fresh input, `inputTokens - cachedReadTokens`, on the second turn. A write
reduction of that shape was measured on Claude. It has not been measured here.
`cacheCreationTokens` staying 0 means a program aimed at cache writes has
nothing to move until a session shows a non-zero write.

## Controls the client documents, which are not savings

These are switches and commands in the 1.0.41 user guide. None of them has a
before/after token figure on this model in this file.

`/context` splits the window into system prompt, messages, reasoning, and
free. Tool definitions, skills, and MCP servers are already inside those
totals. Unused servers are therefore a prefix candidate. They are not yet a
measured saving. For a scripted trial, `--tools` fixes the list so two runs
share a prefix, and `--max-turns` stops a search that will not arrive.

Compaction reclaims the window and drops detail. A keep-note should name the
open question and the file that settles it, not the conclusion. `/flush`
before compact writes a memory summary and costs a model call. It is not a
claims table.

Resume and fork carry history into the session total. The guide says so.
`grok -p -r` appends. `--fork-session` still inherits. A new `grok -p` is the
cheap reboot. A fork is not.

Do not cap tool output with a hook that trims a command the model already
chose. On Claude that increased cache writes and lost correct answers. The
restriction that worked there changed the plan, so the model searched instead
of reading the file. Whether that repeats on Grok is test 6.

Do not lower `--effort` first. The flag exists. There is no accuracy-versus-tokens
measurement for `grok-4.7-build` in this corpus. An A/B scored by `grok usage`
and by evidence arrival is the test.

Sum the turns from `grok usage`. Do not keep the max record. Do not add a
parent `usage.json` to its children. One session on this machine had a
`usage.json` input total 29,519,212 above the sum of its own `updates.jsonl`
records, and that directory's `subagents/` tree contained no `updates.jsonl`.
The gap is unexplained. A fleet total that sums every `usage.json` can count
inherited history twice until that gap is closed.
