# C005 prediction, frozen 2026-09-30 BEFORE any output was examined

## Claim under attack
"Anything this tool cannot measure, it declines to print, and says why, in the
place the number would have gone." README.md:21, the boxed callout. The
repository calls the refusals the feature.

## Boundary stated before measuring
- Scope: user-visible output of `replay cost`, human and JSON.
- The established part is INTERNAL: surface.Classify reaches ClassUndetermined
  with a reason. The recorded gap is that the printed surface after a refusal
  has never been inspected. This attack is that gap and nothing wider.

## Deciding layer
For the internal verdict: surface.Class, surface.WriteContract.
For the printed surface: NONE. No type governs what reaches the screen, which
is the same structural absence C004 found.

## Observable inputs
A ledger corpus whose requests name a model the compiled price table does not
know. Usage is present and provider-reported; only the PRICE is unavailable.
This isolates one unmeasurable quantity rather than breaking the reader.

## External assumptions
- ASSUMPTION: a model id absent from the price table is genuinely unpriceable
  rather than falling through to a default row. If a substring match catches
  it, this fixture measures something else and the test must say so.
- ASSUMPTION: refusal is conveyed by an unpriced or unreadable disclosure. A
  refusal expressed some other way would be understated, which is the error
  C004 made.

## FROZEN PREDICTION
The report will NOT print a bare $0.00 for the unpriced requests. It will
disclose them, because `unpriced` already exists as a JSON key.

The sharper prediction, and the one I expect to fail: THE TOTAL. An unpriced
request contributes nothing to the sum, so a total computed over a corpus
that is partly unpriced is understated. I predict the total is printed as a
total, with the unpriced count disclosed elsewhere rather than attached to the
figure it changes.

If so the claim's wording is false at the words "in the place the number would
have gone": the disclosure exists but is not where the affected number is.

## Pre-committed consequence
- If the total is unqualified where it is printed: C005 narrows. Result moves
  to REFUTED if the disclosure is absent entirely, or stays BOUNDED with the
  placement recorded if it is present but detached.
- If the total carries its own qualification: C005 strengthens within its
  measured scope and a new bounded claim records it.
- Either way the test is not weakened to protect the claim.

---

# Result, appended after measuring

**C005 REFUTED. A defect was found and is NOT patched here.**

## What was measured

One transcript, four requests, identical usage. Two on `claude-opus-5`, two on
`totally-unknown-model-x9`. The assumption was checked first:
`cachemodel.PriceFor` returns priced for the former and unpriced for the
latter, so exactly one quantity is unavailable and nothing is malformed.

| Surface | Reported |
|---|---|
| `totalRequests` | 4 |
| `unpriced` | **0** |
| Human total | **$0.06**, the two priceable requests only |
| Human disclosure | **none** |

The report then states: *"That is what your agent work actually cost."*

## The defect, precisely

**`unpriced` has transcript granularity. Pricing has record granularity.**

`renderCost`'s own doc comment says it: "unpriced is transcripts that were READ
and priced to nothing". The disclosure at `cost.go:488` fires only when a
WHOLE transcript is unpriced. A transcript holding a mix is counted as priced,
and its unpriceable records are excluded from the total in silence.

The counter built to prevent exactly this reports zero in exactly this case.

## Why the prediction is preserved

The frozen prediction above expected the total to be understated with the
disclosure present but detached. **It was worse: the disclosure is absent and
the counter reads zero.** The prediction was directionally right and
specifically wrong, and both halves are recorded.

## Not patched, and why

Changing what a money figure reports is a production decision. The smallest
correct fix is to count unpriceable RECORDS rather than unpriceable
transcripts, and to attach the disclosure to the total it changes rather than
to the end of the report. That is one counter and one sentence, and it is the
owner's call, not a test campaign's.

The behaviour is pinned by `TestC005_AMixedTranscriptHidesItsUnpricedRecords`
and `TestC005_TheHumanReportAssertsCompletenessOverAPartlyPricedCorpus`, both
of which fail the moment it is fixed and say so in their failure text.
