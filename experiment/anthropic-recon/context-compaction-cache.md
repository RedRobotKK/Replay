# Context, compaction and prompt caching: a reconnaissance map

**Compiled 2026-09-29.** Scope: areas D (context surfaces), E (compaction
surfaces) and G (prompt caching surfaces). Every row carries an evidence class
and, where the row is observable, what kind of observer can see it.

## Read this first: no provider page was fetched on this run

**Every `WebFetch` and `WebSearch` attempt on 2026-09-29 failed**, fourteen
attempts across five URLs, each with the same error text:

> The server-side auto mode classifier is temporarily unavailable, so auto mode
> cannot determine the safety of WebFetch right now.

Delegation to subagents failed on the same classifier. Local file reads were
unaffected, which is why the repository evidence below is complete and the
provider evidence is not.

**The consequence is stated plainly rather than papered over.** This document
contains **zero rows classed DOCUMENTED**, because DOCUMENTED was defined as
"with URL", and a URL nobody opened is a citation to a page, not evidence from
it. Rows that a provider page would settle are classed **UNKNOWN** and listed in
[What a re-run must fetch](#what-a-re-run-must-fetch) with the exact question to
put to each page.

Where this repository encodes a provider rule -- and `internal/cachemodel`
exists to do exactly that -- the row is classed **OBSERVED**, because what was
verified is the repository's contents, not the provider's page. The repository
records its own fetch dates and those are carried through. That distinction is
the whole point of the classification and it is not softened anywhere below.

## Evidence classes used

| Class | Means |
|---|---|
| **DOCUMENTED** | Read on a provider page, with the URL. **Used nowhere in this document.** See the banner above |
| **OBSERVED** | Visible in this repository: source, test fixture, or a dated evidence file |
| **DERIVED** | Computed from OBSERVED values, with the computation shown |
| **HYPOTHESIS** | A mechanism proposed to explain an observation, not tested |
| **UNKNOWN** | Not established. Includes every undocumented provider internal |

**Anthropic's private cache and compaction internals are never inferred.** An
undocumented mechanism is UNKNOWN in the table, not a plausible guess in prose.

## Observability classes used

The question for Replay is not whether a surface exists but who can see it.

| Class | Means |
|---|---|
| **T** | Observable from a transcript on disk, after the fact |
| **P** | Observable from a recording proxy, which sees the request body |
| **T+P** | Both, usually at different fidelity |
| **PROV** | Observable only from provider-side data: the account, the invoice, the provider's own logs |
| **NONE** | Not observable by an external tool at all |

`T` and `P` are not interchangeable. **A transcript is what the client chose to
write down; a proxy is what actually went on the wire.** The system prompt is
the clearest case: it crosses the wire on every request and a Claude Code
transcript does not store it.

---

# D. Context surfaces

The governing question: what is placed **into** the model's context window
versus retrieved dynamically on demand. "Available to Claude" is not the same
claim as "in the context window", and where nothing establishes which, the row
says UNKNOWN.

## D1. Context window sizes

| Row | Statement | Class | Observability | Note |
|---|---|---|---|---|
| D1.1 | Context window size per model | **UNKNOWN** | PROV | No provider page was read on this run. The repository does not encode window sizes anywhere |
| D1.2 | A 1M-token context tier exists, and which models or account tiers get it | **UNKNOWN** | PROV | Not established here. Do not assume from the compaction ceiling in E2.3, which is a different quantity |
| D1.3 | Replay does not know the window size and says so | **OBSERVED** | -- | `internal/analysis/context.go`: `Share` is "over the attributed total, not over the context window: the window is not known here and claiming it would be a guess" |
| D1.4 | A transcript states the window size for its model | **UNKNOWN** | T | No field for it was found in `internal/transcript/claudecode.go`. Absence of a reader is not proof of absence of a field |
| D1.5 | Total prompt size actually processed, per request | **OBSERVED** | T+P | `Usage.PromptTotal() = input_tokens + cache_creation_input_tokens + cache_read_input_tokens` (`internal/transcript/types.go:117`). This is a **measured lower bound on context occupancy**, not the window |

**D1.5 is the load-bearing row of this section.** An external tool cannot read
the window, but it can read exactly how many tokens the provider processed, and
the sum of the three usage fields is that number. Occupancy as a percentage
needs a denominator nobody publishes in a machine-readable form.

## D2. What enters context, by source

| Row | Surface | In context, or retrieved | Class | Observability |
|---|---|---|---|---|
| D2.1 | Conversation history | In context on every request, resent each turn | **OBSERVED** | T+P |
| D2.2 | Tool results | In context as `tool_result` blocks, carried forward | **OBSERVED** | T+P |
| D2.3 | Tool **definitions** | In context, as a block that precedes the messages | **OBSERVED** | **P only** |
| D2.4 | System prompt | In context | **OBSERVED** | **P only** |
| D2.5 | CLAUDE.md | Whether it is injected into the system prompt, injected as a message, or read on demand | **UNKNOWN** | P would settle it |
| D2.6 | Project files | Whether any are preloaded versus read by tool call | **UNKNOWN** | T shows the `Read` calls; it cannot show a preload it never logged |
| D2.7 | Attachments and images | Image blocks are context content | **OBSERVED** | T+P |
| D2.8 | Connectors and MCP servers | Their **tool definitions** enter context; a connector completing its handshake mid-session appends that connector's whole tool block | **OBSERVED** | **P only** |
| D2.9 | Skills | Whether frontmatter only, or the whole body, enters context. Progressive disclosure | **UNKNOWN** | P would settle it |
| D2.10 | Subagent context | Subagent lanes are separate request streams | **OBSERVED** | T+P |
| D2.11 | Agent handoff payload | What the parent receives back and in what form | **UNKNOWN** | T likely, not verified here |

### Evidence for D2.2 and D2.7

`internal/transcript/types.go` defines `Block` with `ToolUseID`, `ToolName`,
`CallKey`, `IsError` and `Bytes`; `ToolUseID` "links a tool_use block to its
tool_result and back". `internal/transcript/imageblocks_test.go` exists, so
image blocks are parsed. **OBSERVED.**

### Evidence for D2.3, D2.4 and D2.8

`internal/cachemodel/anthropic.go` names the break causes and explains which
observer can decide each one:

> `CausePrefixChange` covers a prefix change where BOTH halves moved [...]
> across the 30-lane trial of 2026-09-06, `system_bytes` never moved once and
> every real prefix change was the tool SET changing.

> `CauseToolsChanged` is the tool definitions alone, which is the common case in
> practice: an MCP connector finishing its handshake mid-session drops
> `WaitForMcpServers` and appends that connector's whole tool block.

**`system_bytes` is a proxy measurement.** The comment on `ClassifyBreak` states
that the first three causes "can be decided from usage and timing alone; the
rest need the message history". A transcript-only observer can therefore reach
TTL-expiry and model-change, and cannot separate a system-prompt change from a
tool-set change. **OBSERVED, and it is the sharpest T-versus-P boundary in this
document.**

### Why D2.5 is UNKNOWN and not a safe guess

CLAUDE.md is plainly available to the model. That is not the question. Whether
its bytes sit in the system block of every request, or are read once into a
message, or are fetched on demand, changes what a cache breakpoint should be
placed around and changes whether editing it breaks a session's prefix.
**A proxy settles this in one session by diffing the system block against the
file. Nothing in this repository has recorded that diff.**

## D3. Context-management controls on the request

| Row | Statement | Class | Observability |
|---|---|---|---|
| D3.1 | Anthropic responses carry a `context_management` object | **OBSERVED** | T+P |
| D3.2 | It contains `applied_edits`, a list | **OBSERVED** | P |
| D3.3 | Each applied edit reports a cleared-token count | **OBSERVED** | P |
| D3.4 | Replay reads both into `AppliedEdits` and `ClearedInputTokens` | **OBSERVED** | -- |
| D3.5 | Which edit strategies exist, their parameter names, their triggers, their defaults | **UNKNOWN** | PROV |
| D3.6 | Whether Claude Code enables any of them | **UNKNOWN** | P would settle it in one session |

**Evidence.** Four live-capture fixtures under
`internal/ledger/testdata/anthropic/` each end their `message_delta` with
`"context_management":{"applied_edits":[]}`. `internal/ledger/response.go:42`
declares `AppliedEdits []struct{...}` and `totals()` returns
`len(c.AppliedEdits), cleared`. `internal/ledger/record.go:254` documents the
pair as "the provider's own" figures.

**Every captured fixture shows an empty `applied_edits` array.** That is a
present-but-empty field, which is a third state and not evidence that the
mechanism is off, absent, or unused elsewhere. No capture in this repository has
a non-empty one.

## D4. Compaction and the context attribution Replay publishes

| Row | Statement | Class |
|---|---|---|
| D4.1 | `EnteredContext` counts a block cleared by compaction, by provider context editing, or by a history-shrinking break as still present | **OBSERVED** |
| D4.2 | The divergence is always in the over-reporting direction | **OBSERVED** |
| D4.3 | It is largest on sessions where Replay's own leading recommendation was taken | **OBSERVED** |

`internal/analysis/context.go` states all three in its type comment, and draws
the conclusion itself: "Calling this 'what is in your context' would be wrong in
the one case the reader most cares about, so it is not called that anywhere."

**This is the honest note that makes the section usable.** A tool that reads
transcripts can attribute what entered context by tool; it cannot, from the same
data, say what is in context now, because the removals are recorded in a
different place than the additions.

---

# E. Compaction surfaces

## E1. What the provider and product documentation say

| Row | Question | Class | Note |
|---|---|---|---|
| E1.1 | Automatic context management summarises earlier messages | **UNKNOWN** | The brief states Anthropic documents this. **Not verified on this run.** No page was fetched, so it is not restated here as though it were |
| E1.2 | Conversations triggering it consume more usage | **UNKNOWN** | Same. See E4 for what the repository can and cannot say about this |
| E1.3 | The documented trigger threshold | **UNKNOWN** | See E2.3 for a **measured** ceiling, which is a different and weaker claim |
| E1.4 | What is documented to survive compaction | **UNKNOWN** | |
| E1.5 | Whether repeated compaction is documented to degrade quality | **UNKNOWN** | |
| E1.6 | Whether behaviour differs by model or by product | **UNKNOWN** | |
| E1.7 | Exact wording of `/compact` and `/clear` | **UNKNOWN** | |

**Nothing in E1 is filled in from recollection.** The brief supplied a summary
of what Anthropic states; a summary in a brief is not a fetched page, and
promoting it to DOCUMENTED here would manufacture a citation.

## E2. What a Claude Code transcript records about compaction

This is where the evidence is strong, and it is all OBSERVED.

| Row | Statement | Class | Observability |
|---|---|---|---|
| E2.1 | A compaction is a **pair** of records, not one | **OBSERVED** | T |
| E2.2 | The boundary is `type:"system"`, `subtype:"compact_boundary"`, carrying `compactMetadata` and no message; the summary is the **next** line, `type:"user"` with `isCompactSummary` set | **OBSERVED** | T |
| E2.3 | `compactMetadata` carries `trigger`, `preTokens`, `postTokens`, `cumulativeDroppedTokens`, `durationMs` | **OBSERVED** | T |
| E2.4 | The boundary is the record that counts; a summary is counted only when it appears without one | **OBSERVED** | -- |
| E2.5 | Absence of `compactMetadata` on a compaction record means a client that stopped reporting sizes, **not** a compaction that dropped nothing | **OBSERVED** | -- |

**Source.** `internal/transcript/claudecode.go:38-43` and `:159-193`, with the
pairing rule spelled out in a comment: "Accepting either marker independently
counted every compaction" twice, which is the defect the pairing fixes.

**E2.2 is the single most useful fact in this document for an external tool.**
Compaction is not a hidden provider event. The client writes it down, with
before and after sizes and a duration, in a format any transcript reader can
parse.

## E3. The measured shape of compaction on a real corpus

From `docs/evidence/compaction-and-index-2026-09-06.md`, measured 2026-09-06 on
one machine, 1,520 transcripts. **All OBSERVED**, with the DERIVED rows marked.

| Quantity | Value | Class |
|---|---:|---|
| Records carrying `compactMetadata` | 39 | OBSERVED |
| Of those, `trigger: "auto"` | **39 of 39** | OBSERVED |
| Median `preTokens` | 999,029 | OBSERVED |
| Median `postTokens` | 23,218 | OBSERVED |
| **Median retention, median of per-event ratios** | **2.55%** | DERIVED |
| Retention as ratio of the medians | 2.32% | DERIVED |
| Total pre / post | 32,038,701 / 1,501,091 | OBSERVED |
| Tokens discarded from context | 30,537,610 | DERIVED |
| Wall clock spent compacting | 73.5 minutes | OBSERVED |
| Per-event duration | 103 to 180 seconds | OBSERVED |
| Events with `preTokens` within 1% of 1,000,000 | **30 of 39** | OBSERVED |
| Transcripts containing a compaction | 9 | OBSERVED |
| Share of all re-billed tokens held by those 9 | 31.8% | DERIVED |

**Two derived readings, and their limits.**

**E3.1, the ceiling.** `preTokens` sits within 1% of one million on 30 of 39
events. The evidence file reads this as "triggered by exhaustion, not by
policy". **DERIVED**, and bounded: it is a fact about one client version, one
account and one model configuration, and it is not a statement that the
threshold is one million, nor that the context window is one million. The
observed quantity is where compaction fired, which is a floor on the window and
not the window.

**E3.2, the anomaly that was kept.** One record reports `postTokens` **above**
`preTokens`: 22,303 against 296,742. It is retained and clamped to a zero drop
rather than discarded, "because a single record producing a negative would
silently offset several good ones"
(`internal/transcript/compaction_test.go`, TestCM2). **OBSERVED.** What produced
it is **UNKNOWN**, and the repository does not name a cause.

**E3.3, the sampling limit, stated by the source itself.** One operator, one
machine, one client version. 39 events in 9 transcripts, and **22 of the 39 come
from a single session**. The evidence file calls this "close to a case study".
Any figure from E3 quoted as a general property of compaction would be a small
sample read as a law.

## E4. What survives, what is lost, what it costs

| Row | Question | Class | Observability |
|---|---|---|---|
| E4.1 | Message history before the boundary is replaced by a summary | **OBSERVED** | T |
| E4.2 | Median 2.55% of the pre-compaction token count survives, on this corpus | **DERIVED** | T |
| E4.3 | Whether CLAUDE.md survives compaction | **UNKNOWN** | P. Depends on D2.5, which is also UNKNOWN |
| E4.4 | Whether project files or tool state survive | **UNKNOWN** | P |
| E4.5 | Whether the system prompt and tool definitions survive | **UNKNOWN** | **P settles this in one session**: they are sent per request, so the question is whether the post-compaction request carries the same block |
| E4.6 | Whether the summarisation call is itself billed, and appears as a request | **UNKNOWN** | T+P would settle it. Not verified here |
| E4.7 | Compaction costs 103 to 180 seconds of operator wall clock per event | **OBSERVED** | T |
| E4.8 | Sessions containing compactions are the expensive ones: 9 transcripts, 31.8% of re-billed tokens | **DERIVED** | T |
| E4.9 | Whether repeated compaction degrades output quality | **UNKNOWN** | **NONE.** No transcript or proxy field measures quality |

**E4.6 deserves its own sentence, because it is the claim in the brief that the
repository can neither confirm nor deny.** "Long conversations that trigger it
consume more usage" is consistent with E4.8, but E4.8 measures something else:
it measures that long sessions re-bill a lot of context, which is true whether
or not the summarisation call is billed. **Treating E4.8 as confirmation of
E1.2 would be a sample agreeing with a census by a different procedure.**

**E4.9 is NONE, not UNKNOWN-but-measurable.** Degradation is an output-quality
claim. No usage field, no ledger record and no transcript record carries it.

## E5. Manual versus automatic

| Row | Statement | Class | Observability |
|---|---|---|---|
| E5.1 | `compactMetadata` carries a `trigger` field | **OBSERVED** | T |
| E5.2 | Every one of the 39 observed records reads `trigger:"auto"` | **OBSERVED** | T |
| E5.3 | Whether a manual `/compact` writes a different `trigger` value | **UNKNOWN** | T would settle it, with one manual `/compact` and one `grep` |
| E5.4 | Whether `/clear` writes any record at all | **UNKNOWN** | T would settle it the same way |

**E5.3 and E5.4 are the cheapest UNKNOWNs in this document to close.** They need
no provider cooperation and no API call: run `/compact`, run `/clear`, read the
transcript.

---

# G. Prompt caching surfaces

## G1. Rules this repository encodes, and where they came from

`internal/cachemodel/anthropic.go` opens by declaring its own standard:

> Every value here is a documented provider rule, not a measurement. When the
> provider changes a rule, this file changes and `RulesVersion` moves.

It carries three dates: `RulesVersion = "anthropic-2026-09-01"`,
`PriceTableVersion = "2026-09-07"`, and `PriceTableCheckedAt = "2026-09-07"`,
the last defined as when the table "was last verified against an independent
database", explicitly "a different fact from `PriceTableVersion`".

**Everything in G1 is therefore OBSERVED as repository content, reflecting a
provider page read on 2026-09-07 by someone other than this run, and not
re-verified on 2026-09-29.** That is 22 days of drift against a table whose own
staleness window is 60 days.

| Row | Rule as encoded | Class | Symbol |
|---|---|---|---|
| G1.1 | Two TTLs: 5 minutes and 1 hour | **OBSERVED** | `TTLShort`, `TTLLong` |
| G1.2 | Cache write costs 1.25x base input at the 5-minute TTL | **OBSERVED** | `WriteMultiplierShort = 1.25` |
| G1.3 | Cache write costs 2.0x base input at the 1-hour TTL | **OBSERVED** | `WriteMultiplierLong = 2.0` |
| G1.4 | Cache read costs 0.10x base input | **OBSERVED** | `ReadMultiplier = 0.10` |
| G1.5 | The Fable and Mythos 5.1 tier reads at 0.025x | **OBSERVED** | `readMultiplierNewest = 0.025` |
| G1.6 | Minimum cacheable prefix, newest tier | 512 tokens | `minPrefixNewest` |
| G1.7 | Minimum cacheable prefix, standard | 1024 tokens | `minPrefixStandard` |
| G1.8 | Minimum cacheable prefix, Opus 4.7 and Haiku 3.5 | 2048 tokens | `minPrefixOpus47` |
| G1.9 | Minimum cacheable prefix, legacy tier | 4096 tokens | `minPrefixLegacy` |
| G1.10 | **A prefix shorter than the minimum silently does not cache** | **OBSERVED** | comment on the constant block |
| G1.11 | Anthropic publishes **no single prefix floor**; the floor moves by model tier | **OBSERVED** | comment on `ClassifyBreak` |
| G1.12 | The TTL a write used is **inferred** from `ephemeral_1h_input_tokens` versus `ephemeral_5m_input_tokens`, defaulting to short without a breakdown | **OBSERVED** | `TTLOf` |

**G1.10 and G1.11 together are the operationally sharp pair.** A short prefix
does not error, it silently does not cache, and the threshold is not one number
you can hardcode. A tool that assumes a single floor is wrong on some tier and
reports a break where none occurred.

**A note on G1.5 and how the repository handles an unknown model.** An unknown
model's read multiple is set to `readMultiplierUnknown = ReadMultiplier`, that
is 0.10, and the comment states the rule as "the most cache-hostile multiple the
table holds", because a lower multiple makes Replay's own cache-preserving
recommendations score better: "Over ten turns a cache-clearing candidate scores
+818% worse at 0.10 and +2786% worse at 0.025: the cheap number makes our own
advice look three times more valuable." **OBSERVED**, and it is an instrument
declining to flatter itself.

## G2. What is not established about caching

These are the UNKNOWNs, and they are the substance of area G.

| Row | Question | Class | Observability |
|---|---|---|---|
| G2.1 | **What forms the cache key** | **UNKNOWN** | **PROV only** |
| G2.2 | Whether caches are shared or isolated across organizations | **UNKNOWN** | PROV |
| G2.3 | Maximum number of cache breakpoints per request | **UNKNOWN** | **P.** No constant for it exists anywhere in `internal/` |
| G2.4 | Which block types accept `cache_control` | **UNKNOWN** | P |
| G2.5 | Whether a hit refreshes the TTL, and by how much | **UNKNOWN** | P could test it; PROV to confirm the rule |
| G2.6 | The full invalidation list: tool definitions, system prompt, images, `tool_choice`, thinking parameter, model | **PARTLY OBSERVED as a cause vocabulary**, see G3; **UNKNOWN as a provider rule** | P |
| G2.7 | Whether a similar-but-not-identical prompt gets **partial** reuse | **UNKNOWN as a rule**; **measurable in magnitude**, see G4 | P |
| G2.8 | Whether cache reads count toward rate limits or input tokens per minute | **UNKNOWN** | PROV |
| G2.9 | Whether caching affects the context limit | **UNKNOWN** | PROV |
| G2.10 | Whether project content is cached | **UNKNOWN** | P. Depends on D2.5 and D2.6, both UNKNOWN |
| G2.11 | Whether Claude.ai and the API behave identically | **UNKNOWN** | PROV. Claude.ai exposes no wire to proxy |
| G2.12 | Cache storage cost, eviction policy, capacity | **UNKNOWN** | PROV |
| G2.13 | Whether Claude Code lets a user set breakpoints, or places them itself | **UNKNOWN** | P |

**G2.1 is the biggest UNKNOWN in this document.** Everything a caching-aware
tool wants to do depends on knowing what must stay byte-identical for a hit, and
that is exactly the boundary between what a provider publishes and what it keeps
private. The three candidate answers -- a prefix hash, an account-scoped prefix
hash, a prefix hash plus model plus parameters -- have different consequences for
whether two lanes of the same session can share an entry, and **nothing observed
here distinguishes them.** It is not inferred.

## G3. What Replay's break-cause vocabulary can and cannot decide

**OBSERVED**, `internal/cachemodel/anthropic.go`. This is a classification an
external tool actually performs, which makes it the practical answer to G2.6.

| Cause | Decidable from | Observability |
|---|---|---|
| `cache expired (gap longer than the TTL)` | usage and timing alone | **T+P** |
| `model changed between requests` | usage and timing alone | **T+P** |
| `system prompt or tool definitions changed` | the message history | **P** |
| `tool definitions changed` | the message history | **P** |
| `system prompt changed` | the message history | **P** |
| `effort or thinking setting changed` | the message history | **P** |
| `an earlier message was edited or removed` | the message history | **P** |
| `client re-rendered history after the system prefix (no edit visible in transcript)` | the message history | **P** |
| `prefix diverged inside the message history at an unknown block` | -- | **P**, and it is the residual |
| `NOT MEASURED (requests overlapped in this lane; the predecessor is not determined)` | -- | **T+P** |

**The `NOT MEASURED` row is the one to carry forward.** Every per-event cache
claim is a claim about a **pair** of requests, and, in the repository's own
words, "Nothing on the Anthropic or OpenAI wire names that predecessor."
`internal/transcript/types.go` encodes this as three states,
`CorrelationUnmeasured`, `CorrelationLaneSerial` and `CorrelationLaneOverlap`,
with the note that "Coding agents fan out by construction -- sub-agents, parallel
tool calls -- so the concurrent case is not a corner."

**A cache-break cause named against an overlapping predecessor reports the race,
not the session.** That is a hard ceiling on what any external observer can say,
and it is not a provider secret; it is missing from the wire.

## G4. Cache telemetry that is actually exposed

| Row | Field | Where | Class | Observability |
|---|---|---|---|---|
| G4.1 | `input_tokens` | usage | **OBSERVED** | T+P |
| G4.2 | `cache_creation_input_tokens` | usage | **OBSERVED** | T+P |
| G4.3 | `cache_read_input_tokens` | usage | **OBSERVED** | T+P |
| G4.4 | `output_tokens` | usage | **OBSERVED** | T+P |
| G4.5 | `output_tokens_details.thinking_tokens` | usage | **OBSERVED** | T+P |
| G4.6 | `cache_creation.ephemeral_5m_input_tokens` | usage | **OBSERVED** | **P**, and T where the client writes it |
| G4.7 | `cache_creation.ephemeral_1h_input_tokens` | usage | **OBSERVED** | **P**, and T where the client writes it |
| G4.8 | `iterations[]`, a per-iteration repeat of the usage block | usage | **OBSERVED** | **P** |
| G4.9 | `context_management.applied_edits` | message_delta | **OBSERVED** | **P** |

**Evidence.** `internal/transcript/types.go:100-113` declares the struct with
"Every field comes from the provider verbatim; nothing here is estimated", and
four SSE fixtures under `internal/ledger/testdata/anthropic/` carry the wire
shape, including one `message_delta` with
`"cache_creation":{"ephemeral_5m_input_tokens":0,"ephemeral_1h_input_tokens":174935}`.

**So the answer to "does Claude Code expose cache telemetry" is yes, at the
request level: OBSERVED.** Per-assistant-message cache creation and cache read
token counts are written into the transcript. What is **not** exposed is the
cache key, the entry's identity, its remaining lifetime, or whether an entry
existed and was missed versus never existed.

**G4.3 also gives a partial answer to G2.7.** `cache_read_input_tokens` is a
count, not a boolean, so an observer can see that 170,642 of a prefix read while
11,383 were freshly written, which is partial reuse **measured in magnitude**.
It does not reveal the matching rule that produced the split. Replay's
`ReadReproduced` / `ReadExceeded` / `ReadBroken` classification
(`internal/cachemodel/anthropic.go`) works on exactly this quantity.

**A third state worth naming.** `internal/cachemodel/measure.go:34`: "Marked is
true when the request carried a cache breakpoint at all [...] A request with no
breakpoint is not testimony." A zero cache read on an unmarked request says
nothing about whether caching works. The same discipline appears in
`docs/SURFACE-REGISTRY.md` for Codex, whose cache-write field is "present and
uniformly zero across the corpus, which is a third state and not a licence to
infer a write happened."

## G5. Does caching change cost, and by how much

| Row | Statement | Class |
|---|---|---|
| G5.1 | A cache-blind budget that prices every usage token at one flat rate runs **8.1x high aggregate** and **16.3x on one model** against this repository's corpus | **DERIVED** |
| G5.2 | Cache-blindness accounts for 101% of that error; the flat rate accounts for -1.1% | **DERIVED** |
| G5.3 | The consequence is throttling: a $500/day ceiling computed that way halts at roughly $62 of real spend | **DERIVED** |
| G5.4 | This is **not** a claim anyone was overcharged | **OBSERVED**, stated by the source |

`internal/cachemodel/ceiling.go` carries all four, and makes the limit explicit:
"Both figures are arithmetic over the same observed token counts, which is why
the ratio holds regardless of who was billed or whether anyone was -- it would
hold on a free tier." Backing file: `docs/evidence/qm-budget-2026-09-08.md`.

**G5 answers "does caching affect monetary cost" with a measured yes, and
answers "does it affect usage limits" with UNKNOWN (G2.8).** Those are different
questions and the repository only reaches the first.

---

# Cross-cutting: the observability answer

For each surface, what kind of observer can see it. This is the section the
recon campaign exists to produce.

## Observable from a transcript alone

| Surface | Fidelity |
|---|---|
| Compaction events, with pre and post token counts, drop totals and duration | **Full.** `compactMetadata`, boundary-and-summary pair |
| Compaction trigger value | **Full**, though only `"auto"` has ever been seen |
| Per-request cache creation and cache read token counts | **Full** |
| Thinking-token share of output | **Full** |
| Total prompt size processed per request | **Full** |
| Tool calls, tool results, error flags, and their sizes | **Full** |
| Image and attachment blocks | **Present**, sized |
| Subagent lanes | **Full**, via `isSidechain` and lane grouping |
| Model per request | **Full** |
| Cache break causes: **TTL expiry and model change only** | **Partial by construction** |
| Request identity | **Degraded on some clients.** `requestId` is written by the `cli` entrypoint; `sdk-cli`, `sdk-ts` and `claude-desktop` do not write it, and the message id substitutes |

## Observable only from a proxy

| Surface | Why the transcript cannot |
|---|---|
| System prompt bytes, and whether CLAUDE.md is in them | The transcript does not store the system block |
| Tool definition block, and its growth when a connector handshakes | Same |
| Cache breakpoint placement and count | A request-body property |
| `cache_creation` TTL split, where the client does not write it | Response detail the client may drop |
| `context_management.applied_edits` and cleared-token counts | Response detail the client may drop |
| `iterations[]` per-iteration usage | Same |
| Whether skills load frontmatter or body | A request-body property |
| Whether project files are preloaded or read on demand | A request-body property |
| Break causes needing message history: system versus tools, effort change, history edit, re-render | Needs the bytes actually sent |

## Observable only from provider-side data

Cache key composition. Cross-organization cache isolation. TTL refresh-on-hit
semantics. Cache eviction and capacity. Whether cache reads count against rate
limits. Whether caching affects the context limit. Documented context window
sizes. Whether Claude.ai and the API behave identically. Actual money billed, as
distinct from list price.

## Not observable at all

Output quality degradation across repeated compactions. Whether a cache entry
existed and was missed, versus never existed: both report
`cache_read_input_tokens: 0`. The identity of the predecessor request whose entry
a read hit, whenever two requests of a lane overlap.

## Where Replay stands today

From `docs/SURFACE-REGISTRY.md`, derived from code at `2974d192`:

| Surface | Cache column | Level |
|---|---|---|
| Claude Code CLI, via `SourceTranscript` | reads cache fields | **L3** |
| Replay proxy, via `SourceLedger` | reads cache fields | **L3** |

**L4 is not claimed by either**, and the registry records why: "of 256
suggestions, 20 are verifiable and all 20 are `pending`. Zero applied, zero
verified." A correction on 2026-09-28 demoted both rows from L4 after the
original entry claimed a level from the existence of a code path rather than
from evidence the path had run.

---

# What a re-run must fetch

Every UNKNOWN above that a provider page would close, with the question to put
to it. **None of these URLs was opened on 2026-09-29.**

| URL | Closes |
|---|---|
| `https://docs.claude.com/en/docs/build-with-claude/prompt-caching` | G2.1 through G2.9, G2.12, and re-verifies G1.1 through G1.11 |
| `https://docs.claude.com/en/docs/about-claude/pricing` | G1.2 through G1.5, and the price table's 22-day drift |
| `https://docs.claude.com/en/docs/build-with-claude/context-windows` | D1.1, D1.2, G2.9 |
| `https://docs.claude.com/en/docs/about-claude/models/overview` | D1.1, D1.2 |
| `https://docs.claude.com/en/docs/claude-code/slash-commands` | E1.7, E5.3, E5.4 |
| `https://docs.claude.com/en/docs/claude-code/memory` | D2.5, E4.3 |
| `https://docs.claude.com/en/docs/claude-code/sub-agents` | D2.10, D2.11 |
| `https://docs.claude.com/en/docs/claude-code/costs` | E1.2, E4.6 |
| `https://docs.claude.com/en/docs/agents-and-tools/tool-use/context-editing` | D3.5, D3.6 |
| `https://support.claude.com` search: "automatic context management" | **E1.1 through E1.6.** The brief says this article exists and states the summarise-and-consume-more-usage mechanics |

**Four UNKNOWNs need no provider page and no paid call**, and are the cheapest
work available: E5.3 and E5.4 need one `/compact` and one `/clear` followed by a
`grep` of the transcript; D2.5 and E4.5 need one proxied session and a diff of
the system block against CLAUDE.md.

---

# Sources

Fetched provider pages: **none.** The classifier outage described at the top of
this document prevented every attempt. The URLs below are the intended targets
and are listed so a re-run has them, **not as citations for any statement
here.**

- [Anthropic prompt caching](https://docs.claude.com/en/docs/build-with-claude/prompt-caching) -- not fetched 2026-09-29
- [Anthropic pricing](https://docs.claude.com/en/docs/about-claude/pricing) -- not fetched 2026-09-29
- [Context windows](https://docs.claude.com/en/docs/build-with-claude/context-windows) -- not fetched 2026-09-29
- [Models overview](https://docs.claude.com/en/docs/about-claude/models/overview) -- not fetched 2026-09-29
- [Claude Code slash commands](https://docs.claude.com/en/docs/claude-code/slash-commands) -- not fetched 2026-09-29
- [Claude Code memory](https://docs.claude.com/en/docs/claude-code/memory) -- not fetched 2026-09-29
- [Claude Code subagents](https://docs.claude.com/en/docs/claude-code/sub-agents) -- not fetched 2026-09-29
- [Claude Code costs](https://docs.claude.com/en/docs/claude-code/costs) -- not fetched 2026-09-29
- [Anthropic support](https://support.claude.com) -- not fetched 2026-09-29

Repository evidence actually read on 2026-09-29, all paths relative to the
repository root:

- `internal/cachemodel/anthropic.go` -- TTLs, multipliers, minimum prefixes, model price rows, break-cause vocabulary, `TTLOf`, `ClassifyBreak`, rules and price table dates
- `internal/cachemodel/ceiling.go` -- cache-blind budget arithmetic, the 8.1x and 16.3x figures, `BlindCostUSD`
- `internal/cachemodel/measure.go` -- `Marked`, and why an unmarked request is not testimony
- `internal/transcript/types.go` -- `Usage`, `Block`, `Message`, `Request`, `PromptTotal`, the three correlation states
- `internal/transcript/claudecode.go` -- `isCompactSummary`, `compactMetadata`, the boundary-and-summary pairing, `isSidechain`, lane grouping, request-id provenance
- `internal/transcript/compaction_test.go` -- CM-1 and CM-2, the clamped growth record
- `internal/analysis/context.go` -- `ContextEntry`, and the statement that the context window is not known
- `internal/analysis/rereads.go` -- `ContextEdits`, `ClearedTokens`
- `internal/ledger/record.go`, `response.go`, `store.go`, `summarize.go` -- `context_management`, `applied_edits`, `AppliedEdits`, `ClearedInputTokens`, `CacheOutcome`, `CacheState`
- `internal/ledger/testdata/anthropic/*.sse` -- four live-capture fixtures carrying the usage and `context_management` wire shapes
- `docs/evidence/compaction-and-index-2026-09-06.md` -- the 39-event compaction corpus and its stated limits
- `docs/evidence/qm-budget-2026-09-08.md` -- cited by `ceiling.go` as the backing file for G5
- `docs/SURFACE-REGISTRY.md` -- conformance matrix, L3 levels, the 2026-09-28 correction
- `docs/SURFACES.md` -- cache model scope, wire families, proxy endpoints
- `docs/TOKEN-PRICES.md` -- no first-party machine-readable price source exists

---

[Documentation index](../../docs/README.md) - [Repository README](../../README.md)
