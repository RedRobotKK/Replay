# Experiment matrix and preregistration templates

**Templates only. Nothing here is authorized to run. Written 2026-09-29 while
the 80-trial campaign was executing, and deliberately kept out of it.**

Every campaign below inherits the frozen rules of the running one: primary
endpoint is a string match on tool output, direction registered before looking,
randomized order, fresh repository copy per run, a changed procedure voids the
run rather than being absorbed, no dropping runs after seeing them.

## The gate ladder

Each gate is shut until the one above it opens. **No gate below A is authorized.**

| Gate | Question | Status |
|---|---|---|
| **A** | C0 vs T2 primary endpoint | **RESOLVED 2026-09-29: NOT REPLICATED.** C0 20/20, T2 20/20, p=1.0000. Ceiling effect; control had no headroom. One failure against the replication gate |
| **A2** | **Task calibration.** Establish a task with measurable headroom, BEFORE any mechanism arm runs | **The only open step.** Not authorized |
| B | T2 vs T4: capability against interpretation | SHUT, gated on A' |
| C | Model replication, materially different model | SHUT |
| D | Repository replication, independent repository | SHUT |
| E | Claim and anchor replication | SHUT |
| F | Ten adversarial anchor classes | SHUT |
| G | Objective task outcome | SHUT, and separately BLOCKED on the absence of any outcome signal |
| H | Independence: agent-generated vs Replay-generated vs none | SHUT |
| I | IP analysis | SHUT. See `IP_GATE.md` |

### Gate A: COMPLETED, NOT REPLICATED

C0 20/20, T2 20/20, Fisher p=1.0000. The frozen rule applied verbatim: "T2 not
below C0: the mechanism is NOT established. Report and stop."

**No immediate 80-to-80 repeat is authorized.** Repeating a ceiling task with
larger n measures nothing more precisely.

### Gate A2: task calibration

**Purpose:** establish a task that can register BOTH successful contradiction
detection AND failure to detect, before any treatment comparison is attempted.

A calibration task must have all six:

1. independently known ground truth
2. an objectively scorable contradiction, no judge
3. **a nontrivial baseline failure rate**, target C0 between 40% and 70%
4. **no post-hoc task selection**: the task is chosen and frozen before trials
5. the task frozen and hashed before any agent runs
6. **difficulty demonstrated before the treatment comparison**, and published

Calibration is a separate, smaller campaign: control arm only, enough runs to
bound the baseline rate, no treatment arms present. **Do not spend a large n on
an uncalibrated task.** That is the error Gate A made.

### Gate B: T4, gated on A2 showing headroom

T2 is the executable-but-insufficient anchor. T4 is the same semantic check plus
an explicit statement that it cannot establish the claim's truth.

**T4 is a mechanism-discrimination experiment, not a product feature test.** It
exists to separate the check's actual capability from the agent's interpretation
of that capability. It answers a scientific question and implies nothing about
what should be built.

### Gate G measurement rule, fixed now

When G is eventually reached, measure independently: contradiction detection;
correct final repository state; task completion; regression rate; unnecessary
tool calls; time and tokens; false closure. **The agent-generated claim
representation may not serve as the sole ground truth**, because that makes the
treatment its own oracle.

## A. Mechanism replication. Gated on R3 positive.

| | |
|---|---|
| **Adds** | T4, the explicit-insufficiency condition |
| **Arms** | C0, T1, T2, T3, T4 |
| **n** | 20/arm |
| **Model** | a third, materially stronger than `claude-haiku-4-5` |
| **Repository** | a third, not operator-authored |
| **Held constant** | claim structure, prompt, tools, effort measured not assumed |

**T4 is the point of this campaign.** It carries T2's anchor unchanged and adds
one sentence: *"This check cannot establish whether the claim is true. It only
locates related implementation."*

| T4 outcome | Reading |
|---|---|
| recovers to C0 or above | harm is **interpretive**; remedy is cheap |
| stays at T2 | harm is in the **capability itself**; remedy is hard |

Neither branch is the one this project would prefer, which is why it is worth
running.

## B. Adversarial evidence. Ten anchor classes.

| # | Anchor class | What it probes |
|---|---|---|
| 1 | correct dispositive | ceiling |
| 2 | stale | does a dead pointer harm? Origin says no (10/10) |
| 3 | executable, insufficient | **the effect under test** |
| 4 | executable + explicitly insufficient | = T4 |
| 5 | irrelevant successful check | is it completion, or relevance? |
| 6 | misleading check | wrong direction, cleanly executed |
| 7 | conflicting evidence | two anchors disagreeing |
| 8 | noisy failed check | does failure preserve search? |
| 9 | multiple anchors, mixed capability | does the strongest dominate? |
| 10 | no dispositive evidence exists | can the agent say "unresolvable"? |

**Measured separately, never collapsed into "premature closure":** detection;
search direction; closure; **false** closure; contradiction handling.

Class 5 is the sharpest discriminator in the set. If an *irrelevant* successful
check harms detection as much as a relevant-but-insufficient one, the mechanism
is completion-as-closure and has nothing to do with targeting.

## C. Task outcome. The blocked one.

This is R10 and it is the only campaign that could ever support claim 3.

Each task requires: a known repository state; **hidden** ground truth; explicit
claims; relevant, irrelevant and stale evidence; an executable-but-insufficient
check; a dispositive check; a reboot boundary; and **deterministic success
criteria**.

Measured: task success; incorrect modifications; false claims; premature
closure; unnecessary tool calls; turns; recovery after reboot; cost.

**Rule.** Deterministic repository assertions wherever an assertion is possible.
An LLM judge is permitted only where no assertion exists, and its use is
declared per metric.

**Status: BLOCKED.** Building this benchmark is the largest single piece of work
in the program and nothing downstream of claim 3 is honest without it.

## D. Independence. The question that could remove independence from the thesis.

| Arm | State |
|---|---|
| A | none |
| B | agent-generated |
| C | Replay-reconstructed |
| D | externally authored |

Semantic information held as constant as practical across B, C and D.

**The difficulty is the experiment.** Holding content constant while varying
authorship is nearly impossible to do cleanly, because an independent
reconstruction that contains exactly what the agent would have written is not
independent in any interesting sense. If the arms cannot be made comparable,
that is the finding, and independence leaves the thesis rather than being
asserted.

## Sample-size rationale, applied throughout

n=20/arm detects the origin's 8/10 against 4/10 by Fisher exact at conventional
power. n=10 did not, which is why the origin is not a result. Larger n on the
first pair is worth less than the same n on a new pair: the threat to validity
is not noise, it is specificity.
