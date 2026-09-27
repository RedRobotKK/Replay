# Do not ship the max Grok reader, 2026-09-27

A constraint on the uncommitted reader, not a release note. `origin/main` has
no `cmd/replay/grok.go`. The file that would publish the wrong total is
untracked in a working tree, and `replay grok` from that tree was executed.

## The total it prints is not the ledger

That reader keeps the `updates.jsonl` record with the largest `inputTokens`
and its help text says the records are running totals. On this machine that
printed 78,413,855 input tokens. The same records sum to 828,573,565.
Thirty-five `usage.json` session totals sum to 276,210,235. Those are three
different quantities.

`grok usage` on the three sessions in
[Official grok usage, 2026-09-27](grok-official-usage-2026-09-27.md) matches
the sum of turns. This session's second turn is 348,136 input tokens after a
first turn of 3,716,413. A reader that keeps the max cannot reproduce that
session object.

The draft guide text that says `costUsdTicks` has no documented scale is a
second false sentence. The Grok 1.0.41 user guide says the scale is 10^10
ticks per USD. What remains true, and what `docs/SURFACES.md` already says,
is that the scale has not been checked against an invoice. "Undocumented" and
"unchecked" are different claims. A work-state row that anchors both of them
to one fixture mixes them.

## The rule to port

Commit `6074075` on `fix/astra-launch-surface` sums turns. When `usage.json`
input is larger than that sum, it records the excess and does not add it.
That file was read and not executed against this corpus. Port the aggregation.
Do not port a comment that says the scale is undocumented.

The excess check has a live case. Session `01a09207` has a `usage.json` input
total 29,519,212 above the sum of its own `updates.jsonl` records, and that
directory's `subagents/` tree contains no `updates.jsonl`. Summing every
`usage.json` for a fleet number can count inherited history twice.
`docs/SURFACES.md` already noted two aggregations differing by 2×. The max is
a third.

## The test that has to land with the command

The golden files are the `grok usage` JSON on this branch, not a synthetic
cumulative sample. A test fails if Replay's Grok total is anything other than
the sum of turns. On the multi-turn files it also fails if the total equals
the max turn or the last turn. A fixture whose records are cumulative by
construction cannot catch this, which is why a synthetic cumulative sample is
the wrong pin.

Until that test exists, `replay grok` should not be in a release. `replay
doctor` saying the surface is present and unread is more honest than a
command that prints a plausible wrong number.

## Pricing, if Grok is later wired into burn or cost

`cachedReadTokens` is inside `inputTokens`. On 832 records from this machine,
input plus output equals total, and adding the cache on top matches once. A
price path that treats the cache field as extra tokens will double-count Grok.

`cacheCreationTokens` was 0 on every turn of the published sessions, and the
max reader reported 0 across 46 sessions. A write column of zeros means the
counter was not observed doing anything. It is not evidence that writes were
free. The number that moves is fresh input, `inputTokens` minus
`cachedReadTokens`.

Do not print dollars from ticks until one session's `costUsdTicks / 1e10` is
compared with the move in `/usage`. The guide states the scale. The allowance
is the check. `docs/SURFACES.md` is right to leave the dollar total unsettled.

## What this does not justify building

The Claude trials say a short table of separate claims changes which file gets
opened, and that an anchor the agent can finish early stops the search before
the file is read. They do not say the binary should grow a memory store.
Nothing on `origin/main` reads `docs/WORK-STATE.md`.

An anchor is either able to falsify its claim or it only names a related
place. An anchor marked as settling "records are cumulative" cannot, if the
records it reads were built to be cumulative. That is a labeling fix, not a
subsystem.

Do not copy a hook that trims tool output after the model has chosen the
command. On Claude that increased cache writes and lost correct answers. Do
not lower reasoning effort, and do not aim a Grok feature at cache writes,
until a `grok -p` pair shows those counters move. The unrun protocol is
[What is left to measure on Grok, 2026-09-27](grok-next-measurements-2026-09-27.md).

## Correction, same day

[Grok release review, 2026-09-27](grok-release-review-2026-09-27.md) leaves
the "do not ship the max" conclusion in place and narrows two sentences
above.

The 78,413,855 / 828,573,565 / 276,210,235 figures are a snapshot. A later
pass on the same machine read 839 records and got 78,809,418 / 840,417,238 /
288,053,908. The three quantities still disagree. Cite the snapshot as a
snapshot.

The instruction to port the `6074075` rule that drops a larger `usage.json`
is not yet justified. The 29,519,212 excess on `01a09207` was not found in
any other session's `updates.jsonl`, and the sub-agent explanation in that
commit's comment was not confirmed. Dropping the excess disagrees with
`grok usage`. The release review's rule is to print both numbers when the
files disagree, not to pick one.

The sentence that `docs/guide/commands.md` already publishes "no documented
scale" is true only of the dirty working tree. `origin/main` has no `replay
grok` section. Do not commit that draft section as written.
