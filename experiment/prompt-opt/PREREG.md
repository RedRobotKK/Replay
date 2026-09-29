# Preregistration: prompt optimization campaign

Written **before execution**. Frozen at `2be96b2`. Balance `$45.77` OBSERVED.
Budget remaining approximately `$2.76` of a `$3.00` ceiling.

## Question

Can prompt structure, with model and reasoning configuration held constant,
reduce provider cost or output volume while preserving correctness?

Not "can prompts be shorter". The target is a movement of the cost by accuracy
frontier that does not come from disabling reasoning.

## Held constant

Model `deepseek-flash`. **Reasoning left at its default (enabled); no reasoning
parameter is sent in any condition.** Same shared corpus, same task set, same
checkers, same `max_tokens`, no sampling parameters set, no tools. Only the
instruction suffix changes.

## Cache control

The corpus is the shared prefix and is **byte-identical across every condition
and every trial**, so cache state is constant by construction rather than by
averaging. One warm call establishes it. Every trial records `cache_read`; a
trial whose cache read departs from the established value is flagged and
excluded from cost comparison rather than silently averaged.

This means input-token differences between conditions are confined to the
uncached suffix. That bounds what this regime can show about brevity, and the
bound is stated in the result rather than discovered afterwards.

## Conditions

Rendered from one parsed component set per task, so no condition can add
information, drop a constraint or leak an answer. See `promptcond.py` and its
10 tests, 6 mutations all killed.

| id | treatment | equivalence |
| --- | --- | --- |
| P0 | baseline, untouched original | control |
| P1 | concise: drops the clause the output format already implies | **LIMITED**: constraint changed |
| P2 | structured: TASK / INPUT / CONSTRAINTS / OUTPUT | clean |
| P3 | procedural: four bounded steps | **LIMITED**: constraint reworded |
| P4 | output-constrained: P0 plus one added clause | clean, single-variable |
| P5 | evidence-oriented: FACTS / QUESTION / REQUIRED DERIVATION / OUTPUT | clean |
| P6 | **negative control**: P2's structure, padded with information-free prose | clean |

P6 is the discriminator the campaign turns on. If P6 behaves like P2 despite
carrying more tokens, structure is doing the work. If P6 behaves like P0, the
simpler account is token count.

## Two properties of the baseline that bound the campaign

1. **P0 is already terse.** It is not a verbose strawman, so a large brevity
   win is not available to be found.
2. **P0 already carries an output constraint.** P4 therefore strengthens an
   existing constraint rather than introducing one, and its effect is expected
   to be small by construction.

## Design

18 tasks across 5 classes, ground truth computed from the corpus at import.
7 conditions. 2 repetitions. 252 trials, executed in a **seeded shuffled order**
so no condition occupies a contiguous block of wall-clock or cache history.
Execution order is recorded.

## Predictions, registered now

| condition | output tokens vs P0 | accuracy vs P0 |
| --- | --- | --- |
| P1 | within 10% | unchanged |
| P2 | within 20% | unchanged |
| P3 | **higher**, the steps invite narration | unchanged or slightly lower |
| P4 | **lower**, this is its only job | unchanged |
| P5 | within 20% | unchanged |
| P6 | within 20% of **P2**, not of P0 | unchanged |

**The honest prior is a null result.** The baseline is already terse and already
output-constrained, so the most likely outcome is that no condition moves cost
materially. A null result is recorded as a result.

## Replication criterion, fixed before any result is seen

A condition is "materially better" only if, against P0:

- mean total reconstructed cost per task is **at least 20% lower**, AND
- accuracy is **no more than one task lower** across the 18.

Any condition meeting both gets **2 further repetitions** on all 18 tasks,
together with P0 re-run in the same batch. The effect is confirmed only if the
cost direction holds with the same sign and accuracy stays within one task.

No condition is re-run because its first result was disappointing.

## Stop conditions

Stop and record the reason if conditions prove not semantically comparable; if
usage recording fails; if cache reads diverge across conditions; if the balance
evidence becomes inconsistent; if the remaining budget cannot discriminate; if
an apparent optimization turns out to need a model or reasoning change; or if a
result needs speculative interpretation to look interesting.

## Scoring

Binary, by the committed checkers. A truncated response is **UNDECIDED** and
leaves the accuracy denominator, never scored wrong. No scalar quality score.
Results are reported as a cost-by-accuracy relation, not a ranking.
