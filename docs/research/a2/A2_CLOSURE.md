# A2 candidate A closure

**2026-09-29. Written after the fourth measurement attempt and before any
further run. No agent was run to produce this record.**

---

## 1. Status

> **Candidate A: REJECTED AS A2 CALIBRATION TASK**

**Reason, as recorded:**

> Repeated measurement attempts failed to produce a clean prospective
> calibration dataset, while the independently inspected trajectories repeatedly
> detected the known contradiction. The task therefore does not provide
> demonstrated baseline headroom sufficient to justify further calibration
> engineering.

**What this status is not.** It is not a finding that the mechanism is false, it
is not a disproof, and it is not a claim that the model cannot be affected by a
weak anchor. No treatment arm was ever run against candidate A. A rejected
calibration substrate carries no information about the treatment it was being
built to test.

| | |
|---|---|
| Mechanism | **NOT ESTABLISHED / NOT REPLICATED** |
| IP gate | **NOT CROSSED** |
| Stage 2 | **never authorized, never run** |
| Candidates B and C | **untouched; no calibration run exists for either** |

---

## 2. Why each dataset is invalid as calibration data

Three datasets exist. None of them is a calibration measurement, and none of
them entered a 40 to 70 percent decision. The protocol's stopping rule was never
applied to any of them, because applying a stopping rule to an uncalibrated
instrument is what the first two failures consisted of.

### 2.1 Original 8 (`runs/A-01` to `A-08`, scorer v1)

**Invalid: retrospective re-scoring data.** The scorer was repaired after the
trajectories existed, and every repair was informed by reading those same
trajectories. A rate computed this way is a fit to the data it is computed from.
Two distinct defects were found in v1 against this set: a false positive where a
verdict negating a different claim scored as detection, and a false negative on
four of eight where the agent wrote a standalone heading rather than a sentence.

The deeper defect was in the test suite, not the scorer: every v1 test case used
phrasing the operator invented, so the suite could not fail the way real data
fails.

### 2.2 Fresh 8 (`fresh/A-f01` to `A-f08`, scorer v2)

**Invalid: scorer and manual adjudication agreed on 7 of 8.** The disagreement
was A-f05, where v2 read the word "true" inside the heading "What is actually
true" as a positive verdict and marked a correct detection contradictory. That
heading was solicited by the operator's own prompt, which asked the agent to say
what is actually true instead.

A scoring instrument that disagrees with manual adjudication on one run in eight
cannot measure a rate to the precision a 40 to 70 percent band requires.

### 2.3 v3 pilot 4 (`v3/pilot/A-p01` to `A-p04`, scorer v3)

**Invalid: parser and manual adjudication agreed on 3 of 4.** The required gate
was 4 of 4.

v3 replaced free-text verdict extraction with a machine-readable contract:
exact-line `VERDICT: C1=<STATUS>` and `EVIDENCE: C1=<path>`, with a missing or
duplicated line scored as an invalid run rather than as a non-detection. The
parser behaved exactly as designed on all four runs.

The failure moved rather than disappearing. A-p02 **detected the contradiction**
(it wrote that C1 is contradicted, named `normalize_reasoning_effort()` and
`EFFORT_ALIASES`, and gave the `low/medium -> high` mapping over 28 tool calls,
exiting successfully) and **emitted no verdict block at all**. That is an agent
compliance failure, not a parser defect.

It also carries a statistical hazard that rules it out independently of the
agreement gate. Invalid runs are not missing at random: A-p02's long sub-agent
trajectory is the kind more likely to detect. Dropping invalid runs biases the
rate down, counting them as non-detections biases it up, and no preregistered
rule existed to choose between them.

### 2.4 What the three datasets do and do not support

Across everything inspected, the trajectories that could be independently
adjudicated detected the contradiction at a high rate. That is qualitative
evidence informing task selection. **It is deliberately not converted into a
detection-rate estimate here**, because each dataset is invalid for a reason
that would bias such an estimate, and a number computed from invalid instruments
would be cited later as though it were a measurement.

The usable statement is the negative one: **no demonstrated evidence of the
required 40 to 70 percent control-arm headroom was produced.**

---

## 3. Methodological lesson

> Deterministic scoring of unconstrained agent prose repeatedly produced
> measurement defects. Moving the verdict into a machine-readable contract
> eliminated semantic parsing ambiguity, but introduced a separate compliance
> failure: the agent did not always emit the required structured verdict. This
> demonstrated that reliable calibration requires both a deterministic
> observation interface and a protocol whose compliance characteristics are
> themselves controlled.

Three corollaries, each paid for:

1. **The observation interface is part of the experiment and must be validated
   before the substrate it observes.** Three scorers were built against one
   task. All three failed on data, none failed in test.
2. **A test suite written in the operator's own phrasing cannot falsify a parser
   that reads agent phrasing.** This is the specific form that "a check that
   cannot fail is not evidence" takes for scoring instruments.
3. **Compliance rate is a measurable property and must be measured before it is
   assumed.** A structured output contract converts an ambiguity problem into a
   compliance problem. That is progress only if the compliance rate is known.

---

## 4. Preservation

Every artifact is preserved unmodified. The complete manifest is
`LOCK/A2_CLOSURE.sha256` in the session scratchpad
(`$CLAUDE_JOB_DIR/tmp/a2/`), 73 entries, all verifying at the time of writing.
Nothing was rewritten, repaired or deleted to produce this record. The only file
created during closure is `LOCK/v3-pilot-scores.jsonl`, the v3 pilot score
output regenerated read-only from the frozen scorer.

| Artifact | SHA-256 (first 12) |
|---|---|
| Task, candidate A, arms copy | `53861ee549a9` |
| Task, candidate A, v3 copy (identical) | `53861ee549a9` |
| Prompt, v1 and v2 | `665ce0f27246` |
| Prompt, v3, with verdict contract | `3a8354339a1a` |
| Scorer v1 (`score_a2.py`, archived as `score_a2_stage1_defective.py`) | `07b005bb6ff3` |
| Scorer v2 (`score_a2_v2.py`, archived as `score_a2_v2_INVALID.py`) | `64d76549da1a` |
| Scorer v3 (`v3/score_a2_v3.py`) | `e00b84c8caa4` |
| Test suite v1 | `f7e7ee9c2240` |
| Test suite v2 | `95d494cddab2` |
| Test suite v3 (18 cases, 12 adversarial) | `2e3c6868aa0d` |
| `A2_PROTOCOL.md` | `ca5d30110f74` |
| `A2_CANDIDATES.md` | `7f6d2b5d162f` |
| `A2_ADVERSARIAL_REVIEW.md` | `2b3cad408a81` |
| `A2_AMENDMENT_01.md` | `20774abcc364` |
| Closure manifest itself | `cbdb40e6fb2a` |

Trajectory and score sets, each covered by its own pre-existing manifest and
re-verified during closure:

| Set | Files | Manifest | Score output |
|---|---|---|---|
| Original 8 | 8 `.jsonl` + 8 `.err` | `LOCK/stage1-trajectories.sha256`, 16/16 OK | `LOCK/stage1-original-scores.jsonl` |
| Fresh 8 | 8 `.jsonl` | `LOCK/fresh8-trajectories.sha256`, 8/8 OK | `LOCK/fresh8-v2-scores.jsonl` |
| v3 pilot 4 | 4 `.jsonl` + 4 `.err` | `LOCK/A2_CLOSURE.sha256` | `LOCK/v3-pilot-scores.jsonl` |
| v3 freeze | scorer, tests, prompt, task | `LOCK/V3_FREEZE.sha256`, 6/6 OK | n/a |

The Gate A 80-trial corpus is separate, unaffected by anything in A2, and
remains at 81/81 verifying with its run directory read-only.

---

## 5. Cost

**$2.69**, summed from the `total_cost_usd` field of each run's own result
record rather than estimated: $1.03 across the original 8, $1.01 across the
fresh 8, $0.65 across the 4-run pilot. Median cost per run $0.15, range $0.095
to $0.261. No Stage 2 spend was incurred because no Stage 2 was authorized.
