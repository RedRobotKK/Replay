# Three wire families, not two, 2026-09-06

**What this measures:** which request shapes a terminal GenAI client actually
sends, and what its rate-limit headers actually say, captured off the wire from
a live authenticated session rather than read from documentation.

## Summary

**The Grok CLI speaks a third wire family that Replay does not parse**, and it
returns the four `x-ratelimit-*` headers this project had never once captured.

The headers turned out to be less useful than they look. **Across 8 model calls
and about 940KB of responses, `remaining` never moved.** They advertise the plan
ceiling; they do not report consumption. That claim was written the other way
round first, from header names before a single value had been read.

**There is no live quota signal anywhere in this client's traffic.** Every
endpoint was checked. What the polled `/settings` endpoint carries instead is an
18-key **server-pushed context strategy**: pruning, flushing and memory
injection, decided by the vendor and obeyed by the client.

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

**Scope: one client, one account, one machine, eight model calls across several
headless turns, plus one multibyte round trip.** Enough to establish the family
exists, is in real use, and that its rate-limit headers do not move under
ordinary work. Not enough to characterise it. No cache hit was observed, no
usage was parsed out of the SSE stream, and there is no second client on this
family to show the shape generalises beyond Grok's implementation.

Raw shapes: `$CLAUDE_JOB_DIR/tmp/grok-shapes.jsonl`.

## Finding 1: the rate-limit headers exist, and did not move

Captured values, identical on every model call:

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

**`remaining` equals `limit` on all of them.** Eight model calls, roughly 940KB
of response, every reading identical:

```text
53000000 / 53000000 tokens        8300 / 8300 requests        x8
```

Eight samples on one account cannot prove the counter never moves. They are
enough to say it does not move under ordinary use, which is the case a guard
would be built for.

**A retraction.** An earlier version of this file called these "a falling
per-request counter at token granularity" and "a significantly higher-fidelity
instrument" than Anthropic's rising utilization fraction. That was written from
header *names*, before a single value had been read. On the values, it does not
fall. A `remaining` figure that always equals the limit is not a measurement of
consumption; it is the same shape as a healthcheck that cannot fail.

Eight samples still cannot show it *never* moves. They are enough to say that a
guard watching these headers for a decrement would not fire during normal work.

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

## Finding 5: no live quota source exists in this client's traffic

`/settings` is polled repeatedly and was the obvious candidate for a heartbeat.
It is gzip, which is why an earlier capture recorded it as 1,767 bytes of
`<non-json>`. Decompressed, it holds **40 keys and zero quota state**: no usage,
no remaining, no window, no reset. The only token-adjacent key is
`flush_soft_threshold_tokens`, which is a context threshold rather than a budget.

Across `/responses`, `/settings`, `/models`, `/feedback/config`, `/traces` and
`/sessions/*`, the `x-ratelimit-*` headers are the only quota-shaped thing on the
wire, and they are frozen.

**So a quota guard for this surface cannot poll the vendor on a heartbeat,
because there is nothing live to poll.** It would have to be built from what
Replay counts itself.

## Finding 6: the context strategy is the server's, not the client's

18 of those 40 settings keys configure context behaviour:

```text
pruning_enabled          pruning_keep_last_n_turns    pruning_soft_trim_threshold
flush_enabled            flush_soft_threshold_tokens  flush_idle_timeout_secs
memory_enabled           memory_embedding_model       memory_embedding_dimensions
memory_search_min_score  memory_search_max_results    memory_mmr_enabled
memory_mmr_lambda        memory_temporal_decay_enabled
memory_temporal_decay_half_life_days                  memory_watcher_enabled
memory_initial_injection_enabled                      memory_initial_injection_min_score
```

| | Who decides what stays in context |
|---|---|
| Anthropic | the **client**, via `cache_control` breakpoints; `context-management-2025-06-27` is opt-in |
| **Grok** | the **server**, pushed as config the client obeys |

The vendor can change pruning and retrieval behaviour without a client release.
For Replay this is the sharpest contrast found: advice of the form "reorder your
prefix" or "cap your tool output" is aimed at a decision the client does not
own on this surface.

## Finding 7: multibyte, and where it actually bites

A round trip of Japanese, an emoji and combining diacritics came back
byte-perfect. The input was **24 characters, 55 UTF-8 bytes, 2.29 bytes per
character** against roughly 1.0 for ASCII prose.

The exposure this creates in Replay is narrower than it first appears, and worth
stating precisely. `analysis.Fit` measures tokens per byte **from the session's
own turns**, so a Japanese session fits a Japanese ratio and self-corrects.
`defaultTokensPerByte = 0.25` is used *only* when a session offers no fittable
turn, and its own comment calls it "a coarse prose average", reported as
estimated.

So the risk is confined to the fallback path: a short CJK session with nothing
to fit on is priced with an English constant, and the figure is marked estimated
without saying that the constant does not apply to its script. Worth a note in
the output rather than a new estimator.

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
