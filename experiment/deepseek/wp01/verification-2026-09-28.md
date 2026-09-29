# WP-01 verification: independent check of DeepSeek's findings

DeepSeek investigated the Replay repository read-only through a bounded tool
loop (`list_files`, `read_file`, `grep`) and produced five findings. Its report
is preserved verbatim alongside this file. **This document is Claude's
independent check; DeepSeek did not adjudicate its own findings.**

Repo pinned at `a9f48a0`, clean tree. No file was modified by the investigation.

## Verdicts

| # | finding | verdict |
|---|---|---|
| F1 | Substring model matching prices ids that were never in the table | **VERIFIED**, and understated |
| F2 | TTL expiry inferred from a transcript timestamp gap | **VERIFIED** |
| F3 | `DefaultTokensPerByte = 0.25` fallback silently substitutes for a fit | **VERIFIED** |
| F4 | `WithTTL` labels a simulated replay "measured, not estimated" | **VERIFIED** — sharpest |
| F5 | `PriceTableCheckedAt` is a compiled constant | **PARTIALLY VERIFIED** |

### F1 — VERIFIED, and worse than reported

`PriceFor` was run directly against the pinned tree:

```text
claude-opus-5            priced=true  input=5  output=25
claude-opus-5-preview    priced=true  input=5  output=25
claude-opus-5-risk       priced=true  input=5  output=25
openai-claude-opus-5     priced=true  input=5  output=25
deepseek-claude-opus-5   priced=true  input=5  output=25
totally-unknown-model    priced=false input=0  output=0
```

DeepSeek reported the preview case. The cross-provider case is **confirmed
empirically**: an id carrying another vendor's prefix plus an Anthropic family
name prices at Anthropic rates. This matters directly for the DeepSeek campaign,
where a model id is provider-chosen.

One citation slip: DeepSeek wrote `PriceFor` returns `Priced: true`. The actual
signature is `(Price, bool)`. The behaviour is as described; the field name is not.

**Not a discovery.** `internal/cachemodel/match.go:27` already says
"`opus-5-preview` still prices as Opus 5, another provider's id carrying an
Anthropic family name still matches". DeepSeek surfaced a documented limitation
rather than finding an unknown one.

### F2 — VERIFIED

`internal/analysis/diff.go:71` emits `"gap %s exceeds TTL %s"` as the Detail for
`CauseTTLExpired`, where the gap is a transcript-derived wall-clock interval. A
long gap between recorded turns is not the same fact as the provider expiring a
cache entry. The cause string reads as an established mechanism.

### F3 — VERIFIED

`internal/analysis/fit.go:106` defines `DefaultTokensPerByte = 0.25`, applied at
`:234` when a session offers no fittable turn. Self-documented at `:337`. A
figure derived through a borrowed constant is an estimate; the concern is
whether that provenance survives to the reader.

### F4 — VERIFIED, and the sharpest finding

`internal/analysis/replay.go:158-161`:

> "WithTTL replays the lane with a different cache TTL... **It is measured, not
> estimated**, because no byte-to-token conversion is involved."

The function body calls `cachemodel.SimulatedUsage(...)`. The justification is
narrow and true — no byte-to-token conversion occurs — but it does not license
the word *measured* for a counterfactual replay under a TTL that was never
applied. The prompt sizes are observed; the outcome under a different policy is
simulated. This is precisely the OBSERVED/COUNTERFACTUAL boundary the project
holds elsewhere.

### F5 — PARTIALLY VERIFIED

`PriceTableCheckedAt = "2026-09-07"` is a compiled constant
(`internal/cachemodel/anthropic.go:54`) exported through `export.go:53`, so it
can go stale between releases — as stated.

But the concern is already substantially mitigated: `PriceTableAgeNote` exists
specifically to warn when the compiled table is old, with a comment explaining
that citing a date without its age "quietly gives a stale number". DeepSeek did
not find this mitigation, so the finding over-states the exposure.

## What DeepSeek did well

- **Read code, not filenames.** It traced `matchesModel` → `continuesWithVersion`
  → `lookup` → `PriceFor`, and `ClassifyBreak` → `FindBreaks` → the Detail string.
- **Found the tests.** It located `matchedges_test.go`, `unknownversion_test.go`
  and `checked_test.go` unprompted and distinguished "pins current behaviour"
  from "makes the defect impossible".
- **Held the evidence labels.** OBSERVED / INFERENCE / MISSING EVIDENCE were used
  consistently and mostly correctly; it did not present inference as defect.
- **Proposed falsifiable checks.** "Add a test requiring `PriceFor(
  "claude-opus-5-preview")` to return false" is directly executable, and is what
  produced the empirical confirmation above.

## What DeepSeek did poorly

- **It did not stop.** Given an explicit instruction to stop at five findings, it
  consumed all 28 permitted rounds still reading files and never produced a
  report. The deliverable required a separate forced-synthesis call.
- **Reasoning starved the output.** The forced call at `max_tokens=6000` returned
  26,845 characters of `reasoning_content` and **zero** content,
  `finish_reason: length`. The analysis existed and could not be read. This is
  the third occurrence of the same failure mode in this campaign.
- **One citation was wrong** (`Priced: true` for a `(Price, bool)` return).
- **It missed an existing mitigation** it had the tools to find (F5's
  `PriceTableAgeNote`).
- **Two of five findings restate code comments.** F1 and F3 are documented in the
  source it read. Useful surfacing, not discovery.

## Engineering changes made

**NONE.** No Replay source was modified. The only files added are this
verification and DeepSeek's report.
