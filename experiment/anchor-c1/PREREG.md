# Preregistration: does an addressable anchor change retrieval?

Written **before any run**. Base `9ade3cc`. Authorised: 75 runs, 25 per arm.

## Question

C1 established that a structured claims table changes what a fresh agent
retrieves (51/60 versus 0/12, p = 1.9e-8). Replay does not emit such a table.
`replay advise` has 5 of the 8 structural properties and is missing one:
**a resolvable per-claim anchor with a sufficiency label.**

This tests whether adding only that primitive reproduces the retrieval effect.

## The single changed variable

All three arms receive **byte-identical** `replay advise` output and a
**byte-identical** task prompt. The only difference is one line appended to one
advice item.

| arm | anchor line on the cache-breaks item |
| --- | --- |
| **A** control | none |
| **B** intervention | `anchor: docs/evidence/break-causes-2026-09-13.md [dispositive]` |
| **C** negative control | `anchor: docs/evidence/calibration-corpus-2026-09-03.md [dispositive]` |

Arm C's path is **real and resolvable but does not contain the answer.** It
tests whether a label alone changes behaviour, separately from whether a
*truthful* anchor does.

No other instruction, context, wording, claim, status or ranking changes.

## The task, identical in every arm

> Which cause accounts for the largest share of re-billed tokens, and what share?

The answer is in `docs/evidence/break-causes-2026-09-13.md` and **nowhere in the
advise output**. It carries a deliberate distractor: the largest cause by
*break count* is a different row from the largest by *share of re-billed
tokens*, so a careless reader answers wrongly even after retrieving.

## Endpoints, fixed now

**PRIMARY.** A tool call whose input names
`break-causes-2026-09-13.md`. A correct answer without such a call is **NOT**
retrieval and is scored as retrieval-negative.

**SECONDARY, scored separately and never merged into one metric:**

| stage | scored |
| --- | --- |
| retrieval | did a tool call open the correct artifact |
| recognition | did any tool call open *any* evidence artifact |
| reasoning | did the answer name the correct cause, `expired` |
| action | tool-call count |
| outcome | did the answer state `43.5` |
| support | answer correct **and** artifact retrieved |
| resource | `total_cost_usd`, `duration_ms`, turns, tokens |

## Analysis, fixed now

Two-sided Fisher exact on the primary endpoint, B versus A and C versus B.
Bootstrap 95% confidence intervals on the difference in proportions.

**Power.** n=25/arm was pre-specified from a previously observed 0.8 versus 0.4
contrast, which at that effect size gives about 80%. **If the realised effect is
smaller, this experiment is underpowered and will be described as such.** No
post-hoc claim of adequate power will be made from a positive result.

## Interpretation, fixed now

| pattern | reading |
| --- | --- |
| B > A and C unlike B | the actionable anchor changes retrieval |
| B approximately A | **primitive not demonstrated** |
| C approximately B | a label or structure effect, not truthful anchoring |
| C causes systematic misretrieval | the anchor is causal, but correct anchors are not thereby shown to help |

## Stop conditions

Stop and report if the harness fails on more than 10% of runs, if arm
assignment leaks, if the answer appears in any arm's prompt, or if runs
contaminate each other. Otherwise **all 75 run**, including after any
positive-looking interim result.

## Contamination controls

Each run is a fresh `claude -p` process with `--disable-slash-commands`, in its
own working directory, with no shared state. Arm order is interleaved rather
than blocked, so drift over time cannot align with an arm. Contamination is
checked per run, not per cell.
