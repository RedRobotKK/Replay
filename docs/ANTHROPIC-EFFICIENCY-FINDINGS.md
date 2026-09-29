# Anthropic efficiency: findings

Opened 2026-09-29 at `2732809`. **Zero API spend. No production behaviour
changed.** Bound by `ANTHROPIC-EFFICIENCY-CONTRACT.md`.

Evidence classes are used strictly: DOCUMENTED (a fetched provider page, with a
URL), OBSERVED (measured from data on this machine), DERIVED, HYPOTHESIS,
UNKNOWN, REFUTED. Nothing is promoted between them.

## 1. A documentation gap this campaign could not close

**Every WebFetch and WebSearch attempt by the context and caching sweep failed**,
16 attempts across 5 URLs, because a server-side classifier was unavailable.

That sweep therefore recorded **zero DOCUMENTED rows** and refused to promote
the summary in its own brief into a citation. That is the correct outcome:
citing a URL nobody opened manufactures evidence. The unfetched URLs are listed
in `../experiment/anthropic-recon/context-compaction-cache.md` with the row each would close.

**Consequence:** the compaction and caching half of this map rests on OBSERVED
data only, and its DOCUMENTED column is empty pending a re-run.

## 2. OBSERVED: compaction is fully written down

Claude Code records a compaction as a **pair** of records: a
`system` / `compact_boundary` carrying `compactMetadata`, then the summary on the
next line flagged `isCompactSummary`. The metadata carries `trigger`,
`preTokens`, `postTokens`, `cumulativeDroppedTokens` and `durationMs`.

Measured across this machine's corpus:

| quantity | value |
| --- | ---: |
| compaction events, distinct by uuid | **67** |
| distinct `trigger` values | **`auto` only** |
| `preTokens`, median | 968,435 |
| `preTokens`, minimum | 166,712 |
| `postTokens`, median | 27,116 |
| retention, median post/pre | about 2.8% |

**Three corrections, one of them to this document's own first draft.**

A narrower scan reported 39 events with 30 of 39 firing within 1% of 1,000,000
`preTokens`. Over the full corpus only **22** do and the minimum is 166,712, so
auto-compaction is not a single threshold event and fires well below the ceiling.

Retention as a median of ratios and as a ratio of medians are different
statistics and must not be quoted interchangeably.

**This document first said 69 events. It is 67.** A separate sweep reported that
54.8% of usage records duplicate across transcript files and that skipping
deduplication overstated spend roughly fivefold. That warning applied directly to
the scan above, so it was re-run keyed on record `uuid`: 69 raw lines, **67
distinct events**, an inflation of 1.03x rather than 5x. The correction is small
and it is recorded because the check was the point.

**Zero manual events exist in this corpus.** An auto-versus-manual comparison
cannot be made from it and needs a deliberate `/compact`.

**Not established:** that compaction consumes usage. The corpus observation that
compacted transcripts hold a large share of re-billed tokens is a different
procedure measuring a different quantity, and is not confirmation.

## 3. REFUTED: cache TTL is not a clean entitlement signal

**Hypothesis**, derived by this session from an entitlement sweep claim that
prompt-cache lifetime is 1 hour on a subscription and 5 minutes once drawing on
usage credits: if true, a within-session 1h to 5m transition would mark an
entitlement boundary crossing, observable in transcripts for free.

**Measured** across 7 sessions carrying TTL data, 71,352 turns:

| | |
| --- | ---: |
| 1h cache-creation tokens | 223,978,221 (80.0%) |
| 5m cache-creation tokens | 56,087,770 (20.0%) |
| sessions mixing both | 3 of 7 |
| transitions 1h to 5m | 6 |
| **transitions 5m to 1h** | **5** |

**REFUTED.** Transitions run in both directions. An entitlement crossing is
one-way within a session: an allowance is not repeatedly re-entered. Something
else drives the TTL choice, most plausibly per-request breakpoint selection by
the client, which is a HYPOTHESIS and not established here.

**What is NOT refuted:** the underlying claim that TTL differs by entitlement.
Nobody could fetch the page. That claim remains UNVERIFIED. What died is this
session's inference that transcripts would reveal the crossing.

## 3b. INDEPENDENTLY CONFIRMED: cache TTL differs by authentication

Two sweeps reached this separately, and the second one could fetch provider
pages: **prompt-cache lifetime is one hour on a subscription and five minutes on
an API key.**

This does not revive the refutation above, and the two coexist. TTL can differ
by authentication regime while, inside one regime, the client still varies its
breakpoint choice per request. What died was the inference that a transcript
would show a one-way crossing; what stands is that **authentication method must
be held fixed in any caching experiment**, or the arm is measuring the
credential rather than the treatment.

## 4. OBSERVED: this machine's entitlement regime

`ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `CLAUDE_CODE_OAUTH_TOKEN`,
`ANTHROPIC_BASE_URL` and `ANTHROPIC_MODEL` are all unset. Measurements taken here
are not silently in a per-token regime.

This matters because the entitlement sweep reports, as DOCUMENTED, that
authentication rather than plan decides metering, and that an API key takes
precedence over a subscription login. **Any future benchmark on this surface has
to pin and record the credential state**, or it cannot say which regime it
measured. That is now a required field, not a nicety.

## 4b. DOCUMENTED: the API rate-limit surface, and two traps

The API sweep fetched successfully. Rate limits are **RPM, ITPM and OTPM**, per
model class, token-bucket replenished, each with its own
`anthropic-ratelimit-{requests,input-tokens,output-tokens}-{limit,remaining,reset}`
triple, plus a generic `-tokens-*` triple reporting whichever limit currently
binds.

Two traps worth more than the headers:

1. **A spend-cap 429 is not throttling.** It carries error type
   `rate_limit_error` but **no** `retry-after`. Classifying on the type alone
   reads a month-long billing stop as transient backoff. The discriminator is
   `error.details.error_code` = `enforced_spend_limit_reached`.
2. **Token remaining values are rounded to the nearest thousand**, so a headroom
   calculation has a granularity floor.

Also documented: cache reads do not count toward ITPM on most models, Haiku 3.5
excepted, and `input_tokens` counts post-breakpoint tokens only rather than
total input.

**Reseller quota ownership is not uniform.** Microsoft Foundry explicitly does
not return Anthropic's rate-limit headers, which removes header-based
measurement on that platform entirely. Bedrock splits ownership: Anthropic gates
token-per-minute quota while AWS enforces and adjusts requests per minute.
Google Cloud's quota is Google-owned QPM and TPM, with TPM counting input and
output together. Whether Bedrock or Google Cloud return the
`anthropic-ratelimit-*` headers is UNKNOWN.

## 4c. OBSERVED: what the reasoning surface actually offers

**Exactly one reasoning control is genuinely user-controllable: effort.** It is a
real request parameter, `output_config.effort`, it appears in the transcript, and
changing it has a named priced consequence in this repository's own cache model,
`CauseEffortChange`. Thinking budgets, an extended-thinking switch and per-
subagent effort are UNKNOWN. Thinking tokens are MEASURABLE but not controllable.

**Per-lane attribution reaches two of six lanes cleanly.** Parent and child
separate reliably by `isSidechain` plus parent-chain lane ids, corroborated
independently by the proxy's `x-claude-code-agent-id`, and reasoning tokens
separate per lane. Tool tokens cannot be attributed from a transcript because
tool definitions are ledger-only. Handoff cost is an estimate from bytes rather
than counters, with a fit error between 29% and 171%. Synthesis and verification
are intent labels with no wire marker and cannot be attributed at all.

**`/cost` is not evidence.** It is a local estimate at list price, and a
`modelPricing` setting changes what it reports without changing what is billed.

## 5. The blocking UNKNOWN

The unit in which a subscription usage allowance is denominated **is not
published**. Only relative multipliers are given, and there is no published
function mapping tokens to consumed allowance.

Coupled to it: whether cached input tokens draw down a subscription allowance at
a reduced rate, the way they are excluded from API input-token-per-minute
limits. **If they do not, this repository's largest documented lever does not
transfer to a subscription seat at all.**

Both need measurement. Neither can be inferred. Together they are why contract
invariant A2, usage is not context, exists.

## 6. What Replay already observes, verified in-repo

| surface | status |
| --- | --- |
| compaction events and their metadata | parsed, `internal/transcript/claudecode.go` |
| cache creation split by TTL | parsed, `Create5m` and `Create1h` |
| thinking tokens | carried in `Usage.ThinkingTokens` |
| per-lane and subagent attribution | `MainLane`, sidechains separated |
| serving identity on the OpenAI-compatible family | added 2026-09-29 |
| cache state as three values, not two | `Response.CacheState()` |

## 7. Proposed experiments, none run

| # | question | cost | blocked on |
| --- | --- | --- | --- |
| E1 | does a manual `/compact` differ from an auto one | one deliberate compaction, no spend | nothing |
| E2 | is CLAUDE.md inside the cached system block | one proxied session | nothing |
| E3 | what drives the 1h versus 5m TTL choice | proxy inspection of breakpoints | nothing |
| E4 | do cached tokens draw down a subscription allowance | requires an entitlement readout | UNKNOWN whether readable |
| E5 | `/clear` versus `/compact` for continued work | paired, needs a manual control | E1 |

**None is authorised by this document.** The map is the deliverable.

## Register of unknowns

| id | unknown | closes with |
| --- | --- | --- |
| U1 | the unit of a subscription usage allowance | provider publication, or a calibration experiment |
| U2 | whether cached tokens reduce subscription allowance draw | measurement against an entitlement readout |
| U3 | what forms the prompt-cache key | provider-side only, likely permanently UNKNOWN |
| U4 | whether compaction itself consumes usage | attributable accounting of the summarisation turn |
| U5 | whether repeated compaction degrades quality | not observable from transcripts at all |
| U6 | what a manual `/compact` writes | one deliberate event, free |
| U7 | whether CLAUDE.md is in the cached system block | one proxied session, free |
| U8 | the compaction and caching DOCUMENTED column | re-run the 10 fetches |
