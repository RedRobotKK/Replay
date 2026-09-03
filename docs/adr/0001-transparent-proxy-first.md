# 0001. Ship a byte-transparent proxy before any context transformation

**Status:** Proposed
**Date:** 2026-09-02

## Context

The PRD v4.0.0 describes a proxy that pads context into fixed-length slots, prunes tool output with a Rust AST sidecar, and rewrites history before forwarding. The adversarial review found that provider prompt caching is an exact-byte prefix match (padding has no effect), that provider history-binding checks reject edited earlier turns on current models, and that pruning tool output breaks agents that edit by exact string match. Any transformation that is not a deterministic function of its input therefore risks a hard provider error, not merely a cache miss.

At the same time, three problems are real and unaddressed by existing tools: developers cannot see what an agent task costs, cache breaks are silent, and secrets leave the machine inside prompts.

## Decision

The first release forwards requests byte-for-byte and adds only observation: usage capture, cost and cache dashboards, and a cache-break detector. Transformations (masking, spend gating, pruning) arrive in later releases, each opt-in, each a deterministic function of its input, and each gated on a benchmark showing no regression in agent task success.

## Consequences

- v0.1 is small and safe to install; it cannot break an agent because it changes nothing on the wire.
- Every later feature inherits a test harness and a baseline from v0.1 against which regressions are measurable.
- The padded-slot mechanism and the Rust sidecar are dropped. Tree-sitter, if ever needed, is used through cgo in the single Go binary.
- We give up early token savings in exchange for not shipping a tool that corrupts sessions.

## Alternatives considered

**Build the PRD as written.** Rejected: the padding mechanism is ineffective by construction, and history rewriting is incompatible with current provider checks.

**Start with masking, since it is the enterprise ask.** Rejected for ordering, not for merit: masking needs the deterministic rendering and history-binding test harness that the transparent proxy establishes first. It is v0.2.

**Translate OpenAI-shaped requests to Anthropic-shaped ones so one endpoint serves every client.** Rejected: re-rendering the prompt destroys byte stability, which is the one property the whole product depends on.
