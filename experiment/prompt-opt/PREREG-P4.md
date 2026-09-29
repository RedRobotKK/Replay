# Preregistration: P4 replication (post-hoc origin, declared)

Written **before** the P4 replication ran, and **after** run 1 and the P6
replication were complete.

## Honest provenance

The P4 effect is **POST-HOC**. The original preregistration's replication
criterion used unpaired mean cost, under which P4 did not qualify (-13.4%, below
the 20% threshold) and P6 did (-24.1%). P6 was replicated as registered and
**failed**: run 1 gave -26.9% output, run 2 gave +3.4%, opposite signs.

A paired per-task analysis was then run. Pairing was not specified in the
original preregistration. It is the statistically correct choice here, because
output tokens vary by more than two orders of magnitude between task classes and
the unpaired mean is dominated by a handful of high-output tasks. Under pairing:

| condition | geometric mean vs P0 | 95% CI | verdict |
| --- | ---: | --- | --- |
| P1 | 1.148 | [0.942, 1.399] | not distinguishable |
| P2 | 1.003 | [0.806, 1.248] | not distinguishable |
| P3 | 1.035 | [0.942, 1.137] | not distinguishable |
| **P4** | **0.851** | **[0.767, 0.945]** | **lower output** |
| P5 | 1.049 | [0.942, 1.168] | not distinguishable |
| P6 | 1.020 | [0.805, 1.292] | not distinguishable |

P6's apparent win disappears under pairing, which is the correct reading of why
its replication failed.

**A post-hoc effect surviving a post-hoc analysis is a hypothesis, not a
result.** This document exists so the P4 replication is a genuine test.

## What P4 is

P0 plus one clause, nothing removed: `Do not explain intermediate work. Return
only the requested result.` It is the only condition that changes exactly one
thing about the baseline. P0 already carries an output constraint, so P4
strengthens rather than introduces one.

## Criterion, fixed before the result is seen

The effect is CONFIRMED only if, on an independent run of P0 and P4 over all 18
tasks:

- the **paired geometric mean of P4/P0 output tokens is below 1.0**, and
- its **95% confidence interval excludes 1.0**, and
- **accuracy is no more than one task lower** than P0.

Any other outcome is recorded as NOT CONFIRMED. The effect will not be rescued
by a third analysis method.

## What it would mean either way

Confirmed: an explicit output constraint reduces output tokens by roughly 15% on
this task set with reasoning enabled, which is a prompt-only lever that does not
touch model or reasoning configuration.

Not confirmed: prompt structure produced no measurable cost effect in this
design, and the campaign returns a null result.

Neither outcome establishes a mechanism. Whether a reduction comes from less
narration, less hidden reasoning, or a shorter answer is NOT_OBSERVED.
