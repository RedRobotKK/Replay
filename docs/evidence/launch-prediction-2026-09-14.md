# Launch KPIs, pre-registered 2026-09-12 22:10 PDT

**What this is:** every number this project expects from the Hacker News and
Product Hunt launch on 2026-09-14, written down **before** the launch, with the
thresholds that decide what each outcome means.

**Why it is here rather than in a planning document.** A threshold chosen after
seeing the number is not a threshold. This repository applies that rule to its
own instruments and has never applied it to a business figure, which is the gap
`RELEASE-CRITERIA.md` closed on 2026-09-13 by extending the measurement gate to
commercial claims. This file is the first figure filed under the extended rule.

**Everything below the baseline is MODELLED.** Two agent-built persona models,
1000 personas each, seeds 20260914 and 20260914. Their segment shares have no
source and are labelled ASSUMED in their own reports. The Hacker News model
carries a free attention parameter that every absolute number scales with. Treat
the ranks and the probabilities as opinions with arithmetic attached, and the
thresholds as commitments.

## The baseline, which is MEASURED

| | |
|---|---|
| Install-script fetches, lifetime | **53**, from **2 distinct IP addresses**, read 2026-09-07 |
| Release-asset downloads | 152, and they include scanners and CI |
| External users ever observed | **zero** |
| `replay pool` members | **1**, and it is the maintainer's own machine |
| Contributed corpora from anyone else | **zero** |

That is the population every figure this project publishes is drawn from, and
closing it is what the launch is for.

## What is predicted

### Hacker News

| | Predicted |
|---|---|
| Reaches the front page | **25%** |
| Points, if front page | median 216 (p10 92, p90 745) |
| Points, if not | median 4 |
| Comments, if front page | median 142, roughly 65 distinct commenters |
| **Installs, if front page** | **median 38** (p90 132) |
| Points at or above 500 | 5% |
| Critical to positive comments | 1.43 to 1 |

### Product Hunt

| | Predicted |
|---|---|
| Top five | **0.00%** across 40,000 draws. Top three needs 284 to 369 upvotes; the p95 here is 98 |
| Top ten | 1.6% |
| Rank | median 28 |
| Visitors | median 198 |
| Installs | median 4 |
| Contributed corpora | **0** |

Recorded because the money path's section 0.4 assumes "a first-place Hacker News
day plus a top-three Product Hunt day". The Product Hunt half of that is not a
low-probability outcome, it is off the distribution, and that sentence should be
corrected rather than relied on.

### The number that actually matters

| | Predicted |
|---|---|
| **At least one contributed corpus that is not the maintainer's, within 7 days** | **15% to 25%** |

Every other row is downstream of this one. The calibration corpus has one member,
`ROADMAP.md` names independence as the v0.8 blocker, and it is the single thing
on the roadmap that cannot be done alone.

## What is recorded on the day, and what is not

**Not recorded as evidence of anything:** upvote totals, rank, badges, newsletter
inclusion, comment counts, stars. Every one is confounded with reciprocity and
browse depth. This project already holds the proof that a flattering count lies:
53 fetches from 2 IP addresses is a 26-fold gap between the number that is easy
to quote and the number that is true.

**Recorded, in this order:**

1. **Distinct source IPs** fetching `/replay.sh` over 48 hours. Not fetch count.
2. Of those, IPs that then fetch a release tarball or `checksums.txt`.
3. **`replay pool` membership delta.** Requires running the binary on real data
   and choosing to send roughly 600 bytes.
4. GitHub unique cloners and release asset downloads, which count distinct
   actors rather than requests.
5. Issues or discussions opened by a non-maintainer account. Lifetime count
   today is zero.

## The thresholds, committed now

**Applause Ratio** = distinct installing IPs divided by total upvotes across both
sites.

| | Reading |
|---|---|
| below 0.03 | The upvotes came from people who cannot run it. The day produced nothing, whatever the rank said |
| 0.03 to 0.10 | The predicted base rate. **This is not traction, it is the model being right** |
| above 0.15 | Something real happened. Find out which segment and why |

**Absolute floor: 25 distinct non-maintainer machines.** Below that, no pricing
question, no retention question and no generalisation question is answerable,
which is the money path's own conclusion that the constraint is distribution
rather than the number on the page. The models predict 4 to 38.

**Corpora: 3.** The minimum that can begin to test whether the measured
concentration, 25 fan-out sessions holding 98.8% of avoidable spend, generalises
past one machine. Models predict 0.

## What each outcome decides

| Outcome | What it means, and what changes |
|---|---|
| Front page, 25+ distinct machines, 3+ corpora | The strongest available case. v0.8 independence unblocks and the population question becomes answerable. Nothing about the commercial plan is settled by it |
| Front page, 25+ machines, 0 corpora | The ask was wrong, not the product. The contribution path is too heavy or the reason to contribute was never stated. Fix the ask, not the tool |
| Front page, under 25 machines | Attention without evaluators. Consistent with the Product Hunt model's finding that 86% of that audience cannot run a terminal binary at all. The channel is wrong |
| No front page | The modal outcome at 75%. It settles nothing and should not be read as a verdict on the product |
| Any thread where the top comment is a stranger reporting their own number | Worth more than the rank. That is the only comment category that constitutes evidence |

## The risk that is being pre-empted rather than predicted

Both audience models independently name the same worst case: a reader finds
`docs/MONEY-PATH.md`, pairs the provisional `$199 per repository per month` with
the `$500,000 post-tax` target and the tax table in section 0.1, adds BUSL so the
project cannot be forked, and posts all three together. The reading requires no
bad faith and it reframes every honest thing in the README as setup.

The response is to link that document first, in the maintainer's own opening
comment, which is the only pre-emption consistent with how the rest of this
project behaves. A document volunteered cannot be used to expose the person who
volunteered it.

Also worth stating plainly on the day, because the model says it will be the
second thread either way: **for a flat-seat reader the recoverable figure is zero
dollars**, that is published as a null result in
[quota-titration-2026-09-06](quota-titration-2026-09-06.md), and it applies to
most people who will read the post.

## The honest limit on this file

The predictions come from two models whose population shares are invented. The
measured population is two IP addresses. Nothing here is a forecast in the sense
that a weather forecast is a forecast; it is a set of commitments about what will
be counted and what each count will be taken to mean.

**The results get appended to this file, dated, including the rows where the
prediction was wrong.** That is the only reason to write it down in advance.

---

[Evidence index](README.md) · [The money path](../MONEY-PATH.md) ·
[Release criteria](../../RELEASE-CRITERIA.md)
