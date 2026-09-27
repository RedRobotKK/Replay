# Official `grok usage` output, 2026-09-27

Stdout of the Grok CLI, captured so the record can be read without this machine.
This is not Replay's reading of those logs, and it is not an invoice.

## Method

```text
binary:  grok 1.0.41 (4220f3b224a6) [stable]
command: grok usage <session-id>
stderr:  empty on all three runs
when:    2026-09-27
```

The local user guide (`~/.grok/docs/user-guide/17-sessions.md`, "The grok usage
Subcommand") says the output is JSON with `sessionId`, `updatedAt`, `session`,
and `turns`, that session totals cover the whole conversation including history
inherited by resume or fork, and that `costUsdTicks` is 10^10 ticks per USD.
The scale sentence is the guide's claim. Nothing here reconciled it against a
statement of account.

The three files below are the command's stdout, unmodified.

| File | Session | Turns |
| --- | --- | ---: |
| [one turn](grok-official-usage-2026-09-27/session-one-turn.json) | `01a09421-9197-77a2-b9bd-8fbf9dc3cc7f` | 1 |
| [this session](grok-official-usage-2026-09-27/session-01a0e417.json) | `01a0e417-88e1-7792-93e1-b5b5d72b7390` | 6 |
| [long session](grok-official-usage-2026-09-27/session-01a09207.json) | `01a09207-8168-7802-b465-0bedbf66f83a` | 131 |

No message text is in these files. They are token and cost counters only.

## What the JSON itself adds up to

On all three files, `session.inputTokens` equals the sum of `turns[].inputTokens`.
The same holds for `outputTokens`, `cachedReadTokens`, `totalTokens`, and
`modelCalls`. The turns are not a monotonic running total.

The one-turn file is the trivial case: the session object and the single turn
are the same numbers. `inputTokens` 1,764,978, `totalTokens` 1,790,172,
`costUsdTicks` 3,906,681,600, model `grok-4.6-build`.

This session, six turns, model `grok-4.7-build`. Turn 2 is smaller than turn 1.
The session total is the sum, not the max and not the last turn.

| Turn | inputTokens | outputTokens | cachedReadTokens | totalTokens | modelCalls | costUsdTicks |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 3,716,413 | 58,694 | 3,330,816 | 3,775,107 | 38 | 9,481,804,400 |
| 2 | 348,136 | 540 | 339,200 | 348,676 | 2 | 648,420,800 |
| 3 | 175,281 | 2,319 | 174,976 | 177,600 | 1 | 346,840,800 |
| 4 | 358,340 | 3,129 | 352,768 | 361,469 | 2 | 701,426,800 |
| 5 | 180,838 | 2,182 | 180,608 | 183,020 | 1 | 353,110,400 |
| 6 | 2,363,388 | 18,708 | 2,329,856 | 2,382,096 | 12 | 6,261,331,200 |
| session | 7,142,396 | 85,572 | 6,708,224 | 7,227,968 | 56 | 17,792,934,400 |

The long session has 131 turns. Across the 130 consecutive steps, `inputTokens`
falls 60 times and rises 70. The maximum turn is 21,452,883 and the last
recorded turn, number 133, is 0. That last turn has no `costUsdTicks` field.
The session cost, 807,645,503,200, equals the sum of the 130 turns that have
the field. Session input is 222,856,819, which equals the sum of the turns,
not the maximum.

## What this does not say

A reader that keeps only the largest record in a session will not reproduce
these session objects. That is a statement about these three files. It is not
a measurement of every Grok install, and it is not a dollar figure. Dividing
`costUsdTicks` by 1e10 is what the guide says to do. No bill was opened.
