# Keepalive is the wrong lever for a coding-agent workload

**Date:** 2026-09-10
**Tier:** estimated (transcripts only; no proxy involved)
**Corpus:** 60 main-lane transcripts under `~/.claude/projects`, 26,675 consecutive
assistant-turn pairs carrying a prefix, 120.7M observed `cache_creation_input_tokens`.

## Why this was measured

Two 2026 papers bear directly on what Replay recommends.

[*Keeping the Cache Warm Pays: Keepalive Economics for Agentic
Workloads*](https://arxiv.org/html/2607.19214) derives a break-even horizon for
holding a prompt cache open with periodic pings:

> I_max ≈ τ(w/r − 1)

with τ the ping interval, `w` the re-prefill multiple and `r` the cached-read
multiple. For Anthropic it reports r=0.10, w=1.25, τ≈4 min, giving a paying band
of roughly **46 minutes**.

[*Don't Break the Cache: An Evaluation of Prompt Caching for Long-Horizon
Agentic Tasks*](https://arxiv.org/html/2601.06007v2) compares three caching
strategies across three providers and reports **78.5%** cost savings on Claude
Sonnet 4.5 from excluding dynamic tool results from the cached prefix.

Both are claims about where agent cache spend goes. Replay has the corpus to
check which one describes this workload, so it was checked rather than assumed.

## Finding 1: the paper's band is stale for the newest models, and it is Replay's own table that says so

`replay rules --export` on the compiled table:

| model | `readMult` |
|---|---|
| `fable-5-1`, `mythos-5-1` | **0.025** |
| `opus-5`, `sonnet-5`, `haiku-4-5`, `fable-5`, `mythos-5` | 0.10 |

Substituting into the paper's own formula, with τ = 4 min:

| r | I_max |
|---|---|
| 0.10 (the paper's assumption) | 46 min |
| 0.025 (current top-tier models) | **196 min** |

The band is 4.3× wider than published, and nothing in the paper is wrong: the
constant moved underneath it. This is the same mechanism `replay doctor`'s
rules-staleness notice exists to warn about, arriving from the opposite
direction — a paper aging rather than a table.

## Finding 2: for this workload the band is nearly empty, and that is the decisive result

Every consecutive pair of assistant turns in one transcript, bucketed by the gap
between them, with the **observed** `cache_creation_input_tokens` on the second
priced at that model's real input price and the counterfactual saving taken as
`creation × input × (w − r)`:

| gap band | n | creation | recoverable by keepalive |
|---|---:|---:|---:|
| A — under 5 min (cache was alive) | 26,022 | 98.5M | **$508.83, and none of it** |
| B — 5 to 46 min (pays at r=0.10) | 598 | 2.1M | $11.65 |
| C — 46 to 196 min (pays only at r=0.025) | 40 | 11.4M | $65.59 |
| D — over 196 min | 15 | 8.7M | not recoverable at any r |

**Recoverable by keepalive: $77.24. Not recoverable: $508.83.**

87% of cache-creation spend on this corpus happens on gaps under five minutes —
the cache had not expired. Something rewrote the prefix while it was still warm.
No ping interval reaches that money, because there was no idle period to bridge.

Keepalive is a real mechanism answering a question this workload does not have.
It is the wrong lever here by roughly 7×.

## Finding 3: the money is in prefix stability, and the second paper says where

`replay blame` on the single largest session in the corpus, independently of the
paper:

```text
1. tool result: mcp__claude-in-chrome__javascript_tool   x407  176k once  77.93M in prompts
2. unaccounted: prefix grew where the transcript cannot see it  x132  132k once  69.77M
3. tool result: mcp__claude-in-chrome__computer          x317  127k once  60.58M in prompts
4. tool call:   mcp__claude-in-chrome__javascript_tool   x407  127k once  60.12M in prompts
```

Ranks 1, 3 and 4 are tool traffic. That is *Don't Break the Cache*'s finding,
reproduced on an unrelated corpus by a different method: theirs by A/B-ing cache
strategies against three providers, this one by attributing carried prompt
tokens in transcripts nobody wrote for the purpose.

The two results agree on the mechanism and disagree with the first paper about
where to spend engineering effort.

## What this changes

- **Do not build keepalive.** $77 against $509 on this corpus, and the larger
  half of the $77 depends on a model-specific constant rather than on any ping.
- **The prefix-stability work is the one that pays**, which is what `replay diff`
  already classifies and what `learn` already selects a policy against.
- Rank 2 is worth naming: **69.77M prompt tokens the transcript cannot attribute
  at all**. That is the largest single unknown in this measurement and it is
  larger than the entire keepalive opportunity.

## What this does not establish

- One machine, one operator, one agent client. There is no population here and
  no claim about anyone else's workload; a corpus of one is what
  `--contribute` exists to fix and the pool has one member.
- 60 of 124 transcripts, main lanes only. Sub-agent lanes are excluded, so
  cross-lane interleaving is not modelled and the gap between two turns of one
  lane is not the gap the provider saw.
- A `cache_creation` under a five-minute gap is attributed to a prefix change.
  It could also be a 1-hour-TTL write, or a sibling request extending the prefix
  — a case `replay` already reports separately ("58 read more than predicted: a
  sibling request extended the prefix"). Band A is therefore an upper bound on
  prefix churn, and the conclusion survives it: even if half of band A were
  mis-attributed, it would still exceed the keepalive opportunity by 3×.
- Prices are list, from the compiled table dated 2026-09-07, and `w = 1.25` is
  the 5-minute write multiple taken from the paper rather than measured here.
- The counterfactual is arithmetic, not an experiment. Nothing was re-run with a
  cache held open; ADR-0002's estimated tier applies.
