# RPL-C037: break-level coverage of the re-billed dollar figure

**Frozen as a candidate 2026-10-01, repaired 2026-10-02.**

The first section is the record as it was frozen, before any repair, and is kept
in the present tense it was written in. The repair and the Phase 0 closure
follow it. Where the two disagree the later section is current.

## The claim

`replay cost` presents `RebilledUSD` and `RebilledTokens` as a paired re-billing
result. `RebilledUSD` covers only breaks whose own record could be priced.
`RebilledTokens` covers every break. No rendered or serialized field identifies
the excluded break tokens. Corpora with different break-level priceability
therefore collapse to the same re-billed dollar and token surface.

`replay since` ranks on `RebilledTokens` alone and is **outside this claim**.

## What this is not

| | population | question |
|---|---|---|
| RPL-C031 / C035 | requests | what share of the requests does `TotalUSD` cover? |
| RPL-C036 | breaks | which rate does a break's deficit pay? |
| **RPL-C037** | **breaks** | **what share of the re-billed tokens is inside `RebilledUSD`?** |

Three populations, three claims. C036 settled the rate and left the coverage
question untouched. C035's disclosure is about requests and about the total,
and the control below holds it identical across two corpora whose break
coverage differs.

## Deciding paths, frozen and unchanged

| path | what it does |
|---|---|
| `cmd/replay/cost.go:433` | `s.RebilledTokens += u.RebilledTokens`, over **every** unit, priced or not |
| `cmd/replay/cost.go:453` | `if u.Unpriced { continue }` opens the priced-only loop |
| `cmd/replay/cost.go:461` | `s.RebilledUSD += u.RebilledUSD`, **inside** that loop |
| `cmd/replay/cost.go:885` | per break, `if p, ok := PriceForAt(br.Turn.Request.Model, ...)`, the second gate |
| `cmd/replay/cost.go:473` | `s.RebilledShare = s.RebilledUSD / s.TotalUSD` |
| `cmd/replay/cost.go:547-549` | the two lines are rendered as one result, the token line indented under the dollar line |
| `cmd/replay/cost.go:616` | C035's disclosure, scoped in its own words to "the requests read" and "the total above" |

`RebilledUSD` is gated twice and `RebilledTokens` never.

`cmd/replay/cost.go:70-78` already states the governing principle for the
unit-level flag: unpriced "is not 'cost zero', and zero dollars is deliberately
not the representation ... the two must stay distinguishable". That holds for
`CostUSD`. It does not yet hold for `RebilledUSD`.

No path above was modified.

## The counterexample

One model name across three dated windows, so the route line and model
identity are constant and only the rate in force varies. Every record carries
`Input 500, Output 300`; a breaking record writes 20,000 and reads 0, a clean
record reads 20,000 and writes 0.

| window | days | input | priced |
|---|---|---:|---|
| A | 2026-09-01 to 09-05 | $1.00/MTok | yes |
| B | 2026-09-06 to 09-10 | $0.50/MTok | yes |
| C | 2026-09-11 to 09-15 | n/a | **no** |

### Pair 1, the collapse

| | records | break tokens | priceable | unpriceable | coverage |
|---|---|---:|---:|---:|---:|
| **P** | break A, break B, break B | 40,000 | 40,000 | 0 | **100%** |
| **Q** | break A, break A, break **C** | 40,000 | 20,000 | 20,000 | **50%** |

The cost legs agree because `1.00 + 0.50 + 0.50 == 1.00 + 1.00 + 0`. Measured,
both arms:

```
  re-billed      $0.02  (37% of the total)
                 40k tokens re-billed
  JSON  rebilledUsd 0.02   rebilledTokens 40000   rebilledShare 0.370370   breaks 2
```

Total, legs, median and p90 identical as well. The only difference anywhere in
the render is that Q additionally prints C035's request-coverage sentence,
which reports requests and governs the total, and does not say that $0.02
excludes 20,000 of the 40,000 tokens printed above it.

### Pair 2, the orthogonal control

Both arms carry exactly one unpriceable **request**, so C035's sentence is
identical in both.

| | records | break tokens | priceable | unpriceable | coverage |
|---|---|---:|---:|---:|---:|
| **R** | break B, break B, break B, clean **C** | 40,000 | 40,000 | 0 | **100%** |
| **Q2** | break A, break A, break **C**, clean A | 40,000 | 20,000 | 20,000 | **50%** |

Both print `The total above covers 75% of the requests read: 3 of 4 priced, 1 on a
model no price table carries.` and both re-bill `$0.02` over `40k tokens` across
`2 breaks`.

`RebilledShare` does differ here, 0.494 against 0.345, because it is
`RebilledUSD / TotalUSD` and the two arms have different as-run totals by
construction. A ratio that moves with its denominator carries no information
about break coverage, and Pair 1 is the arm where even the share is identical.

## The oracle

`cmd/replay/claim_c037_breakcoverage_test.go`. The break-token ledger is
computed from the fixture specification and never from `RebilledUSD` or
`RebilledTokens`: a request reads what the one before it wrote, whatever it did
not read is a deficit on that record, and priceability comes from the fixture's
own declaration. `TestC037_FixtureAssumptions` checks the declaration against
the price table and the oracle's token total against production, which is the
one reconciliation and is about the quantity C036 settled, never the dollars.

Four predicates, frozen before the production implementation was read:

1. a corpus with partial break coverage must produce an observable surface
   different from an otherwise matching fully covered corpus
2. the disclosed coverage must equal `pricedBreakTokens / (priced + unpriced)`
3. fully priceable breaks emit no partial-coverage disclosure
4. a zero-break corpus emits no partial-coverage disclosure

Positive control: the partial corpora, P1/Q and P2/Q2. Negative controls: the
fully priceable corpora P1/P and P2/R, and a zero-break corpus which must carry
neither a disclosure nor a re-billed token line.

Predicates 1 and 2 are **currently false** and the tests pin that, hard-failing
if the surface changes in either direction. A repair must invert them rather
than delete them, as RB1Pop was inverted after the FM repair.

### The detector was blind, again

`c037DisclosedBreakCoverage` first read one line at a time and could not see
its own planted control, because a disclosure sentence wraps and the subject
and the pricing word land on different lines. That is the fourth detector in
this campaign to be blind to the string its control planted, and it was found
the same way as the other three: by reading the output. It is now paragraph
scoped, and `TestC037_TheDetectorDiscriminates` proves it reads 50% from a
planted sentence, does **not** fire on C035's request-coverage sentence, and
does not fire on today's report.

## Rejected candidates

No production disclosure exists, so there is nothing to mutate. Five candidate
disclosures were simulated instead and put to the acceptance rule, which
requires the coverage to equal the oracle's **and** the token total to remain
every break token, so a candidate cannot buy agreement by discarding evidence.

| candidate | verdict | on P1/Q |
|---|---|---|
| correct: priced break tokens over all break tokens | **accepted** on all 5 corpora | 0.50 |
| M1 force the disclosure to 100% | **rejected** | 1.0000 against 0.5000 |
| M2 derive coverage from request counts | **rejected** | 0.6667 against 0.5000 |
| M3 derive coverage from `RebilledUSD > 0` | **rejected** | 1.0000 against 0.5000 |
| M4 read coverage off the unit-level `Unpriced` flag | **rejected** | 1.0000 against 0.5000 |
| M5 drop unpriceable break tokens, claim full coverage | **rejected** | token total 20,000 against 40,000 |

Each was first required to compute something different from the correct
candidate on at least one fixture; a candidate that agreed everywhere would be
reported as mis-targeted rather than killed. This tests the oracle's
discriminating power, not a repair, which is what a freezing pass is for.

M2 is killed only because P1/Q is built so the two populations diverge: 2 of 3
requests priced against 20,000 of 40,000 break tokens. A corpus where they
coincide would not have killed it.

## What is not established

- the five rejected candidates are simulated, not production mutations
- only `replay cost` was measured. `replay advise`, the card and the share
  surface read re-billed figures and were not examined for the same collapse
- the detector is proven against a planted sentence, not a shipped one. A
  disclosure worded without "priced", "price table" or "unpriceable" would not
  be seen

## Separately, and not part of this claim

C035's sentence renders "Those 1 are not in the figure" for a single record.
A wording defect, recorded here so it is not lost, and deliberately kept out of
C037's evidence.

---

[Evidence index](README.md) · [Repository README](../../README.md)

---

# Repair, 2026-10-02

**One counter, one disclosure. No dollar and no token figure moved.**

## The field

`costUnit.UnpricedRebilledTokens int` / `json:"unpricedRebilledTokens,omitempty"`,
and the same field on `costSummary`.

**Semantics.** The part of `RebilledTokens` that `RebilledUSD` does not
represent. The represented half is `RebilledTokens` minus this.

One counter rather than a pair, decided on evidence rather than by following
C035. C035 took paired counters because `Requests` means priced-only in
`costusage.go`, so deriving from it would have been right by coincidence.
`RebilledTokens` has no such ambiguity: it is accumulated unconditionally at
`cost.go:433`, pinned by C036 F4 and by `TestC037_P4`. The exception is the
field rather than the represented part, matching `costUnit.Unpriced` and the
top-level `unpriced` count, so `omitempty` means "nothing excluded" and the
common document gains no key.

## The quantity, and why not break count

| representation | the 90,000-against-three-1,000 corpus |
|---|---:|
| priceable break **count** / total breaks | 25% |
| priceable deficit **tokens** / total deficit tokens | **96%** |

The product reported `$0.090000`, which is exactly 90,000 tokens at $1.00/MTok.
C036 established `RebilledUSD = Σ over represented breaks of (deficit × rate)`,
so the figure is token-weighted and deficits are unequal because
`Deficit = Expected − Actual` depends on what the previous record wrote. Count
coverage would have understated by 71 points. `TestC037R2` hard-fails if a
fixture is ever used where the two coincide.

## Two gates, not one

`RebilledUSD` is withheld per break at `cost.go:885` **and** per unit at
`cost.go:453`. Measured on two sessions where **every break in the corpus is
priceable** and one session's earliest record is not:

```
per-break oracle: 60,000 priceable tokens, 0 unpriceable
summary:          rebilledUSD $0.020000   rebilledTokens 60000
  task sb  unpriced=true   rebilledUsd=$0.040000 (withheld whole by summarise)
  task sa  unpriced=false  rebilledUsd=$0.020000
```

A counter built only at the per-break site would have disclosed **100%**
against a figure representing **20,000 of 60,000**, which is 33%. `summarise`
therefore re-buckets a unit-flagged session's whole `RebilledTokens` into the
excluded side. `TestC037R3` fails if the disclosure ever reads 100% there.

## Before and after

| | before | after |
|---|---|---|
| P, 100% coverage | `$0.02 / 40k tokens`, no disclosure | `$0.02 / 40k tokens`, **no disclosure** |
| Q, 50% coverage | `$0.02 / 40k tokens`, no disclosure | `$0.02 / 40k tokens` **+ "covers 50% of those tokens: 20000 of 40000 were priced"** |

The dollar and token figures are deliberately identical before and after, in
both arms. `TestC037_P1` asserts that in both directions: the arms must now
differ, and `$0.020000` over 40,000 tokens must not have moved.

0% renders `re-billed $0.00 (0% of the total)`, `40k tokens re-billed`, and
`The re-billed figure covers 0% of those tokens: 0 of 40000 were priced, and
40000 ran on a model no price table carries at the time they ran. Those are not
free, and what they cost is not established here.` That is the case the claim
existed for: `$0.00` beside 40,000 tokens must not read as free.

## Production mutations

Against the landed implementation. The five simulated candidates from the
freezing pass are retained as an oracle self-check and are **not** the mutation
evidence.

| | verdict |
|---|---|
| C0 reassociate the coverage arithmetic (control) | **SURVIVED**, as required |
| M1 count breaks instead of their tokens | **KILLED** |
| M2 request-based coverage | **KILLED** |
| M3 per-break only, ignore the unit gate | **KILLED** |
| M4 forced 100% | **KILLED** |
| M5 inverted coverage | **KILLED** |
| M6 drop unpriced tokens from the denominator | **KILLED** |
| M7 derive coverage from `RebilledUSD > 0` | **KILLED** |
| M8 infer priceability from a nonzero contribution | **KILLED** |
| M9 suppress the disclosure, the pre-repair state | **KILLED** |
| M10 drop the lane fold of the counter | **KILLED**, after a fixture was added |

**M10 survived the first run.** Every fixture was single-lane, so the fold line
was never reached, and the survivor said so rather than the oracle saying so.
`TestC037R7` now folds two lanes of one session, one fully priceable and one
not, and asserts the fold produced `lanes=2` before judging the disclosure. The
survivor was a genuinely untested path, not a weak oracle and not a mis-target.

A first attempt at M1 did not compile: `costSummary` carries no break count, so
count-based coverage is not expressible at the render site at all. The mutation
was moved to the per-break accumulator, where it is expressible, and died there.

## Cache schema

Measured, not assumed. `unitSchema()` before and after:

```
before  ... rebilledTokens,rebilledUsd,repeated,requests,session,uncachedUsd,unpriced
after   ... rebilledTokens,rebilledUsd,repeated,requests,session,uncachedUsd,unpriced,unpricedRebilledTokens,unpricedRequests
```

`costIndexKey()` embeds that string, so every index written before the counter
existed is discarded rather than read as `unpriced=0`, which would have
disclosed full coverage silently. `TestC037R6` asserts both halves.

Cold and warm disclose identically on the 100%, 50% and 0% corpora
(`TestC037R5`). No warm-path code was needed: `cachedUnit.Unit` stores the whole
`costUnit`, so the counter rides the index. Accumulating it as a local in the
cold walk is the defect that sank the first EC-00 repair, and it was avoided for
that reason.

## Unchanged

`CostUSD`, `TotalUSD`, `RebilledUSD`, `RebilledTokens`, request-level coverage,
C035's wording, `replay since`, the other re-billed consumers, and the
singular/plural cosmetic bug.

## Post-repair surface audit, 2026-10-02

Two concrete defects found, both inside C037's own surface, both repaired here.

**1. A scope error of mine, reverted.** The audit noticed that `costLaneRow`
omits the counter, so `replay cost --per-task --per-lane --json` emits
`rebilledUsd` beside `rebilledTokens` on each lane row with nothing qualifying
them. I treated that as a C037 defect and added the field. It is not: C037 is a
claim about the summary pair, and `costLaneRow` is a renaming projection for a
different consumer, with no consumer shown to need the counter. **The struct
and `laneRows` are byte-identical to HEAD again**, and `TestC037R8` now asserts
the lane rows carry no counter, so a future widening has to come with its own
claim. The lane-row shape remains an open observation, not a C037 finding.

**2. Stale line citations.** The repair added about thirty lines of comment to
`cost.go`, so every line number this file and the register cited had moved.
Corrected above. Nothing in the repository validates a cited line number, so
this was found by re-resolving each one with grep rather than by a guard.

### A second path inside the same command, NOT repaired

`replay cost --usage <file>` has the identical two-population shape:

| | |
|---|---|
| `costusage.go:194` | `sDeficit += d`, unconditional |
| `costusage.go:196` | `sRebilled += ...` only `if priced` |
| `costusage.go:267` | renders `re-billed $X` and `N tokens re-billed across M cache break(s)` as one pair |

Established by reading the code, **not measured**, and deliberately left
unrepaired: the claim stays bounded rather than being widened speculatively.
The register and its controls now name this path explicitly, so the bound reads
as "the transcript path of `replay cost`" rather than "`replay cost`".

### Verified clean

No `DisallowUnknownFields` anywhere in production, so the new JSON key breaks
no decoder. `replay cost --contribute` copies named fields into
`observation.Corpus`, whose strict `corpusKeys` allow-list is a different type
with a different field set, so the key never reaches it. `replay since`,
`internal/tui`, `internal/card` and `internal/observation` read `RebilledUSD`
and `RebilledTokens` and are untouched.

## Campaign A Phase 0 closure, 2026-10-02

Baseline: branch `fix/compaction-observed-vs-inferred`, HEAD `dc4686b`,
`git diff --check` clean, `gofmt` clean, build ok.

The eight established C037 semantics, re-verified mechanically:

| | status |
|---|---|
| `RebilledTokens` covers every break deficit | `TestC037R8` I2, both row shapes sum to it |
| `RebilledUSD` covers only represented deficits | `TestC037R1`, `R3` |
| coverage is token-weighted, not break-count-weighted | `TestC037R2`, 96% against 25% on one corpus |
| unpriced re-billed tokens stay explicitly represented | `TestC037R1` 0% case, `R8` |
| conservation at unit, fold and summary | `TestC037R8`, six corpora |
| `RebilledUSD > 0` implies a represented population | **was untested; now `TestC037R8` I3a** |
| documentation stays clean where nothing is excluded | `TestC037R1` 100% case, `TestC037_P3` |
| nil-request pricing evidence cannot disappear silently | structural only, see below |

Two findings from this pass.

**The implication had no test.** `RebilledUSD > 0` implying a represented
population was structurally true, because `RebilledUSD` only grows on the branch
that routes the same break's tokens to the represented side, and nothing
asserted it. I3a now does, and the mutation that makes every unit's break
tokens excluded while the dollars survive is **KILLED**. The converse is
deliberately not asserted: `TestC037R4` shows a model priced at $0.00/MTok is
representable, so represented tokens can exist with no dollars.

**The nil-request arm is UNREACHED and is not claimed as tested.** Replacing
`if br.Turn.Request == nil { continue }` with a panic produces no panic anywhere
in the suite. The arm is correct by construction, since it falls through to the
excluded bucket, and no fixture reaches it. Recorded as structural, not as
coverage.

Mutation suite at Phase 0: control **SURVIVED**, M1 to M12 **all KILLED**.
Full suite **37 ok, 0 FAIL**. Race clean, forced uncached, on `cmd/replay`,
`internal/analysis`, `internal/claims`, `internal/cachemodel`.
