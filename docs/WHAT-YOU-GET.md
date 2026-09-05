# What you actually get from pointing Replay at your sessions

**2026-09-04.** Written to answer two blunt questions: what does a user get, and does it recommend
settings. The second answer is better than expected, and it turns on something already in the ledger.

## What Replay reads, and what it never reads

It reads **the transcripts your agent already wrote**. It does **not** read `settings.json`,
`CLAUDE.md`, `.mcp.json` or any other configuration. That boundary is deliberate: `settings.json` can
hold environment variables and credentials, and **a tool that reads your config to advise you on your
config has to be trusted with it.**

Everything below is derived from sessions alone.

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

```
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

**And all of it rests on eleven sessions from one machine.** The mechanism is sound; the thresholds
that would turn "9 tools never called" into "you should disable this" are not calibrated, which is
why the output above states a fact and leaves the decision alone.
