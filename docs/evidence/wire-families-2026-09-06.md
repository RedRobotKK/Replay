# Three wire families, not two, 2026-09-06

**What this measures:** which request shapes a terminal GenAI client actually
sends, captured off the wire from a live authenticated session rather than read
from documentation.

**Headline: the Grok CLI speaks a third family Replay does not parse, and it
returns the `x-ratelimit-*` headers that this project had never once captured.**

---

## How it was found

By setting up a Grok seat behind `replay serve` and watching what came through.
Not by reading an API reference. Every claim below has a captured shape behind
it.

Three assumptions died in the first ten minutes:

- **"Grok speaks OpenAI chat completions."** It does not. It uses `/responses`.
- **"Its origin is `api.x.ai`."** It is `cli-chat-proxy.grok.com`.
- **"Pointing a Grok seat at the proxy closes the chat-completions gate."** It
  exercises a different family entirely, so that gate is exactly as open as it
  was.

## The three families

| Family | Path | Cache control | Usage arrives in | Replay |
|---|---|---|---|---|
| Anthropic Messages | `/v1/messages` | implicit prefix, `cache_control` breakpoints | JSON body | parsed, LIVE fixture |
| OpenAI Chat Completions | `/v1/chat/completions` | automatic, reported as `prompt_cache_hit_tokens` | JSON body | parsed, LIVE fixture (DeepSeek) |
| **OpenAI Responses** | **`/responses`** | **`prompt_cache_key`, named by the client** | **SSE stream** | **NOT PARSED** |

The third column is the part that matters for this engine. Replay's whole
analysis reconstructs an *implicit* prefix: it hashes what the client sent and
infers what the provider must have cached. A client that **declares its own
cache key** is a different measurement problem, and probably an easier one,
because the client has already said what it expects to be reused.

## What the Responses request carries

Captured from two live model calls:

```text
include            input          model        prompt_cache_key
reasoning          store          stream       temperature
tool_choice        tools          max_output_tokens
```

`input` is an array of `{type, role, content}`. `tools` entries are
`{type, name, description, parameters}` with a JSON Schema. `reasoning` is
`{summary}` or `{effort, summary}`.

`prompt_cache_key` appeared on **two of three** `/responses` calls: present on
both large ones (121KB and 113KB responses) and absent on the small 306-byte
one. So it is conditional, not universal, and anything reading it must handle
its absence.

The response is **SSE**, not JSON. `stream: true` on every call, so usage counts
arrive as stream events. That is a fourth parsing shape, separate from the three
above.

## The headers, which are the real find

Grok returns all four of these on every model call:

```text
x-ratelimit-limit-requests        x-ratelimit-remaining-requests
x-ratelimit-limit-tokens          x-ratelimit-remaining-tokens
```

Plus two nobody has seen before:

```text
x-grok-context-window             x-zero-data-retention
x-data-retention
```

**`internal/proxy/quota.go` already allowlists the `x-ratelimit-` prefix.** The
capture code is written and correct. It has never fired, because `/responses` is
NOT PARSED and no ledger record is created to attach headers to.

This is the `RemainingHeaders` path that
[quota titration](quota-titration-2026-09-06.md) recorded as never having seen
real data. It is a **falling** counter, budget left, reported per request at
token granularity. That is a fundamentally different instrument from Anthropic's
**rising** `unified-5h-utilization` fraction, which carries two decimal places,
is account-wide, and needs twenty counter steps before a rate can be fitted at
all. Grok states the remaining budget outright.

## What else the CLI talks to

The model call is not the only traffic. A single `grok -p` turn produced:

```text
POST /responses                  the model call
POST /sessions/<id>/turn-deltas  conversation state, server-side
POST /sessions/<id>/signals
POST /traces                     telemetry
GET  /models  /settings  /feedback/config  /bundle/archive
```

`store: true` in the request plus server-side `turn-deltas` means the
conversation is kept by the vendor, which is a different architecture from
Claude Code writing transcripts to local disk. It also means there is no local
transcript for Replay to read, so for this client the proxy is the **only**
possible route.

## What Replay did correctly

Everything was forwarded byte for byte, and the proxy warned once per path:

> NOT PARSED /responses: Replay forwards this path unchanged and cannot read it.
> No ledger record, no spend cap, no error budget, no loop detection and no
> secret masking apply to it.

A user pointing Grok at Replay is told plainly they are getting nothing, rather
than shown an empty report that looks like a finding. The `502` seen once was
`context canceled`, the client aborting its own request after 3ms, not a proxy
fault; a plain curl through the proxy returns the same status as the origin.

## Method, and why the fixture is shapes rather than payloads

Captured through a shape-only proxy: keys and types recorded, values never.
Strings become `"str"`, numbers `"int"`. `Authorization` and `Cookie` are
dropped before anything is written. No prompt text, no completion text and no
credential is on disk.

That is the honest way to capture a wire format from a live personal account.
A full payload fixture would carry real conversation content, and the registry's
requirement is evidence of the **shape**.

One incidental finding: the first version of that capture proxy forwarded
`Host: 127.0.0.1:4020` upstream and every request was refused with 403.
`replay serve` rewrites the Host header correctly and never had the problem.

## Scope

One client, one account, one machine, three model calls in a single headless
turn. Enough to establish that the family exists and is in real use. **Not**
enough to characterise it: no multi-turn session, no cache hit observed, no
usage numbers parsed out of the SSE stream, and no second client on the same
family to confirm the shape generalises beyond Grok's implementation.

Raw shapes: `$CLAUDE_JOB_DIR/tmp/grok-shapes.jsonl`, 14 requests.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
