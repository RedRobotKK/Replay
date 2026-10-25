# Does events.jsonl account for the 8-turn hole

```text
RESULT: UNEXPLAINED
```

It does not, and the reason is structural rather than a failed join.
**`events.jsonl` contains no token-bearing field at any depth.** It is a
lifecycle log, not an accounting artifact, so it cannot account for the
29,290,876 input-token gap or any part of it.

The experiment nevertheless corrected two errors in the companion forensic and
reduced the unexplained residual to a sharper object. Both are recorded below.

Session: `01a09207-8168-7802-b465-0bedbf66f83a`, the only session on this
machine carrying both a `usage.json` and a populated `updates.jsonl`.

## 1. The inclusion rule, fixed before any sum

Registered before computing a residual:

- A turn is identified by `usage.json`'s `turnNumber` and located in time by its
  `endedAt`.
- A stream record is a `params.update.sessionUpdate == "turn_completed"` entry
  in `updates.jsonl`, located by `params._meta.agentTimestampMs`.
- A turn's stream value is the sum of **every** record landing in the same
  wall-clock second, so a turn with two records is summed, not sampled.
- A record matching no turn is reported as an orphan, never dropped.
- Events are joined only through fields the artifacts carry themselves
  (`turn_number`, `ts`, `session_id`).

No rule below was chosen after seeing a residual.

## 2. events.jsonl has no token data

81,989 lines, 7,060,350 bytes. Enumerating every distinct leaf path at every
depth gives **37 paths**, and not one is a token or cost field:

```
auth_required            error_type      session_id            total_servers
cancellation_category    failed          session_relationship  total_tools
cancellation_context/…   failed_servers  succeeded             transport
conversation_message_c…  is_reinit       target                ts
decision                 loop_index      timeout_sec           turn_number
duration_ms              model_id        tool_call_id          type
enabled                  outcome         tool_name             wait_ms
error_message            phase           servers/…             yolo_mode
                         redirect_kind   schema_version        server_name
```

A raw substring search confirms it: `inputTokens`, `outputTokens`,
`totalTokens`, `cachedReadTokens`, `cacheCreationTokens`, `reasoningTokens`,
`costUsdTicks`, `modelUsage`, `modelCalls`, `usage`, `cost`, `usd` all return
**0 occurrences**. The 829 hits for `token` are the event type `first_token`, a
latency marker carrying no count.

Event types are `phase_changed` (74,598), `tool_started` / `permission_requested`
/ `permission_resolved` (1,361 each), `tool_completed` (1,360),
`loop_started` (833), `first_token` (829), `turn_started` (135),
`turn_ended` (133), and 18 MCP lifecycle events.

**This is a complete negative.** No inclusion rule, join, or window could have
extracted a token total, because none is recorded.

## 3. What the join did establish

`events.jsonl` carries `turn_number` 0–134: 135 turns, no duplicates, no gaps,
all `session_relationship: primary`, one `session_id`, `model_id: grok-4.6`.

`usage.json` carries `turnNumber` 1–133 with **122 and 123 absent**, 131 turns.
Joining the two on turn number:

| | turn numbers |
|---|---|
| in events, not in usage.json | 0, 122, 123, 134 |
| in usage.json, not in events | none |

Joining `usage.json` turns to stream records on timestamp: **all 131 turns
match a record**, 128 on the exact second and 3 within 2 seconds, against 131
distinct `prompt_id`s. The correspondence is 1:1 and complete.

## 4. Correction: there is no one-turn lag

The companion forensic reported "a tail lag from turn 123, the stream trailing
the ledger by exactly one turn." **That is wrong and is withdrawn.**

It was an artifact of comparing the two files **positionally**. Positional
alignment is only valid without gaps, and `usage.json` skips `turnNumber` 122
and 123, so every position after the gap was compared against the wrong turn.

Under the timestamp join the tail aligns exactly, turn by turn, with no lag.
The corrected agreement count is **120 of 131 turns**, not the 113 previously
published.

## 5. Corrected decomposition of the gap

| | input tokens |
|---|---:|
| `usage.json` total | 222,856,819 |
| `updates.jsonl` total | 193,337,607 |
| gap | **29,519,212** |

Ten turns differ. The gap decomposes exactly:

| component | tokens | status |
|---|---:|---|
| band, `turnNumber` 103–110 | 29,290,876 | **unexplained** |
| `turnNumber` 124 | +965,575 | explained, §6 |
| `turnNumber` 128 | +228,336 | explained, §6 |
| orphan record | -965,575 | explained, §6 |
| **total** | **29,519,212** | closes exactly |

## 6. Two components that are explained

**`turnNumber` 124 is a merged turn.** One stream record, 965,575 tokens at
23:36:37, matches no turn. The record at 23:37:59 carries 498,596 and
`usage.json` turn 124 carries 1,464,171:

```
965,575 + 498,596 = 1,464,171
```

Exact. Turn 124 is the sum of two records, which is also where events.jsonl's
extra `turn_number`s 122 and 123 go: three event turns collapse into one ledger
turn. This is a merge, not a loss, and it nets to zero in the gap.

**`turnNumber` 128 is a flagged incomplete record.** Short by 228,336, and its
record is one of only 3 in the session carrying `usageIsIncomplete: true`. The
stream declares this shortfall itself.

Removing both leaves the residual **exactly equal to the 103–110 band**:
29,290,876.

## 7. The residual, stated precisely

Eight contiguous turns, `turnNumber` 103–110, each with **exactly one** stream
record joined at 1ms or better. Not a merge, not flagged incomplete, not an
orphan.

| turn | usage.json | record | delta | uj modelCalls | rec modelCalls |
|---:|---:|---:|---:|---:|---:|
| 103 | 1,112,892 | 462,700 | +650,192 | 17 | 3 |
| 104 | 1,234,534 | 469,527 | +765,007 | 17 | 3 |
| 105 | 2,546,296 | 478,135 | +2,068,161 | 34 | 3 |
| 106 | 5,798,063 | 484,050 | +5,314,013 | 37 | 3 |
| 107 | 7,108,136 | 990,407 | +6,117,729 | 98 | 6 |
| 108 | 2,095,058 | 330,080 | +1,764,978 | 33 | 2 |
| 109 | 7,415,744 | 330,050 | +7,085,694 | 61 | 2 |
| 110 | 6,362,509 | 837,407 | +5,525,102 | 56 | 5 |

The one solid co-occurrence, tested across all 131 turns as a 2x2:

| | tokens differ | tokens same |
|---|---:|---:|
| **modelCalls differ** | 10 | 0 |
| **modelCalls same** | 0 | 120 |

**Zero off-diagonal.** The stream under-reports tokens only when it also
under-reports `modelCalls`, and never otherwise. Session-wide the stream is
short by **333 model calls** (1,354 against 1,017).

This localises the defect to whole omitted model calls rather than token
arithmetic. **It is a co-occurrence, not a demonstrated mechanism**: nothing
here shows why those calls are missing from the stream, and this page does not
claim a cause.

## 8. A candidate tested and rejected

`loop_started` in events.jsonl was tested as an independent call count. On the
handful of band turns inspected first it appeared to match the stream's
`modelCalls` exactly, which would have made events.jsonl a corroborating
witness.

Tested across all 130 joinable turns it **fails**: `loop_started` equals the
stream's `modelCalls` on 58% of turns and `usage.json`'s on 50%. A one-turn
offset, suggested by an apparent pattern in the disagreements, gives 55% and
51%. Neither alignment holds.

**The apparent match was a coincidence of the rows chosen for display.** No
claim rests on it. events.jsonl provides no usable independent call count, just
as it provides no token count.

## 9. NOT_MEASURED

Why 333 model calls are absent from the stream. Whether the missing calls are
concentrated in the band or spread across it. What distinguishes turns 103–110
from turns 111–112, which carry comparable `modelCalls` (17, 5) and agree
exactly. Where the tokens for the omitted calls are recorded before reaching
`usage.json`. Whether any artifact on disk holds them.

## 10. Consequence for the original question

`updates.jsonl` remains a **lower bound** on a session's input tokens, now with
a measured shortfall of 13.2% in the one checkable session and an identified
shape: whole omitted model calls on a minority of turns.

For session `01a01136`, which has no `usage.json`, the figure 581,144,282 stays
**NOT_MEASURED**. This experiment closes the events.jsonl reconstruction path
permanently: the artifact does not carry the data, so no future parse of it
will recover the number.
