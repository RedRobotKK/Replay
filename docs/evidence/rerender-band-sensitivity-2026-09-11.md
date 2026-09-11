# Does the ±10% band decide the re-render headline?

**Measured 2026-09-11.**
**Tier:** estimated — transcripts only, no proxy, so ADR-0002's estimated tier
applies to every figure here. That both sides of the comparison this file is
about are provider-reported integers in 642 of 707 cases is the finding, not a
tier: the numbers are read out of the transcript, which is where the estimated
tier puts them.
**Corpus:** 1,816 transcripts from 118 sessions under `~/.claude/projects` on
one machine, 37,997 compared turns, **806 breaks, 40,087,042 re-billed tokens**.

The band decides **8 of 597** re-render classifications and **1.52 points** of
the share. Across every tolerance from zero to infinity the share moves 8.1
points. The headline has moved anyway — by 8.4 points — and the tolerance is
not why.

## The objection

An external reviewer read the classifier and put it this way: a break is
labelled a client re-render when the actual cache read lands within about 10%
of an estimated "unseen prefix" size; that estimate comes from a byte-to-token
fit whose published error on large lanes runs ±29% to ±170%; therefore a ±10%
band around a coarse estimate will absorb other causes, and
[the 50.8%](break-causes-2026-09-06.md) is a story about a tolerance constant.

Three claims. Two of them are correct.

## What checks out

The band exists and is already named. `internal/analysis/diff.go:27`:

```go
// rerenderTolerance is how far a read may sit above the unseen-prefix
// estimate and still be classified as "everything after the system prefix
// was re-billed". The fit's own error would be more precise but the unseen
// prefix estimate is coarse by nature.
const rerenderTolerance = 0.10
```

and `diff.go:91`:

```go
unseen := float64(fit.UnseenPrefix.Total())
if unseen > 0 && math.Abs(float64(t.Actual)-unseen) <= unseen*rerenderTolerance {
    b.Cause = cachemodel.CauseRerendered
```

The ±29% to ±170% figures are this repository's own. They are the `Fit ±%`
column of [the calibration corpus](calibration-corpus-2026-09-10.md), and the
six largest lanes in today's run read 57, 99, 106, 99, 99, 29.

## What does not

`UnseenPrefix` is not a byte-to-token estimate in the general case. `Fit` sets
it two ways (`internal/analysis/fit.go:212`):

```go
if first.Usage.CacheRead > 0 {
    fit.UnseenPrefix = Measured(first.Usage.CacheRead)
    ...
} else {
    fit.UnseenPrefix = Estimated(max(first.Usage.PromptTotal()-visible, 0))
}
```

When the lane's first request found a warm cache, the unseen prefix is the
provider's own `cache_read_input_tokens` — measured, not fitted. The `Tokens`
type carries the provenance, so the corpus can be asked which branch each
classification actually used. It was asked.

**642 of the 707 breaks that reach this test carry a measured unseen prefix**,
holding 17,416,702 of their 19,654,389 tokens (88.6%). For those, both sides of
the comparison are integers the provider reported. The fit's error bar is not
in the arithmetic.

## What the band can and cannot absorb

The band is the last discriminator in `classify`. Everything above it is
decided first and unconditionally: lane-overlap, the usage-only causes shared
with the proxy (TTL expiry, model change, system/tool change), an effort
change, and an exact history diff. The band can only trade with the branch
below it, `CauseUnknown`.

That is visible in the sweep rather than only in the code. **These three rows
are byte-identical at every tolerance from 0.00 to infinity:**

| cause | breaks | tokens | share |
|---|---:|---:|---:|
| cache expired (gap longer than the TTL) | 83 | 17,083,819 | 42.62% |
| system prompt or tool definitions changed | 11 | 2,575,073 | 6.42% |
| model changed between requests | 5 | 773,761 | 1.93% |

No value of the tolerance moves a single token into or out of them. "It will
absorb other causes" is bounded to one cause, and that cause holds 6.60% of
re-billed tokens at the shipped band.

## Method

1. The repository tree was copied outside the working copy. **The only
   difference is the constant**, verified with `diff -r`:
   `const rerenderTolerance = 0.10` became a `var` plus a setter, so one
   process could sweep it. No production code in the repository was changed;
   this pull request adds no Go. The copy is throwaway and is not committed.
2. For each `.jsonl` under `~/.claude/projects`: parse, take
   `analysis.MainLane`, `Calibrate`, `Fit` — all of which are independent of
   the tolerance — then run the real `analysis.FindBreaks` once per tolerance
   value and tally `Cause` and `Deficit`.
3. Distances were taken by running the same classifier at an infinite
   tolerance, which labels every break that reaches the test, and recording
   `|actual − unseen| / unseen` and the provenance of `unseen` for each.

**The harness was checked against the shipped binary.** `replay corpus` on the
same root, built from this branch, prints the same five counts as the sweep at
the shipped 0.10:

```text
| client re-rendered history after the system prefix (no edit visible in transcript) | 597 |
| prefix diverged inside the message history at an unknown block | 110 |
| cache expired (gap longer than the TTL) | 83 |
| system prompt or tool definitions changed | 11 |
| model changed between requests | 5 |
```

806 breaks, 1,816 transcripts, 118 sessions, both ways.

## The sweep

Only two rows move. Shares are of all 40,087,042 re-billed tokens.

| band | re-render breaks | re-render tokens | re-render share | unknown-block breaks | unknown-block tokens | unknown-block share |
|---:|---:|---:|---:|---:|---:|---:|
| 0% (exact equality) | 589 | 16,398,254 | 40.91% | 118 | 3,256,135 | 8.12% |
| 2% | 593 | 16,802,085 | 41.91% | 114 | 2,852,304 | 7.12% |
| 5% | 596 | 16,980,207 | 42.36% | 111 | 2,674,182 | 6.67% |
| **10% (shipped)** | **597** | **17,008,803** | **42.43%** | **110** | **2,645,586** | **6.60%** |
| 15% | 597 | 17,008,803 | 42.43% | 110 | 2,645,586 | 6.60% |
| 25% | 598 | 17,166,390 | 42.82% | 109 | 2,487,999 | 6.21% |
| 29% (the fit's best published error) | 598 | 17,166,390 | 42.82% | 109 | 2,487,999 | 6.21% |
| 50% | 617 | 18,497,691 | 46.14% | 90 | 1,156,698 | 2.89% |
| 170% (the fit's worst published error) | 670 | 19,506,890 | 48.66% | 37 | 147,499 | 0.37% |
| ∞ (band removed entirely) | 707 | 19,654,389 | 49.03% | 0 | 0 | 0.00% |

Deleting the band and labelling every break that reaches it a re-render adds
8.1 points. Shrinking it to exact integer equality removes 1.5. The shipped
10% sits 1.5 points into an 8.1-point range, and the 10% and 15% rows are
identical to the token because no break in the corpus has a distance between
them.

## The criterion was pre-registered, and it was not met

This is not a threshold picked after seeing the answer.
[`docs/design/benefit-gap-analysis.md`](../design/benefit-gap-analysis.md)
already carried the objection and already wrote down what would settle it:

> **The tolerance is refuted** if perturbing `rerenderTolerance` from 0.10 to
> 0.15 moves more than 10% of breaks between causes — that would show the band,
> not the evidence, is choosing the answer.

0.10 → 0.15 moves **zero breaks and zero tokens**. The refutation condition
fails by its whole margin, on the run its author said would take an afternoon.

The same document reads the code as applying "a ±10% acceptance band to a
quantity whose own median relative error is 49%" and concludes that
`CauseRerendered` is "effectively a coin-flip dressed as a classification". The
premise is the one this file tests: the median relative error of the *fit* is
not the error of the *comparand*, because in 642 of 707 cases the comparand is
not fitted.

## Where the mass actually sits

The distance distribution explains the flatness, and it is not a distribution
in any useful sense:

| distance from the unseen prefix | breaks | tokens |
|---|---:|---:|
| exactly 0 | 589 | 16,398,254 |
| 0 < d ≤ 10% | 8 | 610,549 |
| > 10% | 110 | 2,645,586 |

**Every one of the 589 exact matches has a measured unseen prefix.** Not one of
them rests on the byte-to-token fit. The mechanism is plain once the provenance
is visible: `UnseenPrefix` is the first request's `cache_read_input_tokens`, and
a later turn that re-billed everything after the shared prefix read that same
prefix and was handed back that same integer. Two reports of one number, equal
because they are the same number:

```text
actual=15252    unseen=15252    deficit=236879
actual=19789    unseen=19789    deficit=224141
actual=15252    unseen=15252    deficit=190933
```

The eight breaks the band's width actually decides are these, in full:

| distance | actual | unseen | re-billed | unseen prefix |
|---:|---:|---:|---:|---|
| 0.015% | 19,789 | 19,786 | 136,647 | measured |
| 0.015% | 19,789 | 19,786 | 134,758 | measured |
| 4.851% | 18,829 | 19,789 | 115,831 | measured |
| 0.005% | 22,079 | 22,078 | 106,930 | measured |
| 4.861% | 18,829 | 19,791 | 58,246 | measured |
| 7.797% | 14,639 | 15,877 | 28,596 | measured |
| 0.019% | 26,349 | 26,354 | 25,496 | measured |
| 2.585% | 152,765 | 156,818 | 4,045 | **estimated** |

Four of those eight are off by between one and five tokens — arithmetic slack,
not tolerance. Four sit more than 2% out, and those four are the only breaks in
the corpus where the band's width is doing recognisable work.

**One break in the corpus is both decided by the band and resting on a fitted
estimate. It carries 4,045 tokens: 0.010% of everything re-billed.** That is
the entire surface on which the reviewer's mechanism operates.

## The headline did move, and this is not the reason

The 2026-09-06 reading cannot be reproduced today.

| cause | 2026-09-06 | 2026-09-11 |
|---|---:|---:|
| client re-rendered history | **50.8%** | **42.4%** |
| cache expired (TTL) | 33.9% | 42.6% |
| prefix diverged, block unknown | 7.0% | 6.6% |
| system prompt or tools changed | 5.8% | 6.4% |
| model changed | 2.6% | 1.9% |
| breaks / re-billed tokens | 735 / 31,264,349 | 806 / 40,087,042 |

Two things changed underneath it: the corpus grew from 1,506 transcripts to
1,829 files (1,816 analysable), and the engine has taken 234 commits since,
including four transcript-parser fixes. **This reading cannot separate those
two causes**, the same limit
[the 2026-09-10 corpus](calibration-corpus-2026-09-10.md) records for its own
0.33-point move. What it can say is that the tolerance is not among them: at
every band from 0% to ∞ the 2026-09-11 re-render share stays inside
40.9%–49.0%, and 50.8% is outside that range at every one of them.

A third reading exists and is further out still: the same design review reports
**77.8% / 6.2%** from its own corpus run. Three readings of one classifier give
50.8%, 42.4% and 77.8% for this share. The dispersion is between corpora, not
between tolerances — which is what
[break-causes](break-causes-2026-09-06.md)' own limits section predicted, in
the sentence beginning "This corpus is fan-out heavy".

So the answer to the question asked is: **50.8% is not an artifact of choosing
10%, and it is also not today's number.** Both halves are the result. The
number is sample-dependent to a degree the tolerance never approaches: an
8.1-point range across every possible band, against a 35-point spread across
three corpora.

## Is the instrument able to fail?

A sweep that returned the same figure at every setting would be evidence of a
pinned instrument, not of a stable finding. It does not. Removing the band
entirely moves 110 breaks and 2,645,586 tokens out of the unknown-block row and
takes it to zero; shrinking it to exact equality moves 8 breaks and 610,549
tokens the other way. The knob is connected. It is turned across two orders of
magnitude, and almost nothing follows.

## What this cannot support

- **It does not vindicate the label.** Exact equality establishes that the read
  equalled the shared prefix. It does not establish that a client re-rendered
  anything; the cause string's own parenthesis says "no edit visible in
  transcript", and inference from an absence is what that means. This file
  narrows the objection to the tolerance; it leaves
  [break-causes](break-causes-2026-09-06.md)' own caveat exactly where it was.
- **There is an absorption channel the tolerance does not govern.**
  `firstDivergence` compares each message's UUID and its byte *count*. An edit
  that preserves both is invisible, falls through to the band, and would be
  labelled a re-render — and narrowing the band to zero would not catch it,
  because such a break can still land on exact equality. This is the same
  byte-count limit [seeded blame](seeded-blame-2026-09-11.md) found in
  `systemChanged`, one function over. **Its size here is not measured.** It is
  named because it exists, not because it was found.
- **The eight band-decided breaks are not shown to be re-renders.** They are
  shown to be eight.
- **The result depends on a corpus whose lanes start warm.** 642 of 707 reaching
  breaks have a measured unseen prefix because the first request of the lane
  found a cache. A corpus of cold-start sessions would push weight onto the
  estimated branch, and there the ±29% to ±170% fit error is exactly as relevant
  as the reviewer says. **This measurement does not rule that out; it reports
  that it is not the case here.**
- **One operator, one machine, 118 sessions, fan-out heavy.** Every limit in
  [break-causes](break-causes-2026-09-06.md) applies unchanged, including the
  one that matters most: a single-lane user may see almost no re-render at all.
- Nothing here says a re-render is preventable, or that any layout recovers a
  token of it.

## What it does support

The tolerance constant can be left where it is. It is doing almost no work, and
the work it is doing is visible and small. If the re-render share is wrong, it
is wrong for a reason that is not in `diff.go:31`.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
