# What is the routing baseline, and what does the corpus let us say about it?

**Measured 2026-09-06 against a frozen copy of one machine's transcripts. Read the
sample sizes first: they are different for different halves of this document, and
that difference is the finding.**

## Sample size, before any finding

Two corpora sit behind `replay route`, and "blocked on n=1" has been used for both.
Only one of them is n=1.

| Corpus | What it is | n |
|---|---|---:|
| Transcripts | `~/.claude/projects`, snapshotted at 2026-09-06T22:24:52Z | **1,614 files, 1,606 parsed, 115 sessions, 12 projects** |
| Probe readings | `~/.replay/measurements.jsonl` | **4 lines: exactly 1 reading per model, 4 models** |
| Operators and machines | both corpora | **1** |

The transcript corpus supports measurement. The probe series does not support an
error bar of any kind: one reading per model is zero within-model variance, and all
four readings were taken inside a 35 second window (`takenAt` 01:57:51Z to
01:58:26Z on 2026-09-06), so they are not even four independent occasions.

Every finding below is labelled with which corpus it rests on.

## Method

`replay route <dir> --to <model>` was run over a frozen snapshot rather than over
the live directory, because the live directory grows while it is being read: six
consecutive runs against `~/.claude/projects` returned fitted turn counts of 17,524
through 17,534 for the same model, drifting under this session's own writing. The
snapshot was made with `rsync -a` filtered to `*.jsonl`, was read-only for the
duration, and every figure in this document comes from it. Nothing under
`~/.claude` or `~/.replay` was modified.

Commands, each named beside the figure it produced:

- `replay route <snapshot> --to <model> --json`, run once per candidate destination,
  for topology, sigma and projections.
- `replay corpus <snapshot>`, for session and transcript counts, match rates and
  break causes.
- `replay cost <snapshot> --json`, for the corpus total at list price.
- `replay probe --model <id>` (plan only, nothing sent), for the probe budget.
- Direct reads of `internal/analysis/route.go`, `cmd/replay/route.go` and
  `internal/cachemodel/anthropic.go` for the rate card and the estimator.

One figure was not produced by a replay command and is labelled as such: the
token-level cached share, summed from the `usage` object of every assistant message
in the snapshot, deduplicated on `message.id` because 57,751 of 105,532 usage rows
(54.7%) are repeats of a request already counted in another transcript.

## Scope

One operator, one machine, one account, one client family, first-party API list
prices, service tier `standard`. Nothing here is a claim about any other account,
tier, or geography. The four probe readings record `serviceTier: standard` and
`geo: global`, except the haiku reading, which records `geo: not_available`.

## Finding 1: the baseline is claude-opus-5, and it is not a tie-break artifact

Transcript corpus, n = 1,606 transcripts.

`replay route` does not take a baseline as an argument. It computes one:
`modelCorpus.busiest()` in `cmd/replay/route.go` returns the model with the most
fitted turns, ties broken by sorted name. On this snapshot that is
**claude-opus-5**, with 17,560 of 20,497 fitted turns (85.67%).

The selection is stable across three different weightings, so it is not an artifact
of which quantity `busiest()` happens to sort on:

| Weighting | claude-opus-5 share | Source |
|---|---:|---|
| Fitted turns | 17,560 of 20,497 (85.67%) | `replay route --json`, `dilation.ToTurns` per destination |
| Deduplicated requests | 41,987 of 47,781 (87.87%) | usage sums, deduplicated on `message.id` |
| List-price dollars | $2,623.34 of $3,088.95 (84.93%) | `replay route --json` `observed_usd`; `replay cost --json` `totalUsd` |

The dollar row compares two commands whose request counts differ by 199 of 31,972
(0.62%), because `route` pools only lanes that produced a fit. The share is quoted
to two figures for that reason.

## Finding 2: the routing rule today is one scalar, not a family of curves

Rate card, plus the transcript corpus for the hit rate.

`internal/analysis/route.go` divides the arithmetic in two, and the division decides
what may be claimed. Above the Dilation section, every function is built from
multipliers, prices and rates, and takes no token count. Below it, absolute figures
need sigma, the ratio between two tokenizers on the same content.

The structural half, measured on this snapshot at a hit rate of 98.1903%:

| Quantity | opus-5 | haiku-4-5 | sonnet-5 | opus-4-8 | fable-5-1 |
|---|---:|---:|---:|---:|---:|
| Cache read multiple (alpha) | 0.100 | 0.100 | 0.100 | 0.100 | **0.025** |
| Input, USD per MTok | 5 | 1 | 2 | 5 | 10 |
| Break-even trim, 5m | 90.8% | 90.8% | 90.8% | 90.8% | 96.6% |
| Break-even trim, 1h | 94.2% | 94.2% | 94.2% | 94.2% | 97.9% |

Source: `replay route --json`, `from` and `to` topology objects; the alpha row is the
`ReadMultiplier` and `readMultiplierNewest` constants in
`internal/cachemodel/anthropic.go`.

Four of the five priced models share alpha = 0.100. `InversionShare` solves
`c* = (r - 1) / (alphaFrom + r - 1 - r*alphaTo)`, which at equal alpha reduces to
`c* = (r - 1) / (0.9 * (r - 1)) = 1.111`, outside 0..1, so no boundary is reported.
`CrossRatio` at equal alpha reduces to `priceRatio * sigma`, with the cached share
cancelling out entirely.

So for every same-alpha pair the whole routing decision collapses to one scalar
comparison, and the only empirical input to it is sigma. That is measured, not
asserted: `replay route --json` returns no `inversion_share` field for
claude-haiku-4-5, claude-sonnet-5 or claude-opus-4-8, and returns 0.9524 for
claude-fable-5-1, the one destination whose alpha differs.

## Finding 3: sigma's error bar is a property of the estimator, not of the pair

Transcript corpus, n = 1,606 transcripts. This is the load-bearing finding.

Running `replay route --to claude-opus-5` from a corpus whose baseline is already
claude-opus-5 asks the instrument to measure a model against itself. The true answer
is exactly 1 by construction. The instrument returns:

```text
sigma (tokenizer dilation, claude-opus-5 -> claude-opus-5): 1.0000 +/-85% from 17560 and 17560 turns
```

The point estimate is right to four figures. The error bar is **85.10%**, on 17,560
turns per side.

That number is not noise about claude-opus-5's tokenizer. It is the estimator's own
floor, and the arithmetic says so exactly: `MeasureDilation` adds the two sides in
quadrature with `math.Hypot`, and 0.8510 is 0.6017 times the square root of two, so
claude-opus-5's own pooled fit error is 60.17% and the identity pair simply doubles
it. `gatherByModel` in `cmd/replay/route.go` accumulates
`sumErr[model] += rep.Fit.RelativeError * w` and divides by total turns, so the
figure carried is the turn-weighted **mean of per-session relative errors**, not a
spread recomputed over pooled turns. The file says this in its own comment.

The consequence is the part worth acting on: **a mean does not shrink with sample
size.** Adding transcripts moves it toward the average per-session fit error and no
further. The per-transcript `Fit ±%` column in `replay corpus` ranges from 29 to 171
across this corpus, which is where 60% comes from. Sigma's error bar is therefore
not blocked on corpus size, and no quantity of corpus will fix it. It is blocked on
the estimator.

Measured sigma for every destination, with both bands:

| Destination | Fitted turns | sigma | Quoted band | Band at the 85.10% floor | Contains 1.0 |
|---|---:|---:|---|---|---|
| claude-opus-5 (identity) | 17,560 | 1.0000 | +/-85.1%, [0.149, 1.851] | same | yes, exactly |
| claude-opus-4-8 | 1,956 | 1.0035 | +/-83.0%, [0.170, 1.837] | [0.150, 1.858] | yes |
| claude-sonnet-5 | 382 | 0.9718 | +/-79.1%, [0.203, 1.741] | [0.145, 1.799] | yes |
| claude-fable-5-1 | 67 | 0.6602 | +/-69.5%, [0.201, 1.119] | [0.098, 1.222] | yes |
| claude-haiku-4-5 | 532 | 0.5397 | +/-93.5%, [0.035, 1.044] | [0.080, 0.999] | at the quoted band |
| claude-sonnet-4-6 | 0 | refused | n/a | n/a | n/a |
| claude-opus-4-5 | 0 | refused, unpriced | n/a | n/a | n/a |

Not one measured sigma in this corpus is distinguishable from 1.0 at its own quoted
band. The instrument refuses correctly on the two destinations with no turns, and
suppresses their dollar figures, which is the behaviour the file was written to
produce.

## Finding 4: the deviation, in three units

Transcript corpus, n = 1,606 transcripts.

"Deviation from the baseline" has three measurable senses and they give three
different sizes. All three are reported because quoting one alone would be a choice
dressed as a measurement.

**Deviation by traffic mix.** 2,937 of 20,497 fitted turns, **14.33%**, ran on
something other than claude-opus-5: claude-opus-4-8 1,956, claude-haiku-4-5 532,
claude-sonnet-5 382, claude-fable-5-1 67. This is an exact count within the
snapshot, so it carries no sampling error; it carries the n=1 machine instead.
Source: `dilation.ToTurns` across seven `replay route --json` runs.

**Deviation within a session.** Of 744 cache breaks over 30,366 compared turns,
**6** are attributed to `model changed between requests`, 0.81% of breaks and 0.02%
of turns. Six events carry a counting error of roughly plus or minus 2.4 (the square
root of the count), so 0.81% of breaks is 0.81% plus or minus 0.33 percentage
points. This is the only direct observation in either corpus of routing actually
changing in flight. Source: `replay corpus`, break causes table.

**Deviation in cost, if the baseline were rerouted.** Over the same work, at list
price, with the band carried:

| Destination | Projected | Quoted band | Break-even sigma | Direction survives its band |
|---|---:|---|---:|---|
| claude-haiku-4-5 | $283.18 | [$18.35, $548.01] | 5.000 | yes, cheaper |
| claude-sonnet-5 | $1,019.74 | [$213.13, $1,826.35] | 2.500 | yes, cheaper |
| claude-fable-5-1 | $1,696.63 | [$516.62, $2,876.64] | 1.459 | yes, cheaper |
| claude-opus-4-8 | $2,632.47 | [$446.20, $4,818.74] | 1.000 | **no** |

Observed for the same work on claude-opus-5: **$2,623.34**. Source: `replay route
--json`, `observed_usd` and `projected_usd`; bands are the report's own
`dilation.RelativeError` applied to the projection, which is what the printed report
tells the reader to do.

Break-even sigma is the value at which a destination stops being cheaper:
`sigma* = from / (r * to)` where `from = (1 - c) + alphaFrom*c`,
`to = (1 - c) + alphaTo*c`, and `c` is the cached share. Computed at the measured
token-level cached share of 0.986092 for claude-opus-5. At the non-deduplicated
share of 0.982347 the only figure that moves is claude-fable-5-1's, from 1.459 to
1.373, and the verdict does not change at either value.

The direction of three of the four projections survives even the widened band: the
upper edge of each measured sigma stays below its break-even. The fourth,
claude-opus-4-8, has the same price as the baseline, so `r = 1` and sigma alone
decides; measured sigma is 1.0035 against a break-even of exactly 1.0000, a margin
of 0.35% inside a band of 83%. Nothing can be said about that pair, and the report
agrees: `pays_back` is false at a saving of minus $0.000287 per turn.

## Finding 5: the fable-5-1 inversion, and a comparison the report invites

Transcript corpus for the shares, rate card for the boundary.

claude-fable-5-1 is the one destination with a different alpha (0.025 against
0.100), and it produces a real inversion boundary at a **95.24%** cached share, with
claude-fable-5-1 cheaper per turn above it. Both of this corpus's candidate measures
of "cached share" sit above that boundary:

- request hit rate, 98.1903%, from `replay route --json` `hit_rate`
- token-level cached share for claude-opus-5, 98.6092% deduplicated and 98.2347%
  not, from the usage sums

Those are different quantities. The hit rate counts requests that read anything; the
cached share counts tokens. The printed report puts `Hit rate 98.19%` four lines
above `Cache-read inversion at a 95.24% cached share`, which invites a reader to
compare them directly. On this corpus that comparison happens to give the right
answer, because both quantities exceed the boundary. That is a measurement about
this corpus, not a property of the two quantities, and it should not be relied on
elsewhere.

## Finding 6: passive bounds against the probe, and why the probe earns its n=1

Both corpora.

The transcript corpus, at 1,606 transcripts, bounds the minimum cacheable prefix
only from above and only very loosely, because ordinary traffic never sends a small
prompt with a cache breakpoint on it. The probe series, at one reading per model,
brackets it to four tokens:

| Model | Passive bound, n = 1,606 transcripts | Probe bracket, n = 1 reading | Documented |
|---|---|---|---:|
| claude-opus-5 | at most 13,745 | above 508, at most 512 | 512 |
| claude-sonnet-5 | at most 20,406 | above 1,020, at most 1,024 | 1,024 |
| claude-fable-5-1 | at most 35,483 | above 508, at most 512 | 512 |
| claude-haiku-4-5 | at most 11,984 | above 4,094, at most 4,097 | 4,096 |
| claude-opus-4-8 | at most 20,457 | no reading | 1,024 |
| claude-sonnet-4-6 | no bound reported, 0 fitted turns | no reading | 1,024 |

Sources: `replay corpus` per-model section; `~/.replay/measurements.jsonl`, all four
lines, method `2026-09-06.1`, 9 to 12 probes each at 3 agreeing answers per
decision.

This is the honest case for the probe: a corpus three orders of magnitude larger
cannot produce the tighter number, because the evidence has to be manufactured. It
is also the honest case against over-reading it. Each bracket is a **resolution**,
not an error bar. There is no second reading to disagree with the first.

## What cannot be concluded at this n

Listed explicitly, because each of these is a claim someone could reasonably think
this document supports.

1. **Nothing about any sigma differing from 1.0.** Every measured band contains 1.0.
   The identity pair proves the band is estimator noise: sigma is exactly 1 there by
   construction and still carries 85.10%.
2. **No dollar projection as a point figure.** $283.18 for claude-haiku-4-5 spans
   $18.35 to $548.01 at its own quoted error. The report already says the figure is
   a bound to argue with rather than an invoice; that wording is correct and should
   not be dropped when the number is quoted elsewhere.
3. **Nothing at all about claude-opus-5 against claude-opus-4-8.** Equal price means
   sigma casts the whole vote, and sigma there is 1.0035 plus or minus 83%.
4. **Nothing about claude-sonnet-4-6 or claude-opus-4-5.** Zero fitted turns. The
   tool refuses, correctly.
5. **Nothing about the caching floor moving.** One reading per model, all four
   inside 35 seconds. A floor value is a fact anyone can copy the day it is
   published; "the floor changed on this date" needs at least two readings on
   different occasions and there are none. This is the one thing genuinely blocked
   on n.
6. **No error bar on any probe-derived figure.** A single reading has no spread.
   Anything quoting per-model probe results with a plus-or-minus is quoting the
   bisection resolution and mislabelling it.
7. **Nothing about generalisation.** One operator, one machine, one account. 1,308
   of 1,606 transcripts are claude-opus-5 on one person's work. The 98.19% hit rate
   is that person's habits as much as the provider's behaviour.
8. **Nothing about tier or geography.** All readings are `serviceTier: standard`;
   three are `geo: global` and one is `geo: not_available`.
9. **Nothing about whether the break-even trim thresholds are correctly
   parameterised.** `BreakEvenTrim` takes `h` as the probability that a token
   offered to the cache is served warm, and is fed the per-request hit rate. Whether
   those coincide is not separable on this corpus: they are 98.1903% and 98.6092%
   here, 0.42 percentage points apart, which is inside anything this corpus can
   resolve. No defect is claimed; no agreement is established either.
10. **A caveat on quoting `replay corpus` per-model rows.** Its per-model table heads
    its second column `Sessions`, but the column sums to 1,606, which is the
    transcript count stated in the Totals block above it, not the 115 sessions
    stated in the same block. Figures taken from that column are per transcript.
    This document quotes it as transcripts throughout.

## What would raise n, and what it would cost

**For the probe series, which is the part actually blocked.** The cheapest
measurement that raises n is one more sweep of the same four models on a different
day. It takes n from 1 to 2 per model and is the only step that converts a floor
into a floor that did or did not move, which no quantity of transcripts can produce.

Cost, at first-party list price and the 1.25x short-TTL write multiplier, for the
16-probe budget `replay probe` plans by default. Each probe is one request with
`max_tokens: 1` (`internal/probe/run.go`), so output cost is negligible and is
ignored below:

| Model | Prefix at the recorded bracket | 16 probes there | 16 probes at the 65,536 ceiling |
|---|---:|---:|---:|
| claude-opus-5 | 512 | $0.051 | $6.55 |
| claude-sonnet-5 | 1,024 | $0.041 | $2.62 |
| claude-fable-5-1 | 512 | $0.102 | $13.11 |
| claude-haiku-4-5 | 4,096 | $0.082 | $1.31 |
| **Sweep of all four** | | **$0.28** | **$23.59** |

The realistic column is the one to plan against: the planner tests the documented
prior first (`probe plan` prints "testing the documented 512 first"), and the four
recorded runs each finished in 9 to 12 probes with a clean bracket, well under the
16-probe budget. The whole sweep took 35 seconds of wall clock. Thirty daily sweeps
would cost roughly **$8.30** and would buy the first real within-model variance, the
first error bar the probe series has ever been entitled to, and thirty days of the
one thing `internal/probe/reading.go` argues is not backfillable at any price.

Two models have never been probed at all: claude-opus-4-8 and claude-sonnet-4-6.
Adding them to the sweep costs about $0.10 and $0.06 per run respectively at their
documented 1,024-token floors, and takes the probe corpus from four models to six.

**For sigma, the answer is that raising n will not help.** The error bar is a
turn-weighted mean of per-session relative errors and converges to that mean rather
than shrinking. What would help is refitting tokens-per-byte over pooled turns and
recomputing the spread, instead of averaging per-session errors. That is an
estimator change, costs no provider requests, and is testable against the identity
pair, which must return an error near zero once the estimator is right. Until then,
the identity pair's 85.10% is the honest floor to quote beside any sigma.

## Status

The baseline is measured and is not in doubt. The routing rule is measured and, for
four of five priced models, reduces to a single scalar. The deviation is measured in
three units and reported in all three. What is unmeasurable here is sigma's
precision, and the reason is the estimator rather than the corpus. What is genuinely
blocked on n is the probe series, which is one reading per model and therefore has
no variance to report, at any price above thirty cents.

---

[Evidence index](README.md) · [Documentation index](../README.md)
