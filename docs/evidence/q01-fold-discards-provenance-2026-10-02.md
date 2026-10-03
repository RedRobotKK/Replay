# Q01: foldSessions sums quantities and discards the provenance bits beside them

**2026-10-02. Campaign A Phase 1. PROVEN. Decided by panel. NOT IMPLEMENTED.**

The sections below are in the order they were written. Where an earlier section
disagrees with a later one, the later one is current: the mechanism was
corrected and the A/B/C fork was dissolved rather than resolved.

## The defect

`foldSessions` (`cmd/replay/cost.go:237-292`) turns per-lane `costUnit` rows into
one session row. It sums fifteen quantities. It never recomputes `Unpriced` and
never OR-s `MixedEpochs`.

The folded row takes those two flags from `row := u` at `cost.go:246`, which is
whichever lane of that session was seen first.

**CORRECTED 2026-10-02, after panel review.** The first version of this file said
"files arrive in directory order, so the flags are decided by lane file name."
That is wrong. `cmd/replay/main.go:585-590` sorts **size-descending**, with path
ascending only as a tie-break, so the flags are taken from the **largest** lane
file. The reproduction below works only because its two fixture lanes hold
identical content and therefore tie on size, falling through to the path
tie-break. The defect is real; the cause I published was not.

**And the sharper falsifier, which I had missed.** `cost.go:782-798` appends
every cache-WARM unit first, then walks the cold files. So for a session with one
warm lane and one cold lane, first-seen is the warm lane regardless of size, and
the flags are decided by the contents of `~/.replay/cost-index.json`. That
contradicts the field's own documented invariant eight lines above its
declaration, at `cost.go:83-84`: "a warm run must reach the same disclosure as a
cold one." A warm-versus-cold divergence cannot be produced by a rename and is
the test this repair needs.

There are exactly three writes to either flag in the whole file, and all three
are per-lane at construction:

| | |
|---|---|
| `cost.go:832` | `Unpriced: !priceKnown`, per lane |
| `cost.go:842` | `MixedEpochs: asRun.MixedEpochs()`, per lane |
| `cost.go:997` | the whole-transcript `unpriced` COUNT into the corpus payload, a different quantity |

## Measured

One session, two lanes, identical content in both arms. Lane A leads with a
priceable record and carries two priceable breaks; lane B leads with an
unpriceable record and carries two priceable breaks. The arms differ only in
which lane's file name sorts first, which reaches the result **only through the
size tie-break** described above, not through directory order.

| arm | tasks | totalUsd | medianUsd | rebilledUsd | rebilledTokens | unpricedRebilledTokens | row.Unpriced |
|---|---:|---:|---:|---:|---:|---:|---|
| priceable lane sorts first | 1 | **$0.135000** | $0.135000 | **$0.080000** | 80,000 | 0 | false |
| priceable lane sorts second | **0** | **$0.000000** | $0.000000 | **$0.000000** | 80,000 | **80,000** | true |

Renaming an agent lane file changes a session's reported cost from $0.135000 to
$0.000000, moves `summary.tasks` from 1 to 0, and flips the C037 disclosure from
"nothing excluded" to "all 80,000 tokens excluded".

Reproduction probe preserved outside the repository at
`~/.claude/jobs/581b6292/tmp/q01_probe_test.go.txt`; it uses the C037 fixture
helpers and the lead-unpriced rules document.

## What it invalidates

`cost.go:70-77` defines the flag: "Unpriced means this unit's TOKEN quantities
are known and its DOLLAR quantities are unavailable". For a folded multi-lane
session the flag instead means "the first-sorting lane's dollars were
unavailable", which is not a property of the session and is not what any
consumer reads it as.

Downstream, `summarise`'s priced-only loop (`cost.go:453`) gates `TotalUSD`, the
cost legs, `RebilledUSD`, `Tasks`, the median and the p90 on that flag, and the
C037 re-bucketing at `cost.go:438-442` branches on it too.

**RPL-C037 is not falsified by this.** Its claim is about what the summary
discloses, and the disclosure is computed correctly from the flag it is given.
But C037's correctness is CONDITIONAL on a flag that is ill-defined for a
multi-lane mixed session, and that qualification was not stated when C037 was
closed. It is stated here.

`MixedEpochSessions` (`cost.go:443`, printed at `cost.go:634`) is a silent
undercount by the same mechanism: a session is counted only if its first-seen
lane spans epochs.

## Why this was not repaired in this phase: SUPERSEDED by the panel verdict below

Kept as written, because the reasoning was wrong in an instructive way: all three
options below aggregate a per-lane boolean, and the panel established that the
per-lane boolean is itself the wrong predicate. The preference this section
states for AND was overruled.

Three readings of what `Unpriced` should mean for a folded session, all of which
change a published dollar figure:

- **AND over lanes** (unpriced only if no lane priced). A mixed session becomes
  priced and its partiality is carried by the C035/C037 coverage counters, which
  exist for exactly that. Raises a mixed session's published total from $0.
- **OR over lanes** (unpriced if any lane is). Conservative, withholds more,
  makes the coverage counters largely redundant.
- **Drop the session-level boolean** and gate `summarise` on the coverage
  counters instead. Changes the meaning of `summary.tasks`.

Repository evidence favours AND: `cost.go:70-77`'s own definition is about
whether dollars are AVAILABLE, and C035's counters already express partial
pricing inside a priced row. But all three alter `totalUsd`, `tasks` and the
median for mixed sessions, and `internal/observation` publishes those figures to
a pooled roster other people read. That is a material public-contract change on
a figure already in circulation, so it goes to the panel rather than being
chosen here.

## Not claimed

How often a real corpus contains a multi-lane session with a mixed-priceability
lead. **NOT MEASURED.** The defect is proven to exist and its frequency in the
calibration corpus has not been established.

## Panel verdict, 2026-10-02: the fork was false

Eight seats. Convergence on four points from unrelated directions, one real
split, and a verdict that dissolves the A/B/C question rather than answering it.

**The right predicate already exists in the struct and is record-granular.**
`cost.go:825` computes `priceKnown` from `rep.Lane.Requests[0].Model`, the lane's
FIRST record, while pricing has record granularity at
`internal/analysis/replay.go:85-96`. So both A and B were aggregations of a
predicate that is already wrong. The predicate `summarise` wants at `cost.go:438`
and `:453` is **`u.PricedRequests == 0`**, which is summed in the fold and is
therefore immune to both lane ordering and cache state.

This is RPL-C005 one level up. C005 is already REFUTED for exactly this
granularity mismatch, pinned by
`TestC005_AMixedTranscriptHidesItsUnpricedRecords`.

**Verdict, narrowed:** gate on `PricedRequests == 0`; do NOT delete the
`unpriced` JSON key (ADR-0024 governs `--json` shapes) but make it derived rather
than assignable, so no future `row := u` can carry a stale flag; keep the three
values distinguishable as absence `Requests == 0`, free
`PricedRequests > 0 && CostUSD == 0`, unknown
`PricedRequests == 0 && UnpricedRequests > 0`; and carry per-row coverage into
the per-row output, not only the summary. Do not claim this repairs C005, which
is a separate published field.

`MixedEpochs` was unbundled by all eight seats: a session spans epochs if any
lane does, there is no fork, and `mixedEpochSessions` is a plain undercount.

## The settling measurement, run twice independently

Panel's run, and mine, on `~/.claude/projects` with `HOME` redirected to a
scratch directory so the real index was untouched:

| | panel | mine |
|---|---:|---:|
| sessions / lanes | 981 / 3153 | 981 / 3153 |
| priced requests | 104,902 | 104,917 |
| rows with `unpriced=true` | **0** | **0** |
| rows with `unpricedRequests > 0` | **0** | **0** |
| multi-lane sessions | 29 (3.0%) | 29 (3.0%) |
| dollars in multi-lane rows | $18,707.72 (97.6%) | $18,709.97 (97.6%) |
| totalUsd | $19,176.51 | $19,178.77 |

The small drifts are the corpus growing between runs and are consistent. One
correction to the panel's own figures: it listed lane counts as "2, 4, 9, 10,
571"; the real set is 2, 4, 9, 18, 22, 29, 53, 66, 95, 98, 158, 571 and **1014**.
Its adversarial case was understated, not overstated.

**So the choice is free today.** Incidence is zero, every candidate leaves
`totalUsd`, `tasks`, the median and p90 byte-identical, and the public-figure
objection that sent this to the panel has no subject. What is NOT free is leaving
it: 97.6% of published dollars sit in rows whose flags are selected rather than
computed.

**What would reverse this:** a corpus in which any lane has
`UnpricedRequests > 0`. The moment partially priced rows exist, the per-row
coverage disclosure stops being a condition attached to the repair and becomes
the load-bearing part of it.

## Status

PROVEN. Decided in principle. **NOT IMPLEMENTED** — the repair changes the
RPL-C037 gate and makes a published JSON key derived, and that is a scope
decision for the maintainer rather than for a panel.
