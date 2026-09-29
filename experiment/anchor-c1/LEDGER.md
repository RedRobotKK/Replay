# Anchor research: claim ledger

Frozen from measured values re-read off `c1exp/scored.json`, not from memory.
Written before the C2 discriminator's results are inspected.

## C1, measured

| arm | destination | retrieved | any evidence | cause | share | supported | tools | cost |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| A | none | 6/25 | 6/25 | 10/25 | 6/25 | 6/25 | 4.8 | $0.456 |
| B | `break-causes-2026-09-13.md` (correct) | 25/25 | 25/25 | 25/25 | 25/25 | 25/25 | 3.9 | $0.382 |
| C | `calibration-corpus-2026-09-03.md` (wrong) | 19/25 | 23/25 | 21/25 | 19/25 | 19/25 | 6.2 | $0.517 |

Fisher exact, two-sided: B vs A p = 1.2e-08; C vs A p = 5.4e-04; C vs B p = 0.022.

## The ledger

| # | claim | status | basis |
| --- | --- | --- | --- |
| L1 | An addressable path increases evidence retrieval | **OBSERVED** | B 25/25 vs A 6/25, +76%, CI [+60%, +92%], p = 1.2e-08. Zero trials answered correctly without retrieving, so the endpoint could not be shortcut |
| L2 | A **correct** path is necessary for retrieval | **REFUTED** | C, a deliberately wrong path, reached the correct artifact 19/25 against A's 6/25, p = 5.4e-04 |
| L3 | A correct destination improves downstream resolution | **UNRESOLVED** | C1 cannot separate this. Its arm C was a *topically unrelated* file, so a difference could be destination correctness or destination plausibility. This is what the C2 discriminator exists to test |
| L4 | A correct destination reduces investigation cost | **UNRESOLVED, directionally suggestive** | C1: B 3.9 tools / $0.382 against C 6.2 / $0.517 and A 4.8 / $0.456. Suggestive, not tested: no preregistered cost endpoint or nonparametric test was fixed in C1 |
| L5 | Addressability is itself the mechanism | **SUPPORTED_HYPOTHESIS** | C1 shows most of the retrieval effect survives a wrong path. It does not show that correctness is irrelevant downstream, which is L3 |
| L6 | The aggregate `evidence:` line is a dead end for agents | **OBSERVED** | A-arm trajectories: 19/25 concluded the advisory "can't answer that" and stopped. The failure is not reasoning, it is having no address |

## What C1 measured versus inferred

**Measured:** tool calls naming a path, presence of `43.5` and of `expir` in the
final answer, tool counts, cost, turn counts.

**Inferred, and labelled as such:** that A-arm refusals reflect an absent address
rather than task ambiguity. The trajectories support it; no arm isolates it.

## Endpoint change in C2, declared

C1's primary endpoint was **retrieval**. C2's primary endpoint is
**evidence-supported correct resolution**, which is strictly downstream.

This is a deliberate change, declared before results are seen, because C1 already
settled retrieval and leaving the endpoint there would answer a question that is
no longer open. C1's retrieval numbers are **not** reinterpreted under the new
endpoint; they stand as measured under the old one.
