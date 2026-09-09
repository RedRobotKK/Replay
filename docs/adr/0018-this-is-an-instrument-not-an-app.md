# 18. This is an instrument, and instruments carry their provenance in the type

**Status:** Accepted
**Date:** 2026-09-09

## Context

ADR-0014 catalogued checks that could not fail. This is its sibling for the
product: **figures that could not be wrong, because nothing on the screen said
what they were.**

On 2026-09-09 nine defects were fixed across five packages. Every one of them
was the same shape — *a value standing in for a different value it isn't* — and
none was a miscalculation. The arithmetic was right in every case.

- **A placeholder printed as a measurement.** `Fit` assigns
  `RelativeError = 1` when it measured no spread at all: no fittable turn, or a
  single one. A genuine byte-weighted spread can also land on 1.0. All three
  rendered as `±100%`. On this machine's corpus, 178 of 1735 lanes; one of them
  headlined `1.50M in prompts (±1.50M)`.
- **A map's zero value standing in for an observation.** The advisor appended
  `ob.targets[k].share` for every session, and Go returns the zero value for an
  absent key. A tool that simply went unused for two sessions scored
  `verified, realized 0.300` — **twice** the credit given to genuinely halving
  it. The strongest success the screen could report was a frontend day.
- **A ratio used outside the population it was fitted on.** `Fit` excludes
  prefix-changed turns because tool definitions "are denser than prose and would
  drag the fit". `advisor.go:303` then sized tool definitions with that ratio,
  and hand-built a `Figure` to bolt the prose spread onto it.
- **A drop serving as both the evidence and the measurement.** `track()` read a
  fall in a target's share as proof the advice had been applied, then measured
  that same fall to decide whether the prediction held. Corpus drift from 30% to
  22% reported "verified, 0.080 realized" with nobody having changed anything.
- **An assertion whose oracle came from the thing it checked.** The storyboard
  alignment test compared each row to `Header()`, and `Header()` is built from
  `trafficCols` by the same `Row()` that builds the rows. Both sides move
  together and can never disagree. Separately, its filter admitted 0 of 11 rows.
- **A claim the project's own evidence had refuted.** `replay cost` told
  subscribers that re-billed tokens were "rate-limit budget spent on nothing".
  The titration in `README.md:228-235` measured exactly that across 3.09M tokens
  and the utilisation counter moved zero steps, published as a null.
- **A cursor that was one cell here and two in Tokyo.** `U+25B8`, East Asian
  width class Ambiguous, chosen by a comment that said it was two cells "so the
  columns after it cannot shift".
- **A test suite writing to the reader's home directory.** No `TestMain`, so
  `TestS1` ran `advise` against a fixture and rewrote the operator's real
  `~/.replay/advice.json`; later tests read it back. Both passed alone.
- **One command advertised at three addresses.** All three returned 200 and the
  same 25,481 bytes, so nothing was broken and nobody had cause to notice.

The common property is that each figure was *arithmetically* fine and
*epistemically* silent. Nothing on the wire said whether a number had been
measured, estimated, borrowed from a different population, or never taken at
all — so every consumer downstream, including the renderer, treated them alike.

## Decision

**Replay is a measuring instrument. Its design rules are metrology's, not an
application's.** Five follow.

**1. Provenance is a field, not a comment.** A number travels with how it was
obtained. `Figure.ErrorMeasured`, `sample.seen`, `Tokens{Measured, Estimated}`,
`EstimateOutsideFit` as a distinct name for identical arithmetic. Prose rots and
is not consulted; a field cannot be dropped by a caller who did not read it.
`Turns` had been on `TokenFit` all along, and nothing asked it.

**2. Absence, zero and unknown are three values.** Collapsing any two is how a
screen misleads without anyone lying. This is already the rule for external
data — a null from PostHog means NOT MEASURED, never zero installs — and it is
now the rule inside the process. A mean takes only the readings that exist, and
reports how many there were, because "measured at nothing" and "not measured"
are exactly the pair being kept apart.

**3. An oracle may not derive from the thing it checks.** Where a test's
expectation is computed from the same source as the value, it agrees by
construction. The fix is a literal somebody has to change deliberately: the
traffic table's column starts are written out as `2, 12, 23, 48, 66`.

**4. Saying "not measured" must be cheap on every surface.** The project's
public credibility rests on retracting its own figures — 98.8% corrected to
4.2%, a sample restated from 1363 to 78, a quota result published as a null.
That posture only holds if a screen can decline to answer without looking
broken. Where declining was expensive, the code asserted instead.

**5. The limit is part of the deliverable.** Fixing how a figure is presented
does not fix the figure. `EstimateOutsideFit` removes a borrowed error bar and
explicitly does not claim the estimate is more accurate. The install-URL fix
records that the hosted copy readers actually run is still stale. A change that
quietly implies it closed more than it did is the same defect at the level of
the commit message.

## Consequences

- **The type system carries the epistemics, so it gets wider.** `Figure` grew a
  field; `[]float64` became `[]sample`. That cost is the point: a renderer can
  no longer print a bar without having been handed whether it means anything.
- **Two-part fixes need a test that crosses the join.** Reverting the
  aggregation half of the absence fix left MN1–MN4 green, because all four
  called `track()` directly. A field nothing reads is the built-but-unwired
  shape — and it would have been introduced by the commit fixing an instance of
  it. Every fix here spans a boundary, so every guard must too.
- **Locale is correctness, not presentation.** The ASCII rule for TUI screens is
  a width invariant. `AD11` states it in the package where the character is
  chosen, because catching it at render time caught it four commits late.
- **This ADR does not make any figure more accurate.** The fit's 49% median
  relative error is untouched and unmeasurable here: a Claude tokenizer is not
  public and `count_tokens` needs a key. What changed is that the screens stop
  claiming precision they never had. Naming that gap is the ADR working, not a
  hole in it.

---

[ADR index](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
