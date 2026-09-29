# Prompt optimization campaign: final report

**NULL RESULT.** Frozen at `2be96b2`. Spend `$0.28` OBSERVED / `$0.35081`
DERIVED. 396 trials across three runs. Preregistration in `PREREG.md`, the
post-hoc P4 test in `PREREG-P4.md`, claims in `derived/ledger.json`.

## 1. Executive summary

**What was measured.** Seven prompt conditions over 18 machine-checked tasks
with model, reasoning configuration and shared corpus held constant, then two
independent replications of the two candidate effects.

**What survived. Nothing.** Both candidate effects reversed sign on independent
replication:

| candidate | discovery | replication |
| --- | ---: | ---: |
| P6, unpaired cost | **-24.1%** | **+1.6%** |
| P4, paired output | **0.851** [0.767, 0.945] | **1.104** [0.932, 1.308] |

**Why.** Output-token variability under reasoning-enabled `deepseek-flash` is
larger than any effect the conditions produced. For an identical condition on an
identical task the median relative spread is **44%**, and the control differed
from **itself** by **13%** across two runs with no treatment applied.

**What this campaign is good for.** It establishes a noise floor. Detecting a
20% effect on unpaired means in this design needs roughly **1,169 trials per
arm**; 72 were run. That is the reusable result.

## 2. Experimental design

Corpus is the shared prefix, byte-identical across every trial, so cache state is
constant by construction. Conditions are rendered from one parsed component set
per task, so no condition can add information, drop a constraint or leak an
answer. 10 tests, 6 mutations all killed. Trials shuffled with a recorded seed so
no condition occupies a contiguous block. Reasoning left at the provider default
in every condition; no reasoning parameter sent.

Two baseline properties that bound everything: **P0 is already terse**, and
**P0 already carries an output constraint**, so P4 strengthens rather than
introduces one.

## 3. Raw results, run 1 (252 trials)

| cond | valid | passed | mean fresh in | mean out | mean cost | paired vs P0 |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| P0 | 34 | 31 | 189.6 | 672.6 | $0.0009446 | control |
| P1 | 34 | 31 | 181.6 | 643.6 | $0.0009074 | 1.148 [0.942, 1.399] |
| P2 | 34 | 34 | 211.6 | 596.7 | $0.0008602 | 1.003 [0.806, 1.248] |
| P3 | 35 | 34 | 224.1 | 605.6 | $0.0008746 | 1.035 [0.942, 1.137] |
| P4 | 36 | 34 | 201.6 | 564.1 | $0.0008180 | 0.851 [0.767, 0.945] |
| P5 | 35 | 34 | 223.6 | 692.2 | $0.0009783 | 1.049 [0.942, 1.168] |
| P6 | 35 | 34 | 151.7 | 491.7 | $0.0007170 | 1.020 [0.805, 1.292] |

## 4. Accuracy

**No accuracy effect was observed.** Only three of 18 tasks ever failed, and
they failed scattered across prose and structured conditions:
`transform.reverse.funcname` under P0, P1, P4 and P5; `counting.func.lines`
under P0 and P3; `counting.rawmessage.occurrences` under P1 and P6. With two
repetitions this supports no accuracy claim in either direction.

## 5. Token economics and the pairing correction

The unpaired column above is **misleading and was corrected**. Output tokens span
60 to over 22,000 across task classes, so an unpaired mean is dominated by a few
high-output tasks. P6 reads **-26.9%** unpaired and **1.020** paired on exactly
the same data. Pairing removes between-task variance and is the correct analysis.

Output is **82 to 85%** of trial cost, so cost effects here are output effects.

## 6. Cost reconstruction

Derived `$0.35081` this campaign against an observed balance movement of
`$0.28` ($45.77 to $45.49). The reading is **provisional**: settlement lag was
established in the prior campaign and this balance was read minutes after the
last call. Consistent with lag, derived exceeds observed.

## 7. Prompt comparison

No ranking is given, because no condition is distinguishable from the control
after replication. The observed tradeoff space is a single point: all seven
conditions sit within the noise of one another on both cost and accuracy.

## 8. Error analysis

Failures concentrate on three tasks, all requiring exact counting or
character-level transformation. The failure mode is a confident wrong value, not
a refusal or a format error. **Whether this reflects a reasoning failure or a
tokenisation limit is NOT_OBSERVED**; the final answer alone does not
distinguish them.

## 9. Replication

Both preregistered criteria were applied as written and both failed. P6 was
tested under the original criterion. P4's paired effect was **post-hoc**, was
declared as such in `PREREG-P4.md` before its test ran, and failed. **No third
analysis method was tried**, which is the rule the preregistration set.

## 10. Mechanism analysis

There is no effect to attribute. Brevity, structure and decomposition are all
**NOT_OBSERVED** as cost mechanisms in this design. P6, the deliberately verbose
structured control, was indistinguishable from the terse baseline under pairing,
which is evidence against a simple token-count account as much as against a
structure account.

## 11. Prior art

The null is consistent with published work. Reported decoder-level variance on
identical prompts accounts for roughly a third of movement in replicated cells,
and evaluation-reproducibility work documents that identical calls do not
reliably recover identical structure. "Can We Count on LLMs? The Fixed-Effect
Fallacy" argues directly that capability claims drawn from single-condition runs
are unreliable, which is the error this campaign's replication step caught.

Sources are listed in the session record; none was used to derive a number here.

## 12. Replay implications

**Directly demonstrated.** A preregistered replication step converted two
apparently strong findings into a null. Both would have shipped as results
without it.

**Supported hypothesis.** Any prompt-level optimization claim needs a paired
design and a replication gate. A per-workload noise floor is a prerequisite for
claiming an optimization, not an afterthought.

**Not demonstrated.** That prompt optimization reduces cost. That structure
matters independently of length. That decomposition reduces model work. All
three are NOT_OBSERVED here, not refuted in general.

## 13. IP status

`PATENT PRIOR ART SEARCH = NOT_VERIFIED`. No database queried. No novelty,
patentability or freedom-to-operate conclusion is available or implied.

## 14. Claim ledger

Eleven claims in `derived/ledger.json`, each with evidence class, source, n,
observed value, alternative explanation, limitation and status.

## 15. Evidence gaps

- Whether a sub-noise prompt effect exists. Not resolvable inside this budget.
- The mechanism behind any such effect. NOT_OBSERVED.
- Behaviour with reasoning disabled. Deliberately excluded by the design.
- Other models, other corpora, other task sets. NOT_OBSERVED.
- Whether the three hard tasks fail for reasoning or tokenisation reasons.
- Patent prior art. NOT_VERIFIED.

## 16. Answers

| | question | answer |
| --- | --- | --- |
| Q1 | Does prompt optimization measurably reduce cost? | **No, not in this design.** Both candidates reversed on replication. |
| Q2 | Does it preserve correctness? | No accuracy effect was observed in either direction. |
| Q3 | Does structure matter independently of length? | **NOT_OBSERVED.** The verbose structured control was indistinguishable from the terse baseline. |
| Q4 | Does decomposition reduce unnecessary work? | **NOT_OBSERVED.** P5 was 1.049 [0.942, 1.168]. |
| Q5 | Is the effect task-class dependent? | No effect survived to be class-dependent. |
| Q6 | Does the effect survive replication? | **No.** That is the campaign's result. |
| Q7 | Is there a mechanism worth investigating as Replay IP? | **No**, on this evidence. The reusable finding is methodological: a noise floor and a replication gate. |

## 17. Next experiment

**None recommended.** The one remaining question, whether a sub-noise effect
exists, would need roughly 1,169 trials per arm on unpaired means. A paired
design needs far fewer, but both candidates already failed paired replication.
Spending further here buys precision on an effect there is no evidence for.
