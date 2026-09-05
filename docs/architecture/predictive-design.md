# What the new constraints force in the design

**2026-09-04.** Working through ADR-0009 against the code. The constraints do not add features; they
remove a claim and narrow the schema in three specific places.

## The finding that reshapes it

**Replay cannot tell a finished session from an abandoned one.**

The ledger is append-only, one record per request, and **nothing marks the last one**
(`internal/ledger/store.go`). There is an `Outcome` field but it is `CacheOutcome.Outcome`, a
per-response cache classification, not a session verdict. From the proxy's seat, a session that ended
because the work was done and a session the user quit in frustration are the same thing: **requests
stop arriving.**

That is not a gap to fill. It is a property of where the tool sits. And it decides what "predictive"
is allowed to mean.

## Consequence 1: the claim gets smaller, and truer

**Replay can measure waste. It cannot measure success.** So a predictive claim can never be "this
session is heading for failure". It can only be:

> Sessions with this waste trajectory usually go on to waste more.

That is a **statement about spend, not about outcome**, and it is honest because spend is the only
thing observable from here. It also happens to be the useful half: a person who is told at turn 12
that a quarter of the next hour will buy nothing does not need to be told whether they will
eventually succeed.

**Every predictive feature has to be phrased in that frame or it is overclaiming.** The survivorship
problem does not need solving, because the claim no longer depends on knowing how things ended.

## Consequence 2: waste has to become a series, not a scalar

Today `error_share` and the re-read rate are **session totals**. `ReReads.Rate()` divides across the
whole lane. A total tells you what happened; **prediction needs the shape over time.**

The data exists — the ledger is already per-request — but the aggregation throws the trajectory away.
So:

```
  today   session:  { error_share: 0.22, re_reads: 11, breaks: 3 }
  needed  session:  { turns: [ {t:1, err:0.00, rr:0}, {t:2, err:0.05, rr:0}, … ] }
```

**This is a reporting change, not a collection change.** Nothing new is recorded; the per-turn
figures stop being summed before anyone can see them. That matters because it keeps the privacy
position unchanged for the local report.

## Consequence 3: the corpus payload gets harder, not easier

A single ratio is nearly anonymous. **A turn-by-turn waste curve is a fingerprint of a working
session** — its length, its rhythm, where the difficult parts were. That is a real step up in
sensitivity and it must not be waved through because the numbers still contain no text.

So the corpus carries **binned, not raw** series:

- Turn index bucketed (1–5, 6–10, 11–20, 21–50, 51+), never the exact count.
- Waste values rounded to a coarse grid, never full precision.
- Session length reported as a bucket, since exact length is the strongest identifier in the record.
- **Counts, never names.** `unused_tools: 9` and never which nine. Same rule the redactor needs.

The local report keeps full resolution. **The thing that leaves is deliberately blunter than the
thing you see**, which is the opposite of how telemetry usually works and is the point.

## Consequence 4: thresholds move out of flags and into data

Guards are configured today as absolute numbers on the command line, and they default to off because
nobody has a number. Once a distribution exists the flag should accept a percentile:

```
  --error-budget 0.3      absolute, what exists now, always available
  --error-budget p90      calibrated, refuses where the worst tenth of sessions sit
```

Two rules keep this honest. **A percentile is only accepted when an aggregate has actually been
fetched**, and it says which corpus and which date it resolved against, exactly as the cache rules
carry `anthropic-2026-09-01`. And **the absolute form never goes away**, because a user must always
be able to run this with no aggregate, no network and no trust in our numbers.

## What does not change

Consent, the payload shown as bytes, k-anonymity, robust statistics, the aggregate published back,
and no learned rule without a pull request. **Those were right for the cache corpus and they are
right for this one.**

## The order this has to happen in

1. **Per-turn waste in the local report.** No corpus, no network. Defensible on eleven sessions
   because it describes only your own machine.
2. **Session boundaries.** Decide what ends a session. Idle timeout is the cheap answer and it is
   arbitrary; whatever is chosen has to be stated wherever a trajectory is shown, because the last
   bucket of every curve depends on it.
3. **Binned series in the corpus**, once the local view has proven the shape is worth collecting.
4. **Percentile thresholds**, last, and only for models where the aggregate clears k.

**Steps 1 and 2 are worth doing even if nobody ever submits a corpus.** That is the test for whether
this is a product or a data-collection scheme, and it should stay the test.
