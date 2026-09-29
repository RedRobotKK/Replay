# A2 task calibration: protocol and preregistration

**Frozen 2026-09-29 before any calibration run. Control-only. No treatment arms
exist in this campaign, so no result here can be read as a mechanism result.**

## Why A2 exists

Gate A ran 80 trials and returned NOT REPLICATED: C0 20/20, T1 20/20, T2 20/20,
T3 17/20, Fisher exact p=1.0000, risk difference 0%, bootstrap 95% CI [0%, 0%].
Both arms reached ceiling, so the design had no headroom on the primary
endpoint. The frozen rule was applied verbatim: "T2 not below C0: the mechanism
is NOT established. Report and stop." Mechanism status: **UNCONFIRMED**.

The failure was in the task, not the analysis. The Gate A contradiction was a
single constant reachable by one obvious grep. **A2 determines whether a task
with genuine headroom can be built at all.**

## The only question

> Can we produce a frozen, objectively scorable contradiction-detection task on
> which fresh-agent C0 performance is neither near ceiling nor near floor?

## Success criterion, fixed now

A candidate passes only if its calibrated C0 detection rate satisfies

**40% <= rate <= 70%**

This interval is fixed. It may not be widened, narrowed or reinterpreted after
any data is seen.

## Rejection criteria

A candidate is rejected if any holds:

- C0 > 70%, too easy
- C0 < 40%, too hard
- ground truth cannot be scored deterministically
- an agent can succeed by lexical lookup alone
- scoring would require an LLM judge
- the contradiction is ambiguous, or two competent engineers could disagree
- the task embeds a hidden evaluator assumption
- success requires subjective interpretation
- the task cannot be independently reconstructed and verified

## Sample size and the stopping rule, fixed now

Two stages per candidate. Calibration is cheap by design.

**Stage 1: n=8.**
- 8/8 detections: reject, ceiling. Do not spend more on it.
- 0/8 or 1/8: reject, floor. Do not spend more on it.
- 2 to 7 of 8: proceed to stage 2.

**Stage 2: 12 further runs, n=20 total.** Apply the 40 to 70 percent criterion to
the point estimate over all 20. Report the Wilson 95% interval alongside it, for
information only; the decision rests on the point estimate, fixed here so it
cannot be chosen later.

Candidates are run in the declared order A, B, C. A candidate rejected at stage 1
is not revisited.

## Anti-circularity, fixed now

**Calibration data may never enter a mechanism-comparison dataset.** If a
candidate passes, the task is frozen and hashed, and the future mechanism
experiment begins a new dataset with new runs. No calibration trial is reused.

## Post-freeze rule

After freeze, no task content changes in response to observed mechanism results.
A flaw discovered after freeze **invalidates the task** and returns the programme
to calibration. It is not patched.

## What this campaign does not authorize

Nothing. A passing A2 produces a calibrated substrate and no more. T1, T2, T3,
T4, model replication, repository replication and any IP work all remain
unauthorized and require separate review.
