# A2 Amendment 01, 2026-09-29

## Amendment reason

Candidate A Stage 1 exposed a **false-negative defect** in the deterministic
scorer that was the preregistered measurement instrument at the time the agents
ran.

## Defect

The scorer did not recognise a canonical verdict of `C1 — FALSE` when it appeared
as a standalone line. Three compounding causes:

1. The verdict vocabulary contained no `FALSE` alternative; it matched only
   phrases such as "does not hold" and "is false".
2. The parser bound verdict to claim identifier by requiring **sentence-level
   co-occurrence**, and the sentence splitter treated `C1 — FALSE` as its own
   sentence, separate from the explanation that followed.
3. The mechanism vocabulary was too narrow, requiring `EFFORT_ALIASES` or an
   explicit `low -> high`, and missed the phrasing "transformed via an alias
   mapping".

The sentence-binding rule was itself introduced to close an earlier
**false-positive** defect. Closing one defect opened the other.

**Root cause of the test failure.** Thirteen scorer tests passed before the
campaign because every test case was written in the author's own phrasing. Not
one used the `**C1 — FALSE**` heading format that the model actually produces.
The suite could not fail in the way the real data failed. This is a positive
control failure and is the same class of error this repository has recorded
before: a check that cannot fail is not evidence.

## Consequence

The reported 4/8 rate **cannot be used as a calibration measurement**.

## Status

**Candidate A Stage 1 calibration is INVALIDATED FOR CALIBRATION PURPOSES.**

## Important distinction

The 8 raw trajectories remain valid observations of agent behaviour. They were
produced under a frozen task, a frozen prompt and a frozen repository state, and
their bytes are preserved and hashed. But because the preregistered **objective
scorer** was defective at the moment of measurement, the observation failed the
objective-measurement requirement, and the runs may not be promoted into the
formal calibration dataset.

A retrospective re-score with a corrected scorer is a **diagnostic of the
defect**, not a prospective calibration measurement.

## What this amendment does NOT do

It does not alter the 40 to 70 percent criterion, the two-stage stopping rule,
the candidate order, the task, or the anti-circularity rule. The original
protocol and the defective scorer are preserved unchanged, so the record shows
what instrument was in force when the agents actually ran.
