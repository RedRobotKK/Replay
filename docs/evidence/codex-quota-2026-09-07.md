# A quota counter that moves, 2026-09-07

**What this measures:** whether any coding agent reports a live consumption
signal a client can act on, what unit it counts in, and what a broken cache
costs against it.

## Summary

**Codex writes a rate-limit reading to local disk on every turn, it moves, and
cached reads weigh far less against it than uncached ones. The exact weighting
is not determined by this corpus, and an earlier draft of this file claimed it
was.**

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

## Finding 2: the counter tracks work, and the weighting is not determined here

Attributing every rise in the primary counter to the tokens spent since the
previous reading gives 166 percentage points against a known token mix. Four
weightings fit that equally well, and this corpus cannot separate them, because
nothing here observes the true window size independently.

| Weighting | Tokens per 1% | Implied five-hour window |
|---|---|---|
| uncached input + output | 197,693 | 19,769,258 |
| uncached + 0.10 x cached + output | 488,965 | 48,896,514 |
| uncached + 0.25 x cached + output | 925,874 | 92,587,398 |
| every token | 3,110,418 | 311,041,817 |

**An earlier draft of this file published the first row as the answer. That was
wrong, and it was wrong in the direction that flatters the finding.** Public
reports put a cached read at roughly a tenth of an uncached one for *price*, and
price is not the same quantity as quota weight, so even the second row is an
inference rather than a measurement.

What the corpus does establish is the direction and its size. Of the
516,531,547 tokens in the attributed window, **483,512,448 were cached reads**.
Whatever weight those carry, it is far below one: at full weight the observed
mix would imply a 311M token window on a consumer plan, which is not credible.
So the cache does most of the work of keeping a session inside its window.
How much is unresolved, and this file does not pretend otherwise.

Settling it needs one controlled run: a session with a known uncached volume,
started against a known counter value, on an account whose window size is
otherwise idle. That has not been done.

## Finding 3: what the breaks cost in this currency

[The cache breaks measured on this same corpus](codex-cache-breaks-2026-09-07.md)
re-read 10,635,679 tokens cold. Cold tokens count against quota at full weight:

```text
10,635,679 / 197,693 = 53.8    if quota counts uncached work only
10,635,679 / 488,965 = 21.8    if a cached read weighs a tenth
```

**Cache breaks alone cost between roughly a fifth and a half of a five-hour
window**, depending on a weighting this corpus cannot pin down. Those tokens are
spread across a week rather than one window, so it is a size comparison and not
a claim about any single window.

The phenomenon is reported independently. [openai/codex#4764](https://github.com/openai/codex/issues/4764),
"cache miss can cause higher usage towards limit than expected", records a user
going from 91% to 100% of a five-hour limit on a single prompt, and asks for
exactly the visibility this file is about: "Restore token information. I want to
see exactly how many tokens are and were used."

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
