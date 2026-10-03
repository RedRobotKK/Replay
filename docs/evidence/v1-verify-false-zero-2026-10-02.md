# V1: `replay verify` admitted unpriceable rows into the figure meant to face an invoice

**2026-10-02. 1.0 hardening campaign. RELEASE BLOCKER, REPAIRED.**

## The defect

`compare()` in `cmd/replay/verify.go` built its before/after medians from every
row with no priceability test:

```go
for _, u := range us { costs = append(costs, u.CostUSD) }
```

`cost.go:447-456` already states the governing rule for its own median, in the
same binary on the same struct: "Admitting an unpriced row's zero would move the
median and the p90 by asserting that a session nobody could price cost nothing,
which is the same false zero one column over." `verify` did not apply it.

It also counted those rows toward `minSideTasks = 10`, the evidence gate whose
own comment says "two sessions is an anecdote, and this is the figure an
argument would rest on".

## Measured, before the repair

Ten priced rows at $1.00 on each side. Eleven rows added to the AFTER side whose
records were read and none of which priced:

```
the after-median is $0.0000 ... pulled it off $1.00
verify reports a -100.0% median move between two sides whose priceable rows
  are identical
Enough=true with 10 after-rows, none of which could be priced
```

**A 100% saving, manufactured entirely out of missing evidence**, on the one
figure in the product meant to face an invoice.

## A correction to my own first attempt, which is the more useful finding

The first repair gated on `PricedRequests > 0`. It turned three existing tests
red, whose fixtures build `costUnit{CostUSD: 1}` carrying **no counters at all**.

The fixtures were right and the guard was wrong. A row with no counters is
ABSENCE, not a statement that nothing priced, and excluding it converts missing
evidence into evidence of nothing. That is the first invariant in this
repository.

The predicate therefore distinguishes three states, not two:

| state | counters | treatment |
|---|---|---|
| demonstrably unpriceable | `UnpricedRequests > 0 && PricedRequests == 0` | **excluded** |
| priceable | `PricedRequests > 0` | counted |
| no coverage information | both zero | **counted**, because absence is not a negative |

The predicate reads the counters rather than the `Unpriced` flag, because Q01
established that on a folded row that flag is inherited from whichever lane was
seen first; the counters are summed in the fold and are immune to lane ordering
and to the cost index.

A priced row that genuinely cost $0.00 is a measurement and still counts.

## The exclusion is named, not folded away

`BeforeCounted`/`AfterCounted` are separate from `BeforeTasks`/`AfterTasks`, and
the report says so:

```
  N further session(s) were read and priced nothing, so they are in
  neither median. They are excluded, not free.
```

## Mutations

| | verdict |
|---|---|
| C0 reorder the conjunction (control) | **SURVIVED**, as required |
| M1 admit unpriceable rows again, the pre-repair state | **KILLED** |
| M2 treat ABSENCE as unpriceable, my own first attempt | **KILLED** |
| M3 exclude from the median but not from the evidence gate | **KILLED** |
| M4 exclude priced genuine zeros too | **KILLED** |

M2 is the one worth keeping: the three pre-existing tests are what kill it, so
the repository already defended the absence/negative distinction before this
campaign noticed it.

## Limitations

`comparison` carries no JSON tags and is text-rendered only, so no published
shape changed. This repairs the median and the gate; it does not touch
`VolumeDelta`, which counts rows and is correct to count every row observed.
