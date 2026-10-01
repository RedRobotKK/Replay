# RB oracle, completed and frozen 2026-09-30 before the ranking fixture ran

Supersedes 5e13589c (which covered RB2 only and did not cover the ranking
chain). The RB2 result below was already measured under that earlier freeze
and is carried forward unchanged, not re-frozen.

## The four states the oracle must distinguish
  S1 priced model   + known nonzero token deficit -> tokens reported, dollars reported
  S2 unpriced model + known nonzero token deficit -> TOKENS REPORTED, dollars NOT
  S3 priced model   + genuine zero deficit        -> zero, distinguishable from S2
  S4 unknown/unavailable deficit                  -> no figure, not a zero

The oracle derives the token quantity from the FIXTURE SPECIFICATION. It never
calls the production calculation to learn what to expect.

## The chain under attack
  br.Deficit  (pure token count, internal/analysis/diff.go:48, t.Expected-t.Actual)
    -> deficit accumulated at cmd/replay/cost.go:703-712, before any price lookup
      -> u.RebilledTokens, assigned at cost.go:717 INSIDE the priced branch
        -> since.go:105 worstByRebilledTokens, which ranks windows by that field

## Already measured under the earlier freeze
RB2: two corpora differing only in a model id report 40,000 and 0 re-billed
tokens. The human report loses its re-billed line. Attribution corrected by
mutation: the quantity is gated TWICE, at cost.go:680 and again at :717, and
removing either alone changes nothing.

## FROZEN PREDICTION for the ranking experiment, not yet run
Session A: priced, SMALLER known deficit.
Session B: unpriced, LARGER known deficit.
Oracle winner by re-billed tokens: B.

PREDICTED product winner: A, because B's deficit collapses to zero before the
ranking sees it. If so this is a SECOND observable consequence of the same
compression point rather than a second defect.

POSITIVE CONTROL, also frozen: both sessions priced, B larger. The product
must select B. If it does not, the ranking machinery cannot discriminate at
all and the experiment above proves nothing about the price boolean.

## The architectural question this is really asking
Can Replay know the token quantity while legitimately not knowing the dollar
price? If yes, the two must not share a gate. The answer is derivable from
source independently of any fixture: br.Deficit is computed from Expected
minus Actual, neither of which consults a price table.

## Pre-committed
No production change. If the ranking prediction fails, the consequence is
withdrawn and EC-00 stays a single-surface finding.

---

# Result

## The architectural question, answered

**Can Replay know the token quantity while legitimately not knowing the dollar
price? YES, and it does.**

`br.Deficit` is `t.Expected - t.Actual` (internal/analysis/diff.go:48). Neither
term consults a price table. The deficit is accumulated at cost.go:703-712
before any price lookup runs. The control arm proves it empirically: the same
corpus, priced, reports the deficit correctly.

**So the two quantities are independent in fact and coupled in code.** That is
the architectural finding, and it is stronger than the contract violation.

## RB2, the direct consequence

Two corpora differing only in a model id:

| | re-billed tokens | re-billed USD |
|---|---|---|
| priced | **40,000** | $0.200000 |
| unpriced | **0** | $0.000000 |

The human report loses its re-billed line entirely.

## RB5, the ranking consequence, measured separately

**Control, both priced:** A=10,000 B=40,000, product ranks B worst. Correct.

**Treatment, B unpriced, only the model NAME changed:**

| | oracle | product |
|---|---|---|
| sess-a | 10,000 | 10,000 |
| sess-b | **40,000** | **0** |
| winner | **sess-b** | **sess-a** |

**`replay since` names the wrong session.** Its own doc comment calls that
"the single worst thing this command can do, since naming the right one IS
the product", and records that the comparison was reported UNREACHED because
every fixture carried identical figures.

## Mutations

| Mutation | Outcome |
|---|---|
| Remove BOTH gates, the candidate repair | **KILLED.** RB5 flips, so it is load-bearing |
| Remove the session gate alone | **SURVIVED.** Not sufficient |
| Remove the price gate alone | **SURVIVED.** Not sufficient |
| Force an unpriced nonzero deficit to zero | **INAPPLICABLE.** That is the current behaviour, not a mutation of it |
| Make the ranking consume an independent token count | **INAPPLICABLE.** No such count exists at the ranking layer; it reads the collapsed field |

## Two defects in the tests, recorded

**My first attribution was wrong.** I named cost.go:717 and the mutation
survived. The quantity is gated twice and control never reaches 717.

**My first ranking fixture read an empty array.** The `tasks` array is
conditional on `--per-task` and I omitted the flag, so two absent values read
as two zeroes. The control arm caught it before any conclusion was drawn.

## Claim structure

**One compression defect with two observable surfaces**, not two claims. Both
consequences flow from the same pair of gates on the same quantity, and
removing them fixes both at once. Recorded under RPL-C034.

## Proposed repair boundary, NOT applied

Two sites, and both are required:

1. `cost.go:680` must not drop a session merely because it prices to zero. A
   session with an unpriceable model still has countable tokens.
2. `cost.go:717` must assign `RebilledTokens` outside the price branch.

`worstByRebilledTokens` needs NO change. It discriminates correctly, proven by
RB4, and it is downstream of the collapse rather than part of it.

Everything else in the EC-00 census needs no repair. See the retracted
census in the campaign report.
