# Three wire families, not two, 2026-09-06

**What this measures:** which request shapes a terminal GenAI client actually
sends, and what its rate-limit headers actually say, captured off the wire from
a live authenticated session rather than read from documentation.

## Summary

**The Grok CLI speaks a third wire family that Replay does not parse**, and it
returns the four `x-ratelimit-*` headers this project had never once captured.

The headers turned out to be less useful than they look. `remaining` equalled
`limit` on every observed call, so on this evidence they do not measure
consumption at all. That claim was written the other way round first, on header
names before any value had been read, and is corrected below.

| | |
|---|---|
| Wire family | OpenAI **Responses** (`POST /responses`), not chat completions |
| Origin | `cli-chat-proxy.grok.com`, not `api.x.ai` |
| Replay status | **NOT PARSED.** Forwarded byte for byte, warned once per path |
| Caching | client declares `prompt_cache_key`; the provider does not infer a prefix |
| Transport | SSE. `stream: true` on every call, so usage arrives in stream events |
| Local transcript | none. Conversation state is POSTed to the vendor |

## Method

A shape-only capture proxy in front of the origin. Keys and types recorded,
values never, except for a named allowlist of `x-` headers that carry limits and
capabilities and cannot contain conversation content. `Authorization` and
`Cookie` dropped before anything is written. No prompt text, no completion text
and no credential is on disk.

That is the honest way to record a wire format from a live personal account, and
it is why this file has shapes where a fixture would have payloads.

**Scope: one client, one account, one machine, two model calls in a single
headless turn.** Enough to establish the family exists and is in real use. Not
enough to characterise it. No cache hit was observed, no usage was parsed out of
the SSE stream, and there is no second client on this family to show the shape
generalises beyond Grok's implementation.

Raw shapes: `$CLAUDE_JOB_DIR/tmp/grok-shapes.jsonl`.

## Finding 1: the rate-limit headers exist, and did not move

Captured values, identical across both model calls:

```text
x-ratelimit-limit-requests       = 8300
x-ratelimit-remaining-requests   = 8300
x-ratelimit-limit-tokens         = 53000000
x-ratelimit-remaining-tokens     = 53000000
x-grok-context-window            = 500000
x-data-retention                 = zdr
x-zero-data-retention            = true
x-request-id                     = <uuid, varies per call>
```

**`remaining` equals `limit` on both.** Two calls returning 121KB and 113KB of
response, and the budget reads untouched. Whether the counter updates on a
delay, resets per window, or does not decrement on this plan is unknown from two
samples.

**A retraction.** An earlier version of this file called these "a falling
per-request counter at token granularity" and "a significantly higher-fidelity
instrument" than Anthropic's rising utilization fraction. That was written from
header *names*, before a single value had been read. On the values, it does not
fall. A `remaining` figure that always equals the limit is not a measurement of
consumption; it is the same shape as a healthcheck that cannot fail.

Two samples cannot show it *never* moves, and the headers may still be the right
instrument. They are not evidence of one yet.

**What is still true:** `internal/proxy/quota.go` already allowlists the
`x-ratelimit-` prefix, so the capture code is written and correct. It has never
fired, because `/responses` is NOT PARSED and no ledger record exists to attach
headers to. This remains the `RemainingHeaders` path that
[quota titration](quota-titration-2026-09-06.md) recorded as never having seen
real data.

`x-grok-context-window = 500000` is worth noting separately: neither Anthropic
nor OpenAI advertises the context window on the wire.

## Finding 2: a third caching model

| Family | Path | How the cache is addressed |
|---|---|---|
| Anthropic Messages | `/v1/messages` | provider infers the prefix; `cache_control` marks breakpoints |
| OpenAI Chat Completions | `/v1/chat/completions` | automatic; reported as `prompt_cache_hit_tokens` |
| **OpenAI Responses** | **`/responses`** | **client names its own `prompt_cache_key`** |

This matters for this engine specifically. Replay's analysis reconstructs an
*implicit* prefix: it hashes what the client sent and infers what the provider
must have cached. Against a client that **declares** its cache key, that is a
different measurement problem, and probably an easier one, because the client
has already stated what it expects to be reused.

`prompt_cache_key` is a **request body field, not a header.** It appeared on the
two large model calls and was absent on a small one, so anything reading it must
handle absence.

## Finding 3: the request body

```text
include    input    model    prompt_cache_key    reasoning
store      stream   tools    tool_choice         temperature
max_output_tokens
```

`input` is an array of `{type, role, content}`. `tools` entries are
`{type, name, description, parameters}` carrying a JSON Schema. `reasoning` is
`{summary}` or `{effort, summary}`.

## Finding 4: the model call is not the only traffic

A single `grok -p` turn produced:

```text
POST /responses                  the model call
POST /sessions/<id>/turn-deltas  conversation state, server side
POST /sessions/<id>/signals
POST /traces                     telemetry
GET  /models  /settings  /feedback/config  /bundle/archive
```

There is **no local transcript**. Claude Code writes sessions to disk, which is
what Replay's offline path reads; this client POSTs conversation state to the
vendor instead. For Grok the proxy is not the better route, it is the only one.

**Unresolved:** `store: true` and server-side `turn-deltas` suggest the vendor
keeps the conversation, while `x-zero-data-retention: true` and
`x-data-retention: zdr` say it does not. Plausibly ZDR covers the model provider
while the CLI's session store is separate, but that is a guess and is recorded
here as unresolved rather than reconciled.

## Assumptions that died

Three, inside ten minutes, all mine:

- **"Grok speaks OpenAI chat completions."** It uses `/responses`.
- **"Its origin is `api.x.ai`."** It is `cli-chat-proxy.grok.com`.
- **"A Grok seat closes the chat-completions gate."** It exercises a different
  family, so [RELEASE-CRITERIA.md](../../RELEASE-CRITERIA.md)'s gate on that path
  is exactly as open as it was.

## What Replay did correctly

Everything forwarded byte for byte, with one warning per path:

> NOT PARSED /responses: Replay forwards this path unchanged and cannot read it.
> No ledger record, no spend cap, no error budget, no loop detection and no
> secret masking apply to it.

A user pointing Grok at Replay is told plainly they are getting nothing, rather
than shown an empty report that looks like a finding. One `502` was
`context canceled`, the client aborting its own request after 3ms, not a proxy
fault; a plain curl through the proxy returns the same status as the origin.

Incidentally: the first version of the capture proxy forwarded
`Host: 127.0.0.1:4020` upstream and every request was refused with 403.
`replay serve` rewrites the Host header correctly and never had the problem.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
