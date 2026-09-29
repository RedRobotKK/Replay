# Preregistration: where does `reasoning_effort: "none"` apply?

Written 2026-09-29 **before** the run.

## Why

DS-OPT measured `reasoning_effort: "none"` as 36x fewer output tokens at an
unchanged 12/12. That was **one task class** -- looking up a planted record in
synthetic filler -- and a lookup does not need reasoning by construction. Reading
that result as "disable reasoning" would be the same error as the unconditional
prefix-ordering claim that DS-F3 had to retract.

## Design

Real Replay Go source as the shared corpus, roughly 20,000 tokens. Ground truth
is computed from the source by the harness, never authored by hand.

Two task classes, crossed with reasoning on and off, 3 repetitions each:

- **M, mechanical.** The answer appears verbatim at one location. Name the
  function declared on a given line; give a constant's literal value.
- **R, aggregative.** The answer exists nowhere in the text and must be built by
  applying a rule across the whole document. Count the lines beginning with
  `func `; count exported functions; name the function defined immediately after
  a named one.

## Prediction

| cell | predicted |
| --- | --- |
| M, reasoning on | high pass |
| M, reasoning off | **unchanged** from M-on |
| R, reasoning on | high pass |
| R, reasoning off | **materially lower** than R-on |

The claim under test is that the lever is **class-conditional**: free on lookup,
costly on aggregation.

## Falsifiers

- R-off scoring within noise of R-on kills the conditionality claim, and
  "disable reasoning" would then be a general recommendation for this model.
- M-off scoring below M-on means the lever is not free even on lookup, and the
  DS-OPT result was a property of that specific task rather than of lookups.
- Either class scoring near zero in **both** conditions means the questions are
  broken and the cell measures the instrument, not the model.

## Handling

Truncated answers are recorded undecided and excluded from the denominator. A
response that ran out of room did not answer wrongly; it did not answer. Pass
rates are reported with their n.
