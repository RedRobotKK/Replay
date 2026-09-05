# 0002. Replay engine with calibration gate and two tiers of truth

**Status:** Proposed
**Date:** 2026-09-02

## Context

Replay's headline feature reproduces the provider's caching behavior for recorded sessions and scores alternative context layouts. Two facts constrain it. Client transcripts do not contain the system prompt, tool definitions, or cache markers, which is the part of the prompt that caching keys on first. And any replayed saving assumes the model would have behaved identically under a different layout, which is false in general. Presenting such figures as exact would be debunked the first time someone checked.

## Decision

Every figure Replay prints carries a truth tier. *Estimated* means derived from transcripts alone, with the unseen prefix modeled as an opaque block and per-block attribution carrying an uncertainty range. *Measured* means captured on the wire by the proxy. The two never share a table without the label. Before scoring any alternative, the replay engine must reproduce the provider's reported cache reads and writes for the session; below a 95 percent per-turn match it prints the calibration report and refuses to score. Every replay table states that savings assume unchanged agent behavior. Rules and price tables are versioned and dated in the output.

## Consequences

- Offline replay is honest as a diagnostic with estimates. The proxy's value proposition becomes upgrading estimates to measurements.
- A provider rule change shows up as a calibration drop and stops the tool from being confidently wrong.
- Output is slightly busier. That is the price of being trusted.

## Alternatives considered

**Present one number.** Rejected: the first public debunking would end the project's credibility.

**Require the proxy for any output.** Rejected: transcripts-only replay works for every user, costs nothing, and is the zero-trust first contact the adoption plan depends on.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
