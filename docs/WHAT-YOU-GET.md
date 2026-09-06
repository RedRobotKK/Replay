# What you actually get

**Updated 2026-09-06, after measuring a full month, then measuring whether the measurement was worth
anything, and then measuring whether it was worth anything to the people who are not billed per
token.**

## Who this is for, stated plainly

**It pays for itself if you are billed per token and run agents heavily.** Metered API, Bedrock or
Vertex, with a key you can see the invoice for. That is where a broken cache costs *you* money, and
where a cost-per-task figure changes a decision.

**It does not pay for itself in money on a flat seat.** Claude Max, Team, Copilot, Cursor: a broken
prompt cache produces no invoice line for the subscriber. There is no bill to reduce.

**Whether it costs a flat seat something other than money is an open question, and it was measured
rather than asserted.** A break plainly consumes tokens; whether those tokens are charged against a
subscription's rate-limit window the way they are charged against an invoice is undocumented. Matched
arms — ten cold writes and ten warm reads of the same ~159,000 token prefix — moved 3.09M tokens and
the 5h utilisation counter by **zero** steps
([titration](evidence/quota-titration-2026-09-06.md)). That is a null result at this budget, not a
finding in either direction, and it is published as one.

What survives the null is the part that needed no counter. A re-billed token is context the work did
not get: it occupied the window and bought nothing. That is why `replay cost` states the avoidable
figure in tokens beside the dollars, and says under the figures that on a flat seat the dollars are
list price for somebody else. A subscriber reading a dollars-only report reasonably concludes the
finding is not about them.

**It is worth an afternoon if you are about to run agents unattended.** The median and p90 task cost
are the inputs to any forecast of what an autonomous agent will spend, and almost nobody has them.

## What it actually found, on one real month

The corpus grows daily, so each row names the run it came from rather than pretending to a single
snapshot. The transcript counts differ between them for that reason and no other.

| | | Source |
|---|---:|---|
| Sessions | **78** — a session writes one transcript per lane, so subagents multiply the file count without adding an independent draw | [correction, 2026-09-06](evidence/calibration-corpus-2026-09-06.md) |
| Turns reproduced against real bills | 28,135 of 28,868 (97.46%), over 1,450 transcripts | [same](evidence/calibration-corpus-2026-09-06.md) |
| Spend at list rates | $3,018.99 over 1,505 transcripts | `replay cost`, 2026-09-06 |
| Median task | $0.65 | same run |
| p90 task | $2.16 | same run |
| Re-billed because a cache broke | $150.27 (5%), **31.4M tokens** | same run |
| Cache breaks located and classified | 735, carrying 31.26M re-billed tokens over 1,506 transcripts | [break causes, 2026-09-06](evidence/break-causes-2026-09-06.md) |
| Compactions the client recorded | 39, median 999,029 tokens before against 23,218 after — a retention of **2.55%** | `internal/transcript` over the same corpus |

## What that is worth, honestly

**5% is what was detected, not what is recoverable.** Some breaks have legitimate causes: the file
genuinely changed, compaction fired, the model was switched on purpose. A realistic recovery rate
puts the true figure nearer 2 to 3 percent.

**Which causes those are is now measured rather than guessed**, and the answer moves the argument. On
the full corpus, client re-rendering of history accounts for 50.8% of re-billed tokens and TTL expiry
for 33.9%; the two have opposite shapes, 583 breaks at ~27k tokens each against 39 at ~271k. A
developer going to lunch costs more than a hundred re-renders. Read the sampling note in that file
before quoting any of it: the same measurement over the 40 largest sessions inverted the ranking,
because long sessions contain long gaps and long gaps are what TTL expiry means.

And nothing there establishes that a layout change **recovers** any of it. 63.6% is what layout could
plausibly address. What it would actually recover needs a live trial with a control arm, which is what
`--trial-share` and the graduation rules in `replay learn` exist for.

**It is the fourth lever, not the first.** A committed-spend discount is worth 15 to 30 percent for
one email. Routing cheap turns to a cheaper model is worth 30 to 60 percent of a large share of
traffic. Trimming the always-on context is worth 5 to 15 percent, and it is not free: what you
cut has to be written to the cache again at 1.25x or 2x, so on a warm prefix it only
pays once you are removing most of the prompt rather than a slice of it. Cache forensics comes
after all three, and it is the only one that asks you to install something.

If you are looking for the biggest saving available to you, run those three first. This tool will
still be here, and it will tell you whether the fourth lever is worth pulling on your own data
rather than on somebody's marketing claim.

## What Replay reads, and what it never reads

It reads **the transcripts your agent already wrote**. On every default invocation it does **not**
read `settings.json`, `CLAUDE.md`, `.mcp.json` or any other configuration. That boundary is
deliberate: `settings.json` can hold environment variables and credentials, and **a tool that reads
your config to advise you on your config has to be trusted with it.**

**There is exactly one exception, and it is a flag you type.** `replay advise --apply` reads
`~/.claude/settings.json` (or `$CLAUDE_CONFIG_DIR/settings.json`) to find the current
`promptCacheTtl`, and with `--yes` writes that one key back, after copying the existing file to a
timestamped `.bak-…` sibling and refusing outright if the file is not valid JSON. That key is the
whole of what it reads and the whole of what it writes; `CLAUDE.md` and `.mcp.json` it never opens
at all. The exception is narrow on purpose: `--apply` is the case where you asked for a change to
your config rather than for advice about it, and a tool that offers to make the change has to read
what is there first. Without the flag, `advise` prints its plan and writes only its own
`~/.replay/advice.json`.

Everything else below is derived from sessions alone.

## What you get

### 1. Which turn broke the cache, and what it cost

Not that a break happened. **Which turn, what changed, and how many tokens were re-billed for the
rest of the session.** This is the flagship and it is already built.

### 2. What your setup costs you on every single request

The ledger records `system_bytes`, `tool_bytes`, `tool_count` and **each tool definition by name and
size**. Those are carried on *every* request, so they are the standing overhead of your
configuration, paid before any work begins.

### 3. Which of that overhead did nothing

`advise` already reports **"N tool definitions never called are X% of prompt tokens"**. That is the
sentence with the most leverage in the tool, and it is currently phrased as a measurement rather than
an action.

### 4. The habits, not the session

`advise` aggregates: the file read repeatedly across many sessions, first-turn attachments that are
always loaded, the pattern that breaks the cache every time.

## Does it recommend settings? Nearly, and the last step is free

Today it says *"12 tool definitions never called are 8% of prompt tokens."* Useful, and it stops one
step short of actionable, because the user still has to work out **which** twelve and **where** they
came from.

**It does not need to read any config to close that gap.** MCP tools are named
`mcp__<server>__<tool>`, so the server is already in the name, and the ledger already stores each
name with its byte size. So the advice can become:

```text
  Standing cost of your setup, on every request:
    system prompt + instructions   4,180 tokens
    tool definitions (23 tools)    6,240 tokens

  Never called in 40 sessions:
    mcp__playwright__*      9 tools   3,100 tokens/request   disable this server
    mcp__jira__*            3 tools     820 tokens/request   disable this server

  Disabling both frees 3,920 tokens of context on every turn, about 24% of your
  standing overhead, before any work starts.
```

**Every number there is already computed or trivially derivable.** No config read, no new permission,
no new file. The tool names supply the attribution.

### What it still cannot recommend, and should not pretend to

- **Which model to use.** It sees `model` and `effort` per request, but not whether a cheaper model
  would have produced an acceptable answer. That is an outcome judgement and Replay cannot see
  outcomes.
- **What to put in CLAUDE.md.** It can say the first-turn instructions cost 4,180 tokens on every
  request. It cannot say which paragraph earned its place.
- **Whether to disable a tool you rarely use.** Never-called across 40 sessions is evidence. It is
  not proof you will not need it tomorrow, and that is insurance, not waste.

## The framing that makes any of this land

Per the waste definition: **for a subscription user none of this is about money.** The sentence that
matters is not "this cost you $2" but:

> **Your setup spends 10,400 tokens of every request before you have typed anything, and 3,900 of
> those are tools that have never been called.**

That is context the actual work does not get, on every turn, in every session. **It is why a long
session gets vague near the end**, and it is a thing the user has already experienced without knowing
the cause.

## Honest state

Items 1 to 4 exist. The named-server attribution in the box above **does not** — it is a presentation
change over data already in the ledger, and it is the single highest-leverage thing left to build.

**And all of it rests on 78 sessions from one machine.** The mechanism is sound; the thresholds
that would turn "9 tools never called" into "you should disable this" are not calibrated, which is
why the output above states a fact and leaves the decision alone.

One item did land since this was written, and it is item 1's missing half rather than the box above.
`replay context` now subtracts: it reports how many compactions fired, how many tokens the client
recorded as dropped, and the share by which its own ranking overstates the context that is actually
there. Before that, a session that had compacted was ranked as though everything it ever loaded were
still present — at a median retention of 2.55%, that is a report describing a context ninety-seven
percent of which had already been thrown away.

---

[Documentation index](README.md) · [Repository README](../README.md)
