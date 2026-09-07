# Codex cache breaks with no observable cause, 2026-09-07

**What this measures:** how often a Codex session's prompt cache stops holding,
what it costs when it does, and which client-side explanations the log is able
to rule out.

## Summary

**80 cache breaks across 6,751 turns re-read 10,635,679 tokens cold. Nothing
the log records explains a single one of them.**

On the Anthropic surface a cache break has to be inferred: hash the prefix,
notice it changed. Codex states `cached_input_tokens` on every turn, so a break
is read off the log rather than deduced. A break here is a turn whose cached
share collapses from above 50% to below 10% while reading at least 2,000 input
tokens, which is a wide enough gap that ordinary variation does not register.

Five client-side causes were tested against the same 148 sessions. All five are
ruled out. The sixth, and the one that explained every Anthropic break, cannot
be tested at all, because Codex does not record it.

| Hypothesis | Result |
|---|---|
| Cache TTL expiry | ruled out. Median gap before a break 7.2s, before a warm turn 5.6s. **0 of 80** followed a pause over five minutes |
| Model switch | ruled out. **0 of 80**. Same model on both sides of every break |
| Context grew past a boundary | ruled out. Input size ratio at the break, **median 1.00x** |
| Compaction fallout | ruled out for the three largest, which followed no compaction |
| Any recorded turn context change | ruled out. **0 of 80** across eleven fields |
| **Tool definitions changed** | **cannot be tested. Codex does not log them** |

## Method

148 rollout files from `~/.codex/sessions` and `~/.codex/archived_sessions` on
one machine, written by CLI 0.116 and 0.117, spanning 2026-03-19 to 2026-03-27.
Read with `replay codex`, which sums per-turn deltas rather than the running
total; see [the quota note](codex-quota-2026-09-07.md) for why that distinction
matters.

Each break was then re-walked to compare the turn against the one before it on
timestamp, model, input size, compaction state, and every field
`turn_context` carries: `developer_instructions`, `personality`, `effort`,
`truncation_policy`, `collaboration_mode`, `summary`, `approval_policy`,
`sandbox_policy`, `cwd`, `current_date`, `realtime_active`.

## Finding 1: the prefix did not change and the cache went anyway

The three largest breaks, with the turn before each:

```text
cached 99% -> 9%    input 114,890 -> 115,219    105,107 tokens cold
cached 94% -> 6%    input 122,618 -> 122,935    116,023 tokens cold
cached 100% -> 4%   input 191,095 -> 191,345    184,433 tokens cold
```

The context grew by roughly 300 tokens, which is one ordinary turn, and between
95 and 99 percent of a stable prefix stopped being cached. Same model, seconds
apart, no compaction, no recorded configuration change.

## Finding 2: the one variable that matters is not in the log

`turn_context` carries fourteen keys and none of them is the tool set. There is
no tool list, no MCP server list, no count and no hash. Since a tool block
arriving mid-session was the cause of **every** break measured on the Anthropic
surface ([lane isolation](lane-isolation-2026-09-06.md)), its absence here is
the difference between a diagnosis and a shrug.

So this file does not claim provider-side eviction. It claims that the cause is
either the one thing the client does not record, or something outside the
client, and that the log as written cannot separate those.

## What would settle it

**One field.** A hash of the tool set on `turn_context` would make Codex cache
behaviour auditable by anyone reading their own logs. It is the smallest change
that turns this file from an open question into a measurement.

The proxy is the other place tool definitions are visible, and that route is
closing: `codex doctor` on 0.153.4 reports a **Responses WebSocket** transport
with HTTPS as fallback, and the local trace database shows
`codex_api::endpoint::responses_websocket` as the largest API target at 48,323
rows against 12,414 for `codex_api::sse::responses`. Replay is an HTTP proxy. A
WebSocket upgrade is forwarded and cannot be parsed.

## Scope, honestly

One machine, one account, one plan, 148 sessions, 6,751 turns, two CLI versions,
predominantly one model. Enough to establish that these breaks happen, that they
are expensive, and that the recorded fields do not explain them. **Not enough to
establish a rate**, because a single operator's habits shape when a cache is
warm, and not enough to say anything about other accounts or plan tiers.

The thresholds are a judgement: 50% down to 10%, with a 2,000 token floor. A
narrower definition would find more breaks and a wider one fewer. The figures
here move with that choice and the choice is stated so it can be argued with.

Reproduce with `replay codex`, which prints the break count and the cold total.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
