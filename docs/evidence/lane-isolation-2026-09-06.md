# Lane isolation, and a headline retracted the same day, 2026-09-06

**What this measures:** how much of a fan-out session's re-billing actually comes
from a changed cacheable prefix, and what the answer was when the instrument
measuring it was comparing each parallel sub-agent against a different one.

**Headline: 4.2%.** Published the same morning as 98.8%.

---

## The first number, and why it was wrong

A randomised trial ran 30 lanes through `replay serve` and produced this:

> 3,734,134 of 3,778,706 re-billed tokens, **98.8%**, attributed to
> `system prompt or tool definitions changed`.

It went into a commit message and a source comment as established fact.

It was not a fact about the world. It was a fact about the instrument.
`stats.observe` compared each request's prefix hash against **one session-wide
field**, `st.prefixHash`. A fan-out session runs several agent lanes at once,
each carrying its own tool set, so lane A's request overwrote the field and lane
B's next request was judged against lane A's hash. Neither lane had changed
anything and both were recorded as having changed their prefix.

Re-reading the same ledger lane by lane:

| | Events | Deficit tokens | Share |
|---|---|---|---|
| As published | 34 | 3,734,134 | 98.8% |
| Lane-correct re-read | **3** | 416,887 | 11.0% |
| Forged by the session-wide compare | 31 | 3,317,247 | — |

**All thirty sub-agent lanes were internally stable and never changed prefix at
all.** Thirty-one of the thirty-four events had not happened.

**Both figures are retracted.** 11.0% is not the answer either: it was derived
from a ledger *written by* the broken classifier, so its `broken` labels and its
deficit figures are themselves products of the session-wide comparison. It is a
less-polluted ghost of the same error.

## The measurement, through a corrected proxy

Re-run 17:44 to 17:50 on the fixed build. 60 requests, 5 sessions, 15 sub-agent
lanes plus the main loop, haiku, one operator, one machine.

| | |
|---|---|
| Outcomes | 35 reproduced, 3 broken, 2 exceeded |
| Total prompt tokens billed | 7,996,448 |
| **Re-billed by cache break** | **336,060** |
| **Share** | **4.2%** |

This is a different quantity from either retracted figure, not a third estimate
of the same one. The retracted numbers were shares of the old run's own corrupted
deficit total; 4.2% is re-billed tokens against every prompt token actually
billed, which is the figure that means something to whoever pays.

## The cause distribution, and a prediction that could have failed

```
3   336,060   tool definitions changed
0         0   system prompt changed
0         0   system prompt or tool definitions changed  (combined fallback)
```

A composition analysis of the old ledger predicted **zero** system-prompt
changes: across every consecutive pair in the trial, `system_bytes` never moved
once. The corrected run came back zero. **Had it come back non-zero, that finding
would be retracted here too.**

**All three breaks are in the main loop. Zero sub-agent lane breaks across 15
lanes** — which the lane-correct re-read had inferred from corrupted data, now
produced directly by a working instrument.

Every break is an MCP connector's tool block arriving mid-session:

```
[157,080] added 3 tool(s): mcp__claude_ai_Otter_ai__otter_fetch,
          otter_get_user_info, otter_search; removed 1 tool(s): WaitForMcpServers
[140,623] added 39 tool(s): mcp__claude_ai_Calendly__availability-...
[ 38,357] added 199 tool(s): ListMcpResourcesTool, ReadMcpResource...
```

**157,080 tokens re-billed because three Otter.ai tools finished connecting.** A
cached prefix is keyed on content, so any change to the tool definitions voids
all of it. Under the old cause string this read "system prompt or tool
definitions changed" and an operator had nowhere to go with it.

## What the correction cost, and what it saved

Five fields carried the same defect: `prefixHash`, `last`, `model`/`lastSeen`,
the `context`/`whatIf`/`reReads` trio, and the first-request gate. The lesson had
already been written down against a sixth, `errorByLane`, keyed by agent ID with
a comment explaining exactly why. **It was applied to one field out of six.**

The fifth is the one worth keeping. An unseen lane's usage is the zero value,
`ExpectedRead` of it is 0, and an opening request reads 0, so the two matched
exactly and scored **"reproduced"**. Every sub-agent lane in a fan-out
contributed a cache hit that never happened. It produced no error, no break and
no anomaly — it only inflated a success metric, which is the direction nobody
audits. It was found by a mutation that survived, not by anyone reading the code.

**What it saved:** the 98.8% figure was about to justify building a prefix
compression subsystem. There is no 98% defect to compress. The remaining waste is
three connector handshakes, and the fix for those is client-side sequencing —
bind MCP tools before the first cached request, not after.

## Scope, honestly

60 requests, one operator, one machine, one synthetic fan-out prompt on haiku.
This shows the instrument no longer lies about its own traffic. It does **not**
establish a production distribution: the real ledger sessions on hand carry zero
sub-agent lanes, so they cannot speak to the fan-out case at all.

## Method

Reproduce with `replay serve --ledger <dir>`, point an agent at it with
`ANTHROPIC_BASE_URL`, and run a prompt that spawns parallel sub-agents. The
per-lane figures are on `/replay/status` under `context_by_lane`, and the break
detail is in the ledger's `cause_detail` field.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
