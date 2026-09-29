# DeepSeek prefix cache, characterised

2026-09-29. 49 calls, $0.0135 DERIVED, 0 errors, model `deepseek-flash`,
`/v1/chat/completions` unless stated. PEAK pricing regime throughout.

Balance was $46.01 before and $46.01 after. The spend is an order of magnitude
below the balance endpoint's $0.01 resolution, so **none of this is reconciled
against money**. Every cost here is DERIVED.

## The block model

    cached_tokens = 128 * max(0, floor(shared_prefix_tokens / 128) - 1)

Two claims, both OBSERVED:

1. The cache is quantised to **128-token blocks**. Every cached figure in 17
   measurements was an exact multiple of 128.
2. The **final block is never served from cache**. A shared prefix must span at
   at least two blocks, **256 tokens**, before any hit occurs.

### Evidence

Twelve rungs fitted, then five predicted in advance and confirmed:

| stage | n | exact | note |
| --- | ---: | ---: | --- |
| P2 fit | 7 | 7/7 | model derived here, so not evidence by itself |
| falsification, preregistered | 5 | 4/5 | the miss was an estimate, see below |
| fully observed | 5 | 5/5 | shared prefix is the cold call's own reported token count |

The preregistered round predicted 4608 at the 4096-word rung and observed 4480.
The block model was not what failed: the prediction needed a word-to-token
estimate, that estimate was off by 111 tokens, and 111 tokens is enough to move
the block floor by one. Refitting all twelve rungs against **observed** prompt
tokens gives 12/12.

Because correcting a prediction after seeing the result is how a model gets
rescued rather than tested, the final round removed the estimate entirely: the
warm prompt is a strict extension of the cold prompt, so the shared prefix is
exactly the cold call's own reported token count, and each prediction was
printed before its warm call was issued. 5/5.

## What busts the cache, and what does not

| change | cached | conclusion |
| --- | ---: | --- |
| identical prefix (control) | 3,200 | |
| **one leading space** | **0** | matching is byte-exact, not semantic |
| `temperature` 0 -> 0.9 | 3,200 | sampling parameters are not part of the key |
| `max_tokens` 16 -> 64 | 3,200 | output cap is not part of the key |
| prefix truncated 40 chars at the END | 3,200 | only the leading bytes matter |

A single leading space is the whole finding: anything that prepends to a prompt
-- a timestamp, a session id, a varying system preamble -- destroys the cache
for every request that follows it. Put variable content at the END.

## Availability is immediate

Populate, then query at +0.0s, +0.5s, +2.0s: 3,200 cached at every delay. There
is no warm-up window to wait out, which is what makes warm-then-fan correct:
issue one call, await it, fan out the rest with no sleep.

## The cache is shared across dialects

A prefix populated through `/v1/chat/completions` was served to
`/anthropic/v1/messages` at 3,200 cached tokens on what was that endpoint's
first call. The cache key is the prefix, not the endpoint. A mixed-dialect
workload shares one cache.

## What this corrects in the harness

`fanout.MIN_WARM_PREFIX_CHARS` was 2000, a guess. The real boundary is 256
tokens. The guess sat near the right answer for English prose and would have
been wrong by roughly 2.6x on code, which is the workload Replay actually cares
about. It is now derived from the measured block size, biased toward warming,
because a needless warm costs one call of latency and a missed warm costs full
input price on the shared prefix for every question in the fan-out.

## Not measured

- **Cache TTL.** Documented as "hours to days"; nothing here ran long enough.
- **Cache-hit pricing.** Still unvalidated against money. Every run so far is
  below the balance endpoint's resolution floor. A batch large enough to clear
  $0.01 with a substantial hit share is the experiment that would settle it.
- **`deepseek-v4-pro`.** All of this is `deepseek-flash`. Block size is likely
  an infrastructure property rather than a model one, which makes it a cheap
  check and an untested assumption until it is run.
