# Ollama reports no cache, but it cannot hide one, 2026-09-09

**What this measures:** whether a prompt-cache hit is observable on the Ollama
surface at all, given that neither of its APIs reports cached tokens. Measured
on this machine against `qwen2.5-coder:7b`.

## The problem this exists to solve

Task 10 is "build `replay advise` for the Ollama surface", and the obstacle has
always been stated as: Ollama reports no cached tokens, so there is nothing to
analyse. That is true of the token counts and was verified again here rather
than assumed.

**OpenAI-compatible endpoint** (`/v1/chat/completions`), same request twice:

```json
{"prompt_tokens": 3229, "completion_tokens": 2, "total_tokens": 3231}
{"prompt_tokens": 3229, "completion_tokens": 2, "total_tokens": 3231}
```

No `prompt_tokens_details`, no `cached_tokens`, no difference between a cold
prompt and a warm one.

**Native endpoint** (`/api/chat`) is no better on counts: `prompt_eval_count` is
2823 on both. It reports the size of the prompt, not what was evaluated.

So a cache hit is invisible in every quantity either API reports. That is where
the question has previously stopped.

## The finding: the hit is in the duration

`prompt_eval_duration` is not invariant. Same prompt, three consecutive calls:

| Call | `prompt_eval_count` | `prompt_eval_duration` |
|---|---:|---:|
| 1 | 2823 | 12,118 ms |
| 2 | 2823 | 45 ms |
| 3 | 2823 | 43 ms |

**This is not model-load warm-up**, which is the obvious confound and the reason
the control below matters more than the table above. Alternating between two
prompts that differ only in their opening header:

| Request | Per-token prompt eval |
|---|---:|
| A (warm) | 45.2 us |
| B, header changed | **4178.0 us** |
| B again | 15.7 us |
| A again | 18.8 us |

Changing the prefix sends it straight back to cold and coming back to A finds it
still warm, so the model was loaded throughout. The signal is the cache.

Replicated, four independent prefixes, cold and warm measured back to back:

| | n | mean | sd | min | max |
|---|---:|---:|---:|---:|---:|
| cold | 4 | **4211.9 us/tok** | 23.8 | 4178.1 | 4232.8 |
| warm | 4 | **15.4 us/tok** | 1.5 | 14.5 | 17.6 |

**273x separation, and the extremes do not overlap**: the slowest warm reading is
17.6 us against the fastest cold at 4178.1. This is not a marginal discriminator
that needs a careful threshold. It is two populations 237x apart at their nearest
points.

## It is a measurement, not a boolean

If duration only said hit or miss it would be of limited use. It tracks *how
much* was re-evaluated. Holding total prompt length roughly constant and varying
how much of the prefix is shared with an already-warm request:

| Shared | `prompt_eval_count` | Duration | Implied uncached | Implied cached | Expected | Error |
|---:|---:|---:|---:|---:|---:|---:|
| 1.00 | 2824 | 43.9 ms | 10 | 99.6% | 100% | -0.4pp |
| 0.75 | 2624 | 2459.9 ms | 584 | 77.7% | 75% | +2.7pp |
| 0.50 | 2424 | 4337.0 ms | 1030 | 57.5% | 50% | +7.5pp |
| 0.25 | 2224 | 6372.9 ms | 1513 | 32.0% | 25% | +7.0pp |
| 0.00 | 1226 | 5047.6 ms | 1198 | 2.3% | 0% | +2.3pp |

So the quantity Ollama does not report can be recovered from one it does:

```text
cached_tokens ~= prompt_eval_count - (prompt_eval_duration / cold_rate)
```

where `cold_rate` is the per-token cost of evaluating an uncached prompt, which
is a property of the model and the machine and has to be calibrated rather than
assumed.

**The error column is the honest part.** Recovery is within 8 percentage points
across the range, biased positive in the middle. Some of that is likely the
*expected* column rather than the estimator: the unique filler used to hold the
prompt length constant does not tokenize at the same rate as the shared unit, so
the true shared-token fraction is not exactly the unit fraction quoted. The
endpoints, where that confound is smallest, agree to 0.4 and 2.3pp.

## What this implies for `replay advise` on Ollama

The surface is analysable, and the shape of the analysis differs from the
metered providers in one way worth stating plainly: **there is no bill.** A local
break costs wall-clock and GPU, not money, so the unit to report is seconds.

On these numbers a fully broken 2,823-token prefix costs **11.9 seconds** of
recomputation against 43 ms warm. For an agent loop that re-lays its prefix every
turn, that is the whole latency budget.

`cold_rate` is the one calibration this needs, and it is exactly the shape of
measurement `replay probe` already performs for cache floors: cheap, local, free
of provider cost, and a per-machine constant rather than a published one.

## What this does not establish

1. **One model, one machine, one quantization.** `qwen2.5-coder:7b` on Apple
   Silicon. `cold_rate` will differ by model size, quantization and hardware, and
   nothing here bounds that variation.
2. ~~**Nothing under concurrency.**~~ **Resolved the same day — see below.**
3. **Nothing about eviction.** A stayed warm after B, so this Ollama holds more
   than one prefix, but the slot count, eviction order and capacity were not
   measured.
4. **No claim that the recovered figure is accurate enough to price with.** It is
   accurate enough to rank what to cut, which is what `advise` needs, and it
   should be reported with its band rather than as a token count.
5. **It does not close READINESS failure mode 1.** That needs an OpenAI-compatible
   provider that *reports* cached tokens, to exercise `usage.FromInclusive`.
   Ollama reports none, as re-verified above, so no amount of local work
   substitutes for one paid call to a provider like DeepSeek.

## Concurrency, which was the largest untested assumption

The limit above was written first and tested afterwards, because a duration-based
estimator is exactly the kind concurrency should distort: parallel requests share
one GPU, so a request that waits ought to look slower than one that does not.

It does not distort, and the reason is more useful than the result.

**Four warm concurrently, then four cold concurrently:**

| | mean | serial baseline |
|---|---:|---:|
| warm | 15.1 us/tok | 15.4 |
| cold | 4054.5 us/tok | 4211.9 |

269x separation against 273x serial, and no overlap between the extremes.

**Six fired together, alternating warm and cold** — the realistic shape, since an
agent fleet does not politely batch by cache state:

| Kind | Tokens | Duration | Per token |
|---|---:|---:|---:|
| warm | 2824 | 40.9 ms | 14.5 us |
| cold | 2841 | 11515.8 ms | 4053.4 us |
| warm | 2824 | 41.1 ms | 14.5 us |
| cold | 2841 | 11510.3 ms | 4051.5 us |
| warm | 2824 | 40.4 ms | 14.3 us |
| cold | 2841 | 11544.6 ms | 4063.6 us |

Still no overlap.

**Why it survives:** `prompt_eval_duration` reports the evaluation work for that
request, not the elapsed time the caller waited. In the four-cold run the wall
clock per request spanned **11.6 s to 46.6 s** as requests queued behind one
another, while `prompt_eval_duration` stayed inside **11,511 to 11,545 ms** — a
0.3% spread across a 4x spread in wall clock.

That is the property the estimator needs, and it is worth stating as the
load-bearing assumption rather than a happy accident: **an estimator built on
wall-clock latency would have been destroyed by queueing.** One built on
`prompt_eval_duration` is not, because the provider has already separated the two.

Untested beyond four-way concurrency on one machine, and a server configured with
more parallel slots than the GPU can hold may behave differently.

## Method

Ollama on `127.0.0.1:11434`, `qwen2.5-coder:7b`, `temperature: 0`,
`num_predict: 3`, `stream: false`. Prompts built from a repeated unit string with
a varying header. Durations are `prompt_eval_duration` as reported by
`/api/chat`, converted from nanoseconds. Cold readings are the first sight of a
given prefix; warm readings are an immediate repeat of the identical request. No
provider requests and no cost.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
