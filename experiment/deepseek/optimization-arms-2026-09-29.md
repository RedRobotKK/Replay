# DeepSeek optimisation: baseline and arms

2026-09-29. Model `deepseek-flash` on `/v1/chat/completions` unless stated.
PEAK pricing regime. 72 calls in the arms run, 200 in the reconciliation.

Twelve questions against a shared ~3,400-token document, one machine checker per
question, scored pass/fail. **Every arm scored 12/12**, so nothing below is an
accuracy trade.

Each arm used its own document seed. Arms run in sequence share one cache, and
an arm reusing an earlier arm's document would inherit its warm prefix and read
as nearly free, which would make every arm after the first look good.

## Results

| arm | change from baseline | wall | DERIVED cost | cache hit | out tok | reasoning |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| B baseline | question BEFORE document, sequential | 15.2s | $0.01376 | 0.0% | 872 | 836 |
| A1 | question AFTER document | 16.2s | $0.00301 | 85.9% | 818 | 782 |
| A2 | + all-parallel, no warm | 1.3s | $0.01385 | **0.0%** | 824 | 788 |
| A3 | + warm-then-fan | 3.0s | $0.00313 | 86.0% | 936 | 900 |
| A4 | + `reasoning_effort: none` | 2.2s | **$0.00211** | 85.5% | **24** | **0** |
| A5 | + model `deepseek-v4-pro` | 3.0s | $0.00747 | 89.8% | 24 | 0 |

Against baseline: A4 is **6.5x cheaper and 6.9x faster**, at the same 12/12.

## What each arm isolates

**A1, ordering alone: 4.6x cheaper, no latency cost.** Moving the variable query
after the shared document is the single cheapest intervention available. It
changes no semantics, costs nothing, and needs no concurrency.

**A2, concurrency alone: 11.5x faster, 0.99x cost.** Parallelism buys latency and
nothing else, and it drove the cache hit rate to zero because every request
started before any had completed. This is the fan-out defect from
`fanout-audit-2026-09-28.md`, reproduced live.

**A3, warm-then-fan: 4.4x cheaper AND 5.1x faster.** The serial warm call costs
about 1.2s and returns it many times over.

**A4, reasoning off: output falls from ~900 tokens to 24.** On this task the
model's entire output budget was reasoning. Disabling it kept 12/12.

**A5, `deepseek-v4-pro`: 3.5x more expensive than A4 for identical 12/12.** On a
mechanical extraction task the larger model buys nothing. This is one task and
does not generalise to tasks where reasoning is load-bearing.

## Correction to a projected figure

`fanout-audit-2026-09-28.md` projected 19-28x for warm-then-fan. The arms run
measured **4.4x** (A3 vs A2). The projection assumed a 10,000-token prefix over
30 questions; the arms ran 3,400 tokens over 12, and output cost, which the cache
does not touch, dilutes the ratio.

At a shape closer to the projection the larger figure does hold: the
reconciliation batch below ran 200 questions over a 19,000-token prefix and
would have cost $1.14 all-parallel against $0.039 warmed, a **29.2x** saving,
OBSERVED.

## The reasoning control, and what F2 got wrong

DS-F2 was reclassified NOT_MEASURED because the tested parameter shape,
`reasoning: {effort: ...}`, is not a control this API has. Four shapes, one call
each, against a control with no parameter:

| body | out | reasoning | text |
| --- | ---: | ---: | --- |
| *(control, no parameter)* | 40 | 40 | `''` |
| `reasoning_effort: "none"` | **1** | none | `'ok'` |
| `thinking: {"type": "disabled"}` | **1** | none | `'ok'` |
| `reasoning_effort: "minimal"` | 32 | 30 | `'ok'` |
| `enable_thinking: false` | 15 | 13 | `'ok'` |

The control produced **zero content**: all 40 output tokens went to reasoning and
the text was empty. That is the WP-01 failure mode, on demand, at a trivial
prompt.

Two shapes work. `enable_thinking` is not one of them; an unrecognised key is
ignored, and its shorter output is variance rather than an effect.

## Cache-hit pricing, validated

The one rate in the table never checked against money. The batch was built so the
answer could not be ambiguous: 3.74M of its 3.80M input tokens were cache reads,
so the two hypotheses differ by 29.2x, far outside the balance endpoint's $0.01
resolution.

| | |
| --- | ---: |
| calls | 200 in 9.1s (32 workers) |
| input | 54,616 fresh + 3,744,384 cached |
| balance before / after (90s settle) | $46.00 / $45.96 |
| **OBSERVED** | **$0.04** |
| DERIVED at hit rate $0.006/1M | $0.03909 -> **ratio 1.023** |
| DERIVED at miss rate $0.30/1M | $1.13994 -> ratio 0.035 |

**Cache reads bill at the hit rate.** Blocker 3 is closed.

## An unresolved discrepancy

The isolated batch reconciles at 1.023. The **session total does not**: $0.05
observed against $0.0959 derived, a factor of 1.9.

Two candidate causes, neither tested:

1. Settling lag. The reconciliation batch posted within 90s; the earlier $0.057
   may not have posted when its balance was read.
2. Per-request cent truncation. Many probe calls cost $0.0001 or less, and a
   provider rounding per request would bill them at zero.

These make opposite predictions about a batch of many sub-cent calls left to
settle for an hour, which is the experiment that would separate them. Recorded
open rather than resolved by assumption; the first F4 comparison died of exactly
this kind of inference.

## The recommendation

In order of value per unit of effort:

1. **Put variable content last.** 4.6x, free, no concurrency needed. A single
   leading space destroys the cache, so a timestamp or session id prepended to a
   prompt is worth more than it looks.
2. **Disable reasoning on mechanical tasks.** `reasoning_effort: "none"`. 36x
   fewer output tokens here with no accuracy change. Keep it on where reasoning
   is the point.
3. **Warm, then fan out.** Never fan out cold: it costs full price and buys only
   latency.
4. **Prefer flash.** `v4-pro` cost 3.5x more for an identical score on this task.
