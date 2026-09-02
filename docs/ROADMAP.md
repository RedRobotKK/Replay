# Roadmap

What ships in each release, the acceptance gate for each, and what is deliberately deferred. This is the execution plan; the vision lives in `docs/prd/`.

The scope below follows the recommendations in [`reviews/PRD-v4-adversarial-review.md`](reviews/PRD-v4-adversarial-review.md). Items marked **proposed** are awaiting maintainer sign-off and may change.

## v0.1: Transparent proxy with visibility (proposed)

**Goal:** install in five minutes, change nothing about how the agent behaves, and show what a task cost.

- Single Go binary. TCP loopback listener with header-token authentication.
- Byte-transparent passthrough for `/v1/messages*` (Anthropic) and `/v1/chat/completions` (OpenAI-compatible), including SSE streaming.
- Per-request capture of provider `usage` fields. Local dashboard: cost, tokens, cache-read ratio per session.
- Cache-break detector: diff adjacent request prefixes, report the first divergence and a likely cause.
- One environment variable disables everything. Uninstall leaves nothing behind.

**Acceptance:** zero behavior change on a fixed suite of Claude Code and Aider tasks; added latency p99 under a published budget; cache-read ratio matches direct-to-provider within measurement noise.

## v0.2: Secret masking and spend control (proposed)

- Deterministic placeholders keyed by HMAC of the secret; encrypted vault persisted on disk; key in the OS keychain.
- Rehydration across SSE chunk boundaries and inside tool-call JSON. Thinking blocks pass through untouched.
- Dollar-denominated circuit breaker, fail-closed before the next request, per session and per day, with a visible override.

**Acceptance:** provider history-binding check passes in error mode across a 50-turn masked session; a daemon restart mid-session loses nothing; documented false-positive rate on a test corpus.

## v0.3: Opt-in context tools (proposed)

- MCP server exposing session history and summaries as resources.
- Tool-output pruning as an opt-in per-glob feature, never applied to the last-read or last-edited file, gated on an A/B benchmark of task success, edit failures, cost, and latency.

## Deferred until a user asks

Vector store, agent-to-agent mesh bus, Rust sidecar. Padded cache slots are rejected outright; see ADR-0001 and the review.

## Not goals

- Translating between provider API shapes.
- Replacing the provider's own compaction or context editing.
- Running anywhere but the developer's machine.
