# C005 oracle, frozen 2026-09-30 before running the fixture matrix

## Disclosure of prior exposure
The partially-measurable case (fixture 3/10) was measured in an earlier turn
and this freeze cannot apply to it. It is rebuilt from scratch here with an
independent oracle, as instructed, but it is not blind. Every other fixture
below is unseen.

## The refusal path, discovered from source
cmd/replay/cost.go, inside forEachSession:

    if err != nil || rep == nil || session == nil { unreadable++; return nil }
    ...
    asRun := analysis.AsRunSession(session)
    if asRun.CostUSD <= 0 { unpriced++; return nil }

Two counters, both PER SESSION FILE:
  unreadable  the file produced no session at all
  unpriced    the WHOLE session priced to <= 0

Neither is per record. There is no third state.

## The oracle
For each fixture the test knows the ground truth because it constructs it, and
classifies each requested quantity WITHOUT calling Replay:

  OBSERVED      provider reported it and it is priceable
  RECONSTRUCTABLE derivable from local artifacts
  UNKNOWN       evidence present, insufficient to resolve
  CONTRADICTORY two records disagree on one quantity
  NO_ENDPOINT   no evidence endpoint exists for the fact
  OUT_OF_SCOPE  deliberately outside Replay's measurement

## FROZEN PREDICTIONS, per fixture
1 fully measurable      -> normal output, no disclosure. EXPECT PASS
2 completely unmeasurable -> refusal with a reason. EXPECT PASS
3 partially measurable  -> total over the measurable half, NO disclosure,
                           unpriced=0. EXPECT FAIL (already observed once)
4 mixed epistemic classes -> one authoritative figure, distinctions lost.
                           EXPECT FAIL
5 contradictory evidence -> PREDICT INAPPLICABLE at this layer: the ledger
                           reader has no two-source reconciliation for one
                           quantity, so there is nothing to contradict
6 stale evidence        -> PREDICT the price table's own staleness note is the
                           only stale mechanism, and it is not per record
7 missing endpoint      -> PREDICT indistinguishable from fixture 2
8 zero vs absent        -> PREDICT FAIL. `CostUSD <= 0` cannot tell a session
                           that genuinely cost zero from one that could not be
                           priced, so a true zero is reported as unpriced
9 cross-session contamination -> PREDICT PASS, since the join refuses
                           synthesised ids and sessions are read per file
10 aggregation attack   -> PREDICT a grand total labelled as what the work
                           cost, no subtotal, unpriced=0, no explanation

## Pre-committed rules
- An INAPPLICABLE fixture is recorded as such, not forced into a pass or fail.
- NOT_MEASURED, NO_ENDPOINT, UNKNOWN, PARTIAL and ZERO are kept distinct. If
  the product's taxonomy cannot express one, that is an epistemic finding and
  is recorded rather than silently mapped.
- No production code is repaired.

---

# Result

## Fixture matrix, measured

| id | fixture | oracle says | product says | verdict |
|---|---|---|---|---|
| MX1 | fully measurable | 0/2 unmeasurable | unpriced=0, silent | **CORRECT** |
| MX2 | completely unmeasurable | 2/2 | unpriced=1, discloses | **CORRECT** |
| MX3 | partially measurable | 2/4 | **unpriced=0, silent** | **FAILS** |
| MX4 | mixed across sessions | 2/4 | unpriced=1, discloses | **PARTIAL.** The wholly unpriced session is disclosed; the half unpriced one is not |
| MX8 | zero versus absent | **0/1, a real ZERO** | **unpriced=1** | **FAILS** |
| MX9 | cross-session foreign evidence | 1/2 | unpriced=1, discloses | **CORRECT.** No rescue |
| MX10 | aggregation attack | 2/4 | unpriced=0, silent | **FAILS** |

## Two defects, one line

Both come from `if asRun.CostUSD <= 0 { unpriced++; return nil }`.

**D1, under-report.** A mixed session has a cost above zero, so it never
increments the counter. Its unpriceable records contribute nothing to the
total and nothing is said. MX3 and MX10.

**D2, over-report.** `<= 0` cannot tell a session that genuinely cost nothing
from one that could not be priced, so a real zero is reported as unmeasurable.
MX8.

They are inverses, and fixing one by moving the comparison would worsen the
other. The correct repair is a per-record count plus a separate zero state,
and it is not made here.

## Taxonomy finding

The oracle needs five states: OBSERVED, UNKNOWN, ZERO, PARTIAL, NO_ENDPOINT.

**The product has two counters and no third state.** PARTIAL and ZERO are
inexpressible, which is why D1 and D2 exist at all. Recorded as an epistemic
finding rather than mapped onto the nearest available value.

## Two defects in the TEST, recorded not hidden

**MX8's first version did not test what it claimed.** It set input and output
to zero and left 9,000 cache reads priced, so the session was not free. Fixed
with an explicit `free` flag that zeroes every usage field, after which the
defect appeared.

**MX2's first disclosure detector missed the wholly-unpriced report**, which
says "No transcript could be priced" and contains none of the three phrases
the detector looked for. The gap was mine and is recorded rather than patched
silently.

## Mutations

| Mutation | Outcome |
|---|---|
| Zero no longer treated as unpriced | **KILLED** |
| Refusal path removed entirely | **KILLED** |
| Disclosure message dropped | **KILLED** |
| Count transcripts instead of records | **INAPPLICABLE.** That is the current behaviour, not a mutation of it |
| Substitute an estimated rate when evidence is absent | **INAPPLICABLE.** No such fallback exists |
| Accept stale evidence | **INAPPLICABLE.** No per-record staleness mechanism exists |
| Resolve contradictory evidence first-wins or last-wins | **INAPPLICABLE.** The reader has no two-source reconciliation for one quantity, so there is nothing to contradict |
| Accept foreign evidence | **Covered by MX9**, which shows it is not accepted |

**Four of ten are inapplicable because the architecture has no corresponding
mechanism.** That is the result, not a shortfall in the attack.

## Split

| Claim | Obligation | Result |
|---|---|---|
| **RPL-C030** | refusal correctness at session granularity | **BOUNDED** |
| **RPL-C031** | partial-coverage correctness | **REFUTED** |
| **RPL-C032** | zero versus absent | **REFUTED** |

Explanation correctness and aggregation correctness were NOT split out: the
explanation is accurate whenever it fires, and the aggregation failure is the
same defect as C031 rather than a separate one.

## Not repaired

No production code changed.
