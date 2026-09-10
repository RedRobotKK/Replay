# 0019. Surfaces declare what they can be asked, and the declaration is probed

**Status:** Proposed
**Date:** 2026-09-09

## Context

Replay reads one surface. Every figure it prints was derived from Claude Code's
JSONL transcripts or captured on the wire in front of the first-party Messages
API. Extending to other providers is asked for regularly and is usually framed
as a parsing problem. It is not one. Parsing is the easy part.

**The wire shape is the product.** The measurement Replay exists to make is
cache accounting, and `internal/transcript.WireUsage` reads four fields that
decide whether it is possible at all:

```go
CacheCreation int   `json:"cache_creation_input_tokens"`
CacheRead     int   `json:"cache_read_input_tokens"`
CacheBreak    *struct{ Short, Long int }  // ephemeral_5m / ephemeral_1h
```

Those fields separate a cache **write** from a cache **read**, and separate the
five-minute tier from the one-hour tier. Avoidable spend is the difference
between what was written and what a different layout would have read. A surface
that does not distinguish a write from a read cannot answer the question,
however cleanly its responses parse.

The surfaces differ on exactly this, and not by accident — they differ because
their caching products differ:

- **Anthropic first-party** reports all four fields. Caching is explicit:
  the caller places `cache_control` markers, so there is a decision that can
  have been made wrong, which is what makes advice possible.
- **Bedrock and Vertex** re-expose the same fields. `classifyRoute` already
  distinguishes them, and they carry a second fact Replay cannot otherwise
  establish: neither sells a flat seat, so that traffic is metered by
  construction.
- **OpenAI** reports one number, `cached_tokens` under `prompt_tokens_details`.
  Caching is automatic. There is no write signal, no TTL tier, and no caller
  decision to have gotten wrong. A hit rate is computable. Avoidable spend is
  not.
- **Gemini** reports `cachedContentTokenCount` against explicit cache objects
  with caller-set TTLs. A different shape of the same question, answerable, but
  scoped to caches the caller created rather than to the prompt prefix.
- **xAI (Grok)** was filed under the OpenAI chat-completions family in this
  record's first draft, and the repository's own frozen-defect harness rejected
  it — FD-5 exists because that exact assumption was made and retracted once
  before. Measured off a live authenticated session
  ([wire families, 2026-09-06](../evidence/wire-families-2026-09-06.md)): it
  posts `POST /responses` to `cli-chat-proxy.grok.com`, not chat completions and
  not `api.x.ai`. It is a **third wire family**, and Replay does not parse it
  today. Its caching is a third model again — the client declares a
  `prompt_cache_key` and the provider does not infer a prefix. Transport is SSE
  with `stream: true` on every call, so usage arrives in stream events rather
  than a response body. No cache hit was observed and no usage was parsed out of
  that stream, so its cache capability is genuinely **unprobed**, not assumed.
- **Ollama and local runtimes** report no cache accounting and no prices. There
  is no money on this surface at all. Context growth and tool-call bytes are
  real and measurable; nothing denominated in dollars is.
- **Cursor** is not one surface but three partial ones, and calling it closed
  was wrong. Its model settings accept an OpenAI base-URL override, so
  bring-your-own-key traffic is proxy-reachable like any OpenAI-compatible
  endpoint. Its chat history is kept in local SQLite under the editor's
  workspace storage, which is readable but undocumented, unstable across
  versions, and — decisively — carries no usage object, so it yields prompt
  structure and context growth and no money. Team plans expose an
  administrative usage API giving spend per user with no cache accounting.
  What *is* closed is the default path: Pro and agent-mode traffic routed
  through Cursor's own backend, which ignores a local proxy. Two of these three
  paths are unverified in this repository and are recorded as such below.
- **Copilot and the other editor assistants** sit in the same shape as Cursor:
  an enterprise usage API for spend, a proxy setting that covers some traffic
  and not the agent path, and no per-request cache accounting anywhere. Closed
  for the measurement Replay exists to make, open for context growth if someone
  does the work.

Two further facts constrain the design.

**A missing capability must not read as a zero.** ADR-0018 fixed the rule that
absence, zero and unknown are three values. Adding OpenAI under the current
architecture would emit `avoidableUsd: 0` on every OpenAI report — a measured
zero that was never measured. Since 2026-09 those figures flow into pooled
corpus contributions across contributors, so the defect would not stay local to
one reader's terminal; it would bias a published population figure downward and
look exactly like good news.

**A hand-maintained capability table rots.** `PriceTableCheckedAt` exists as a
fact separate from `PriceTableVersion` for this reason: with one date, a table
that is old and correct is indistinguishable from one nobody has looked at. A
table asserting what six providers report would be stale within a quarter and
would fail silently, because a field that stopped being emitted reads as a zero.

## Decision

A surface is described by what it can be asked, not by a parser that implies it
can be asked anything.

```go
type Surface struct {
    Name, Route  string
    Intake       Intake      // Transcript | Proxy | UsageAPI
    Reports      Capability  // what this wire actually carries
    RulesVersion string
    PriceTable   string
    ProbedAt     string      // when Reports was last established by observation
}

type Capability uint16

const (
    CapCacheRead Capability = 1 << iota  // read tokens reported
    CapCacheWrite                        // write distinguished from read
    CapCacheTTLTiers                     // 5m and 1h separable
    CapPerTurnUsage
    CapRequestID
    CapListPrices                        // a price table exists for these ids
)
```

Four rules follow, and each is enforceable rather than advisory:

1. **A figure requiring an absent capability reports that it was not measured
   on this surface.** Never zero, never omitted. The existing truth tiers from
   ADR-0002 gain a third state: a figure is *Measured*, *Estimated*, or
   *Unavailable on this surface*, and the third names the surface and the
   missing capability so a reader knows it is a property of where they are
   pointed and not of their traffic.

2. **`Reports` is probed, not asserted.** A capability probe sends one minimal
   request twice against a surface, reads what the usage object actually
   contains, and derives the bitset from the response. `ProbedAt` dates it, and
   a descriptor whose probe is older than the staleness window prints its age
   the way `PriceTableAgeNote` already does. This is ADR-0014 applied to
   provider coverage: a capability claim that has never been observed to fail
   is not evidence.

3. **Intake is separate from capability.** Three adapters, not six parsers.
   Transcript intake reads files an agent already wrote. Proxy intake is
   ADR-0001's byte-transparent pass-through, unchanged. Usage-API intake reads
   the vendor's own billing records. A surface may be reachable by more than
   one, and the same `Capability` vocabulary describes what each yields.

4. **Corpora from different surfaces never pool.** `Pool.Add` already refuses
   submissions whose `RulesVersion` disagrees. That rule generalizes for free
   and is why it was written before a second surface existed.

### Connecting to each surface, and what supporting it means

The mechanism is named per surface, along with the honest scope of support:

| Surface | How Replay connects | Automatable | What it can then answer |
|---|---|---|---|
| Anthropic 1P (Claude Code) | Read JSONL from the known transcript roots | Fully — discovery, no credentials, no spend | Everything. The reference surface |
| Anthropic 1P (direct) | Proxy on loopback; `ANTHROPIC_BASE_URL` | Yes — env injection into a child process | Everything, Measured tier |
| Bedrock | Proxy; SDK endpoint override | Yes, where the SDK honours it | Everything, plus metered by construction |
| Vertex | Proxy; SDK endpoint override | Yes, same caveat | Same as Bedrock |
| OpenAI | Proxy; `OPENAI_BASE_URL` | Yes | Cache hit rate, prefix stability, context growth, spend. **Not avoidable spend** |
| xAI (Grok) — proxy | Proxy; `/responses` at `cli-chat-proxy.grok.com`. **A third wire family, not parsed today** | Yes, once parsed | Unknown until probed; SSE usage events must be read first |
| xAI (Grok) — transcript | Read `~/.grok/sessions` (3.8 GB / 6,787 files on one machine) | Yes | Prompt structure, context growth. Usage fields **unverified** |
| Gemini | Proxy; endpoint override | Yes | Utilisation of explicit caches; prefix stability; context growth; spend |
| Ollama / local | Proxy; `OLLAMA_HOST` | Yes | Prefix stability, context growth, tool-call bytes. No money |
| Cursor — BYO key | Proxy; the editor's OpenAI base-URL override | Yes | As OpenAI, for the traffic that honours it |
| Cursor — local history | Read the editor's workspace SQLite | Unverified | Prompt structure, context growth. No usage object, so no money |
| Cursor / Copilot — teams | Credentialed pull from the admin usage API | Unverified | Spend per user. No cache accounting |
| Cursor / Copilot — agent path | No mechanism exists | No | Nothing. Routed through the vendor's backend |
| Vendor usage APIs | Credentialed pull | Partly — auth per vendor | Ground truth to calibrate the above against |

Rows marked **unverified** have not been read or probed by anyone working on
this repository. They are listed because a path that plausibly exists and has
not been checked is a different thing from one that does not exist, and
collapsing the two is the distinction this whole record is about. `ProbedAt` is
where that gets recorded per surface rather than in a table in a document.

### What Replay is worth where no cache write is reported

The capability table above reads as a list of things Replay cannot do on a
third-party surface. That framing is wrong, and correcting it changes what
should be built.

**The provider's usage object reports a consequence. The request bytes contain
the cause.** A cache write tells a reader they were re-billed. It does not tell
them why, and "why" is the only part they can act on. The cause is always the
same thing — the prompt prefix changed between turns when it did not need to —
and it is visible in the requests alone, on any surface, with no cooperation
from the provider whatsoever. ADR-0001's byte-transparent proxy already puts
those bytes in reach; `internal/analysis/blame.go` and the break-cause
machinery already do the attribution.

So the analyses split by what they actually consume, not by provider:

| Analysis | Needs | Anthropic | OpenAI / Grok | Gemini | Ollama | Cursor BYO |
|---|---|---|---|---|---|---|
| Prefix stability, and what mutated it | request bytes | ✅ | ✅ | ✅ | ✅ | ✅ |
| Tool-definition weight and attribution | request bytes | ✅ | ✅ | ✅ | ✅ | ✅ |
| Tool-result bloat, trimmable blocks | request bytes | ✅ | ✅ | ✅ | ✅ | ✅ |
| Repeated content in one context | request bytes | ✅ | ✅ | ✅ | ✅ | ✅ |
| Context growth per turn | request bytes | ✅ | ✅ | ✅ | ✅ | ✅ |
| Total and per-task spend | tokens + price table | ✅ | ✅ | ✅ | — | ✅ |
| Cache hit rate | `CapCacheRead` | ✅ | ✅ | ✅ | — | ✅ |
| **Avoidable spend** | `CapCacheWrite` | ✅ | ❌ | partial | — | ❌ |

One row is unavailable off the reference surface. Seven are not.

But there is a second axis, and the evidence says it matters more than the
capability bitset: **who decides what stays in context.**

Advice of the form "reorder your prefix" or "cap your tool output" is only
actionable where the *client* owns that decision. On Anthropic it does — the
caller places `cache_control` breakpoints, and `context-management-2025-06-27`
is opt-in. On Grok it does not: the polled `/settings` endpoint carries an
18-key server-pushed context strategy — pruning, flushing and memory injection —
that the vendor sets and the client obeys, and the vendor can change it without
a client release. That was the sharpest contrast the wire capture found, and it
means a prefix-stability finding on that surface names something its user cannot
act on.

So the descriptor carries it, and advice is gated on it rather than on the
capability bits:

```go
type Decides uint8
const (
    DecidesUnknown Decides = iota
    DecidesClient   // the caller owns the prefix: advice is actionable
    DecidesServer   // the vendor prunes and injects: advice names a lever they lack
    DecidesMixed
)
```

| Surface | Cache model | Who decides context | Prefix advice actionable |
|---|---|---|---|
| Anthropic | explicit `cache_control` | client | ✅ |
| OpenAI | automatic, prefix-keyed | client | ✅ — and it is their **only** lever |
| Gemini | explicit cache objects | client | ✅ for those caches |
| Grok | client-declared `prompt_cache_key` | **server** (18-key pushed strategy) | ❌ — the lever is the key, not the prefix |
| Ollama | none | client | ✅, paid in latency not dollars |
| Cursor BYO | as the upstream model | client | ✅ for that traffic |
| Cursor agent | vendor-managed | vendor | ❌ |

OpenAI is where the argument is strongest, and Grok is where the first draft of
this record got it backwards. OpenAI's caching is automatic and keyed entirely
on the prefix: there is no marker to get wrong, which sounds like less to
diagnose and is the opposite — prefix stability is the only lever those users
have, and nothing in their tooling tells them when they broke it. They get a hit
rate with no attribution. Grok's users have a different lever and would be
handed the wrong advice by the same analysis.

Ollama stays worth supporting despite there being no money on it: a local
runtime re-processing a mutating prefix pays in latency and in a context window
that fills sooner, both measurable from the same bytes.

Automation lands in three tiers, and they should ship in this order because
each is a precondition for trusting the next:

1. **Discovery.** Glob the known transcript roots, sniff the first lines,
   classify. Already partly built in `defaultTranscriptRoots` and
   `explainNoCorpus`. No credentials, no spend, no network.
2. **Attach.** `replay wrap <cmd>` runs the proxy on a loopback port and sets
   the base-URL variables for the child process. Works for anything honouring a
   base URL. It fails on anything that pins its endpoint or its certificate,
   and it must say so rather than reporting an empty corpus.
3. **Probe.** The capability probe above. It costs real requests against a real
   key per surface, which is why it is gated behind the same constraint as the
   other measurements this project has declined to fake.

## Consequences

- Adding a surface becomes a bounded task with a defined refusal: implement an
  intake, probe the capability set, and let every analysis that needs a missing
  capability say so. It stops being an open-ended promise that the tool "works
  with OpenAI".
- The product argument stops depending on the avoidable-spend figure. Seven of
  the eight analyses need only request bytes, so a third-party surface is not a
  degraded version of the reference surface — it is the same diagnostic with one
  row missing and a different headline. That headline is *what mutated your
  prefix*, and on automatic-caching surfaces it is the only lever their users
  have.
- Replay can say, for the first time, what it *cannot* tell a reader on the
  surface they are actually using. That is a better first contact than a report
  full of zeroes.
- The headline claim narrows in public. "Works with any provider" is not
  available; "reads six surfaces, and answers a different question on each,
  stated per surface" is. This is the same trade ADR-0002 made when it put a
  truth tier on every figure, and it is accepted for the same reason.
- Output gets busier again. A surface line joins the rules version and the
  price table date on reports that already carry both.
- Tier 3 stays blocked on keys and spend, so the capability table ships
  hand-written and dated, with `ProbedAt` empty and printed as unprobed. That
  is a known weakness, disclosed in the field rather than in a comment, and it
  is the specific thing to fix first when a key exists.
- Risk accepted: a provider that adds a cache-write field will be under-read
  until it is re-probed. The probe date is what makes that discoverable instead
  of invisible, and it is strictly better than the current situation, where the
  question is not asked at all.

## Alternatives considered

**A `Provider` interface with a `Parse` method.** The obvious design, and it
loses because it implies every surface answers the same questions. The
implementations that cannot would return zero, which is the defect this
repository is organised against, arriving through the front door and now
flowing into pooled contributions.

**Support only Anthropic and say so.** Honest and cheap, and it was the status
quo. It loses because the closed surfaces are closed for a reason worth stating
and the open ones are genuinely reachable — Ollama and OpenAI both answer real
questions about context growth, and refusing to ask them serves nobody.

**Normalise every provider into the Anthropic usage shape.** Rejected: it makes
the absence of a field indistinguishable from a zero in that field at the point
of conversion, which is the same defect one layer lower and harder to see.

**Derive capability from the model id.** Rejected for the reason `classifyRoute`
already documents about billing mode: the id settles the route and does not
settle what the endpoint reports. A first-party id is emitted by an API key and
a subscription alike, and a capability guessed from a string would be reported
with the same confidence as one observed.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
