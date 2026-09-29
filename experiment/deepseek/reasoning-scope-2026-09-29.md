# Where `reasoning_effort: "none"` applies, and where it destroys the answer

2026-09-29. `deepseek-flash`, real Replay source as the corpus (48,167 chars,
`internal/advisor/advisor.go` + `internal/transcript/wire.go`), 84 calls,
$0.0418 DERIVED. Ground truth computed from the source by the harness, never
authored by hand. Predictions registered in
`reasoning-scope-prereg-2026-09-29.md` before the run.

## Result

Two task classes, 3 repetitions each, `max_tokens=4096`. Zero truncated, zero
undecided.

| cell | pass | rate | output tokens | reasoning |
| --- | ---: | ---: | ---: | ---: |
| **M** mechanical, reasoning ON | 18/18 | 100% | 1,075 | 1,003 |
| **M** mechanical, reasoning OFF | 17/18 | **94%** | 55 | 0 |
| **R** aggregative, reasoning ON | 24/24 | 100% | 22,337 | 22,280 |
| **R** aggregative, reasoning OFF | 7/24 | **29%** | 2,328 | 0 |

The prediction holds. The lever is **class-conditional**: on a lookup it is
nearly free and cuts output 19.5x; on an aggregation it cuts output 9.6x and
takes the answer with it.

DS-OPT's 12/12 at `reasoning_effort: "none"` was a lookup task. Read as a
general recommendation it would have been the unconditional prefix-ordering
claim over again.

## It does not degrade gracefully

The aggregative failures are not near-misses or refusals. They are confident
wrong numbers:

| question | truth | answered |
| --- | ---: | ---: |
| count lines beginning `func ` | 39 | **67** |
| count exported functions | 17 | **22** |
| count lines beginning `type ` | 14 | **1** |
| count `===== FILE:` markers | 2 | **6** |

The single mechanical failure is the same shape: asked for the value of
`KindHotFile`, it returned `tool-inputs`, which is the **first** constant in that
block rather than the named one. Plausible, wrong, and carrying no signal that
it is wrong.

This is what makes the lever dangerous rather than merely limited. A degradation
that announced itself could be caught at runtime. This one cannot be
distinguished from a correct answer without independently computing the answer.

## The economics, including the awkward part

Output cost per correct answer:

| cell | output cost | per correct |
| --- | ---: | ---: |
| M on | $0.001290 | $0.000072 |
| M off | $0.000066 | **$0.000004** |
| R on | $0.026804 | $0.001117 |
| R off | $0.002794 | **$0.000399** |

Reasoning-off is cheaper per correct answer **even on the class where it is
wrong 71% of the time**, by 2.8x. That is a real number and it is mostly a trap:
it is only exploitable when an independent checker can say which answers are the
correct ones. Where that checker exists and is cheap, sampling a cheap model
several times and filtering beats one expensive call. Where the checker is as
expensive as the task -- which is exactly the case for counting, the class that
failed -- the route is closed, and the $0.000399 buys answers you cannot
identify.

## The rule

- **Lookup, extraction, single-location retrieval** -> `reasoning_effort: "none"`.
  19.5x fewer output tokens, no measurable accuracy cost at n=18.
- **Counting, aggregation, multi-hop, anything whose answer is not written down
  anywhere in the input** -> leave reasoning on. 29% is not a discount.
- **Where a cheap independent checker exists** -> reasoning-off plus retry may
  still win. Measure it per workload; do not assume it.

## An instrument failure, recorded

The first run of this experiment scored M-ON at 0/0 with 12 truncations and
M-OFF at 0/12. Neither was a model result. The mechanical questions asked which
function was declared on a given **line number** of a 48,000-character document,
which is a positional counting task wearing a lookup's label, and `max_tokens`
was 1024, low enough that reasoning-on never reached an answer.

Both were fixed before the run above: the mechanical class was rebuilt from real
typed string constants, which are pure locate-and-copy, and the output budget was
raised to 4096. The void cell is recorded rather than deleted because the
mislabelling is the interesting part -- a question class is an assumption, and
this one was wrong.

## Limits

One model, one corpus, one repetition count. n=18 and n=24 per cell. The 94%
mechanical figure rests on a single failure and should not be quoted as a 6-point
gap from 100%; what it supports is "no large cost on lookups", not a point
estimate.
