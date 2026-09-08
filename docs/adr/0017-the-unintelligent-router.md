# 17. The unintelligent router

**Status:** Rejected
**Date:** 2026-09-07
**Rejected:** 2026-09-07, same day, by red and blue review. Kept rather than
deleted because what killed it is worth more than the proposal was.

## Why this was rejected

**The wedge does not exist.** The claim that no competitor routes on cache state
is false. LiteLLM ships `PromptCachingDeploymentCheck`, a prefix-hash to model_id
index with end-to-end tests across four providers. OpenRouter ships provider
sticky routing, triggered by an observed cache hit and gated on cache-read
pricing. Prefix and KV-aware routing is table stakes at the serving layer:
Dynamo, SGLang, AIBrix, llm-d, vLLM. This was the entire argument for building
it, and it was wrong.

**Switching models does not destroy the cache.** Anthropic documents the cache as
per-model with hash keys and no cross-model eviction, so entries coexist: a Haiku
turn fails to REFRESH the Opus entry, it does not evict it. LiteLLM benchmarked
4,684 real switch-backs and found the earlier model's cache still warm 99.3% of
the time at a 1h TTL. In this repository's own ledger, "model changed" accounts
for 6 of 744 cache breaks. The session-boundary constraint, which was the
sharpest thing in this document, rested on a false premise.

**The decisive band is worth almost nothing.** sonnet-5 caches at a 1024-token
floor and beats opus-5 on both input and output price, so it dominates the
1024-4096 range outright. That cuts the band where the floor decides anything by
86%, to a 512-token window worth about $0.0005 per request.

**The input signal is below the resolution the rule needs.** Placing a prefix
relative to a 4096-token floor requires knowing its length to better than the
estimator provides: `tokensPerByte = 0.25` is passed unfitted, is known biased
low for JSON schemas, and has measured error between 29% and 171%. This project's
own straddle doctrine forbids deciding on a figure whose error bar spans the
threshold.

**The most likely production failure, named precisely:** the router decides
output-heavy tasks with an input-only rule, and is auditably, checkably wrong.
That is the worst available combination, because the audit trail this design is
proudest of would read as evidence that it works.

**One figure in the first draft was withdrawn** before rejection: a projection of
$98.61 to $2,016.22 that appears in no evidence file, cannot be reproduced, and
which the tool itself refuses to produce. It reached this document from an agent
summary rather than from a run.

## What survives, and is worth keeping

The **name** and the **discipline**. A router that reads no prompt, calls no
model, decides deterministically and states the rule that fired is still the
right shape for any routing this project ever does. What was wrong was the belief
that cache-floor arithmetic gave it something to decide.

Two premises also survived review and are worth recording as verified: prefix
stability IS knowable before the wire (`internal/proxy/preflight.go:14-21`
compares against the lane's own last hash, computed before the forward at
`server.go:563`), and the floors are correct, documented and independently probed
to a four-token bracket.

---

## Context

Model routing is a contested category. OpenRouter, Martian, NotDiamond, RouteLLM
and Portkey's conditional routing all sell some version of it, and they share two
assumptions: that the routing decision is **semantic** (read the prompt, infer what
it needs, pick a model), and that it is made **per request**.

Measured on this repository's own corpus, both assumptions are wrong for the case
that matters, and wrong in ways that cost money rather than merely underperforming.

### The price table does not say what a router assumes it says

Input tokens, per million, from the compiled table dated 2026-09-07:

| | cold | cached | cacheable floor |
|---|---|---|---|
| `opus-5` | $5.00 | $0.50 | 512 tokens |
| `haiku-4-5` | $1.00 | $0.10 | 4096 tokens |

Four states, not two:

| | | | |
|---|---|---|---|
| opus cached vs haiku cached | $0.50 vs $0.10 | haiku | 5.0x |
| **opus cached vs haiku COLD** | **$0.50 vs $1.00** | **opus** | **2.0x** |
| opus cold vs haiku cached | $5.00 vs $0.10 | haiku | 50.0x |
| opus cold vs haiku cold | $5.00 vs $1.00 | haiku | 5.0x |

**A prefix between 512 and 4096 tokens caches on Opus 5 and cannot cache on Haiku
at all.** On the input side, routing "down" to the cheaper model doubles the cost
in that band.

**And on its own that observation is not actionable, because the table above omits
output tokens.** Opus 5 bills output at $25 per million against Haiku's $5. Writing
the per-turn comparison out in full, with prefix L and output O:

    opus cached   0.5L + 25O
    haiku cold    1.0L +  5O

Opus wins only when 20O < 0.5L, which is **O < L/40**. At a 2,400-token prefix that
is sixty output tokens. An agent turn is essentially never sixty tokens, so across
the whole 512 to 4096 band Haiku wins on any realistic output length, and the input
side is the smaller term.

Cache writes do not rescue it. The 1.25x multiple at 5m and 2.00x at 1h give Opus a
twelve turn and twenty turn amortisation threshold respectively, which the four
state table has no term for at all.

**The finding that survives is narrower than it first appeared:** the floor makes
the input side of the comparison invert, and the input side is not what decides the
turn. The router's rule must therefore be stated over the whole per-turn cost
including output, not over input price, and it will recommend switching far less
often than the table suggests.

This is not a hypothetical. On the maintainer's corpus `claude-haiku-4-5` billed
**$0.1241 per request** against `claude-opus-5` at **$0.0995**, on a model five
times cheaper per token. The corpus figure is confounded (short tasks are routed to
Haiku, so Haiku requests are short by construction), but the mechanism is not: the
floors are published.

### The cache is per model, so switching destroys it

Route turn 12 to Haiku and turn 13 back to Opus and the entire prefix is re-billed
at write prices, because Opus's cache was keyed on content it no longer has. On
this corpus the cached share is roughly 95% of prompt tokens. An arbitrage that
must first beat re-billing 95% of the context almost never wins.

**So the routing decision belongs at a session boundary, not at a request.** That
single constraint disqualifies most of what the category sells.

### The tool already says its own answer is uncertain

`replay route --to claude-haiku-4-5` on this machine **refuses to give a dollar
figure at all**: "sigma unmeasured for this pair (claude-haiku-4-5: no turns on
the wire)". It reports a hit rate and break-even trims and suppresses the rest,
because the tokenizer dilation between the two models has never been measured on
comparable work.

An earlier draft of this ADR quoted a projection of $98.61 to $2,016.22 against
$1,056.14. **That figure is withdrawn. It appears in no evidence file, it cannot be
reproduced, and the tool declines to produce it.** It reached this document from a
summary rather than from a run, which is the failure this project exists to catch.

The bands are also not the estimator's floor any more. That claim describes an
estimator superseded by `PoolFits` (internal/analysis/pool.go), which narrows with
Kish-effective n. Bad for the sentence, good for the design: it is what makes a
real gate possible rather than an intention.

## Decision

Build the routing decision as a **structural** rule, not a semantic one, and call it
what it is.

1. **It does not read the prompt.** Every input is metadata Replay already holds:
   prefix length, whether the prefix hash is stable, main loop or sub-agent lane,
   and the session's own output-length distribution, which the arithmetic above
   makes the most important of the four rather than a refinement.

   Tool blocks are deliberately NOT an input to v1. They are ledger-only, and the
   ledger on the maintainer's machine holds five sessions against a 1,606
   transcript corpus, so a rule depending on them would be untestable.
   Content is never inspected, which keeps `serve`'s existing guarantee intact:
   the ledger holds block kinds, sizes, timings and usage, and no message text.

2. **It does not call a model to decide.** A classifier in the hot path costs money
   and latency that the arbitrage must then beat, and it makes every decision
   unauditable. This follows the classifier discipline already binding on this
   project: deterministic rules ship before any model, and `UNKNOWN` is a
   first-class label rather than a fallback.

3. **It decides at session boundaries.** Per-request routing is refused, with the
   cache arithmetic above as the reason.

4. **It is advisory before it is enforcing.** With bands of plus or minus twenty
   times, a router that acts is wrong often enough to cost more than it saves.
   v1 prints the decision it would have made and what the rule was. Promotion to
   enforcement is gated on the bands narrowing, measured, not on confidence.

5. **Every decision states its rule.** "2,400-token stable prefix, caches on opus-5,
   below haiku-4-5's 4096 floor, staying" is a sentence a person can check. A
   routing decision nobody can audit is a spend decision nobody can audit.

## The name

**The unintelligent router.** It is accurate rather than modest: there is no
intelligence in it, and that is the feature. Everything it knows is arithmetic over
published floors and measured prefix lengths. In a category that sells inference
about your intent, the differentiator is a router that declines to infer anything
and can therefore show its work.

## Consequences

**What this cannot do.** It cannot tell that a prompt is simple enough for a smaller
model, because that is a semantic judgement and it makes none. A user who wants that
should use a semantic router; this is not one and must never be marketed as one.

**What only this can do.** No competitor routes on cache state, because none of them
reconstructs the prefix. That is the entire wedge, and it exists because the
measurement came first.

**The risk that would kill it.** If providers converge on a single cacheable-prefix
floor, or drop the floor entirely, row two of the table disappears and the
structural signal loses most of its value. That is a provider decision, outside this
project's control, and it should be watched rather than assumed away.

---

[ADR index](README.md) · [Routing baseline](../evidence/routing-baseline-2026-09-06.md)
