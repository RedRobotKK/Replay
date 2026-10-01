# C004 RED oracle, frozen 2026-09-30 before inspecting any UNSEEN output surface

## Disclosure of prior exposure
Two surfaces were already inspected in earlier turns of this campaign and this
freeze cannot apply to them:
  - `replay cost` human report (SEEN)
  - `replay cost --json` (SEEN)
Every other surface below is UNSEEN at the moment of this freeze.

## Operational claim
C004: every printed figure carries an explicit, truthful measurement tier.

"Figure" means every user-visible numerical or material assertion: token
counts, prices and rates, calculated cost, usage totals, cache quantities,
percentages and ratios, timestamps and date ranges where they establish
measurement scope, reconstructed quantities, report summaries, machine
readable output, quantitative claims inside warnings and errors, and
documented examples presented as actual results.

## The oracle
For each figure the oracle determines, WITHOUT calling Replay's own tier or
classification code:

  figure -> source evidence -> transformation -> epistemic status -> displayed status

It is a reference judgement built from the fixture's own ground truth, which
the test constructs and therefore knows independently.

## Proposed split, to be confirmed by the evidence
C004 is suspected to be three claims with three different proof obligations:
  C004a  a truth vocabulary exists
  C004b  every relevant output carries a truth status
  C004c  the status carried is epistemically correct

## FROZEN PREDICTIONS, one per sub-claim
- C004a: the vocabulary EXISTS as words and is NOT a type. Already evidenced:
  no enum, 15+ scattered literals across 11 packages. Predicted BOUNDED at
  best, because two documents disagree on what the words are.
- C004b: FALSIFIED. Predicted that a majority of output surfaces carry no
  tier at all. The two seen surfaces already split one-for-one.
- C004c: NOT_MEASURED. Predicted that correctness cannot be assessed because
  there is no machine-readable status to compare against a ground truth. An
  absent status cannot be wrong, only missing.

## Pre-committed rules
- "The strings exist" is NOT evidence that the mechanism works.
- If no machine-enforced tier mechanism exists, the verdict is
  UNRESOLVED / NOT ESTABLISHED with the evidence "tier vocabulary exists, no
  typed enforcement path exists". That is a result, not a prompt to build one.
- No implementation is repaired in this pass.
- If the specification conflict has no later authoritative resolution, the
  VOCABULARY itself is classified unresolved rather than a third tier invented.

---

# Result, appended after measuring

## The precondition finding, which is evidence about the mechanism

| | |
|---|---|
| Tier type or enum | **none** |
| Field carrying a tier beside a figure | **none** |
| Vocabulary | 15+ free string literals across 11 packages |
| ADR-0002 | two tiers: estimated, measured. **Does not contain the word structural** |
| README | three tiers, adding structural |
| ADRs superseding 0002 | **none of 28** |

**The specification is in conflict before compliance is measured.**

## Output surface inventory

14 CLI surfaces sharing one callable signature, driven over a ledger fixture.

| | |
|---|---|
| Ran against this fixture | 9 of 14 |
| Emitted a figure | **4** |
| Of those, carried a status statement | **3** |

**`replay advise` is a CONFIRMED COUNTEREXAMPLE: it emits 10% without a status
statement.** Isolated by removing it from the sweep, where figures fall 4 to 3
while status holds at 3.

**The campaign does NOT establish that it is the only such surface.** Five of
fourteen did not run and are unmeasured, and the JSON row is under-detected,
so the census is incomplete in two directions.

**The JSON surface, as two separate facts.**

1. **Direct structural inspection established** that `replay cost --json`
   attaches no tier, provenance or basis field to any of its numerical values.
   Seven top-level keys, none of them one. That finding stands on its own.
2. **Separately, the figure detector under-detected JSON**, because it looks
   for a dollar sign while JSON emits bare numbers such as `totalUsd`. So the
   sweep did NOT exhaustively inventory JSON figures.

**The detector's limitation is campaign evidence, not a product pass.** The
inventory row for that surface must not be read as one.

**Five of fourteen surfaces did not run** against this fixture (budget,
ceiling, prefix, route, and one further) and are UNMEASURED. They are neither
passes nor failures.

## The three-way split

| Claim | Obligation | Result |
|---|---|---|
| **RPL-C027** | a truth vocabulary exists | **UNRESOLVED** |
| **RPL-C028** | every relevant output carries a status | **REFUTED** |
| **RPL-C029** | the status carried is correct | **NO_ENDPOINT** |

C029 is NO_ENDPOINT rather than NOT_MEASURED, and the distinction is the
finding: nothing was left unmeasured, there is simply no status that could be
correct or incorrect.

## Mutations

| Mutation | Outcome |
|---|---|
| Resolve the spec conflict in README | **killed** |
| Widen the status detector to match everything | **killed** |
| Neutralise the figure detector | **killed** |
| Remove an output path from the sweep | **observable**, isolates advise |
| Alter a tier/value association | **INAPPLICABLE. No association exists** |
| Flip observed to estimated, or estimated to measured | **INAPPLICABLE. No typed status exists to flip** |

**Three of the six mutations the attack called for cannot be performed, and
that is the result rather than a shortfall in the attack.**

## Not repaired

Nothing was fixed. Establishing what the product does came first, and it does
not yet have a mechanism to repair.
