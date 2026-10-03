# C035: partial priceability now survives to the user. CLOSED

**UNCOMMITTED pending review.** Working title C035; the registered claim this
belongs under is RPL-C031.

## Decision: Candidate A, paired counters

Chosen on two measured facts, not on field count.

**`burn.go` has its own `requests` total and deliberately did not use it as the
coverage denominator**, computing `priced/(priced+unpriced)` at `burn.go:153`.
The one precedent that actually solves this problem decoupled coverage from its
request total on purpose.

**`costusage.go:157-168` already increments `rep.UnpricedRequests` separately
while its `Requests` counts priced records only.** Two divergent production
meanings of `Requests` exist today, so Candidate B's `priced = Requests −
Unpriced` would have been correct only by coincidence of which path built it.

## Invariant

    PricedRequests + UnpricedRequests == Requests

Self-checking by construction: `Requests++` is unconditional at
`replay.go:59` and exactly one of the two branches fires.

## Verified states

Signature is `unpriced / priced / unpriceable`.

| State | Signature | Disclosure |
|---|---|---|
| ALL PRICEABLE | **0/4/0** | none, correctly |
| MIXED | **0/2/2** | "covers 50% of the requests read" |
| NONE PRICEABLE | **1/0/4** | existing behaviour, unchanged |

## Oracle

The oracle's disclosure detector was blind to the new sentence and was widened,
then re-proven to discriminate: it must fire on MIXED and must not fire on ALL
PRICEABLE, both asserted. **The gap was found by reading the output, not by
trusting the green result** — the same failure class as the C004 false pass.

## Mutations

Five applied, five killed. The `summarise` fold survived its first attempt
because the anchor matched `foldSessions` at line 236 rather than `summarise`
at line 398. Re-targeted, killed. **Mis-targeted mutation, not a weak test** —
and it exposed the gap recorded as C036 below.

## Regression

37 packages, 0 failures. Race clean. Two production files changed:
`internal/analysis/replay.go` and `cmd/replay/cost.go`.

## Preserved

`Requests` = records observed. `CostUSD` = priced subset. Unpriceable is not
zero-cost: the disclosure says the excluded records "are not free; what they
cost is not established here."

---

# Follow-up claims, verified 2026-10-01

## RPL-C035-LF, the lane fold. PROVEN, and the gap is closed.

`foldSessions` merges rows that share a session id, and those rows come from
separate FILES: Claude Code writes a sub-agent lane to
`<session>/subagents/agent-<id>.jsonl` carrying the parent's session id
(cost.go:35,158). C035's fixture was single-file, so cost.go:236 was never
reached and a mutation against it survived.

**LF1** builds two lane files under one session id, priced on one lane and
unpriceable on the other, and asserts the folded row reports 2 and 2 with
`priced + unpriceable == requests`.

| | |
|---|---|
| Result | **2 lanes folded into 1 row, priced=2 unpriceable=2 requests=4** |
| Mutation at cost.go:236 | **KILLED** |
| The same mutation against PT1 | **survived**, which is what proves LF1 reaches a path PT1 cannot |

**It was a proof gap, not desirable coverage.** The C035 claim is about
sessions, sessions span lanes, and a broken fold would falsify it for every
multi-lane session while leaving the single-lane proof intact.

## RPL-C035-RB, the population mismatch. PROVEN.

Prediction frozen as `b929d5e9` before the fixture ran. Both halves held.

| corpus | TotalUSD | RebilledUSD |
|---|---|---|
| 2 priced | $0.270000 | $0.100000 |
| the same 2 plus 2 unpriceable | **$0.270000** | **$0.300000** |

**The denominator did not move and the numerator tripled.** They cover
different populations, which is the finding; the 222% ratio observed on a
1-priced/3-unpriceable corpus is a consequence of it.

Mutations: zeroing the numerator **KILLED**, removing the deficit accumulation
**KILLED**. Zeroing only the token field **survived**, correctly: RB1 asserts
on dollars, and that control is recorded rather than hidden.

## RPL-C035-FM, first-model pricing. PROVEN, and it is a distinct claim.

RB is about WHICH RECORDS enter the numerator. FM is about WHAT RATE it is
monetised at. Separately falsifiable, so separately registered.

Identical 60,000-token deficit, identical set of models, order reversed:

| first record | rate | re-billed |
|---|---|---|
| claude-opus-5 | $5.00/MTok | **$0.300000** |
| claude-3-5-haiku | $0.80/MTok | **$0.048000** |

**A 6.25x difference decided by arrival order alone.** Mutation removing the
rate from the numerator: **KILLED**, after the oracle was hardened.

### A defect in the oracle, recorded

FM1's first version logged a withdrawal and returned when the two orderings
priced identically, which made it blind to the rate being removed altogether:
both orderings then match and the test passed. **The third time a logged
finding has failed to kill a mutation in this campaign.** The branch is now a
hard failure, justified by the fixture's own precondition that the two models
price differently.

**No repair was made to any of the three.**

---

# FM repaired, RB resolved by it. 2026-10-01

## The repair

Oracle frozen as `ceead7e7` before the production change.

`Break.Turn.Request` carries the model and the timestamp (calibrate.go:23), so
the per-record rate was always available where the deficit is summed. The
first-request rate was a choice, not a constraint.

**One site, `cmd/replay/cost.go`, the deficit loop.** Each break is monetised
at its own record's rate and accumulated alongside the tokens, instead of the
session total being monetised once at `Requests[0].Model`. Nothing in
`internal/analysis` changed.

## Before and after

| | before | after |
|---|---|---|
| 4 records, lead expensive, breakers cheap | **$0.300000** | **$0.048000** |
| same, lead cheap | $0.048000 | $0.048000 |
| 2 unpriceable records added | **added $0.032** | **adds $0.000** |
| their deficit tokens | kept | kept |

## An oracle defect, found by the repair

F1's first version compared three permutations of the same records and
demanded identical pricing. **That cannot hold:** a break needs a predecessor,
so the first record never breaks, and permuting changes WHICH records break.
Per-record pricing then legitimately differs.

The real defect was never "order matters", it was "the rate comes from
`Requests[0].Model`". F1 now holds the BREAKING records identical and varies
only the lead record, which cannot break and must therefore not affect the
figure. **The repair exposed the flaw in the test, not the other way round.**

## RB: RESOLVED BY FM, no second repair

The prediction frozen with the oracle held. With each break priced at its own
record's rate, an unpriceable record contributes tokens and no dollars, so the
numerator now covers the population the denominator does.

| corpus | TotalUSD | RebilledUSD | tokens |
|---|---|---|---|
| 2 priced | $0.270000 | $0.100000 | 20,000 |
| plus 2 unpriceable | $0.270000 | **$0.100000** | **60,000** |

The 1-priced/3-unpriceable corpus reports **0% instead of 222%**, correctly:
the only priced record is the lead, which never breaks.

**No percentage was clamped.** The ratio fell because the numerator stopped
counting records the denominator excludes.

Both defect-proofs were inverted rather than deleted, so neither defect can
return unnoticed, and RB1Pop additionally asserts the deficit TOKENS still
grow, because fixing a ratio by discarding evidence would re-break EC-00.

## Mutations, against the repaired path

| | |
|---|---|
| Revert to the session-global first-request rate | **KILLED** |
| Discard a record's contribution | **KILLED** |
| Bypass per-record selection entirely | **KILLED** |
| Discard deficit tokens | **KILLED** |
| No-op control | **SURVIVED**, as required |
