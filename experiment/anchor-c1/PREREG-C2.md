# Preregistration: addressability versus destination correctness

Written **before any run**. Follows the C1 discriminator, which established that
an addressable anchor moves retrieval from 6/25 to 25/25, and that a wrong but
resolvable path still reaches 19/25.

## Question

C1 could not separate two things. Does a fresh agent benefit from **having
somewhere to go**, or from the destination **actually holding the evidence**?

## Arms, three, all re-run fresh

Re-running A and B rather than reusing C1's counts, so drift between experiments
cannot be mistaken for an arm effect.

| arm | anchor line |
| --- | --- |
| **A** | none |
| **B** | `docs/evidence/break-causes-2026-09-13.md [dispositive]` |
| **C2** | `docs/evidence/codex-cache-breaks-2026-09-07.md [dispositive]` |

**C2 differs from B in the destination path only.**

## Why this destination

`codex-cache-breaks-2026-09-07.md` is real, 100 lines, and about cache breaks,
so it is maximally plausible to investigate. It is titled "Codex cache breaks
with **no observable cause**" and contains no cause ranking, no `43.5`, no
"expired", and **no reference to the correct artifact**, so it is not a pointer.
It is a different file from C1's wrong-anchor arm, avoiding that confound.

An agent that follows it lands on an honest dead end.

## Primary comparison, fixed now

**B versus C2.** If retrieval is similar but downstream resolution differs, then
destination correctness matters after addressability. If the two are similar all
the way through, correctness matters less than having an executable path.

**B versus A** is retained to confirm the addressability effect is still present.

## Endpoints, strictly separate, never collapsed

1. retrieval of the referenced artifact
2. reach of the **correct** evidence artifact
3. extraction of the relevant fact
4. correct final answer
5. evidence-supported final answer

Also: tool calls, cost, recovery after a wrong destination, and the number of
irrelevant files opened before reaching the correct evidence.

## C2 trajectory classification, fixed now

followed the supplied path / discovered it was irrelevant / searched elsewhere /
found the correct artifact / answered from the aggregate advisory instead /
stopped after the distractor.

## Statistics

Two-sided Fisher exact, bootstrap 95% CIs, raw counts throughout. n=25/arm.
**The design target was a 0.8/0.4 contrast. If the observed B-versus-C2 effect
is smaller, this experiment is underpowered for it and will be reported as
such.** No post-hoc power claim.

## Stop conditions

Harness failure above 10%, arm leakage, answer appearing in any prompt, or
cross-trial contamination. Otherwise all 75 run.

## No implementation follows this experiment regardless of outcome
