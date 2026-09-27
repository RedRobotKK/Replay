# What this Grok client still has not shown, 2026-09-27

A note for the team, not a reader change. Nothing here was implemented.
"Myself" means the Grok CLI on this machine and the session that wrote this
file, not the weights.

Session `01a0e417-88e1-7792-93e1-b5b5d72b7390`, read from its `usage.json`
at the end of this pass. Fourteen turns. Model `grok-4.7-build`.

| | inputTokens | cachedReadTokens | fresh (`input` minus `cached`) |
| --- | ---: | ---: | ---: |
| Session | 26,799,498 | 25,194,240 | 1,605,258 |
| Smallest turn (3) | 175,281 | 174,976 | 305 |
| Largest turn (14) | 6,294,751 | 6,218,624 | 76,127 |

`cacheCreationTokens` is 0 on all 14 turns. Cached reads are 94% of the
session input. The turn that cost the most was not the one that generated
the most new text. It was the one that re-read the largest prefix.

## Where the cost actually moves

The ledger already says the prefix is the bill. Turns 2 through 5 of the
published snapshot were 175,281 to 358,340 input tokens, with fresh input
in the hundreds or low thousands. Turn 14 re-read 6,218,624 cached tokens
to add 76,127 fresh ones. A change that shrinks the prefix shrinks the next
turn. A change that only shortens the answer does not.

That is an observation about this session. It is not yet a measured saving
from any particular cut. The cuts that are worth trying, because the guide
says tool definitions, skills, and MCP servers are already inside the
context split `/context` shows, are unused MCP servers and tool output that
gets pulled back into the next turn. This process had MCP servers that did
not connect. Whether removing them changes turn 15's `inputTokens` is
**NOT_MEASURED**.

Compaction is a window control, not a ledger control. The one compact
observed on `01a09207` took the context from 401,088 tokens to 8,080 and
left every `turn_completed` line in place. `usage.json` keeps those turns.
Compacting does not make the recorded session smaller. It can only make a
later turn's prefix smaller, and that later turn was not measured.

`cacheCreationTokens` staying at 0 means there is nothing in this ledger to
optimize under the name "cache writes." An observed zero is not a free
write. Until a session shows a non-zero write, a write-reduction feature has
no counter to move.

Do not sum a parent `usage.json` with its children. The
[usage-versus-updates note](grok-usage-vs-updates-2026-09-27.md) shows that
counts 29,519,212 input tokens twice. That is a reader bug, not a model
saving, and it is the one optimization already proved.

## What I want learned next

These are the holes. None of them is a request to ship a reader.

1. The 24 child sessions from 2026-09-11 that are not part of the
   29,519,212. Their parent turns equal the log, so they were not added on
   top. Whether those parent turns already contain the same tokens is
   **UNRESOLVED**. That is the remaining double-count question.

2. The directory with 653 `turn_completed` records, summing to 581,144,282
   input tokens, and no `usage.json`. Why the ledger file was never written,
   and what `grok usage` would have printed, is **NOT_MEASURED**. A tool
   that only opens `usage.json` is blind to it.

3. One session's `costUsdTicks` against the movement in `/usage`. The guide
   says 10^10 ticks per USD. No allowance was read. Dollars stay
   **NOT_MEASURED**.

4. The turn after a compact. The compact itself is observed. The next
   turn's `inputTokens` is not. That is the only measurement that can say
   whether compacting is worth doing for quota rather than for the window.

5. A fresh `grok -p` on the planted second reader, with no commit id and no
   branch name in the prompt. The search-targeting result so far is
   `claude-sonnet-5`. It has not been run on `grok-4.7-build`. Until it is,
   I cannot say whether a claims table changes what I open.

6. Turn 9 of this session: 222,579 fresh tokens against 305 on turn 3.
   Something in the prefix missed the cache. The ledger shows the miss. It
   does not say which bytes missed. That is the highest-value trace inside
   a session I already have, and it needs the request, not another summary.

`numTurns` on `turn_completed` in `01a09207` was 20, 7, 5 on the first
events, while `usage.json` `turnNumber` counted 1, 2, 3. Those fields are
not the same counter. What `numTurns` counts is **NOT_MEASURED**.

## What I would not spend a run on

Another score on the old `searched` proxy. A lower reasoning effort with no
`grok usage` before and after. A hook that trims tool output after the call
was chosen. Treating the zero write counter as a saving. A work-state
subsystem in the binary. The open questions above are smaller than any of
those, and each one can come out negative.
