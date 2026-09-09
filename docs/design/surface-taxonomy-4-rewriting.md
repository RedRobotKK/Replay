# Surface taxonomy, part 4: prompt restructuring

**2026-09-09.** Byte-for-byte passthrough is a setting, not an identity: `--mask`,
`--context-edit-trigger` and the OpenAI usage parameter each already modify a body, and
`docs/requirements.md` PX-2 states the promise as "when no policy is enabled". So the question
"could Replay reshape a prompt into a more contextual, enriched form?" is not blocked by the
promise. It is blocked by something narrower and harder: **Replay has no instrument that can see
whether the reshaped prompt produced a better answer, and three of its four existing guardrails move
in the *good* direction when a rewrite makes the answer confidently wrong.**

This document takes the surfaces one at a time, draws the safe/unsafe line where the code actually
puts it rather than where ADR-0011 puts it, works the FIFO question, and then says plainly what
cannot be done.

---

## 0. What the what-if machinery can and cannot simulate today

The proxy scores candidate layouts on every request without sending them. `stats.rescore`
(`internal/proxy/state.go:457`) runs after the response is delivered, builds the lane, calls
`analysis.AnalyzeLane` and stores `[]WhatIf` per lane (`state.go:509`). `WhatIf` is explicitly
dry-run: "the candidate is simulated from measured usage and never sent to the provider"
(`state.go:122-124`).

**The candidate set is fixed and small.** `LaneReport.Policies` (`internal/analysis/report.go:67-79`)
returns exactly:

| Candidate | Source | Estimated? |
|---|---|---|
| as-run | `AsRun` (`internal/analysis/replay.go:67`) | no — reported usage only |
| TTL 5m | `WithTTL` (`replay.go:144`) | no — "measured, not estimated, because no byte-to-token conversion is involved" (`replay.go:143`) |
| TTL 1h | `WithTTL` | no |
| context-edit @ 50% of largest prompt | `WithContextEdit` (`replay.go:175`) | **yes** — block sizes come from the fit (`replay.go:174`) |
| context-edit @ 75% | `WithContextEdit` | yes |

and nothing else. Everything past as-run is gated on `Calibration.Passes()`
(`report.go:70`, `internal/analysis/calibrate.go:86-88`), so a session whose cache reads could not be
reproduced is scored for nothing.

**What it therefore cannot simulate.** Every candidate above changes *when the cache expires* or
*how much the provider clears*. None of them changes the arrangement of bytes. The simulator's core,
`cacheState.serve(at, prompt, tail, observed)` (`replay.go:106`), takes scalars: a prompt size, an
uncached tail size, and the read the client's own behaviour made available. **There is no code path
that derives `tail` from a proposed layout**, so there is no way to ask "what would the tail have
been if the tool list had been in yesterday's order". Scoring any rearrangement transform requires a
new simulator input, not a new policy row.

**And breakpoint placement cannot be scored at all today.** The summariser counts `cache_control`
markers and discards their positions (`internal/ledger/summarize.go:82,88,111`), a limitation
`internal/ledger/evidence.go:26-37` already names in another context: "the proxy records how many
`cache_control` markers a request carried but never where they sat […] Recording the block index of
each marker would close the gap; it is a small change to the summariser and the single most valuable
capture this feed is missing." **`breakpoint-on-stable-block` is ADR-0011's flagship stage-2
transform and its offline scoring is blocked on a capture the ledger does not take.** That is the
first thing to build, and it is a summariser change, not a rewriter.

---

## A. The surfaces where content could be reshaped

Thirteen. Stage numbers are ADR-0011's. "Tier" is defined in section B.

| # | Transform | Primitive | Tier | Semantics-preserving? | Stage | Buys |
|---|---|---|---|---|---|---|
| 1 | `breakpoint-on-stable-block` | cache breakpoints | 0 | **yes**, provably | 2 | cache reads, measurable next turn |
| 2 | TTL selection (5m / 1h) | request parameter | 0 | **yes**, provably | 2 | fewer writes on slow sessions |
| 3 | `openai-include-usage` (**shipped**) | request parameter | 0 | **yes**, provably | 2 | nothing on cost; makes guards work at all |
| 4 | `stable-tool-order` | tool-definition ordering | 1 | content yes, sequence no | 2 | cache reads — **magnitude unmeasured, plausibly zero** |
| 5 | Canonical JSON key order in a tool def | tool definitions | **2** | **no** — looks safe, isn't | 3 | cache reads |
| 6 | `hoist-stable-prefix` | system prompt / message order | **2** | **no** — ADR-0011 mis-sorts this | **3, not 2** | cache reads |
| 7 | `context_management` (**shipped**) | tool results, provider-side | 2 | no — blocks are cleared | 2 (shipped) | smaller reads, traded against a write |
| 8 | Masking + scoped rehydration (**shipped**) | any message text | 2 | no — model reads a placeholder | shipped | not cost; secrecy |
| 9 | Per-block tool-result byte cap | tool results | 2 | no | **offline only, permanently** | fewer tokens |
| 10 | Dropping never-called tool definitions | tool definitions | 2 | no — removes a capability | advice only | fewer tokens on every request |
| 11 | Splitting first-turn instruction files | system prompt / first turn | 2 | no | advice only | cacheable prefix |
| 12 | Client-side history editing | message history | 2 | no | **forbidden** | nothing — 400s and invalidates anyway |
| 13 | Prefix-safe action ordering | the agent's plan, not the bytes | n/a | n/a | advice only | one re-prefill instead of N |

### The four that are genuinely safe

**1. Breakpoint placement.** A `cache_control` marker is metadata; the model reads byte-identical
content (`docs/architecture/rewriting-prompts.md:33-36`). It fails safe — a badly placed breakpoint
costs a cache write, it cannot produce a wrong answer. It is ADR-0003 kind one, "a request parameter
the client left unset" (`docs/adr/0003-policy-application-constraints.md:12`), and the proxy already
knows whether the client set one (`Prompt.CacheControlCount`, `internal/ledger/record.go:131-132`),
so the "never override a marker the client set" rule is enforceable from data already on the record.
**Blocked on capturing marker positions (§0).**

**2. TTL selection.** Already a scored candidate and already flagged as measured rather than
estimated (`replay.go:143`). Live application is the same shape as the context-edit pin.

**3. `openai-include-usage`.** Shipped at `internal/proxy/server.go:544-552`, and the comment states
the reasoning exactly: without it that family reports no usage and "every guard below sees a free
request". It changes the response shape, not the prompt.

**4. `stable-tool-order` — safe, and its value is unmeasured.** Two findings, and the second is the
one that matters.

*It is detectable today and is not named.* `PrefixHash` is order-sensitive: it hashes each tool's raw
JSON in slice order, then the system block (`summarize.go:91`, `hashOf` at `summarize.go:134-141`).
`diffPrefix` is name-keyed (`internal/proxy/causedetail.go:98-121`), so a **pure reorder produces an
empty delta** — no added, no removed, no resized — and `prefixDelta.cause()` falls through to its
default arm (`causedetail.go:57-62`), reporting `CausePrefixChange` with an empty detail string. So a
reorder currently surfaces as "the prefix changed and we cannot say why". **Naming that case costs
one branch and would produce the evidence this transform needs.** Adding it is the honest prerequisite
to shipping the transform.

*The corpus evidence available today says reordering is not the problem.* Across the 30-lane trial of
2026-09-06, "every real prefix change was the tool SET changing" (`causedetail.go:20-23`), and the
three genuine events were an MCP connector finishing its handshake and appending a whole block
(`internal/proxy/preflight.go:26-35`). Not one was a permutation. So the transform ADR-0011 names
first is safe and, on this repository's own measurements, may be worth nothing. Say so rather than
ship it on the strength of its safety.

### The one ADR-0011 sorts wrongly

**6. `hoist-stable-prefix`.** ADR-0011:43 lists it in stage 2 alongside `stable-tool-order`, under the
stage's own definition: "byte-identical content reaching the model" (ADR-0011:37). Moving a date out
of the system prompt and into the first user message does not deliver byte-identical content; it
delivers the same characters in a different sequence, across a role boundary. A language model's
output can depend on both. **It is a tier-2 transform in a tier-1 slot**, and it should sit behind the
same trial-and-revert machinery ADR-0011 reserves for stage 3, or be delivered as advice (`§11`)
where the user makes the change in their own configuration and can see it.

### The one that looks safe and is not

**5. Canonical JSON key order inside a tool definition.** Two clients serialising the same schema with
different key order produce different `PrefixHash` values and break each other's cache. Canonicalising
key order is provably lossless *to a JSON parser* and is not lossless *to a tokeniser*: the model reads
the serialised text, so `{"name":…,"description":…}` and `{"description":…,"name":…}` are different
inputs. **This is the sharpest illustration of the line in this document**, because the transform is
trivially provable under the wrong equality relation.

### The ones already shipped that cross the line on purpose

**7. `context_management`.** Applied at `server.go:791-823`: decided at the session's first request,
pinned for the session's life, logged with body hashes before and after, never the bodies
(`server.go:789-790, 822`). This is a content-altering transform — blocks are cleared — that ships
because the provider executes it and the client's own history binding stays intact (ADR-0003:12). It
is the precedent every stage-2 transform is asked to fit, and it is guarded by the re-read rate
(§B and `internal/proxy/trial.go:66-70`).

**8. Masking.** `server.go:511-514`, `server.go:668`. The model reads an HMAC-derived placeholder, not
the secret. Deterministic — the same secret always maps to the same placeholder
(`docs/adr/0004-masking-and-scoped-rehydration.md`, Decision) — which is what keeps the cache and makes
it ADR-0003 kind three. It is the one thing in the program that **fails secure rather than open**
(`server.go:656-666`). Masking matters to this document because it proves the governing rule is not
"never alter semantics": it is "alter semantics only where a *different* guarantee pays for it, and
name the guarantee". Masking pays with secrecy. A reworder has nothing to pay with.

### The ones the codebase has already refused

**9. Tool-result trimming.** Offline scoring exists; the live trimmer deliberately does not ship, and
`internal/analysis/trim.go:16-21` records why: Go's `json.Marshal` HTML-escapes `<`, `>` and `&`, so
decode-cut-re-marshal "can return a block six times the cap on HTML, JSX, XML, git conflict markers
and shell redirects, which breaks the idempotence the design rested on. Un-trimming a block previously
sent trimmed is itself a history edit, which the proxy's own detector already names." The record even
has a landing place for it that nothing writes (`record.go:54-65`).

**10. Dropping never-called tool definitions.** The advisor finds them for free — MCP naming carries
the attribution, so no config file is read (`internal/advisor/advisor.go:230-283`, `serverOf` at
`advisor.go:217-228`) — and delivers "disable this server if the work does not need it"
(`advisor.go:415-421`) rather than doing it. That is correct, and `internal/analysis/order.go:24-33`
already gives the general argument: the error modes are asymmetric in the direction Replay cannot see.
A wrong "safe" costs one re-prefill, bounded. A wrong removal costs "a worse engineering outcome that
is unbounded, silent, and invisible to a tool that measures spend rather than success."

**12. Client-side history editing.** ADR-0003:8 — a 400 on current models for organisations created on
or after 2026-08-31, and cache invalidation from the edited point on every model, which defeats the
purpose. Savings visible in replay are "reported as unreachable rather than hidden" (ADR-0003:17),
which is what `WhatIf.ReachableLive` carries (`state.go:136-137`).

---

## B. The line between safe and unsafe

Two tiers is not enough. The code distinguishes three, and conflating the first two is what puts
`hoist-stable-prefix` in the wrong stage.

### Tier 0 — metadata only

The token sequence the model reads is bit-identical. Only fields the model never reads change:
`cache_control`, `ttl`, `stream_options`, `context_management`'s *parameters*.

**The test that polices it.** Decode the body in and out; delete exactly the keys the transform
declares it may touch; re-serialise both through the same canonical marshaller; require byte equality
of the remainder. It is a total function of the two bodies and it needs no corpus.

**It must be shown to fail (ADR-0014).** Mutate the transform to also drop one tool definition and
require the test to go red. `docs/adr/0014-checks-must-be-able-to-fail.md` catalogues what happens
otherwise; the mutation is the check on the check.

### Tier 1 — permutation of self-delimiting units

`stable-tool-order`. The set of tool definitions is preserved; the byte sequence is not.

**The test that polices it.** Multiset equality of the serialised tool elements — sort both
`[]json.RawMessage` by their bytes and compare — plus byte equality of everything outside the `tools`
array. Mutation: change one character of one description and require red.

**What that test does not prove.** It proves the *content* is preserved. It does not prove the
*output* is preserved: a model's answer can depend on the order of a list, and nothing here can rule
that out. Tier 1 is provably content-preserving and **not** provably output-preserving. That gap is
small and it is real, and it is the reason a tier-1 transform still belongs off by default, pinned per
session, and hash-logged like every other one.

### Tier 2 — everything else

Rewording, summarising, trimming, hoisting, key-canonicalising, dropping tools, masking. Nothing about
what the model reads is preserved, and **no structural test exists**, because there is no equality
relation to test under.

### The trap, stated precisely

**A transform that changes what the model sees cannot be verified by a cost measurement, because cost
falls whether the transform helped or hurt.** A rewrite that halves the prompt halves the bill in
exactly the same way whether the answer got better, stayed the same, or became confidently wrong. The
outcome variable and the optimisation target are the same number. `order.go:32-33` already states the
general form: "The bill would fall while the work got worse, on instrumentation incapable of noticing."

### What *would* verify a tier-2 transform

Four things, and Replay has one of them:

1. **A task suite with a machine-checkable success criterion** — tests pass, build green, diff accepted,
   PR merged. Replay has none, and by design: `docs/WHAT-YOU-GET.md` commits to not reading
   `settings.json`, `CLAUDE.md` or `.mcp.json` on a default invocation, a boundary `cmd/replay/prefix.go:29-35`
   restates. The tool that would need to see your project's outcomes is the tool that promised not to
   look at your project.
2. **Randomised assignment.** *Replay has this.* `TrialSettings.treated` splits sessions on a stable
   FNV hash of the session id (`trial.go:56-64`), so the split survives restarts and ordering.
3. **A pre-registered effect size and an N that can resolve it.** Nothing exists. `RevertAfter`
   defaults to 2 breached sessions (`trial.go:33-34`), which is a stopping rule for harm, not a power
   calculation for benefit.
4. **An outcome measure independent of the optimisation target.** Nothing exists, and item 1 is why.

### What Replay has instead: a harm detector, not a benefit detector

Three rework proxies, all already computed:

- **retries** — `Record.Retries` (`record.go:82-84`), set at `server.go:592`
- **error share** — `ErrorCosts` over four classes: tool results flagged as errors, failed edits,
  identical tool call repeated, context overflow or compaction (`internal/analysis/errors.go:14-19`),
  aggregated per lane at `state.go:493-496,527`

- **re-read rate** — `CountReReads` (`internal/analysis/rereads.go:61`), with the rate before and after
  the provider's first clear reported separately (`rereads.go:42-57`)

wired to an auto-revert: `TrialSettings.breached` (`trial.go:68-70`) requires at least
`minGuardrailReads = 5` reads after the first clear before it will judge anything, and `noteBreach`
(`trial.go:75-98`) reverts the policy for new sessions and persists the revert once
`RevertAfter` sessions have breached. **That machinery can fail, and has** — which is exactly what
ADR-0014 demands and what ADR-0011:68-70 already concedes: "That is not proof the rewrite helped. It
is detection that it hurt."

### And the harm detector's blind spot, which is the worst one

**A rewrite that makes the model confidently wrong on the first attempt moves every guardrail in the
good direction.** Fewer retries, because it did not retry. Fewer failed edits, because it did not edit
twice. A lower re-read rate, because it never went back to check. A lower bill, because the wrong
answer was shorter. Replay would report a successful trial. **The failure mode Replay is least able to
see is the one a prompt rewriter is most likely to cause**, and no amount of tuning the guardrail
thresholds changes that, because the signal has the wrong sign.

---

## C. The FIFO question

Replay already has one queue discipline, and it is the model for judging every other idea here.

### Shipped: hold parallel siblings behind a warming prefix

`internal/proxy/siblings.go`. A cache entry becomes readable only once the first response that writes
it begins streaming, so parallel sub-agents with the same prefix all pay the write price
(`siblings.go:11-16`). With the policy on, a request whose prefix is in flight and not yet cached waits
for that first response to begin, then goes out and reads the entry. `enter` (`siblings.go:66-96`)
returns a leader token or blocks on the leader's `done` channel, `MaxWait`, or the request's own
cancellation; `began` (`siblings.go:100-113`) marks the prefix warm and releases the followers. Wired
at `server.go:563-577`, counted as `Held`/`HeldMS` on the session summary (`state.go:113-116`,
`state.go:588-590`) and on the record (`server.go:568`). Default wait 10s (`siblings.go:23-26`), warm
window `TTLShort` (`siblings.go:32`), table bounded at 1024 prefixes (`siblings.go:36`).

**This is sound and it is the template**: it changes *when* a request is sent and never a byte of it.
Its worst case is bounded latency, disclosed on the status endpoint.

**Its one real flaw, named.** On the OpenAI family the prefix hash is derived from the *first block's
identity only* — kind, byte length, and tool name (`internal/ledger/openai.go:65`, `blockIdentity` at
`openai.go:78-92`). Two unrelated requests whose first block is the same kind and the same size collide
and one is held behind the other. The cost is a bounded wait, never a wrong answer, so the failure mode
is the acceptable one — but it should be documented rather than discovered.

### The other seven ideas

| Idea | Sound? | What breaks | Verdict |
|---|---|---|---|
| **Deferral until a prefix is warm** | yes, when a writer is known in flight | nothing new | **shipped** (above) |
| **Speculative deferral** — hold on a *guess* that a writer will appear | no | trades certain latency for uncertain saving; no in-flight leader means nothing to wait for | reject |
| **Deduplication** — collapse two identical in-flight requests into one response | **no** | sampling is stochastic; a client that made two calls expects two answers, two request-ids, two usage records it can reconcile. Returning one response twice *is* changing the answers | reject; `DetectLoop` (`internal/proxy/guards.go:288-314`) already exists to **tell** the agent it is repeating itself, and `LoopLimits.Block` lets the operator refuse. Advise or refuse, never silently coalesce |
| **Caching a response** | **no** | ends "out → the configured provider, every proxied request, byte for byte" (`docs/SURFACES.md:91`). A cached reply has no provider request-id, no usage, no quota headers (`quotaFrom`, `server.go:595`), so every downstream guard sees a free request | reject |
| **Reordering the queue** | no value | the proxy cannot know the client's dependency order. Requests the client issued sequentially may be sequential *because* the second depends on the first — in which case they are not concurrent and there is nothing to reorder. The genuinely-parallel case is the sibling gate's, already solved | reject as redundant |
| **Speculative prefetch** — a synthetic request to warm a prefix | no | it spends the user's money on a request they did not make, and it is **dominated by the sibling gate**: the fan-out leader pays the write anyway, so a warmer only wins if issued *before the client is ready*, which is a bet on a prefix that may never materialise, settled at the 1.25× write multiplier (`internal/cachemodel/anthropic.go:348`). ADR-0001 already rejected the family (padded cache slots; `docs/ROADMAP.md`, "Padded cache slots are rejected outright") | reject |
| **Cross-lane coalescing** — hold a sub-agent until the main loop's prefix is warm | no | different lanes have different tool sets and therefore different prefixes (`preflight.go:67-72`, `report.go:169-173`). Assuming lanes share a prefix is precisely the lane-isolation defect: judged session-wide, 31 of 34 prefix-change events were forged (`preflight.go:26-30`) | reject |

### The best FIFO idea, and its flaw

The best *new* idea is a **deadline-aware sibling hold**: wait only when the estimated write saving
exceeds the latency cost at an exchange rate the operator states, instead of the current unconditional
wait up to `MaxWait`.

**Its flaw is fatal in the same way ADR-0017 was fatal.** The saving must be estimated from prefix
bytes through a fitted ratio. `tokensPerByte = 0.25` is passed unfitted and is known biased low for
JSON schemas (`preflight.go:41-48`), and the fitted ratio's relative error is a byte-weighted standard
deviation that reaches ±159% across sessions (`order.go:87-89`; ADR-0017 puts the measured band at
29–171%). This project's own straddle doctrine forbids deciding on a figure whose error bar spans the
threshold (`preflight.go:99-110`, `analysis.PreFlightDeficit.Straddles`), so a deadline rule built on
that estimate would straddle its own threshold on most requests and would have to fall back to the
unconditional wait it was meant to replace. **The unconditional 10-second bound is not a crude version
of the smart rule; it is the only version the measurement supports.**

---

## D. What this cannot do

**Replay measures cost and structure. It never reads a model's output.** `stripText`
(`summarize.go:114`) removes block text before a summary leaves the summariser — "block text is dropped
before the summary leaves this function" (`summarize.go:69`). The response tap records status, usage,
request-id and quota headers (`server.go:591-601`) and nothing else. Every field of `WhatIf` is a cost
or a structure figure: effective tokens, share vs as-run, cached share, list cost
(`state.go:125-137`). There is no place in the program where an answer's quality is or could be
represented.

**So any claim about "accuracy" needs a signal the tool does not have, and §B names what it would have
to be:** a per-session outcome that is machine-checkable, externally determined, and independent of
token count. Tests passing. A build going green. A diff accepted. A PR merged. Replay sees none of
these, and the reason is a deliberate boundary — the tool that promised not to read your configuration
is the tool that would need to watch your project succeed or fail.

**Even the honest half is asymmetric.** The trial machinery can detect that a rewrite made things
worse and revert it (`trial.go:68-98`). It cannot detect that one helped, and it is structurally blind
to the specific failure a rewriter causes most easily: a confidently wrong answer accepted on the first
attempt lowers retries, lowers the re-read rate, lowers error share and lowers cost. Every instrument
Replay owns would call that a success.

**There is a difference worth naming between the two failures ADR-0014 is about.** A check that cannot
fail is a bug — the grep standing in for a capability guarantee, the assertion comparing a field to
itself. A check that *cannot be constructed* is a different thing, and this is one: there is no
arrangement of the data Replay holds that measures answer quality. Building a weak one would be worse
than having none, because it would be the first number in this tool with nothing behind it, and
`replay learn`'s refusal to recommend a policy whose calibration is weak is, as
`docs/architecture/rewriting-prompts.md:60-62` puts it, "the credibility of this project."

### What to do, in order

1. **Capture `cache_control` marker positions in the summariser.** It unblocks offline scoring of
   `breakpoint-on-stable-block`, and `evidence.go:34-37` already calls it "the single most valuable
   capture this feed is missing". One change, two features.
2. **Name the pure-reorder case in `prefixDelta.cause()`** (`causedetail.go:57-62`). One branch. It
   turns "the prefix changed and we cannot say why" into the evidence `stable-tool-order` needs, and it
   may well show the transform is worth nothing — which is a result, not a failure.
3. **Build `replay rewrite` as ADR-0011 stage 1 describes.** It does not exist: `cmd/replay/main.go`
   has no `rewrite` case in its subcommand dispatch. Offline, sends nothing, and ADR-0011:32 is right
   that most of the value is there.
4. **Move `hoist-stable-prefix` from stage 2 to stage 3**, or deliver it through the advisor.
5. **Do not build a reworder inside this binary.** Stage 3's `--rewrite-command` seam is the right
   answer if one is ever wanted: the user brings the model, pays its latency, and owns the risk — at
   the price ADR-0011:89-92 already names, which is that "no `exec.Command` anywhere"
   (`docs/SURFACES.md:158`) stops being a checkable property of this program.

---

[Design notes](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
