# Research

## START HERE

> ### [FINAL-CLOSEOUT.md](FINAL-CLOSEOUT.md) is the authoritative research closeout
>
> It is the one auditable answer to what was discovered, what was ruled out, and
> whether any concrete technical mechanism warrants dedicated prior-art
> investigation. Everything else in this directory is a source it reconciles.
> Where this README and the closeout differ, **the closeout is correct.**

The campaign is **frozen**. Nothing in this directory authorizes an experiment.

## CURRENT STATUS, 2026-09-29

| | |
|---|---|
| Original structured retrieval/search effect | **OBSERVED** |
| Weak-anchor replication | **NOT REPLICATED** |
| Replication interpretation | **ceiling effect / insufficient task headroom** |
| Durable work-state mechanism | **NOT ESTABLISHED / NOT REPLICATED** |
| A2 calibration | **CLOSED.** Candidate set exhausted without a calibrated task. No treatment arm ever ran |
| DeepSeek cache geometry | **MEASURED**, mechanism NOT_OBSERVED, anticipated in prior art |
| DeepSeek reasoning-setting effect (M3) | **EFFECT OBSERVED.** Watchlist **CLOSED**. Candidate 2 eliminated, candidate 3 survives |
| Billing reconstruction | **CONDITIONALLY CONSISTENT**, as a lower bound |
| Established mechanisms | **NONE.** Five candidates, all NOT A MECHANISM YET |
| Prior-art gate | **CLOSED.** A closed gate, **not a cleared field**: no patent database has ever been searched |
| IP gate | **NOT CROSSED** |
| Production implementation | **NONE AUTHORIZED** |

**Experiments run:** the 80-trial Gate A replication, three A2 calibration
attempts, the DeepSeek A1 to A10 suite, and the M3 discriminator (12 calls,
$0.0024, 2026-09-29).

**Experiments NOT run, and NOT authorized:** the candidate 1 against candidate 3
discrimination, the five billing input-side measurements, A9/A10 replication,
the `deepseek-v4-pro` rung sweep, and any prior-art or patent search. These are
described in the closeout's section 10 as future work. **A description is not
an authorization, and none of them has been performed.**

> **Research documents do not authorize experiments. Experiments require an
> explicit gate.**

## Canonical replication result

Every document in this bundle that mentions the replication agrees with this
block. Where one states a subset, it states no figure that differs from it.

| | |
|---|---|
| C0, no anchor | 20/20 = 100% |
| T1, dispositive | 20/20 = 100% |
| T2, executable but insufficient | 20/20 = 100% |
| T3, stale | 17/20 |
| Primary, C0 vs T2, Fisher exact two-tailed | **p = 1.0000** |
| Risk difference | **0%** |
| Bootstrap 95% CI | **[0%, 0%]** |
| Verdict | **NOT REPLICATED** |
| Mechanism status | **UNCONFIRMED / NOT REPLICATED** |

Frozen rule, quoted from `PREREG.md`:

> T2 **not** below C0: **the mechanism is NOT established.** Report and stop.

Interpretation: **ceiling effect.** Both conditions reached 100%, so there was no
observed headroom on the primary endpoint. **The replication did not establish
the mechanism. That is not the same as the mechanism not existing**, and the
result does not discriminate between the two.

## The campaign that has run

Executed outside this repository, in a job scratch directory, against
`deepseek-cursor-proxy` at `ea3da01` on `claude-haiku-4-5-20251001`.
Preregistration, arm definitions, scorer, extractor and analysis specification
were frozen and hashed before any trial ran; all 81 raw artifacts are hashed,
read-only, and verify against the lock manifest. **Nothing in this directory
modifies them.**

## Documents

| Document | What it holds |
|---|---|
| [Research synthesis](RESEARCH_SYNTHESIS.md) | **Start here.** What we know, what we suspect, what is unresolved, what would falsify it, the single next experiment, and what not to build |
| [Research programme](DURABLE_WORK_STATE_RESEARCH.md) | R1 to R18, each with hypothesis, control, observable, confounders, sample size, stop condition and status |
| [Mechanism and data model](MECHANISM_AND_DATA_MODEL.md) | Observed phenomenon separated from candidate mechanism, eleven alternative explanations with their discriminating experiments, and the proposed schemas |
| [Experiment matrix](EXPERIMENT_MATRIX.md) | Gates A to I. Gate A resolved, A-prime is the only open step |
| [Prior art matrix](PRIOR_ART_MATRIX.md) | Capability reconciliation with exact and partial overlap, and the published sources that cover parts of it |
| [Provider evidence matrix](PROVIDER_EVIDENCE_MATRIX.md) | Per surface, separating OBSERVED BY REPLAY from PROVIDER ASSERTION from NOT AVAILABLE |
| [Product fit](PRODUCT_FIT.md) | Six positions against current evidence, no scores and no ranking |
| [IP gate](IP_GATE.md) | Twelve conditions, two currently failing. NOT CROSSED |

## Evidence hierarchy used throughout

OBSERVED (read in a primary source or measured here) > DERIVED > SUPPORTED
HYPOTHESIS > NOT_VERIFIED > NOT_MEASURED. Three absences are kept apart: absent
from the searched sample, documented absence, and actual novelty. The first is
never presented as the third.

## Current hypotheses, held as hypotheses

That completion of an executable check may be read as closure; that typing a
check by what it can settle may change behaviour; that observer independence may
matter beyond the information observed. None is supported today, and the
candidate mechanism competes with eleven alternatives.

## Prior-art status

Several claimed distinctions are published, including evidence-conditioned action
gating (ECLoop, measured on SWE-bench Verified), fact-versus-inference
separation (Hindsight's stated thesis), premature-commitment diagnosis, and
stale-state benchmarking (STALE). The narrow residual is the weak-anchor harm
question, for which no matching result was found in the sources examined and
which remains untested at adequate difficulty.

## Product-fit status

One position has a completed demonstration (billing reconciliation). One has no
substitute found in 28 products (independent observation) and its central
question is unrun. Three are occupied by others with published results.

## IP gate

**NOT CROSSED.** Exact replication fails; the prior-art condition fails. See
`IP_GATE.md`.

## Revenue

[REVENUE-PIVOT.md](REVENUE-PIVOT.md) closes the product question and states what
replaces it. The individual $25/month thesis is dead on the repository's own
arithmetic, the paid gate and paid feed are both killed, and what survives is
the operator rather than the software: three work samples, ranked buyers, and
one fixed-scope commercial experiment with a falsification criterion. It
proposes no feature and reopens no experiment.

## What the closeout contains

[FINAL-CLOSEOUT.md](FINAL-CLOSEOUT.md): the evidence freeze and manifests, a
35-entry evidence ledger, the DeepSeek A1 to A10 reconciliation, the billing
result, Track C, the A2 line, prior-art status in three parts, the mechanism and
IP gate, standing constraints on any continuation, and the M3 discriminator
result.

**Zero of five mechanism candidates stand as triggered.** The one that did was
closed by the experiment written to close it. The IP gate remains NOT CROSSED.

## A2 calibration status

**The declared candidate set is exhausted without a calibrated task.** Candidate
A is REJECTED AS AN A2 CALIBRATION TASK after four measurement attempts, none of
which produced a valid prospective dataset. Candidate B fails the
objective-scorability requirement on grounds preregistered before any data.
Candidate C fails the baseline-headroom requirement by a prediction likewise
recorded before any data. No treatment arm was ever run, so none of this is a
mechanism result.

`a2/A2_CLOSURE.md` is the closure record and preservation manifest.
`a2/A2_GATE_REVIEW.md` reviews the remaining candidates against the six
preregistered requirements and proposes the next design. Nothing is authorized
to run.
