# Final Report: Five Architectural Assumptions in Replay Go

Findings are ranked by technical risk. Every statement below is labeled
OBSERVED / INFERENCE / HYPOTHESIS / MISSING EVIDENCE, and no additional
repository exploration was performed.

---

## Finding 1

**Finding:** Model pricing can report a known-unpriced model as if it were
priced at the nearest Anthropic family row.

**Why this is an architectural assumption:** The pricing path trusts
substring/model-family matching as a proxy for a real row in the official price
table. That proxy is known to accept ids for which no row was ever validated.

**Exact file:** `internal/cachemodel/match.go`; call site
`internal/cachemodel/anthropic.go` via `lookup`.

**Exact symbol/function/test:** `matchesModel`, `continuesWithVersion`,
`lookup`, `PriceFor`; pinned behavior in
`internal/cachemodel/matchedges_test.go`:
`TestME1_TheVersionGuardsEdges`.

**What the code actually does:**
- OBSERVED: `matchesModel` first does substring matching.
- OBSERVED: `continuesWithVersion` rejects some trailing digit/version
  collisions, but explicitly allows trailing bare hyphen suffixes, preview
  spellings, and non-hyphen word separators.
- OBSERVED: The file comment says this was deliberate to keep `claude-opus-5`
  matching `claude-opus-5-risk`.
- OBSERVED: The comment also says the approach is not correct and that
  `opus-5-preview` still prices as Opus 5.
- INFERENCE: `PriceFor` can therefore return `Priced: true` and non-zero
  unit prices for an id such as `claude-opus-5-preview`, even though no
  row explicitly represents that model.
- INFERENCE: A dollar figure derived from that price can be rendered as a
  documented cache/cost estimate.

**What stronger claim could accidentally be inferred:**
- INFERENCE: A reader of `priced`, `Priced`, or a resulting dollar estimate
  may infer the row was validated against an independent price table for that
  exact model id.
- INFERENCE: Cross-provider ids containing an Anthropic family name can also be
  matched as if they were Anthropic models.

**Why the evidence is insufficient for that stronger claim:**
- OBSERVED: The in-code comment explicitly disclaims correctness of matching,
  so `Priced: true` is not evidence that the exact id exists in the official
  table.
- OBSERVED: The version guard is deliberately narrow and documented as leaving
  known cases wrong.
- MISSING EVIDENCE: No observed code checks whether the matched family/version
  row actually applies to the requested model id.

**Existing test coverage:**
- OBSERVED: Existing edge test pins the current intended matching behavior for
  preview-like and hyphen-like suffixes.
- OBSERVED: Existing tests establish that a possibly-colliding version number
  does not silently price as the preceding version.
- INFERENCE: Existing tests do not make the known preview/cross-provider defect
  impossible; they record the current behavior.
- MISSING EVIDENCE: No observed test asserts that `claude-opus-5-preview` is
  unpriced, nor that an id with another provider prefix plus an Anthropic
  family name is rejected.

**What additional test or evidence would falsify/confirm the concern:**
- Add a test requiring `PriceFor("claude-opus-5-preview")` to return
  `Priced: false`, or requiring the UI/status layer to emit an unresolved-model
  warning even when the family substring matches.
- Add a round-trip check against a real official model list rather than relying
  on substring membership.

---

## Finding 2

**Finding:** A transcript-derived timestamp gap is treated as evidence that a
cache break was caused by TTL expiration.

**Why this is an architectural assumption:** The analysis assumes the
`Turn.Gap` recovered from a transcript is the inter-request start-to-start
interval, when the parser may be measuring completion-to-completion time.

**Exact file:** `internal/analysis/calibrate.go` documents the limitation;
`internal/analysis/diff.go` consumes it;
`internal/cachemodel/anthropic.go` defines the classification.

**Exact symbol/function/test:** `Turn.Gap`, `FindBreaks`, `ClassifyBreak`,
`CauseTTLExpired`. The limitation is referenced near the code comment invoking
`TestTranscriptRequestsCarryNoDuration`.

**What the code actually does:**
- OBSERVED: `Turn.Gap` is computed from consecutive transcript timestamps.
- OBSERVED: For Claude Code transcripts, the documentation says the parser
  places the assistant response on the request timestamp, so the gap is
  completion-to-completion rather than start-to-start.
- OBSERVED: `FindBreaks` calls `ClassifyBreak`, which can emit
  `CauseTTLExpired` when `gap > TTL`.
- INFERENCE: A user-facing diff or report can read this as “the cache expired
  because the request was later than the cache TTL,” not merely “the recovered
  gap exceeded the TTL.”

**What stronger claim could accidentally be inferred:**
- INFERENCE: That the provider cache entry actually exceeded its TTL because of
  elapsed wall-clock time between user requests.
- INFERENCE: That elapsed time between request starts was measured.
- INFERENCE: That the break has a definitively identified cause, suitable for
  one-line remedy advice.

**Why the evidence is insufficient for that stronger claim:**
- OBSERVED: The source notes `Gap` is completion-to-completion and that no
  duration is recoverable from the transcript.
- OBSERVED: Completion-to-completion gap includes differences in model response
  duration and therefore cannot be equated with start-to-start cache age.
- INFERENCE: A long prior response can create a `CauseTTLExpired`
  classification even if the actual cache age was shorter than the TTL.

**Existing test coverage:**
- OBSERVED: The transcript limitation is referenced by a test name,
  `TestTranscriptRequestsCarryNoDuration`.
- OBSERVED: That test or nearby tests pin only the absence of recoverable
  duration.
- MISSING EVIDENCE: No observed test places a completion gap near the TTL
  boundary and verifies that the reported cause is hedged or unknown rather
  than definitive.
- MISSING EVIDENCE: The exact test file was not inspected in this analysis.

**What additional test or evidence would falsify/confirm the concern:**
- Construct a transcript where start-to-start age is below the TTL but
  completion-to-completion gap is above it, and assert that the system does not
  report “cache expired.”
- Or mark the result as an inference rather than `CauseTTLExpired` when only
  transcript timestamps are available.

---

## Finding 3

**Finding:** The fallback token ratio `DefaultTokensPerByte = 0.25` is an
asserted constant, not a measurement, yet it becomes the basis of estimated
token/block figures.

**Why this is an architectural assumption:** Once a replay lane has no fit,
the repository substitutes a single global ratio for all byte counts. This
implicitly assumes the ratio is a reasonable default for arbitrary input
traffic.

**Exact file:** `internal/analysis/fit.go`.

**Exact symbol/function/test:** `DefaultTokensPerByte`, `Fit`,
`EstimateTokens`, `RelativeError`.

**What the code actually does:**
- OBSERVED: `DefaultTokensPerByte` is assigned `0.25`.
- OBSERVED: When a usable regression fit is unavailable, the code uses this
  constant to estimate tokens from bytes.
- OBSERVED: It sets `RelativeError` to `1`, or marks the estimate as not
  directly measured, for that fallback path.
- OBSERVED: Source comments say the constant is known to be biased low and that
  observed ratios in tests range from approximately `0.439` to `1.261`.
- INFERENCE: Sessions that cannot be fit can produce block-size and token
  estimates based on a value below nearly all observed ratios.

**What stronger claim could accidentally be inferred:**
- INFERENCE: A reader may infer the token estimate is grounded in provider
  measurements for the analyzed material.
- INFERENCE: For unfittable sessions, a concrete block division or per-block
  share is the actual measured block layout.
- INFERENCE: Error reporting is specific enough to distinguish materially
  reliable estimates from default fallback in every output path.

**Why the evidence is insufficient for that stronger claim:**
- OBSERVED: The code comments admit the constant is not fitted to current data.
- OBSERVED: Error flags are coarse (`RelativeError = 1`), which is not the same
  as a validated confidence interval for the fallback.
- MISSING EVIDENCE: No observed mechanism forces every downstream view to
  display that the estimate is based on the fallback rather than on measured
  provider usage.

**Existing test coverage:**
- OBSERVED: Tests exist around fit error and fit notes, including behavior when
  no fit is available.
- OBSERVED: These tests expect a fallback estimate, not absence of an estimate.
- INFERENCE: This means the system is designed to produce a number for
  unfittable lanes rather than declining to estimate.
- MISSING EVIDENCE: No observed test asserts that every rendered number is
  accompanied by a prominent bias-low explanation.

**What additional test or evidence would falsify/confirm the concern:**
- Add a test that scans reported context figures and fails if an estimate
  produced by the fallback is rendered without an explicit “fallback ratio”
  label.
- Replace or validate `DefaultTokensPerByte` with a dataset-driven default per
  modality or source rather than a single hard-coded assertion.

---

## Finding 4

**Finding:** `WithTTL` presents alternative-policy results as measured rather
than estimated, even though the underlying counterfactual is a simulation.

**Why this is an architectural assumption:** The architecture treats the
cache simulation as observational for a policy change while the provider’s
behavior is being inferred from a small set of recorded lane observations.

**Exact file:** `internal/analysis/replay.go`.

**Exact symbol/function/test:** `WithTTL`, `newCacheState`,
`observedAvailability`.

**What the code actually does:**
- OBSERVED: `WithTTL` simulates an alternative TTL or context policy.
- OBSERVED: The documentation distinguishes some results as estimated when
  byte-to-token conversion is involved, but states that `WithTTL` returns
  “measured, not estimated” because it does not perform byte-to-token
  conversion.
- OBSERVED: The simulation reconstructs a cache state from the first observed
  `CacheRead` and then applies assumptions about minimum prefix size, TTL
  expiry, and whether changed context would have altered behavior.
- OBSERVED: Observed unavailable reads are used as constraints where they can
  be classified.
- INFERENCE: The result is still a counterfactual model, not a
  contemporaneously observed provider measurement; there is no changed-TTL
  deployment against the same traffic.

**What stronger claim could accidentally be inferred:**
- INFERENCE: The alternative-policy dollar or token figure is empirically
  measured from a run under that policy.
- INFERENCE: The provider cache behaves outside the observed policy exactly as
  the simulator models it.
- INFERENCE: Because `Estimated: false` appears in some path, the result is on
  the same evidentiary level as an A/B measurement.

**Why the evidence is insufficient for that stronger claim:**
- OBSERVED: The code relies on an inferred cache state and explicit policy
  assumptions.
- OBSERVED: The first request’s observed `CacheRead` is used to seed a
  simulated state, which is a modeling decision.
- MISSING EVIDENCE: No observed evidence shows the simulator’s out-of-policy
  behavior has been validated against known ground-truth cache behavior for all
  models and edge cases.

**Existing test coverage:**
- OBSERVED: Tests likely exercise the simulation path and consistency with
  observed lane behavior.
- MISSING EVIDENCE: This analysis did not inspect a separate `replay_test.go`
  sufficiently to enumerate it.
- INFERENCE: Existing tests likely validate arithmetic and status behavior, not
  external validity of the modeled provider cache algorithm.

**What additional test or evidence would falsify/confirm the concern:**
- Split traffic or use provider ground truth under a second TTL and compare
  simulator output against observed cache reads/writes.
- Change the status layer to label results as “simulated counterfactual”
  rather than “measured, not estimated,” or expose a distinct `Simulated`
  field.

---

## Finding 5

**Finding:** A compiled `PriceTableCheckedAt` constant is presented as evidence
that the table was independently checked, without a traceable verification
record.

**Why this is an architectural assumption:** The code assumes that the
presence of a bumped timestamp in source is sufficient evidence that an
external price-table verification occurred and covered the active table.

**Exact file:** `internal/cachemodel/anthropic.go`.

**Exact symbol/function/test:** `PriceTableCheckedAt`,
`PriceTableAgeNoteAt`, `PriceTableAgeNote`.

**What the code actually does:**
- OBSERVED: `PriceTableCheckedAt` is a constant in source.
- OBSERVED: It is used to compute an age note reporting that the table was
  checked against an independent database and showed no disagreement.
- OBSERVED: Comments instruct maintainers to move the timestamp only after
  actually running a check.
- INFERENCE: A user or automation receiving that note may treat the external
  check as an artifact or logged result of the current code state, not as a
  human-maintained assertion.
- INFERENCE: The check can become stale or be moved inconsistently despite the
  instruction.

**What stronger claim could accidentally be inferred:**
- INFERENCE: The repository executed an independent price check and stored
  evidence from that run.
- INFERENCE: The check covers the exact table current at invocation and was
  performed at the time of the build or command.
- INFERENCE: “No disagreement” means a current dataset was compared, not a
  remembered or historical dataset.

**Why the evidence is insufficient for that stronger claim:**
- OBSERVED: The only evidence observed is a source constant and an instruction
  in a comment.
- MISSING EVIDENCE: No observed digest, schema, fixture, log artifact, or test
  proves that the stated check ran against the active table.
- INFERENCE: The note’s truth depends on maintainer discipline rather than on
  executable verification.

**Existing test coverage:**
- OBSERVED: Tests exercise the note’s text and staleness suppression logic.
- MISSING EVIDENCE: No observed test verifies that the note corresponds to an
  actual independent comparison artifact.

**What additional test or evidence would falsify/confirm the concern:**
- Add a golden test with an independently generated digest or fixture from the
  check run, and make `PriceTableCheckedAt` consume that artifact rather than a
  manually maintained constant.
- Add CI validation that refuses a changed table digest unless the check
  artifact is updated in the same commit.