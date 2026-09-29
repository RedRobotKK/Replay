# Model, reasoning, tool and agent surfaces

**Compiled 2026-09-29. Reconnaissance, not a recommendation.**

This file maps four surfaces -- models, reasoning controls, tools, and agents --
and says for each item how the claim was established. It is written so that a
reader can tell a fetched fact from an inference, and so that anything nobody
checked is visible as a gap rather than absent.

## Evidence classes

Every row below carries exactly one of these. Nothing carries two.

| Class | Meaning |
|---|---|
| **DOCUMENTED** | Stated on a vendor page, with the URL and the fetch date beside it |
| **OBSERVED** | Visible in this repository: source, a test, a fixture or a dated evidence file |
| **DERIVED** | Arithmetic on an OBSERVED or DOCUMENTED quantity, with the derivation shown |
| **HYPOTHESIS** | A proposition with a named falsifier. Not a finding |
| **UNKNOWN** | Nobody established it. Includes items this pass tried to fetch and could not |

## Method, and a failure that shapes this document

**No provider API was called. No credential was read, printed or transmitted.**
The only tools used were local file reads and attempted documentation fetches.

**The documentation fetches did not succeed.** `WebFetch`, `WebSearch` and
outbound `curl` were all refused throughout this session by the harness with:

```text
The server-side auto mode classifier is temporarily unavailable, so auto mode
cannot determine the safety of <tool> right now.
```

Twelve attempts across roughly the whole session, against
`docs.claude.com` and against a plain `curl` of the same paths. Local reads and
writes worked the entire time, so this is a harness gate on network egress and
not a network fault.

**The consequence is stated once here and enforced in every table below: this
pass produced zero DOCUMENTED rows.** Every cell that wanted a vendor URL is
**UNKNOWN (not fetched 2026-09-29)** and names the URL that would settle it. The
Sources section lists those URLs as unfetched rather than as citations, because
a URL nobody opened is a reading list and not evidence.

This matters more than it looks. The brief for this pass named specific vendor
wording to quote -- the factor list affecting usage limits, the model-role
guidance, the extended-thinking parameters -- and the honest status of all of it
is that it was not read today. Writing it from recollection would have produced
a document that looks complete and cites nothing, which is the failure mode this
repository's evidence convention exists to prevent.

**What survives is the OBSERVED half, and it is substantial.** This repository
parses the Claude Code transcript format, sits in front of the provider as a
proxy, and has dated measurements of fan-out, lane isolation, quota movement and
compaction. Those carry the weight here.

---

## The three forbidden assumptions

The brief names three propositions that must be treated as hypotheses. They are
recorded here at the top because each one reappears inside the sections below,
and a reader who meets them there should already know their status.

### Assumption 1: higher effort produces a better result

**HYPOTHESIS. Not tested here, and not tested anywhere in this repository.**

`effort` is OBSERVED as a wire field (see section I), so the input side is
measurable. The output side -- "better result" -- has no definition in this
repository and no instrument. Nothing has compared two effort settings on the
same task with a scored outcome.

**Falsifier:** a paired run of the same task at two effort levels, scored by a
rubric fixed before the run, with enough trials to separate the arms. None
exists. Until one does, any statement in either direction is opinion.

### Assumption 2: lower effort produces better economics

**HYPOTHESIS. Structurally suspect, for a reason this repository already
measured on an adjacent quantity.**

Lower effort reduces reasoning tokens per request. It does not obviously reduce
tokens per completed task, because a request that answers less can be followed
by more requests. The repository has the ingredients to test this -- thinking
tokens are a measured per-request field, and lanes are separable -- and has never
run it.

**Falsifier:** measure total tokens to task completion, not tokens per request,
across effort levels on a fixed task set. A result showing low effort costs more
in total kills the assumption outright.

### Assumption 3: a cheaper model is cheaper at the task level

**HYPOTHESIS, and the one this repository comes closest to refuting.**

A cheaper per-token model that needs more turns, more tool calls or more rework
can cost more per completed task. Per-token price is a rate; the bill is a rate
times a quantity, and the quantity is exactly what changes when the model
changes.

**What the repository establishes.** `docs/evidence/routing-baseline-2026-09-06.md`
attempts precisely this comparison and **refuses to answer it**. Its projections
for rerouting one corpus away from `claude-opus-5`:

| Destination | Projected | Quoted band |
|---|---:|---|
| claude-haiku-4-5 | $283.18 | [$18.35, $548.01] |
| claude-sonnet-5 | $1,019.74 | [$213.13, $1,826.35] |
| claude-fable-5-1 | $1,696.63 | [$516.62, $2,876.64] |
| claude-opus-4-8 | $2,632.47 | [$446.20, $4,818.74] |

Observed for the same work on `claude-opus-5`: **$2,623.34**.

The bands are wider than the differences in three of four cases, and the file
says plainly that **no measured sigma in the corpus is distinguishable from 1.0
at its own quoted band**. Its own limit 1 reads: "Nothing about any sigma
differing from 1.0."

**And that projection holds turn count fixed.** It reprices the same turns at
another model's rate card. It does not model a cheaper model needing more turns,
because nothing in the corpus can: limit 7 records that 1,308 of 1,606
transcripts are one model on one person's work, a monoculture with no second
model's turns on the wire to compare against.

**So the strongest honest statement is:** the cheapest available instrument, run
on 1,606 transcripts, could not establish that any model is cheaper at the task
level, and its authors said so. **A per-token price comparison is not a
task-level cost comparison, and this repository has the negative result to prove
the gap is real rather than pedantic.**

**Falsifier that would settle it:** the same task set completed end to end on two
models, counting every turn, every tool call and every rework cycle, priced at
completion rather than per request. It has not been run.

**Wherever model choice appears below, this paragraph applies and is not
repeated.**

---

## H. Model surfaces

### H.1 Model families and names

**No vendor model list was fetched (see Method). The names below are OBSERVED in
this repository and in this session, which establishes that the strings exist
and reach the wire. It does not establish that they are the current documented
set, that the list is complete, or that any of them is still offered.**

Two independent OBSERVED sources.

**Source A: this session's own environment.** The running harness reports the
model as `claude-opus-5`, named "Opus 5". OBSERVED 2026-09-29.

**Source B: the compiled price and cache table**, `internal/cachemodel/anthropic.go`,
`RulesVersion = "anthropic-2026-09-01"`, `PriceTableVersion = "2026-09-07"`,
`PriceTableCheckedAt = "2026-09-07"`. Every row below is OBSERVED in that file.

| Model string matched | Min cacheable prefix | Input $/MTok | Output $/MTok | Cache-read multiple | Priced |
|---|---:|---:|---:|---:|---|
| `fable-5-1` | 512 | 10 | 50 | 0.025 | yes |
| `mythos-5-1` | 512 | 10 | 50 | 0.025 | yes |
| `fable-5` | 512 | 10 | 50 | 0.10 | yes |
| `mythos-5` | 512 | 10 | 50 | 0.10 | yes |
| `opus-5` | 512 | 5 | 25 | 0.10 | yes |
| `opus-4-8` | 1024 | 5 | 25 | 0.10 | yes |
| `opus-4-7` | 2048 | 5 | 25 | 0.10 | yes |
| `opus-4-6` | 4096 | 5 | 25 | 0.10 | yes |
| `opus-4-5` | 4096 | 5 | 25 | 0.10 | yes |
| `haiku-4-5` | 4096 | 1 | 5 | 0.10 | yes |
| `sonnet-5` | 1024 | 2 | 10 | 0.10 | yes |
| `sonnet-4-6` | 1024 | 3 | 15 | 0.10 | yes |
| `sonnet-4-5` | 1024 | 3 | 15 | 0.10 | yes |
| `sonnet-4` | 1024 | 3 | 15 | 0.10 | yes |
| `sonnet` (bare) | 1024 | -- | -- | 0.10 | **no** |
| `3-5-haiku` | 2048 | 0.80 | 4 | 0.10 | yes |
| `opus-4-1` | 1024 | 15 | 75 | 0.10 | yes |
| `opus-4` | 1024 | 15 | 75 | 0.10 | yes |
| `haiku` (bare) | 1024 | -- | -- | 0.10 | **no** |

Family prefixes the table will answer for: `claude`, `opus`, `sonnet`, `haiku`,
`fable`, `mythos`. OBSERVED at `anthropicFamilies`.

**Three things this table is evidence of, beyond the names.**

**Aliases exist and reach the wire.** The bare `sonnet` and `haiku` rows exist
because those strings turn up as model identifiers in real records. They are
carried `priced: false`, which is the table's way of saying it knows the string
and refuses to price it. OBSERVED. **What the full alias set is, and what each
alias resolves to on a given date, is UNKNOWN** -- resolution happens on the
provider side and this repository sees only the string the client sent.

**The cache floor is per model and is not published as one number.** 512, 1024,
2048 and 4096 all appear. `ClassifyBreak`'s comment states the reason directly:
"Anthropic publishes no single prefix floor: the floor moves by model tier and
lives in the dated rules document rather than in CacheRules." OBSERVED. A prefix
under the floor "silently does not cache" -- OBSERVED as a code comment, and
independently bracketed by measurement (H.4).

**Pricing is not uniform within a family.** `opus-4` and `opus-4-1` are $15/$75
while `opus-4-5` through `opus-5` are $5/$25. A family name is not a price.

### H.2 Model routing and switching

**Routing is OBSERVABLE after the fact and is not a vendor API this repository
can read.**

| Item | Class | Evidence |
|---|---|---|
| The model of each request is on the wire and in the transcript | **OBSERVED** | `transcript.Request.Model`, populated from `message.model` |
| A mid-session model change is detectable | **OBSERVED** | `CauseModelChanged BreakCause = "model changed between requests"`, a named cache-break cause in `internal/cachemodel/anthropic.go` |
| How often it actually happens | **OBSERVED, and it is rare** | `routing-baseline-2026-09-06.md` finding 4: **6 of 744 cache breaks** over 30,366 compared turns, 0.81% of breaks and 0.02% of turns. Counting error roughly +/- 2.4 on a count of 6 |
| Traffic mix across models in one corpus | **OBSERVED** | 2,937 of 20,497 fitted turns (14.33%) ran on something other than the baseline `claude-opus-5` |
| Whether the provider reroutes server-side | **UNKNOWN** | Nothing observable distinguishes a client switching models from a provider doing so. Do not infer provider internals |
| `/model`, `ANTHROPIC_MODEL`, per-subagent model fields | **UNKNOWN (not fetched 2026-09-29)** | Would come from `https://docs.claude.com/en/docs/claude-code/model-config` and `.../settings` |

**The baseline selection is itself a measured thing.** `replay route` computes
its baseline rather than taking one, and the selection was checked for stability
across three weightings -- fitted turns (85.67%), deduplicated requests (87.87%)
and list-price dollars (84.93%) -- so it is not a tie-break artifact. OBSERVED.

### H.3 Provider guidance on which model for which work

**PROVIDER GUIDANCE, relayed, not verified this session, and explicitly not a
validated optimization policy.**

The brief for this pass states that Anthropic's Claude Code documentation
currently describes **Sonnet as the general-purpose default for much coding
work, Opus for harder reasoning, and Haiku for fast mechanical work.**

It is recorded here **as provider guidance relayed by the requester**, with
three qualifications that are not optional:

1. **It was not fetched this session.** The page that carries it is unread as of
   2026-09-29. Class: **UNKNOWN (not fetched)** as a documented quote; the
   substance is carried as relayed guidance.
2. **Guidance is not a measurement.** A vendor describing a model as suited to a
   class of work is a design intent. It is not a claim that routing that work to
   that model minimises tokens, dollars, latency or rework, and it should never
   be cited as though it were.
3. **Assumption 3 applies in full.** "Haiku for fast mechanical work" is a
   statement about fit, not about task-level cost. This repository's own routing
   evidence could not establish task-level cost ordering between any two models.

**Treating this guidance as an optimization policy is exactly the error the
three forbidden assumptions name.** It is a reasonable default for a person with
no measurement. It is not a finding.

### H.4 Context windows, output limits, latency

| Item | Class | Evidence |
|---|---|---|
| Per-model context window | **UNKNOWN (not fetched 2026-09-29)** | `https://docs.claude.com/en/docs/about-claude/models/overview`. **This repository does not encode context windows at all** -- `internal/cachemodel/anthropic.go` carries prices and cache floors and no window |
| Per-model max output tokens | **UNKNOWN (not fetched 2026-09-29)** | Same URL. Not encoded here either |
| Context window advertised on the wire | **OBSERVED, and Anthropic does not do it** | `wire-families-2026-09-06.md`: `x-grok-context-window = 500000` is noted specifically because "neither Anthropic nor OpenAI advertises the context window on the wire" |
| Effective working ceiling before compaction | **OBSERVED, 1,520 transcripts** | `compaction-and-index-2026-09-06.md`: 39 auto-compaction events, `preTokens` within 1% of 1,000,000 on **30 of 39**. Median pre 999,029, median post 23,218, **median retention 2.55%** (median of per-event ratios; the ratio of medians is 2.32% and is a different statistic) |
| Compaction is exhaustion-triggered, not policy-triggered | **DERIVED** | From the clustering of `preTokens` at the ceiling above. Every observed event was `trigger: "auto"` |
| Compaction latency | **OBSERVED** | 103 to 180 seconds per event; 73.5 minutes of wall clock across 39 events |
| Minimum cacheable prefix, measured | **OBSERVED, bracketed** | `routing-baseline-2026-09-06.md` finding 6: opus-5 above 508 at most 512; sonnet-5 above 1,020 at most 1,024; fable-5-1 above 508 at most 512; haiku-4-5 above 4,094 at most 4,097. **n = 1 reading per model, all four inside 35 seconds.** Each bracket is a resolution, not an error bar |
| Per-model latency | **UNKNOWN** | `docs/SURFACE-REGISTRY.md` marks Claude Code CLI performance `NOT_MEASURED`. The Replay proxy row has `LatencyMS` observed, which times the proxy's own hop and is not a per-model comparison |

**The caching floor is the one item here that no corpus can supply.** Finding 6
is explicit: 1,606 transcripts bound it only from above and very loosely,
because ordinary traffic never sends a small prompt with a cache breakpoint on
it. The evidence has to be manufactured. That is worth carrying because it
generalises -- **some surface facts are not latent in observational data at any
volume.**

### H.5 Per-model quota behaviour

**This is the weakest area in the whole document, and the repository has
retracted itself on it twice.**

| Item | Class | Evidence |
|---|---|---|
| Rate-limit headers exist on Anthropic responses | **OBSERVED, and contested within this repository** | `predictor.go` records 142 responses carrying `anthropic-ratelimit-unified-*` in a 2026-09-06 trial. A later census found **zero records carrying any `anthropic-ratelimit-*` header, ever -- the key absent, not empty** -- and `retry-after` never once observed. Both statements are in the repository. They are not reconciled |
| The counter is account-wide, not per request | **OBSERVED** | `quota-titration-2026-09-06.md`: an earlier calibration attributing a step to four probe requests was withdrawn once it was noticed the same account ran an interactive session throughout. "Passive attribution of an account-wide counter to individual requests does not work" |
| Cache breaks consume subscription allowance | **UNKNOWN. Explicitly NOT MEASURED** | `subscription-allowance-2026-09-09.md` is **retracted in full**, same day it was written. Its banner: the multiplier "exists only as prose", the multiplicand was already retracted elsewhere, "the arithmetic multiplied a retracted number by an unrecorded one" |
| A quota titration succeeded | **No. It refused, correctly** | `quota-titration-2026-09-06.md`: **3.09M tokens moved the 5h counter by zero steps** at 0.01 resolution. 1,012,229 write tokens and 2,078,382 read tokens, utilization 0.25 to 0.25 |
| Whether the limit is weighted by model | **UNKNOWN, and named as untested** | The same file lists "Opus rather than haiku" as a cheaper route to a reading precisely because "if the limit is weighted by model -- untested, and worth testing on its own -- haiku is the slowest possible way to move it" |
| What the vendor says affects usage limits | **UNKNOWN (not fetched 2026-09-29)** | See section J.5 |

**The honest summary of per-model quota behaviour is: nobody here knows, the one
experiment run returned a null, and the conditions under which it would return
anything are recorded.** A ratio near 12.5 and a ratio near 1.0 "remain equally
consistent with what has been measured."

**Why the experiment was not simply made bigger.** Moving the counter 20 steps
per arm would take well over 30M tokens per arm at haiku volumes, "which is a
large share of a 5-hour window and would risk locking the operator out -- to
measure lockout." That is a real constraint on this class of reconnaissance and
not a failure of diligence.

---

## I. Reasoning surfaces

Each row is scored on five columns, strictly.

- **DOCUMENTED** -- a vendor page states it. **Every cell in this column is `not fetched` this session.**
- **OBSERVABLE** -- an external tool could actually see it. A field that exists in a struct but is never populated is **not** observable.
- **CONTROLLABLE** -- a user can set it deliberately, and the setting reaches the provider.
- **MEASURABLE** -- it yields a number an external tool can read per request.
- **UNKNOWN** -- what remains open.

### I.1 The table

| Item | DOCUMENTED | OBSERVABLE | CONTROLLABLE | MEASURABLE | UNKNOWN |
|---|---|---|---|---|---|
| **Effort level** | not fetched | **YES.** `output_config.effort` on the request body (`internal/ledger/summarize.go:21`); `effort` on the transcript line (`internal/transcript/claudecode.go:37`); carried to `transcript.Request.Effort` and `ledger.Record.Effort`. A test asserts the literal value `"high"` round-trips (`ledger_test.go:31`) | **YES, by the user.** It is a request parameter the client sets. **NO for the values** -- the legal set is not established here | **YES.** A string per request, per lane | The legal value set; what each value does to the provider; whether it is per request or sticky |
| **Extended thinking blocks** | not fetched | **YES.** `KindThinking = "thinking"` is a first-class block kind (`internal/transcript/types.go:20`), decoded with its byte size at `wire.go:178`, labelled `assistant thinking` | **PARTIAL.** Whether thinking runs is influenced by effort and by client settings. No direct on/off switch is observable in this repository | **YES, twice over.** Byte size per block, and token count per request (next row) | Whether blocks returned are full or summarized reasoning |
| **Reasoning / thinking tokens** | not fetched | **YES.** The provider sends `usage.output_tokens_details.thinking_tokens` (`internal/transcript/wire.go:123-125,140`). A Claude Code transcript fixture asserts it is non-zero (`claudecode_test.go:37`); conformance tests pin exact values | **NO.** It is a report, not a control | **YES. This is the strongest measurable in section I.** An integer per request, joinable to a lane | Whether it is a subset of `output_tokens` or additive. The struct comment says "the share of Output spent on reasoning", which is OBSERVED as this repository's reading and **not** confirmed against a vendor page |
| **Thinking budgets** (`budget_tokens`) | not fetched | **NO.** No `budget_tokens` field appears anywhere in this repository's wire model. Absence of a parser is not absence of the parameter | **UNKNOWN** | **NO** | Whether the parameter exists on this wire at all, its minimum, and how it interacts with `max_tokens` |
| **Adaptive thinking** | not fetched | **NO.** Nothing observable distinguishes "the model chose to think more" from "effort was set higher". Both land as a larger `thinking_tokens` | **NO** | **Only as an outcome.** The token count moves; the cause is not recoverable | Whether it exists as a distinct mechanism. **Do not infer provider internals** |
| **Per-model reasoning behaviour** | not fetched | **PARTIAL.** `thinking_tokens` is per request and every request carries its model, so the join is trivially available | **NO** | **YES, as a join.** Thinking tokens by model is computable today from any corpus | Nobody in this repository has run it. No dated evidence file reports thinking tokens by model |
| **Reasoning during tool use** | not fetched | **YES, structurally.** Thinking blocks and `tool_use` blocks are separate kinds within one assistant message, both carrying byte sizes and both ordered | **NO** | **YES.** Thinking bytes before and after a tool call are separable by block order within a message | Whether reasoning persists across a tool round trip on the provider side |
| **Reasoning in subagents** | not fetched | **YES.** A subagent lane is a separate set of requests (section K), each with its own `usage`, so `thinking_tokens` is available per lane | **Inherits the effort control.** Whether a subagent can be given a different effort is **UNKNOWN (not fetched)** | **YES, per lane** | Whether subagent effort is independently settable |
| **Planning versus execution** | not fetched | **NO, as a mode.** No plan-mode marker appears in this repository's transcript model. `transcript.Request` has `Model`, `Effort`, `Epoch`, `Usage` and no mode field | **UNKNOWN** | **NO** | Whether a plan/execute distinction is recorded anywhere a reader could see |

### I.2 One inference that is available and that nobody has drawn

**A change to effort or thinking settings breaks the prompt cache, and it does so
under its own named cause.**

```go
CauseEffortChange BreakCause = "effort or thinking setting changed"
```

OBSERVED at `internal/cachemodel/anthropic.go`. It sits in the same bounded
vocabulary as TTL expiry and model change, which means **the cost of changing
effort mid-session is a measurable quantity with a dedicated label already in
the instrument.**

**DERIVED consequence:** toggling effort is not free. It invalidates the cached
prefix, and a re-laid prefix is billed at the write multiple of 1.25 rather than
the read multiple of 0.10, a factor of 12.5 on the affected tokens.

**What is not established:** how often this fires in practice.
`routing-baseline-2026-09-06.md` reports the break-cause distribution and
`CauseEffortChange` does not appear in the reported rows. Whether that means
zero occurrences or a row omitted from the report is **UNKNOWN**. The count is
one query away from anyone with the corpus and nobody has run it.

### I.3 What is genuinely controllable by a user

Reduced to the short answer, because it is the question that matters.

**Controllable, confirmed OBSERVED on the wire:**

- **Effort.** It is a request parameter, it is visible, it round-trips, and
  changing it has a named and priced consequence. **This is the only reasoning
  control this pass could confirm end to end.**

**Controllable but unconfirmed here:**

- Whether extended thinking has a direct on/off switch separate from effort.
- Whether a thinking budget can be set in tokens.
- Whether subagents take an independent effort setting.

All three are **UNKNOWN (not fetched 2026-09-29)** and would be settled by
`https://docs.claude.com/en/docs/build-with-claude/extended-thinking`.

**Not controllable:**

- Reasoning token count. It is reported, never requested.
- Whether the model thinks harder on a given turn.

**A caution on the whole section.** Effort being controllable and reasoning
tokens being measurable does **not** mean the loop is closed. Closing it needs
the outcome side, and there is no outcome instrument here. **Assumptions 1 and 2
remain hypotheses precisely because the input is measurable and the output is
not.** A surface that is half measurable invites exactly the unfalsifiable claim
this document is structured to refuse.

---

## J. Tool surfaces

### J.1 The dimensions, and which of them this repository can actually see

Before the per-tool table, a limit that governs it.

**Tool definitions are not in a Claude Code transcript.** OBSERVED, and stated
in the type itself:

```go
// Tools are the tool definitions the request carried, by name and
// size, known only from the ledger. Transcripts do not show them.
Tools []ToolDef
```

`transcript.Source.PrefixVisible()` returns true only for `SourceLedger`. The
system prompt and tool definitions "are present in each request's context, so
nothing ahead of the first message needs estimating. **Only the proxy sees
them.**"

**So the token cost of a tool's definition is measurable from a proxy and is not
measurable from a transcript.** Every "token cost" cell below carries that
split. A reader working from transcripts alone should treat the definition cost
as present in `input_tokens` and unattributable.

### J.2 Per-surface table

Columns: token cost, context impact, latency, quota impact, boundable output,
parallelisable, cacheable, summarisable or persistable externally.

| Surface | Token cost | Context impact | Latency | Quota | Bounded output | Parallel | Cacheable | External persist |
|---|---|---|---|---|---|---|---|---|
| **Built-in tools** (Read, Edit, Write, Grep, Glob) | **OBSERVED via proxy only.** `ToolDef{Name, Bytes}` gives each definition's decoded size. Definitions sit in the cached prefix | Results land as `tool_result` blocks with measured `Bytes` (`transcript.Block`) | UNKNOWN. Not measured here | UNKNOWN (see J.5) | **HYPOTHESIS.** Read takes offset/limit and Grep takes patterns, so output is bounded by parameter choice. No measurement here | **OBSERVED as possible.** `CorrelationLaneOverlap` exists precisely because requests overlap in a lane | **Definitions: YES**, they sit in the prefix. **Results: NO**, each result is new content appended after the breakpoint | Results are in the transcript and can be read back |
| **MCP servers** | **OBSERVED, and this is the largest single tool cost in the corpus.** See J.3 | Definitions enter the prefix mid-session, voiding it | Handshake completes after session start, which is the whole defect | UNKNOWN | UNKNOWN. No `MAX_MCP_OUTPUT_TOKENS`-style control is referenced in this repository | Same as built-ins | **Definitions are cacheable and that is the problem** -- arriving late, they void what was cached | Tool names are persisted verbatim in the ledger |
| **Filesystem** | Result bytes measured per block | Additive, never removed except by compaction | UNKNOWN | UNKNOWN | **HYPOTHESIS.** Bounding a read is a parameter choice, unmeasured here | Yes | No | Yes |
| **Shell** | Result bytes measured per block | Additive | UNKNOWN | UNKNOWN | **HYPOTHESIS.** Piping through `head` bounds it. Unmeasured | Yes | No | Yes |
| **Search / web** | UNKNOWN. **This session could not use them at all** (Method) | Additive | **OBSERVED as unavailable 2026-09-29**, harness-gated | UNKNOWN | UNKNOWN | UNKNOWN | No | Yes |
| **Browser** | UNKNOWN. Not exercised | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | No | UNKNOWN |
| **Connectors** | **OBSERVED.** Counted in J.3 | **OBSERVED as a prefix-voiding event** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Definitions yes, and see J.3 | Names persisted |
| **IDE tools** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN |
| **git** | Shell-shaped | Additive | UNKNOWN | UNKNOWN | HYPOTHESIS, as shell | Yes | No | Yes |
| **tests** | Shell-shaped, and output can be very large | Additive, and large results are the ones worth bounding | Long-running | UNKNOWN | **HYPOTHESIS**, and the highest-value case | Yes | No | Yes |
| **Subagents** | See section K | **Separate lane.** This is the one surface whose context impact on the parent is bounded by construction | UNKNOWN | UNKNOWN | **The returned report is the bound** | **OBSERVED.** 15 to 30 concurrent lanes exercised | **NO, and it costs.** See K.3 | Lanes are in the transcript |
| **Parallel tool calls** | Additive | Additive | Reduces wall clock | UNKNOWN | n/a | That is what they are | n/a | Yes |

### J.3 The measured cost of tool definitions

This is the one place in section J with a hard number, and it is worth stating
in full because it inverts the intuition that tool cost is per call.

**OBSERVED**, `docs/evidence/lane-isolation-2026-09-06.md`, corrected run of
2026-09-06, 60 requests across 5 sessions, 15 subagent lanes plus the main loop:

| | |
|---|---:|
| Total prompt tokens billed | 7,996,448 |
| Re-billed by cache break | 336,060 |
| Share | **4.2%** |

**Every one of the three breaks was an MCP connector's tool block arriving
mid-session:**

```text
[157,080] added 3 tool(s): mcp__claude_ai_Otter_ai__otter_fetch,
          otter_get_user_info, otter_search; removed 1 tool(s): WaitForMcpServers
[140,623] added 39 tool(s): mcp__claude_ai_Calendly__availability-...
[ 38,357] added 199 tool(s): ListMcpResourcesTool, ReadMcpResource...
```

**157,080 tokens re-billed because three Otter.ai tools finished connecting.**

Three properties of this that matter for any tool-cost model:

1. **The cost is not proportional to the tools added.** Three tools cost 157,080
   and 199 tools cost 38,357. The cost is the size of the prefix that gets
   voided, not the size of the addition. A late arrival early in a long session
   is cheap; a late arrival after a large prefix has accumulated is expensive.
2. **Zero of the breaks were in a subagent lane.** All 15 subagent lanes were
   internally stable across the run. The fix named is client-side sequencing:
   bind MCP tools before the first cached request.
3. **The number replaced a retracted 98.8%** which was an instrument artifact,
   and the retraction is what makes 4.2% worth quoting. The instrument had been
   comparing each parallel lane's prefix against a different lane's hash.

**Scope, stated by the source:** 60 requests, one operator, one machine, one
synthetic fan-out prompt on haiku. It shows the instrument no longer lies about
its own traffic. It does **not** establish a production distribution.

**DERIVED, and it is the operational point:** on this surface, the dominant tool
cost is a **connection-timing** cost, not a per-call cost. Advice aimed at
reducing tool calls addresses a different quantity from the one that produced
4.2% here.

### J.4 What bounds and caching actually mean here

| Property | Class | Note |
|---|---|---|
| Output can be bounded | **HYPOTHESIS across the board** | Every built-in tool takes parameters that bound output. **No measurement in this repository compares bounded against unbounded output on any dimension.** The proposition is plausible and untested here |
| Calls can be parallelised | **OBSERVED** | `CorrelationLaneOverlap` exists because overlapping requests are real, and its consequence is recorded: where two requests of a lane were in flight together, cause attribution is **NOT MEASURED** rather than guessed |
| Results can be cached | **OBSERVED, and mostly NO** | Tool *definitions* sit in the cacheable prefix. Tool *results* append after it. A result is new content and is written, not read |
| Results can be summarised | **OBSERVED as a client behaviour with a measured cost** | Compaction is the mechanism: median retention 2.55%, 30.5M tokens discarded across 39 events, 73.5 minutes of wall clock |
| Results can be persisted externally | **OBSERVED** | Everything is in the transcript on disk. This repository is an existence proof |

**One caution on parallelisation.** It buys wall clock and costs attribution.
The `CorrelationLaneOverlap` constant's comment is precise about the trade:
"Which of them wrote the entry this one read is not determined by anything
observable, so per-event attribution against a predecessor is NOT MEASURED
rather than guessed." **Parallelism degrades the measurability of the thing it
speeds up.**

### J.5 Tool usage and usage limits

**The brief asked for the current vendor wording naming tool usage as a factor
affecting usage limits, quoted verbatim. It was not obtained.**

**Class: UNKNOWN (not fetched 2026-09-29).**

Attempted and refused: `WebFetch`, `WebSearch` and `curl` against
`docs.claude.com` and a support-site search, twelve times across the session,
each refused by the harness classifier gate (see Method).

**The URLs that would settle it**, listed in the Sources section as unfetched:

- `https://docs.claude.com/en/docs/claude-code/costs`
- The Anthropic support article on Claude Code / Max plan usage limits, on
  `support.claude.com` or `support.anthropic.com`

**No paraphrase is offered.** A quote requested verbatim and supplied from
recollection would be indistinguishable in this document from one that was read,
which is the specific harm the evidence-class convention prevents.

**What this repository can say about the same question, independently:**
whether tool usage consumes a subscription allowance is **NOT MEASURED** here
either. The one experiment that tried adjacent ground returned a null at 3.09M
tokens, and the file that extrapolated from it was retracted in full. See H.5.

---

## K. Agent and subagent surfaces

### K.1 The lane model, as this repository reconstructs it

**OBSERVED**, `internal/transcript/claudecode.go` and `types.go`.

A Claude Code transcript is JSONL. Each line carries, among other fields:

| Field | Purpose |
|---|---|
| `uuid` | line identity |
| `parentUuid` | the chain that makes a lane |
| `sessionId` | the session |
| `requestId` | the provider's request id, where the client writes one |
| `apiBlockIndex` | orders lines within one request |
| **`isSidechain`** | **the parent/child discriminator** |
| `effort` | the effort setting |
| `message.model` | the model |
| `message.usage` | the provider's accounting |
| `isCompactSummary`, `compactMetadata` | compaction events |
| `isApiErrorMessage` | a client-written failure record, not a provider response |

Reconstruction, OBSERVED at `claudecode.go:200-240`:

1. Lines are grouped by request key -- `requestId` where present, else the
   message id.
2. Each group is sorted by `apiBlockIndex`.
3. A lane id is the root `uuid` of the parent chain (`buildRequest`, line 365).
4. `session.Lane(laneID, group[0].IsSidechain)` files the lane, tagged sidechain
   or not.

```go
// Lane is one linear sequence of requests sharing a conversation root.
// The main loop is one lane; each sub-agent conversation is another.
type Lane struct {
	ID        string
	Sidechain bool
	Requests  []*Request
}
```

**The proxy has a second, independent discriminator:** the client sends
`x-claude-code-agent-id` (`internal/proxy/server.go:80`), empty for the main
loop, and the ledger keys lanes on it (`internal/ledger/store.go:377`).

**So there are two routes to parent/child separation, from two different
sources, and they agree in shape.** That is stronger than either alone.

### K.2 Parallel versus sequential, and what it costs to get wrong

**OBSERVED.** The per-lane keying was not there originally and the failure mode
is documented in detail, which makes it the most useful thing in this section
for anyone building an external tool.

`lane-isolation-2026-09-06.md`: `stats.observe` compared each request's prefix
hash against **one session-wide field**. In a fan-out session, lane A's request
overwrote the field and lane B was judged against lane A's hash.

| | Events | Deficit tokens | Share |
|---|---:|---:|---:|
| As published | 34 | 3,734,134 | 98.8% |
| Lane-correct re-read | 3 | 416,887 | 11.0% |
| Forged by the session-wide compare | 31 | 3,317,247 | -- |

**Thirty-one of thirty-four events had not happened.** Both figures were
retracted, including the corrected one, because it was derived from a ledger
written by the broken classifier.

**Five fields carried the same defect**, and the fifth is the one worth carrying
forward:

> An unseen lane's usage is the zero value, `ExpectedRead` of it is 0, and an
> opening request reads 0, so the two matched exactly and scored "reproduced".
> Every sub-agent lane in a fan-out contributed a cache hit that never happened.
> It produced no error, no break and no anomaly -- it only inflated a success
> metric, which is the direction nobody audits.

It was found by a surviving mutation, not by reading the code.

**DERIVED, and it is the design rule for any external attribution tool:** a
lane-unaware instrument does not fail loudly on fan-out. It reports success.
**Per-lane keying is a correctness requirement, not a reporting nicety.**

**Scale exercised:** 15 subagent lanes plus the main loop in the corrected run;
30 lanes in the earlier trial. `docs/SURFACES.md` records the honest ceiling:
"the highest concurrency ever exercised is a handful of parallel sub-agents.
Behaviour under a large fleet sharing one proxy is untested."

### K.3 The cost of fanning out

**OBSERVED, then substantially corrected by its own authors the same day. Both
halves are needed.**

`docs/evidence/fan-out-premium-2026-09-06.md`. Siblings in a fan-out do not
share the cache write for the prefix they have in common. Each writes it again.
A write is billed at 1.25x and a read at 0.1x, so a sibling that writes pays
12.5x a sibling that reads.

| Lanes opened together | Groups | Premium (aggregate) | Premium (median group) |
|---|---:|---:|---:|
| 2 or more | 483 | 1.68x | 1.48x |
| 3 or more | 79 | 2.56x | 2.24x |
| 5 or more | 24 | 3.34x | 2.93x |

**The correction, and it is the more important half.** Adversarial review the
same day showed the estimator decomposes as:

```text
premium = [ SUM(Pi) / (k * P_max) ] x [ 1.25k / (1.25 + 0.1(k-1)) ]
              empirical                  pure arithmetic in k
```

The right-hand factor contains no data. It is fixed by group size and the
provider's own multipliers, and it exceeds 1 for every `k > 1`. **No corpus can
produce a premium below 1.**

| Lanes | Measured | Arithmetic ceiling | Implied dispersion |
|---:|---:|---:|---:|
| 2 | 1.68x | 1.852x | 0.907 |
| 3 | 2.56x | 2.586x | 0.990 |
| 5 | 3.34x | 3.788x | 0.882 |

Measured values sit at 88 to 99% of the ceiling. **The only empirical content in
the table is the dispersion ratio -- how equal the sibling prompts are -- and it
is about 0.9 and flat.** The monotonic rise from 1.68x to 3.34x is the shape of
the estimator, not a finding about fan-out.

**What survives:** the cost is real. An operator running five lanes genuinely is
billed about 3.3x a shared-prefix baseline. **What does not survive:**
presenting it as a measured discovery.

**A quantity that cannot come out low is not evidence.** That is the general
lesson and it applies directly to any agent-cost metric anyone builds next.

### K.4 The specific question: can an external tool separate the lanes?

**Can an external tool reading a Claude Code transcript separate parent tokens,
child tokens, handoff tokens, tool tokens, synthesis tokens and verification
tokens?**

**Short answer: two of the six, cleanly. One more by estimate. Three not at all.**

| Lane | Verdict | Class | Basis |
|---|---|---|---|
| **Parent tokens** | **YES** | **OBSERVED** | Non-sidechain lanes, `isSidechain: false`, grouped by parent chain. `analysis.MainLane` picks "the largest non-sidechain lane". Each request carries its own `usage` |
| **Child tokens** | **YES** | **OBSERVED** | Sidechain lanes, one per subagent conversation, each with its own `usage`. Independently confirmed by `x-claude-code-agent-id` at the proxy |
| **Tool tokens** | **NO from a transcript. PARTIAL from a proxy** | **OBSERVED** | Tool *definitions* are "known only from the ledger. Transcripts do not show them", and `PrefixVisible()` is true only for `SourceLedger`. Tool *results* are visible as `tool_result` blocks with `Bytes` and a `ToolName`, but bytes are not tokens and results are not separately billed |
| **Handoff tokens** | **ESTIMATE ONLY** | **DERIVED** | The task prompt is a user message opening the sidechain lane; the report returns as a `tool_result` in the parent. Both are visible as blocks with byte counts. **Neither is a usage counter.** Converting bytes to tokens needs the byte-to-token fit, which this repository reports per transcript with a relative error ranging **29% to 171%** |
| **Synthesis tokens** | **NO** | **UNKNOWN** | "Synthesis" is a semantic role, not a wire fact. No field marks a request as synthesis. `transcript.Request` carries `Model`, `Effort`, `Epoch`, `Timestamp`, `Usage`, `Context`, `Output`, `Tools` and no role or phase |
| **Verification tokens** | **NO** | **UNKNOWN** | Same reason. Nothing distinguishes a verifying turn from any other turn |

**Four conditions on the two clean answers. All four are load-bearing and the
first has been got wrong in production.**

**1. Deduplicate first, or the answer is wrong by roughly 2x.** A subagent lane
re-renders its parent's requests, so the same usage record appears in more than
one file.

> **54.8% of usage records in this corpus are duplicates (54,636 of 99,747). An
> earlier attempt at this measurement that skipped the dedupe overstated project
> spend by about 5x. Any analysis of this corpus that sums usage across files
> without deduplicating is wrong.**

The routing baseline independently reports 57,751 of 105,532 rows (54.7%) as
repeats. **Two measurements, both near 55%.** Deduplicate on `requestId`, or on
`message.id` where the client writes no `requestId`.

**2. The join key is not always the provider's.** `Request.IDMeasured` records
whether the id came off the wire at all; false means it was synthesised from the
record's position in its file and **identifies the request only within that
file.** `Request.IDFromMessage` records that the id is the message id rather
than the request id -- the `cli` entrypoint writes a top-level `requestId`, while
`sdk-cli`, `sdk-ts` and `claude-desktop` do not. An external tool that treats all
ids alike will silently fail to dedupe across files for three of four clients.

**3. Overlapping requests void per-event attribution.** `CorrelationLaneOverlap`
marks a request whose predecessor is not determined. Lane *totals* survive;
per-event causal attribution does not, and the correct output is **NOT MEASURED**
rather than a guess.

**4. Client-written error lines are not provider requests.** `isApiErrorMessage`
lines carry `model: "<synthetic>"` and all-zero usage. Forty exist in a
1,821-transcript corpus. Counting them cost a lane its model identity -- one
placeholder at the head of a lane took it from `claude-opus-5` to `<synthetic>`,
which is in no price table, and its re-billed figure from 1,586,545 tokens to
zero.

**One bonus lane the question did not ask for, and it is free.** **Reasoning
tokens are separable per request and therefore per lane**, from
`usage.output_tokens_details.thinking_tokens` (`internal/transcript/wire.go:123-125`),
asserted non-zero against a real Claude Code fixture. **Thinking tokens by lane
is computable from any existing corpus today and nobody has computed it.**

**The shape of the answer.** The three separable lanes -- parent, child,
reasoning -- are separable because **the provider reports a usage object per
request and the client records a structural marker per line.** The three
inseparable ones -- handoff, synthesis, verification -- are inseparable because
**they are descriptions of intent, and no instrument reports intent.** That
boundary is not a gap in the transcript format. It is the difference between
what was billed and what it was for, and closing it needs a marker the client
does not write.

### K.5 Failed and abandoned agents

| Item | Class | Evidence |
|---|---|---|
| A failed agent's tokens are still billed | **DERIVED** | Each request in a sidechain lane carries its own `usage` regardless of how the lane ended. Nothing zeroes it |
| A failed agent is identifiable as failed | **UNKNOWN** | No terminal-status field appears in this repository's lane model. `isApiErrorMessage` marks a failed *call*, not an abandoned *agent* |
| The cost of duplicate exploration across siblings | **UNKNOWN** | `CallKey` on a `tool_use` block "identifies a tool call by tool name and input without holding the input: identical calls share a key". **The key exists. No evidence file reports cross-lane duplicate call rates.** This is the cheapest unrun measurement identified in this pass |
| Result aggregation cost | **ESTIMATE ONLY** | Same limit as handoff tokens |
| Verification cost | **UNKNOWN** | Same limit as verification tokens |

**`CallKey` deserves the emphasis.** It is a field built for detecting identical
tool calls, it is populated, and no dated evidence file in `docs/evidence/` uses
it to report how often parallel siblings do the same work twice. That is a
measurement this repository can already take.

---

## Summary of what is genuinely established

**Solid, OBSERVED, reproducible from files on disk:**

- Parent and child token attribution from a transcript, via `isSidechain` and
  parent-chain lane ids, subject to four named conditions.
- Reasoning tokens per request, from `output_tokens_details.thinking_tokens`.
- Effort as a controllable, visible request parameter with a named and priced
  cache consequence.
- Tool definitions are invisible to a transcript reader and visible to a proxy.
- The dominant measured tool cost on this surface is connection timing, not call
  volume: 4.2% of prompt tokens re-billed, all three events MCP handshakes.
- 54.8% of usage records duplicate across files. Dedupe or be wrong by ~2x.

**Established as negative results, which are findings:**

- 3.09M tokens moved a 5h quota counter by zero steps.
- No measured sigma between models is distinguishable from 1.0 at its own band.
- The fan-out premium table is 88 to 99% arithmetic.

**Not established, and named as such:**

- Every DOCUMENTED cell in this document. Zero vendor pages were fetched.
- Per-model context windows, output limits, latency, quota weighting.
- Handoff, synthesis and verification token separation.
- All three forbidden assumptions, which remain hypotheses with falsifiers
  attached.

---

## Sources

### Attempted and NOT fetched, 2026-09-29

Every URL below was targeted this session and refused by the harness before any
request left the machine. **None of these is a citation. They are a reading list
for the next pass.**

- [Claude models overview](https://docs.claude.com/en/docs/about-claude/models/overview) -- model families, IDs, aliases, context windows, output limits
- [Extended thinking](https://docs.claude.com/en/docs/build-with-claude/extended-thinking) -- thinking parameters, budgets, billing, thinking during tool use
- [Claude Code subagents](https://docs.claude.com/en/docs/claude-code/sub-agents) -- separate context, per-subagent model and tools, parallelism, result return
- [Claude Code costs](https://docs.claude.com/en/docs/claude-code/costs) -- what drives token usage and cost
- [Claude Code model configuration](https://docs.claude.com/en/docs/claude-code/model-config) -- `/model`, aliases, `ANTHROPIC_MODEL`, model-role guidance
- [Claude Code settings](https://docs.claude.com/en/docs/claude-code/settings) -- tool and output bounding controls
- [Claude Code MCP](https://docs.claude.com/en/docs/claude-code/mcp) -- MCP configuration and output limits
- [Tool use overview](https://docs.claude.com/en/docs/agents-and-tools/tool-use/overview) -- tool definition token overhead, parallel tool use
- [Anthropic support: usage limits](https://support.claude.com) -- the verbatim wording on factors affecting usage limits, requested by this brief and **not obtained**

### Read, in this repository, 2026-09-29

Paths are relative to the repository root.

- `docs/SURFACES.md` -- surface inventory, provider surface, per-lane status keys
- `docs/AGENT-SURFACE.md` -- agent-facing documentation gaps
- `docs/SURFACE-REGISTRY.md` -- conformance matrix, `NOT_MEASURED` rules
- `docs/TOKEN-PRICES.md` -- price source probe, unpriced-model failure mode
- `docs/evidence/lane-isolation-2026-09-06.md` -- 4.2%, the 98.8% retraction, MCP handshake breaks
- `docs/evidence/fan-out-premium-2026-09-06.md` -- fan-out premium and its same-day arithmetic correction
- `docs/evidence/routing-baseline-2026-09-06.md` -- baseline selection, sigma bands, cache-floor brackets, model mix
- `docs/evidence/quota-titration-2026-09-06.md` -- the 3.09M-token null
- `docs/evidence/subscription-allowance-2026-09-09.md` -- retracted in full; read as a specification for an untaken measurement
- `docs/evidence/wire-families-2026-09-06.md` -- wire families, context window not advertised, `context-management-2025-06-27` noted as opt-in
- `docs/evidence/compaction-and-index-2026-09-06.md` -- compaction ceiling, retention, latency
- `AGENTS.md` -- the non-negotiable on never touching thinking blocks or signatures
- `internal/cachemodel/anthropic.go` -- price table, cache floors, break-cause vocabulary
- `internal/transcript/types.go` -- block kinds, `Usage`, `Request`, `Lane`, correlation constants
- `internal/transcript/wire.go` -- `WireUsage`, `output_tokens_details.thinking_tokens`
- `internal/transcript/claudecode.go` -- transcript line shape, request grouping, lane derivation
- `internal/proxy/server.go` -- `x-claude-code-session-id`, `x-claude-code-agent-id`
- `internal/ledger/store.go`, `record.go`, `summarize.go` -- lane keying, `output_config.effort`

---

[Documentation index](../../docs/README.md) · [Repository README](../../README.md)
