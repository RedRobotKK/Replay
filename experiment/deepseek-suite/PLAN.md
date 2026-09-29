# Preregistration: DeepSeek cache mechanism suite (Track A)

Written before execution. Git commit `30dc687`. Balance at writing `$45.78`
OBSERVED. Campaign ceiling `$3.00`, spent to date `$0.23` OBSERVED.

Every prediction below is recorded now so the result is a test rather than a
description. Where a prior campaign already measured something, the prediction
is that prior result, and a disagreement is a finding about replication.

## The competing cache formulas

Two hypotheses about how many tokens of a shared prefix are served from cache,
as a function of the shared prefix length `n` in provider-reported tokens.

| | formula |
| --- | --- |
| **H-A** | `cached = 128 * floor(n / 128)` |
| **H-B** | `cached = 128 * max(0, floor(n / 128) - 1)` |

H-B is the formula a prior run fitted to 17 rungs. H-A is the operator's
proposed alternative. **They disagree at every n >= 128**, which makes the
boundary series a clean discriminator rather than a confirmation exercise.

At `n = 256`: H-A predicts 256 cached, H-B predicts 128. At `n = 200`: H-A
predicts 128, H-B predicts 0. A single rung at either point separates them.

**H-D** was added before A2 ran, after a prior-art sweep found a published
vendor figure the campaign had missed. DeepSeek's context-caching announcement
of **2 August 2024** states verbatim: "The cache system uses 64 tokens as a
storage unit; content less than 64 tokens will not be cached."

| | formula |
| --- | --- |
| **H-D1** | `cached = 64 * floor(n / 64)` |
| **H-D2** | `cached = 64 * max(0, floor(n / 64) - 1)` |

That page describes DeepSeek V2 and the MLA architecture and quotes a cache-hit
price of **$0.014 per million**, against the current flash rate of $0.006 peak.
It is a different model generation on a different price list, so it is a
hypothesis about today's behaviour rather than a specification of it.

A fourth possibility is registered explicitly: **H-C, none of these hold**,
evidenced by any cached count that is not a multiple of the candidate unit, or
by a rung whose cached count matches no prediction.

### H-D tested against existing evidence before any new spend

Fitting all four formulas to the 17 rungs already OBSERVED:

| formula | exact fit |
| --- | ---: |
| H-A `128*floor(n/128)` | 3/17 |
| **H-B `128*(floor(n/128)-1)`** | **17/17** |
| H-D1 `64*floor(n/64)` | 1/17 |
| H-D2 `64*(floor(n/64)-1)` | 3/17 |

Note that every observed cached count is a multiple of 64 as well as of 128,
because 128 is a multiple of 64. The multiples test alone therefore does NOT
separate the two unit sizes. The formulas do.

**A2 still runs.** This fit is retrospective against rungs chosen for a
different purpose, and H-D deserves a prospective test at boundaries chosen to
discriminate it. What the fit changes is the prediction: H-B, now against three
named alternatives instead of one.

## Method note that decides the whole track

Token counts will **not** be estimated from character counts. A prior
preregistration missed one rung for exactly that reason: the word-to-token
estimate was off by 111 tokens, enough to move the block floor by one, and the
model was blamed for an instrument error.

Instead the warm prompt is a **strict extension** of the cold prompt. The shared
prefix is then exactly the cold call's own `prompt_tokens`, which the provider
reports. Each prediction is computed from that OBSERVED count and printed before
the warm call is issued.

Rungs are therefore not forced to land on exact targets. Prefixes are generated
across a dense spread and the provider's reported count is taken as `n`.

---

## A1 — Prefix identity

**Question.** Is cache identity sensitive to byte-level changes?

**IV.** A single-character perturbation of an otherwise identical prefix.
**DV.** `prompt_cache_hit_tokens`.
**Controls.** Model, endpoint, parameters, prefix content, ordering.

**Arms.** exact identical (positive control) / one leading space / one trailing
space / one changed character mid-prefix / changed capitalization / changed
punctuation.

**Predictions.** Leading space: **0 cached** (prior observation). Changed
character mid-prefix: cached up to the block containing the change, so a change
late in a long prefix should still serve the earlier blocks. Trailing space:
**cached unchanged**, because the perturbation is at the end and the leading
bytes are intact. Capitalization and punctuation: same as a changed character.

**Falsifier.** A leading space that still serves cache refutes byte-exact
matching. A trailing-space change that zeroes the cache would mean matching is
whole-string rather than prefix-wise, which would refute the prefix model
itself.

## A2 — Prefix-length quantization

**Question.** H-A, H-B, or H-C?

**Procedure.** Approximately 20 rungs spread over `n` from ~100 to ~700 tokens,
dense around 128, 256, 384, 512 and 640. Cold call defines the prefix and
reports `n`; prediction from both formulas is printed; warm call observes.

**Prediction.** H-B, from the prior 17-rung fit. Registered as the prediction
rather than the conclusion.

**Falsifier.** Any cached count not a multiple of 128 refutes quantization
itself. Agreement with H-A at the boundary rungs refutes H-B.

## A3 — Minimum cacheable prefix

**Question.** What is the actual threshold below which no cache is served?

**Prediction.** 256 tokens under H-B, 128 under H-A. A2's boundary rungs answer
this directly, so A3 adds only rungs in `[120, 270]` if A2 is ambiguous there.
**No separate spend unless A2 leaves it open.**

## A4 — Warm-cache timing

**Question.** Is cache population completion-dependent?

**Arms.** (1) Sequential: A completes, then B with an identical prefix.
(2) Concurrent: B dispatched before A completes.

**Prediction.** Sequential B hits. Concurrent B misses.

**Falsifier.** A concurrent B that hits refutes completion-dependence and would
mean the prior 0% parallel hit rate had another cause.

**Confound to control.** "Before A completes" must be established from
timestamps and wall-clock, not from the dispatch order in the code. Dispatch
order is not evidence of arrival order.

## A5 — Fan-out behaviour

**Question.** Does parallelism trade cache efficiency for latency
systematically?

**Arms.** cold then sequential fan-out; cold then concurrent fan-out. Identical
prompts, identical prefix, own prefix per arm so neither inherits the other.

**Prediction.** Concurrent: near 0% hit, wall time far lower, aggregate cost
near the all-miss figure. Sequential: high hit rate, wall time roughly n times
one call.

**Falsifier.** Concurrent fan-out showing a substantial hit rate.

## A6 — Parameter invariance

**IV.** One request parameter at a time: `temperature`, `max_tokens`, `top_p`,
`frequency_penalty`, `presence_penalty`, `stop`.

**Prediction.** None affect cache identity. `temperature` and `max_tokens` are
prior observations; the rest are **NOT_OBSERVED** and genuinely open.

**Falsifier.** Any parameter whose change zeroes the hit.

**Labelling rule.** Any parameter not in the vendor documentation is marked
experimental in the result, regardless of what it does.

## A7 — Cross-endpoint cache

**Question.** Is the cache keyed on the prefix rather than the endpoint?

**Procedure.** Cold on endpoint A, warm on A (establishes A works), equivalent
prefix on endpoint B, then the reversed order with a fresh prefix.

**Prior status.** Observed ONCE, not replicated. Treated as **NOT_OBSERVED for
the general claim**.

**Prediction.** B hits on a prefix populated by A, in both directions.

**Confound that must be addressed.** The two dialects wrap the prefix in
different request bodies. "Equivalent prefix" means the same user-message text;
whether the serialised prompt seen by the model is byte-identical is
**ASSUMED, not established**. If B misses, that assumption is the first
suspect, not the cache model. This is recorded now so a miss is not
misread later.

## A8 — Cross-model cache

**Question.** Is cache state scoped to the account, the endpoint, or the model?

**Procedure.** Populate a prefix on `deepseek-flash`, request the identical
prefix on `deepseek-v4-pro`, and the reverse.

**Prediction.** No sharing. Model-scoped caching is the common implementation.

**Value.** This is the strongest falsification test in the track, because
account-global sharing and model-scoped caching make opposite predictions and
A7's result alone cannot distinguish them.

**Cost note.** `deepseek-v4-pro` is 4.4x the price of flash. Prefixes here are
kept small for that reason, and the arm is skipped if the budget has moved.

---

## Budget and stop conditions

**Estimate.** A1 ~14 calls, A2 ~40, A4 ~6, A5 ~26, A6 ~14, A7 ~8, A8 ~6.
Roughly 114 calls, mostly small prefixes. Estimated **under $0.05 DERIVED**.

**Stop immediately and record the reason if:** any cached count is not a
multiple of 128 (redesign A2 before continuing); a positive control fails;
actual spend exceeds 3x the estimate; an arm's research question is already
answered by an earlier arm; the provider returns anything other than HTTP 200 on
a control.

**No new hypothesis is folded into a running experiment.** Anything discovered
mid-run gets a new ID.
