# A quota counter that moves, 2026-09-07

**What this measures:** whether any coding agent reports a live consumption
signal a client can act on, what unit it counts in, and what a broken cache
costs against it.

## Summary

**Codex writes a rate-limit reading to local disk on every turn, it counts
uncached work rather than total tokens, and the cache breaks measured on the
same corpus cost the equivalent of 54% of a five-hour window.**

This is the second currency [the money path](../MONEY-PATH.md) concluded did not
exist. That conclusion rested on
[quota titration](quota-titration-2026-09-06.md), which pushed 3.09M tokens
through the Anthropic surface and moved the utilisation counter by **zero
steps**, so a subscription seat had nothing to recover and no budget to bill
against. On Codex, 3.11M tokens moves the primary counter by **one percent**.
Nearly identical volume. One meter registers it and the other does not.

| | |
|---|---|
| Where | `~/.codex` rollout logs, on every `token_count` event |
| Samples | 6,871 across 148 sessions, 2026-03-20 to 2026-03-27 |
| Windows | primary 300 minutes, secondary 10,080 minutes, each with `resets_at` |
| Observed range | primary 0 to 52%, secondary 0 to **89%** |
| Movement | 304 rises, 59 resets. It is not frozen |

## Finding 1: it is the first one that moves

Two prior surfaces were checked and neither carries a usable signal. Anthropic
reports a rising utilization fraction on the wire only, and the titration above
could not move it. The Grok CLI advertises four `x-ratelimit-*` headers whose
`remaining` sat at exactly `limit` across 8 model calls and roughly 940KB of
responses ([wire families](wire-families-2026-09-06.md)); a `remaining` that
always equals the limit is the same shape as a healthcheck that cannot fail.

Codex reports:

```json
{"limit_id":"codex","plan_type":"plus",
 "primary":  {"used_percent":18.0,"window_minutes":300,  "resets_at":1774130675},
 "secondary":{"used_percent":53.0,"window_minutes":10080,"resets_at":1774296354}}
```

It needs no proxy. The client writes it to disk itself, which means it is
readable on the same offline path Replay already uses for transcripts.

## Finding 2: the unit is uncached work, not tokens

Attributing every rise in the primary counter to the tokens spent since the
previous reading:

| Basis | Tokens per 1% | Implied five-hour window |
|---|---|---|
| Total tokens | 3,111,636 | 311,163,583 |
| **Uncached input + output** | **197,693** | **19,769,258** |

A 311M token window on a consumer plan is not credible; a 20M one is. Cached
reads are close to free against quota, in the same way and for the same reason
that they are close to free against price.

The consequence is larger than the arithmetic. At the corpus's cache hit rate,
516,531,547 tokens of work consumed 32,816,969 of quota. Had the cache not held,
identical output would have cost the full 516M, **15.7 times more quota for the
same work**. On a metered account a broken cache is a bill. On a subscription it
is the difference between working and being rate-limited.

## Finding 3: what the breaks cost in this currency

[The cache breaks measured on this same corpus](codex-cache-breaks-2026-09-07.md)
re-read 10,635,679 tokens cold. Cold tokens count against quota at full weight:

```text
10,635,679 / 197,693 = 53.8
```

**Cache breaks alone burned the equivalent of 54% of a five-hour window.** Those
tokens are spread across a week rather than one window, so this is a size
comparison and not a claim that any single window was half consumed by breaks.

## Scope, honestly

One account, one `plan_type` of `plus`, one week, one machine, 6,871 samples.

**Only `gpt-5.1-codex-mini` produced attributable rises.** Sessions on `gpt-5.4`
exist in the corpus and none of them coincided with a counter move, so this file
says **nothing** about whether models weigh differently against quota. That is
the obvious next measurement and it is not made here.

`used_percent` is reported as a whole number, so every reading carries plus or
minus one percent of quantisation, which on a 20M window is plus or minus
198,000 tokens. The window size is therefore written as approximately 20M rather
than as a figure, and a second account would be needed before it could be called
a plan property rather than an observation.

Reproduce with `replay codex`, which prints the latest reading for both windows.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
