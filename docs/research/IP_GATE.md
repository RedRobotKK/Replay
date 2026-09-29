# IP gate

**Deliberately hard. Nothing here is a legal conclusion, and nothing here
authorizes a filing decision. Written 2026-09-29.**

## Current status: NOT CROSSED

Two conditions fail independently, and either alone is sufficient to keep the
gate shut.

**Condition 3 fails.** The 2026-09-29 replication returned NOT REPLICATED: C0
20/20, T2 20/20, Fisher exact p=1.0000. Both arms reached ceiling, so the design
had no headroom and the result does not discriminate absence of the mechanism
from insufficient task difficulty.

### What this status does NOT say

It does **not** conclude that there is no patentable invention here. That is a
legal determination, it belongs to counsel, and nothing in this programme is
positioned to make it.

The correct formulation is narrower:

> **No present evidence justifies treating the hypothesized weak-anchor
> mechanism as an IP candidate.**

**The gate stays available for reopening.** If later evidence establishes a
reproducible, technically specific mechanism that survives prior-art analysis,
the twelve conditions are re-evaluated from the top. A closed gate is not a
closed question.

**Condition 11 fails, and its failure is the more serious one.** A prior-art
search conducted 2026-09-29 found the following already published:

| Claimed distinction | Prior art |
|---|---|
| Evidence-sufficiency gating on agent action | **ECLoop**, [2607.28815](https://arxiv.org/abs/2607.28815), 2026-07-30. Implemented; Pass@1 +4.8 to +11.8 on all 500 SWE-bench Verified |
| Fact versus inference separation | **Hindsight**, [2512.12818](https://arxiv.org/abs/2512.12818). Their stated thesis: existing systems "blur the line between evidence and inference" |
| Premature closure in agents | [2606.22936](https://arxiv.org/abs/2606.22936), 2026-06-22, with a hidden-state detector at AUROC 0.97 |
| A passing check leaving a claim unsettled | **"Building to the Test"**, [2606.28430](https://arxiv.org/abs/2606.28430), 2026-06-26 |
| Verification status changing downstream behaviour | **"Silence Is Endorsement"**, [2609.20211](https://arxiv.org/abs/2609.20211), date REQUIRES VERIFICATION. Effects 5% to 60% and 9% to 98% |
| Evidence sufficiency of a record, benchmarked | **DEMM-Bench**, [2606.20634](https://arxiv.org/abs/2606.20634) |
| Stale-state detection | **STALE**, [2605.06527](https://arxiv.org/abs/2605.06527), 2026-05-07 |

## The twelve conditions

All must hold. None may be waived by an interesting result.

1. Statistically credible behavioural effect
2. Preregistered direction, registered before looking
3. Exact replication **FAILS today**
4. Two or more materially different models
5. Two or more independent repositories or tasks
6. At least one independently constructed claim/evidence representation
7. Mechanism survives T4, isolating capability from interpretation
8. Effect survives the adversarial anchor classes
9. Objective task-outcome consequence demonstrated **currently unmeasurable**
10. Concrete technical implementation specified
11. Serious prior-art search performed **FAILS today**
12. Patent counsel review before any filing decision

## Five things that are NOT the same, and are routinely conflated

- **Scientific novelty** is the finding not being in the literature. Weakened by the sources in the table above, ECLoop and Hindsight in particular.
- **Product differentiation** is a customer noticing a difference. Independent of all of the above.
- **Patent novelty** is a legal question about prior disclosure, decided by counsel, not by a search that failed to find something.
- **Patent eligibility** is a separate legal question this document takes no position on.
- **Obviousness** is the hardest bar and the one a counterintuitive result feels like it clears and usually does not.
- **Freedom to operate** is a different question again, and ECLoop's existence bears on it.

## Explicitly insufficient triggers

Serious IP work is **not** authorized because the 80-run result is positive;
because the searched sample lacked an exact implementation; because the product
sounds differentiated; because the mechanism is useful; or because the mechanism
is counterintuitive. **A market search finding nothing is not an IP conclusion.**

## What remains, honestly

The narrow residual: none of the published works listed above tests whether an
executable, correctly targeted, **non-dispositive** anchor makes contradiction
detection *worse than supplying no anchor at all*. ECLoop assumes insufficient
evidence is bad and gates on it. "Silence Is Endorsement" removes a status
rather than supplying a weak check. "Building to the Test" has no no-anchor
control.

That residual is one cell in a table the field is actively filling in, and the
experiment designed to measure it has not yet been run at a difficulty capable
of measuring anything.
