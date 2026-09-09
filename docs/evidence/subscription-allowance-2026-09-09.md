# What a cache break costs when you are not billed per token, 2026-09-09

**What this measures:** whether re-billed tokens consume a Claude subscription's
usage allowance, and roughly how much of one this machine's corpus has spent on
them. Derived from an existing trial rather than newly measured, and the
extrapolation is the weakest part — it is stated in full below rather than
buried.

## Why this document exists

Every dollar figure Replay prints is list price. On a Pro or Max seat that is
list price *for somebody else*: the operator pays a flat fee and is not billed
per token, so `$159.13 avoidable` is not money anyone lost.

`replay cost` already says so, unprompted, and the wording is right:

> On a subscription seat - Claude Pro or Max, Copilot, Cursor - none of that is
> money: you are not billed per token, so the dollars above are list price for
> someone who is. The tokens are still yours. They are context the work did not
> get, and rate-limit budget spent on nothing.

The last clause is the claim this document tests. "Rate-limit budget spent on
nothing" is either a real cost in a second currency, or a figure of speech. It
turns out to be real, and to be the more relevant of the two costs for anyone on
a subscription — which is most people who would run this tool.

## The measurement it rests on

Not new. `internal/analysis/predictor.go` records a 30-lane randomized trial run
2026-09-06, whose purpose was to decide whether provider rate-limit headers could
drive a pre-flight policy:

| Quantity | Value |
|---|---|
| Responses carrying `anthropic-ratelimit-unified-*` | 142 |
| Re-billed tokens observed | 3,778,706 |
| 5h utilization moved | 0.10 -> 0.15 |
| Header resolution | 0.01 |
| Steps resolved | 5, each spanning ~28 requests across both arms |

Its conclusion for that purpose was negative and is worth carrying: **a single
140,000-token full-prefix re-lay does not move the counter by one tick**, so the
header "can be a denominator and never a trigger." Nothing below contradicts
that. A cache break is invisible individually and only legible in aggregate.

## The finding

Re-billed tokens consume the 5h allowance. On this machine, from
`replay cost --json` on 2026-09-09:

| Quantity | Value |
|---|---:|
| Avoidable (re-billed) tokens | 33,778,322 |
| Avoidable share of total | 4.89% |
| List-price equivalent | $159.13 |
| Requests priced | 34,321 |

Scaling the trial's observed rate:

    33,778,322 / 3,778,706 = 8.94x the measured range
    8.94 x 0.05            = 0.45 of one 5-hour window

**Roughly 0.45 of a full 5-hour Max window, spent re-sending content that had
already been sent.** In the currency a subscriber actually pays in, that is the
cost of the cache breaks in this corpus.

## What this figure cannot support

Listed explicitly, because the number is quotable and the limits are not obvious.

1. **It is an 8.94x extrapolation.** The trial observed 3.78M re-billed tokens;
   this applies its rate to 33.8M. Linearity across that range is assumed and was
   not tested. If consumption is sub-linear — plausible if the provider discounts
   cache reads against the allowance the way it discounts them in dollars — the
   true figure is lower and possibly much lower.
2. **The step itself is coarse.** Five steps at 0.01 resolution is a ±0.01 read
   on a 0.05 total, or ±20% before any other error. At 0.04 and 0.06 the answer
   is 0.36 and 0.54 windows. Quote the range, not 0.45 alone.
3. **It is cumulative, not an event.** The 33.8M spans 1,690 transcripts over
   months. This is "0.45 windows' worth of allowance across the whole corpus",
   never "half a window was lost on a given afternoon". Per `predictor.go`, no
   single break is even visible at the header's resolution.
4. **One machine, one account, one tier.** All trial readings were
   `serviceTier: standard`. Nothing here generalises to Pro, to Team, or to a
   different geography.
5. **It does not establish the provider's accounting rule.** Observing that
   re-billed tokens move the counter is not the same as knowing the weight cache
   reads carry against the allowance. Whether the 0.10x dollar discount has an
   allowance analogue is **not measured**, and it is the single question that
   would most change this figure.
6. **n = 142 responses carried the header at all.** Most responses did not.

## What would settle it

The cheapest decisive measurement is a paired run: the same prompt issued twice
at a controlled prefix size, once warm and once deliberately cache-broken, with
`anthropic-ratelimit-unified-*` captured on both, repeated enough times to
resolve the difference above the 0.01 granularity. That directly measures the
allowance weight of a cache read against an uncached token, which is limit 5 and
the parameter everything else here is sensitive to.

`internal/proxy/quota.go` already parses these headers, so the capture path
exists. What does not exist is the paired trial.

## Method and provenance

Trial figures are quoted from the header comment of
`internal/analysis/predictor.go`, recorded 2026-09-06. Corpus figures are from
`replay cost --json` on 2026-09-09 over `~/.claude/projects`, 1,690 transcripts,
34,321 priced requests, list prices dated 2026-06-24. The arithmetic linking them
is the two lines shown above and nothing further. **No new provider requests were
made for this document.**

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
