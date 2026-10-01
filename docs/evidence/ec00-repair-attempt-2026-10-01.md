# RPL-C034 post-repair invariants, stated 2026-09-30 BEFORE any production edit

I1 TOKEN     If br.Deficit is known, RebilledTokens == br.Deficit, whether or
             not a price exists.
I2 DOLLAR    If price is unavailable, RebilledUSD must not silently become a
             measured $0. The existing `unpriced` disclosure must still fire.
I3 RANKING   `replay since --per-task` ranks by known re-billed tokens even
             when the dollar price is unknown.
I4 ZERO      A genuinely zero deficit stays distinguishable from an
             unavailable one.
I5 REGRESSION A fully priced session behaves exactly as before. Measured by
             the existing suite, not by assertion.

## Blast radius to measure, not to reason about
Letting a previously dropped session through gate 1 adds a row to the task
list. MedianUSD and P90USD are computed over those rows, so a $0 row could
move them. I5 is therefore decided by running the full suite, and if an
existing test moves, the repair is wrong or its scope is larger than claimed.

---

# Result: the repair was attempted and WITHDRAWN

## What was built
Four edits to cmd/replay/cost.go, in the codebase's own presence-field idiom:
a `Unpriced bool` on costUnit; gate 1 at :680 flagging instead of dropping;
`RebilledTokens` hoisted out of the price branch at :717; and `summarise`
taking its dollar statistics over priced rows only, so an unpriced row could
not put a false zero into the median.

## What it achieved
| Invariant | Result |
|---|---|
| I1 TOKEN, deficit reported without a price | **held**, 40,000 on both arms |
| I2 DOLLAR, no false measured $0 | **held** |
| I3 RANKING, `since` names the larger | **held**, product named sess-b |
| I4 ZERO, genuine zero distinguishable | **held** |
| **I5 REGRESSION** | **VIOLATED** |

`TestAnUnpricedSessionIsExcludedAndCounted` failed first: `summary.tasks`
counts PRICED rows and the repair made it 2. That was resolved narrowly by
`s.Tasks = len(costs)`, after which the existing contract test passed.

## Why it was withdrawn
A second regression, which the narrow fix did not reach.

**An unpriced session that now produces a unit gets CACHED.** On the warm
index path the cached row is counted as priced, and the `unpriced` disclosure
reads zero.

Measured on one corpus, one wholly unpriceable session:

| run | `unpriced` |
|---|---|
| cold | **1** |
| warm | **0** |

**This is the same defect `TestRJ3` already guards for the unjoinable count**,
which exists because "a disclosure that survives only while the cache is cold
is a disclosure that disappears the moment anyone uses the tool twice."

So the repair traded a token-suppression defect for a disclosure-suppression
defect, and the second is the kind the repository has already been bitten by
once.

## The corrected repair boundary
Not two sites. **Three.**

1. `cost.go:680` must flag rather than drop.
2. `cost.go:717` must assign `RebilledTokens` outside the price branch.
3. **The cache/index path must carry the unpriced flag**, exactly as it
   already carries the unjoinable count, or the disclosure dies warm.

`worstByRebilledTokens` still needs no change. RB4 proves it discriminates.

## State
Reverted. `cmd/replay/cost.go` is byte-identical to 8582262. RB2 and RB5 are
restored to defect-pins that state the invariants a repair must satisfy and
fail the moment one lands.

## The central distinction, unchanged by any of this
**Replay already possesses the token evidence. The bug is not missing
evidence. The bug is discarding known token evidence because an unrelated
dollar-price lookup failed.**
