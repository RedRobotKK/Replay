# SP-01: the ceiling gate's PASS verdict did not state what its figure excludes

**2026-10-02. Convergence campaign, Campaign 2 (claim conservation). REPAIRED.**

## Hypothesis

A downstream representation must never be stronger than the canonical claim it
renders. `costSummary` is the canonical claim. The ceiling gate renders part of
it. H0: every public exit already carries the summary's own statements of what
its dollars do not cover.

## Setup

The canonical summary carries four independent holes, which the repository had
already established as separate claims:

| hole | granularity | claim |
|---|---|---|
| `unpriced` | whole transcripts | pre-existing |
| `unreadable` | whole transcripts | pre-existing |
| `UnpricedRequests` | records inside priced transcripts | RPL-C035 |
| `UnpricedRebilledTokens` | break tokens outside the re-billed dollars | RPL-C037 |

Audit of every exit that prints a dollar figure, verified by reading source:

| exit | carries coverage? |
|---|---|
| `renderCost` human report | **yes**, all four |
| `--json` summary | **yes**, the counters serialize |
| `checkRebilledCeiling` | **no** on PASS; two of four on FAIL |
| `cardData` / `internal/card` | **no**, and `card.Data` has no field for it |
| `contributeCorpus` payload | whole-transcript `unpriced` only |

**H0 rejected.** `renderCost` was the only exit reading the C035/C037 counters.

## The asymmetry, which is the finding

`checkRebilledCeiling` refuses correctly when nothing priced (`Tasks == 0` emits
NOT MEASURED and returns an error). Its FAIL branch names excluded and unreadable
transcripts, "so the real figure is higher". Its **PASS branch printed one line
and nothing else.**

A pass is the verdict excluded spend could overturn. Silence was strongest
exactly where it was least affordable, and it was a green build.

## RED, before the repair

`cmd/replay/claim_surface_parity_test.go`, three tests. The FAIL branch is the
positive control: a detector that cannot see disclosure there is broken rather
than the gate.

```
the PASS verdict does not name excluded transcripts ... the FAIL verdict does,
  over the same summary
the PASS verdict does not name unreadable transcripts
pass verdict: 1 of 4 requests priced nothing and the gate says nothing about them
fail verdict: 20000 of 40000 re-billed tokens are outside the dollar figure
  this ceiling compared, and the gate says nothing about them
```

The negative control passed from the start: a fully covered summary disclosed
nothing.

## Repair

One shared helper, `rebilledFigureCaveats(s, unpriced, unreadable) []string`,
called from **both** verdicts. No new fields: `costSummary` already carried every
input. The FAIL branch's two inline disclosures were replaced by the helper, so
the two branches can no longer drift apart.

Four holes kept apart rather than summed, deliberately: they have different
causes and different remedies, and a single coverage number would say which of
them nobody could act on.

Rendered, PASS:

```
  GATE: re-billed spend $1.00 is within the $100.00 ceiling.
  The figure it was compared against is not complete:
    2 transcript(s) were excluded as unpriced, so the real figure is higher.
    1 transcript(s) could not be read at all, so the real figure is higher still.
    1 of 4 requests read priced nothing, so this figure covers part of the work.
    20000 of 40000 re-billed tokens are outside this figure, on a model no price
      table carries.
```

## Mutations, against the landed implementation

| | verdict |
|---|---|
| C0 reorder two caveats (control) | **SURVIVED**, as required |
| M1 PASS verdict silent again, the pre-repair state | **KILLED** |
| M2 drop the C035 request-coverage caveat | **KILLED** |
| M3 drop the C037 break-token caveat | **KILLED** |
| M4 caveat fires with nothing to qualify | **KILLED** |

## Result

- Gate: **ESTABLISHED.** Both verdicts now carry all four holes, mutation-proven.
- **Still open, and NOT repaired here:** the card and the corpus payload. Both
  need a structural change, `card.Data` has no field for coverage and
  `corpusFigures` carries only the whole-transcript count. Widening either is a
  published-shape decision, not a rendering one, so it is recorded rather than
  taken.

## Limitations

Tests the renderer given a canonical summary, which is the right boundary for a
conservation test but says nothing about whether the summary itself is correct;
that is C035/C036/C037's territory. The gate's pre-existing "across 2 session"
pluralisation was left alone as out of scope.

## Next

The card and the corpus payload, as one decision about whether a published
artifact carries its own coverage.
