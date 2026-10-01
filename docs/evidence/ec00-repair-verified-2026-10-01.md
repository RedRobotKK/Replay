# RPL-C034 repaired: before, mechanism, repair, after

**Companion to the earlier files, which are NOT superseded and NOT edited.**
`ec00-rebilled-tokens-2026-09-30.md` holds the original demonstration,
`ec00-repair-attempt-2026-10-01.md` the withdrawn two-site attempt, and
`ec00-cache-representation-2026-10-01.md` the index analysis. This one records
what landed.

## Before

Two corpora differing only in a model id:

| | re-billed tokens | re-billed USD |
|---|---|---|
| priced | 40,000 | $0.200000 |
| unpriced | **0** | $0.000000 |

`replay since` named the wrong session: with the unpriced session carrying
40,000 against the other's 10,000, the command named the smaller one.

## Mechanism

A deficit is `Expected - Actual` and consults no price table, so it is known
whether or not a price is. The quantity was gated TWICE: a session whose model
was unpriceable was dropped at `cost.go:680` before the deficit it had already
computed could reach the report, and the assignment at `:717` sat inside the
price branch. Removing either alone changed nothing.

## Repair, 6794522

Three sites. `costUnit` carries an explicit `Unpriced` flag; the gate flags
instead of dropping so the unit reaches `cache.put`; `RebilledTokens` is
assigned outside the price branch; and the warm path reconstructs the
disclosure from the serialized flag, the same replay `join.addCached` performs
one line above it.

Priceability is now asked of the price table rather than inferred from a cost
of zero.

## After

| | before | after |
|---|---|---|
| unpriced arm, re-billed tokens | 0 | **40,000** |
| `since` winner, B unpriced | sess-a | **sess-b** |
| cold vs warm disclosure | n/a | **equal** |
| genuine zero reported as unpriced | yes | **no** |

Five mutations, five kills. Full suite 37 packages, race clean.

## Contract

`docs/TOKEN-PRICES.md:53` — "An unpriced model must never count as zero" — is
**satisfied for the re-billed token count at `replay cost` and `replay since`,
and is NOT established for the other seven consumers of the price boolean.**
Those were censused from source and never measured at the user-visible
boundary. The census does not become a verification by being quoted twice.

## What the repair did NOT close

**RPL-C031 is unchanged and was re-measured, not assumed.** It is about RECORD
granularity within one session; the repair works at SESSION granularity. MX3
and MX10 still report `unpriced=0` with no disclosure for a session holding two
priceable and two unpriceable records. A record-level counter is a separate
change and was not made.
