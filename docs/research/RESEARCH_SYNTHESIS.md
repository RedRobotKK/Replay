# Research synthesis

**The executive document. Written 2026-09-29 against a completed evidence lock.
Every claim carries its epistemic status.**

## Executive finding

The original structured-state experiments demonstrated a behavioural effect on
**retrieval and search**. The targeted weak-executable-anchor replication did
**not** reproduce the hypothesized contradiction-detection effect, because both
C0 and T2 reached 100% detection. **The mechanism remains unconfirmed.**

The frozen interpretation rule, quoted rather than paraphrased:

> T2 **not** below C0: **the mechanism is NOT established.** Report and stop.

That is weaker than refutation, and it is the strongest statement the
preregistration licenses.

## What we actually learned

1. **Structured representations can change retrieval and search behaviour.**
   0/8 with no cursor, 0/4 with the same claim in prose, 9 to 10 of 10 with a
   three-row table; separately 51/60 against 0/12, p=1.9e-08, effort flat at 21
   to 25 tool calls per arm. OBSERVED.
2. **Retrieval behaviour is not task success.** No surface examined carries a
   task-outcome signal, so the second cannot currently be measured at all.
3. **The weak-anchor mechanism is not currently replicated.** C0 100%, T2 100%,
   Fisher p=1.0000.
4. **Task calibration matters, and this is the most transferable finding.** A
   ceiling erases the observable treatment difference. A control at 100% makes
   the maximum observable reduction exactly zero.
5. **Evidence-conditioned action gating already has strong prior art.** ECLoop,
   implemented, Pass@1 +4.8 to +11.8 on all 500 SWE-bench Verified instances.
6. **The independent-observer direction is a separate architectural and product
   question from generic agent memory**, and the 28-product survey found no
   substitute for it: first-party instrumentation is not independence.
7. **A calibration substrate needs a validated observation interface before it
   needs a task.** Deterministic scoring of unconstrained agent prose repeatedly
   produced measurement defects. Moving the verdict into a machine-readable
   contract eliminated semantic parsing ambiguity, but introduced a separate
   compliance failure: the agent did not always emit the required structured
   verdict. This demonstrated that reliable calibration requires both a
   deterministic observation interface and a protocol whose compliance
   characteristics are themselves controlled. Full record in
   `a2/A2_CLOSURE.md`.

## What remains unresolved

The narrow unresolved experimental question retained by this programme is
whether supplying an executable but nondispositive verification path can reduce
contradiction detection relative to no verification path.

**That question was not established by the completed replication.** It is one
open question among those below, and its retention here reflects that it is
cheap to test, not that it is the most important question facing the product.

- Can the weak-anchor effect be reproduced on a **calibrated** task?
- Does T4 distinguish the check's capability from the agent's interpretation of it?
- Does any effect replicate across models?
- Does it replicate across repositories?
- Does it affect **objective task outcomes**? Currently unmeasurable.
- Does independently generated state behave differently from agent-generated state?

## What NOT to conclude

Stated explicitly because each is an easy slip:

- Replay has **not** demonstrated improved task completion.
- Replay has **not** demonstrated prevention of premature closure.
- Replay has **not** demonstrated a novel evidence-gating mechanism.
- Replay has **not** established patentability, and this programme cannot.
- **The failed replication does not prove the hypothesized phenomenon cannot
  occur.** It had no headroom to observe it.
- ECLoop establishes relevant prior art. It does **not** decide patentability.
- "No matching capability was found in the sources examined" is not "no prior
  art exists."
- The rejection of candidate A is **not** a mechanism result. No treatment arm
  was ever run against it, and a rejected calibration substrate carries no
  information about the treatment it was being built to test.
- The candidate A datasets are **not** a detection-rate estimate. Each is
  invalid for a reason that would bias such an estimate, so they are qualitative
  evidence informing task selection and nothing more.

## Next authorized work

**Not another 80-run campaign.** Repeating a ceiling task with larger n measures
nothing more precisely.

**Gate A2, task calibration.** Before another mechanism campaign is authorized,
all six must hold: independently known ground truth; an objectively scorable
contradiction with no judge; a demonstrated baseline failure rate with C0
between 40% and 70%; no post-hoc task selection; the task frozen and hashed
before any agent runs; and the difficulty published before any treatment arm is
compared.

Calibration is a control-arm-only campaign. No treatment arms are present, so it
cannot be read as a mechanism result.

**A2 state as of 2026-09-29.** Candidate A is **REJECTED AS AN A2 CALIBRATION
TASK**: repeated measurement attempts failed to produce a clean prospective
calibration dataset, while the independently inspected trajectories repeatedly
detected the known contradiction, so the task does not provide demonstrated
baseline headroom sufficient to justify further calibration engineering.
Candidate A nevertheless repeatedly exhibited very high contradiction detection
in the trajectories that could be independently adjudicated, providing no
demonstrated evidence of the required 40 to 70 percent control-arm headroom. No
Stage 2 was authorized and none was run. Candidates B and C have no calibration
data of any kind. See `a2/A2_CLOSURE.md` and `a2/A2_GATE_REVIEW.md`.

## What product directions remain viable

Described, not ranked; detail in `PRODUCT_FIT.md`. Billing reconciliation is the
only position with a completed demonstration. Independent observation is the
only position where the survey found no substitute, and its central question is
unrun. Work-state reconstruction has a real retrieval finding and a mechanism a
competitor could copy quickly. Memory, verification layer and
evidence-conditioned support are occupied by others with published results.

## When this becomes an IP question

See `IP_GATE.md`. Twelve conditions, all required, two currently failing. The
status is **NOT CROSSED**, which is not a finding that no invention exists.
